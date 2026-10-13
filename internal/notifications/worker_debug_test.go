package notifications_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"git.hyhy.fun/rsplab/iolink/internal/notifications"
	"git.hyhy.fun/rsplab/iolink/internal/observability"
	"git.hyhy.fun/rsplab/iolink/internal/wechat"
)

type traceDeliveryStore struct {
	result    notifications.Result
	requestID string
	finishErr error
	claimID   string
	claimErr  error
}

func (s *traceDeliveryStore) Claim(ctx context.Context, _ time.Time, _ time.Duration, _ int) (notifications.Claim, bool, error) {
	s.claimID = observability.RequestID(ctx)
	if s.claimErr != nil {
		return notifications.Claim{}, false, s.claimErr
	}
	return notifications.Claim{ID: 1, Attempts: 1, LeaseToken: "private-lease", Delivery: notifications.Delivery{RecipientID: "private-recipient", DeviceNo: "private-device", Message: "private-message"}}, true, nil
}

func (s *traceDeliveryStore) Finish(ctx context.Context, _ notifications.Claim, result notifications.Result, _ time.Time) error {
	s.result, s.requestID = result, observability.RequestID(ctx)
	return s.finishErr
}

func TestWorkerCorrelatesAttemptAndDownstreamWithoutSensitiveData(t *testing.T) {
	for _, tc := range []struct {
		name       string
		status     int
		finishErr  error
		wantResult string
		wantFinal  string
	}{
		{name: "sent", status: 200, wantResult: "sent", wantFinal: "sent"},
		{name: "provider rejected", status: 403, wantResult: "failed", wantFinal: "failed"},
		{name: "persistence failed", status: 200, finishErr: errors.New("private-store-secret"), wantResult: "sent", wantFinal: "failed"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Given
			var logs bytes.Buffer
			logger, err := observability.Console(observability.Config{Level: "debug"}, &logs)
			if err != nil {
				t.Fatal(err)
			}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/token" {
					_, _ = io.WriteString(w, `{"access_token":"private-token","expires_in":3600}`)
					return
				}
				w.WriteHeader(tc.status)
				_, _ = io.WriteString(w, `{"errcode":0}`)
			}))
			defer server.Close()
			store := &traceDeliveryStore{finishErr: tc.finishErr}
			sender := wechat.NewSender(wechat.Config{AppID: "app", Secret: "private-secret", TemplateID: "template", HTTP: server.Client(), TokenURL: server.URL + "/token", SendURL: server.URL + "/send", Logger: logger})
			worker, err := notifications.NewWorker(notifications.Config{Store: store, Sender: sender, Logger: logger})
			if err != nil {
				t.Fatal(err)
			}

			// When
			ok, err := worker.RunOnce(observability.ContextWithRequestID(context.Background(), "upstream-id"))

			// Then
			wantError := tc.status != 200 || tc.finishErr != nil
			if !ok || (err != nil) != wantError || store.result.Status != tc.wantResult {
				t.Fatalf("ok=%v error=%v status=%s", ok, err, store.result.Status)
			}
			if store.requestID == "" || store.requestID == "upstream-id" {
				t.Fatal("attempt must have its own generated request ID")
			}
			if store.claimID != store.requestID {
				t.Fatal("claim and delivery contexts must share an attempt ID")
			}
			for _, secret := range []string{"private-lease", "private-recipient", "private-device", "private-message", "private-token", "private-secret", "private-store-secret", server.URL} {
				if strings.Contains(logs.String(), secret) {
					t.Fatalf("diagnostics exposed forbidden fixture %q", secret)
				}
			}
			seen := map[string]bool{}
			decoder := json.NewDecoder(&logs)
			for decoder.More() {
				var record map[string]any
				if err := decoder.Decode(&record); err != nil {
					t.Fatal(err)
				}
				if record["request_id"] != store.requestID {
					t.Fatalf("attempt correlation changed: %v", record)
				}
				key := record["stage"].(string) + "/" + record["operation"].(string) + "/" + record["outcome"].(string)
				seen[key] = true
				if key == "outbox/finish/failed" && tc.finishErr != nil && record["level"] != "WARN" {
					t.Fatal("persistence failure must be visible at warn console level")
				}
				raw, err := json.Marshal(record)
				if err != nil {
					t.Fatal(err)
				}
				t.Log(string(raw))
			}
			for _, key := range []string{"outbox/claim/claimed", "notification/send/started", "notification/send/completed", "wechat/access_token/started", "wechat/access_token/success", "wechat/subscribe_send/started", "outbox/finish/started", "outbox/finish/" + tc.wantFinal} {
				if !seen[key] {
					t.Fatalf("missing attempt stage %s", key)
				}
			}
		})
	}
}

func TestWorkerClaimFailureIsCorrelatedAndSafe(t *testing.T) {
	var logs bytes.Buffer
	logger, err := observability.Console(observability.Config{Level: "warn"}, &logs)
	if err != nil {
		t.Fatal(err)
	}
	store := &traceDeliveryStore{claimErr: errors.New("private-claim-query")}
	worker, err := notifications.NewWorker(notifications.Config{Store: store, Logger: logger})
	if err != nil {
		t.Fatal(err)
	}
	claimed, err := worker.RunOnce(context.Background())
	if claimed || !errors.Is(err, store.claimErr) || store.requestID != "" {
		t.Fatal("claim failure changed processing semantics")
	}
	var record map[string]any
	if err := json.Unmarshal(logs.Bytes(), &record); err != nil {
		t.Fatal(err)
	}
	if store.claimID == "" || record["request_id"] != store.claimID || record["operation"] != "claim" || record["outcome"] != "failed" || record["level"] != "WARN" {
		t.Fatalf("missing correlated claim failure: %s", logs.String())
	}
	if strings.Contains(logs.String(), "private-claim-query") {
		t.Fatal("claim error text leaked")
	}
}
