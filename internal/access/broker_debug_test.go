package access

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"git.hyhy.fun/rsplab/iolink/internal/observability"
	"github.com/mochi-mqtt/server/v2/packets"
)

func TestPublishEmitsCorrelatedSafeRecords(t *testing.T) {
	var output bytes.Buffer
	log, err := observability.Console(observability.Config{Level: "debug"}, &output)
	if err != nil {
		t.Fatal(err)
	}
	handler := &fakeHandler{}
	s := New(Config{}, handler, fakeAuth{ok: true}, log)
	h := newBrokerHook(s)
	cl := client("device-secret")
	h.OnSessionEstablished(cl, packets.Packet{})
	_, err = h.OnPublish(cl, packets.Packet{
		TopicName:   "iolink/up/device-secret/properties",
		Payload:     []byte(`{"temperature":26,"message_id":"client-secret"}`),
		FixedHeader: packets.FixedHeader{Qos: 1},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(handler.events) == 0 || handler.events[len(handler.events)-1].IngestID == "" {
		t.Fatalf("missing event correlation: %+v", handler.events)
	}
	ingestID := handler.events[len(handler.events)-1].IngestID
	var records []map[string]any
	for _, line := range strings.Split(strings.TrimSpace(output.String()), "\n") {
		var record map[string]any
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			t.Fatalf("invalid JSONL record %q: %v", line, err)
		}
		if record["ingest_id"] == ingestID {
			records = append(records, record)
		}
	}
	if len(records) < 2 {
		t.Fatalf("want ingress and dispatch records, got %d: %s", len(records), output.String())
	}
	for _, forbidden := range []string{"device-secret", "client-secret", "iolink/up/"} {
		if strings.Contains(output.String(), forbidden) {
			t.Fatalf("sensitive client value leaked: %q", forbidden)
		}
	}
}

func TestRejectedPublishStillHasCorrelation(t *testing.T) {
	var output bytes.Buffer
	log, err := observability.Console(observability.Config{Level: "debug"}, &output)
	if err != nil {
		t.Fatal(err)
	}
	s := New(Config{}, &fakeHandler{}, fakeAuth{ok: true}, log)
	h := newBrokerHook(s)
	cl := client("device")
	h.OnSessionEstablished(cl, packets.Packet{})
	if _, err := h.OnPublish(cl, packets.Packet{TopicName: "invalid", Payload: []byte(`{}`), FixedHeader: packets.FixedHeader{Qos: 1}}); err == nil {
		t.Fatal("invalid topic accepted")
	}
	ids := map[string]int{}
	for _, line := range strings.Split(strings.TrimSpace(output.String()), "\n") {
		var record struct {
			IngestID string `json:"ingest_id"`
		}
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			t.Fatal(err)
		}
		if record.IngestID != "" {
			ids[record.IngestID]++
		}
	}
	if len(ids) != 1 {
		t.Fatalf("rejected flow should have one correlation id: %#v", ids)
	}
	for _, count := range ids {
		if count < 2 {
			t.Fatalf("rejected flow records=%d", count)
		}
	}
}
