package core

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"git.hyhy.fun/rsplab/iolink/internal/event"
	"git.hyhy.fun/rsplab/iolink/internal/observability"
)

func TestHandleEventValidationDumpUsesCorrelationAndSafeError(t *testing.T) {
	var output bytes.Buffer
	log, err := observability.Console(observability.Config{Level: "debug"}, &output)
	if err != nil {
		t.Fatal(err)
	}
	s := &Service{log: log}
	_ = s.HandleEvent(event.Event{IngestID: "server-generated-id", Kind: "invalid", DeviceNo: "device-secret", MessageID: "message-secret"})
	lines := strings.Split(strings.TrimSpace(output.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected validation start and rejection records, got %d: %s", len(lines), output.String())
	}
	var rejected map[string]any
	if err := json.Unmarshal([]byte(lines[1]), &rejected); err != nil {
		t.Fatal(err)
	}
	if rejected["ingest_id"] != "server-generated-id" || rejected["stage"] != "core_validation" || rejected["outcome"] != "rejected" {
		t.Fatalf("missing diagnostic fields: %#v", rejected)
	}
	for _, secret := range []string{"device-secret", "message-secret"} {
		if strings.Contains(output.String(), secret) {
			t.Fatalf("sensitive value leaked: %s", secret)
		}
	}
}
