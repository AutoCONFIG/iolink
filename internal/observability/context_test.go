package observability

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"
)

func TestDerivedLoggerPreservesContextAndRedaction(t *testing.T) {
	for _, grouped := range []bool{false, true} {
		t.Run(map[bool]string{false: "attrs", true: "group"}[grouped], func(t *testing.T) {
			var out bytes.Buffer
			logger, err := Console(Config{Level: "debug"}, &out)
			if err != nil {
				t.Fatal(err)
			}
			logger = logger.With("stage", "database")
			if grouped {
				logger = logger.WithGroup("trace")
			}
			ctx := ContextWithIngestID(ContextWithRequestID(context.Background(), "request-server"), "ingest-server")
			logger.DebugContext(ctx, "internal stage", "status", "secret-status", "kind", "secret-kind", "fields", "secret-fields", "protocol", "secret-protocol", "retain", "secret-retain", "field", "secret-field", "count", 3, "qos", 1)
			var record map[string]any
			if err := json.Unmarshal(out.Bytes(), &record); err != nil {
				t.Fatal(err)
			}
			if grouped {
				record = record["trace"].(map[string]any)
			}
			if record["request_id"] != "request-server" || record["ingest_id"] != "ingest-server" || record["count"] != float64(3) || record["qos"] != float64(1) {
				t.Fatalf("context or numeric attrs lost: %s", out.String())
			}
			if strings.Contains(out.String(), "secret-") {
				t.Fatalf("unsafe diagnostic value: %s", out.String())
			}
		})
	}
}

func TestContextHandlerDoesNotMutateSharedRecord(t *testing.T) {
	var out bytes.Buffer
	logger, err := Console(Config{Level: "debug"}, &out)
	if err != nil {
		t.Fatal(err)
	}
	record := slog.Record{}
	for i := 0; i < 8; i++ {
		record.AddAttrs(slog.Int("count", i))
	}
	ctx := ContextWithRequestID(context.Background(), "server-id")
	if err := logger.Handler().Handle(ctx, record); err != nil {
		t.Fatal(err)
	}
	// Reusing the caller's record must not detect attributes written into its spare capacity.
	record.AddAttrs(slog.String("outcome", "next"))
	if err := logger.Handler().Handle(context.Background(), record); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	var reused map[string]any
	if err := json.Unmarshal([]byte(lines[1]), &reused); err != nil {
		t.Fatal(err)
	}
	if reused["outcome"] != "next" || reused["request_id"] != nil || reused["!BUG"] != nil {
		t.Fatalf("caller record polluted by prior context: %s", lines[1])
	}
}
