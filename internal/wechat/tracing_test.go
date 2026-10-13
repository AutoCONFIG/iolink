package wechat

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
)

func TestSenderTracesSafeDownstreamOutcomes(t *testing.T) {
	for _, tc := range []struct {
		name      string
		operation string
		status    int
		body      string
		wantErr   error
		errorCode string
		timeout   bool
	}{
		{name: "success", operation: "subscribe_send", status: 200, body: `{"errcode":0}`, errorCode: ""},
		{name: "token http failure", operation: "access_token", status: 400, body: `private-body-secret`, wantErr: notifications.ErrPermanent, errorCode: "permanent"},
		{name: "send http failure", operation: "subscribe_send", status: 403, body: `private-body-secret`, wantErr: notifications.ErrPermanent, errorCode: "permanent"},
		{name: "malformed send response", operation: "subscribe_send", status: 200, body: `private-body-secret`, wantErr: notifications.ErrUnknown, errorCode: "unknown"},
		{name: "provider failure", operation: "subscribe_send", status: 200, body: `{"errcode":40003,"errmsg":"private-body-secret"}`, wantErr: notifications.ErrPermanent, errorCode: "permanent"},
		{name: "token timeout", operation: "access_token", status: 0, wantErr: notifications.ErrRetryable, errorCode: "deadline_exceeded", timeout: true},
		{name: "send timeout", operation: "subscribe_send", status: 0, wantErr: notifications.ErrUnknown, errorCode: "deadline_exceeded", timeout: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Given
			var logs bytes.Buffer
			logger, err := observability.Console(observability.Config{Level: "debug"}, &logs)
			if err != nil {
				t.Fatal(err)
			}
			blocked := make(chan struct{})
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				operation := "subscribe_send"
				if r.URL.Path == "/token" {
					operation = "access_token"
				}
				if operation == tc.operation {
					if tc.timeout {
						<-blocked
						return
					}
					w.WriteHeader(tc.status)
					_, _ = io.WriteString(w, tc.body)
					return
				}
				_, _ = io.WriteString(w, `{"access_token":"private-access-token","expires_in":3600}`)
			}))
			defer server.Close()
			defer close(blocked)
			client := server.Client()
			if tc.timeout {
				client.Timeout = 100 * time.Millisecond
			}
			sender := NewSender(Config{AppID: "private-app-id", Secret: "private-app-secret", TemplateID: "private-template-id", Page: "private-page", TokenURL: server.URL + "/token", SendURL: server.URL + "/send", HTTP: client, Logger: logger})
			delivery := testDelivery()
			delivery.RecipientID, delivery.DeviceNo, delivery.Message = "private-recipient-id", "private-device-id", "private-body-secret"
			ctx := observability.ContextWithRequestID(context.Background(), "delivery-correlation")

			// When
			err = sender.Send(ctx, delivery)

			// Then
			if tc.wantErr == nil && err != nil || tc.wantErr != nil && !errors.Is(err, tc.wantErr) {
				t.Fatalf("delivery error=%v want=%v", err, tc.wantErr)
			}
			for _, secret := range []string{"private-app-id", "private-app-secret", "private-template-id", "private-page", "private-access-token", "private-recipient-id", "private-device-id", "private-body-secret", server.URL, "access_token=", "grant_type=", "Content-Type"} {
				if strings.Contains(logs.String(), secret) {
					t.Fatalf("diagnostics exposed forbidden fixture %q", secret)
				}
			}
			var completed bool
			decoder := json.NewDecoder(&logs)
			for decoder.More() {
				var record map[string]any
				if err := decoder.Decode(&record); err != nil {
					t.Fatal(err)
				}
				if record["request_id"] != "delivery-correlation" || record["stage"] != "wechat" {
					t.Fatalf("uncorrelated downstream record: %v", record)
				}
				if record["operation"] == tc.operation && record["outcome"] != "started" {
					completed = true
					wantOutcome := "success"
					if tc.wantErr != nil {
						wantOutcome = "failed"
					}
					if record["status"] != float64(tc.status) || record["outcome"] != wantOutcome || record["error_code"] != tc.errorCode || record["duration_ms"] == nil {
						t.Fatalf("downstream outcome mismatch: %v", record)
					}
				}
				raw, err := json.Marshal(record)
				if err != nil {
					t.Fatal(err)
				}
				t.Log(string(raw))
			}
			if !completed {
				t.Fatal("missing terminal downstream record")
			}
		})
	}
}

func TestSenderTraceReportsCachedTokenWithoutAnotherTokenRequest(t *testing.T) {
	// Given
	var logs bytes.Buffer
	logger, err := observability.Console(observability.Config{Level: "debug"}, &logs)
	if err != nil {
		t.Fatal(err)
	}
	tokenCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/token" {
			tokenCalls++
			_, _ = io.WriteString(w, `{"access_token":"private-token","expires_in":3600}`)
			return
		}
		_, _ = io.WriteString(w, `{"errcode":0}`)
	}))
	defer server.Close()
	sender := NewSender(Config{AppID: "app", Secret: "secret", TemplateID: "template", TokenURL: server.URL + "/token", SendURL: server.URL + "/send", HTTP: server.Client(), Logger: logger})
	ctx := observability.ContextWithRequestID(context.Background(), "cached-delivery")
	if err := sender.Send(ctx, testDelivery()); err != nil {
		t.Fatal(err)
	}
	logs.Reset()

	// When
	err = sender.Send(ctx, testDelivery())

	// Then
	if err != nil || tokenCalls != 1 {
		t.Fatalf("error=%v token requests=%d", err, tokenCalls)
	}
	if !strings.Contains(logs.String(), `"outcome":"cache_hit"`) || strings.Contains(logs.String(), "private-token") {
		t.Fatalf("cached token trace=%s", logs.String())
	}
	t.Log(logs.String())
}
