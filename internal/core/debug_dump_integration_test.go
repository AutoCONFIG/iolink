package core_test

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

	"git.hyhy.fun/rsplab/iolink/internal/access"
	"git.hyhy.fun/rsplab/iolink/internal/core"
	"git.hyhy.fun/rsplab/iolink/internal/license"
	"git.hyhy.fun/rsplab/iolink/internal/observability"
	"git.hyhy.fun/rsplab/iolink/internal/operations"
)

type chainNoticeWriter struct{ terminal chan map[string]any }

func (w chainNoticeWriter) Write(raw []byte) (int, error) {
	var record map[string]any
	if json.Unmarshal(raw, &record) == nil && record["stage"] == "mqtt_dispatch" && (record["outcome"] == "accepted" || record["outcome"] == "rejected") {
		w.terminal <- record
	}
	return len(raw), nil
}

type debugChainFixture struct {
	service *core.Service
	pool    *pgxpool.Pool
	logger  *slog.Logger
	path    string
	done    chan map[string]any
}

func newDebugChainFixture(t *testing.T) debugChainFixture {
	t.Helper()
	_, original := setup(t)
	dir := t.TempDir()
	done := make(chan map[string]any, 8)
	logger, sink, err := observability.New(observability.Config{Dir: dir, Version: "chain", Level: "debug", MaxMB: 20, Backups: 2}, chainNoticeWriter{done})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := sink.Close(); err != nil {
			t.Error(err)
		}
	})
	config := original.Config().Copy()
	config.ConnConfig.Tracer = observability.QueryTracer{Logger: logger}
	pool, err := pgxpool.NewWithConfig(t.Context(), config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	service, err := core.New(t.Context(), pool, logger)
	if err != nil {
		t.Fatal(err)
	}
	return debugChainFixture{service: service, pool: pool, logger: logger, path: filepath.Join(dir, "chain", "iolinkd.jsonl"), done: done}
}

func (f debugChainFixture) records(t *testing.T) []map[string]any {
	t.Helper()
	raw, err := os.ReadFile(f.path)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{`"one"`, "test-secret", "iolink-one-seed01", "private-message-id", "private-payload-secret", "private-sql-error", "private-device-name", "private-device-model", "private-http-query", "private-http-body", "private-header", "iolink/up/", "INSERT INTO", "SELECT "} {
		if strings.Contains(string(raw), forbidden) {
			t.Fatalf("diagnostic file exposed forbidden fixture %q", forbidden)
		}
	}
	if evidenceDir := os.Getenv("IOLINK_TEST_DUMP_EVIDENCE_DIR"); evidenceDir != "" {
		if err := os.MkdirAll(evidenceDir, 0o750); err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(evidenceDir, strings.ReplaceAll(t.Name(), "/", "-")+".jsonl")
		if err := os.WriteFile(path, raw, 0o600); err != nil {
			t.Fatal(err)
		}
		t.Logf("JSONL artifact: %s", path)
	}
	var records []map[string]any
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	for decoder.More() {
		var record map[string]any
		if err := decoder.Decode(&record); err != nil {
			t.Fatal(err)
		}
		records = append(records, record)
	}
	return records
}

type chainExpectation struct {
	key, id string
	stages  []string
}

func assertChainStages(t *testing.T, records []map[string]any, want chainExpectation) {
	t.Helper()
	if want.id == "" {
		t.Fatal("missing server-generated correlation ID")
	}
	seen, spans := map[string]bool{}, map[string]string{}
	for _, record := range records {
		if record[want.key] != want.id {
			continue
		}
		stage, _ := record["stage"].(string)
		outcome, _ := record["outcome"].(string)
		seen[stage+"/"+outcome] = true
		if stage == "database" {
			span, _ := record["span_id"].(string)
			if span == "" {
				t.Fatal("database record has no span ID")
			}
			if outcome != "started" && spans[span] != "started" {
				t.Fatal("database terminal record has no correlated start")
			}
			spans[span] = outcome
		}
		encoded, err := json.Marshal(record)
		if err != nil {
			t.Fatal(err)
		}
		t.Log(string(encoded))
	}
	for _, stage := range want.stages {
		if !seen[stage] {
			t.Fatalf("missing correlated stage %s for %s=%s", stage, want.key, want.id)
		}
	}
	for _, outcome := range spans {
		if outcome == "started" {
			t.Fatal("database start record has no correlated terminal record")
		}
	}
}

func TestDebugDumpRealMQTTCorePostgresCorrelation(t *testing.T) {
	for _, scenario := range []string{"success", "duplicate", "database_failure"} {
		t.Run(scenario, func(t *testing.T) {
			// Given
			f := newDebugChainFixture(t)
			if scenario == "database_failure" {
				execute(t, f.pool, `CREATE FUNCTION fail_debug_dump() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'private-sql-error'; END $$; CREATE TRIGGER fail_debug_dump BEFORE INSERT ON sensor_data FOR EACH ROW EXECUTE FUNCTION fail_debug_dump()`)
			}
			listener, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			addr := listener.Addr().String()
			if err := listener.Close(); err != nil {
				t.Fatal(err)
			}
			broker := access.New(access.Config{MQTTAddr: addr}, f.service, f.service, f.logger)
			if err := broker.Serve(); err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := broker.Close(); err != nil {
					t.Error(err)
				}
			}()
			connection, code := rawConnect(t, addr, 4, false)
			if code != 0 {
				t.Fatalf("MQTT connection rejected: %d", code)
			}
			payload := `{"message_id":"private-message-id","temperature":27,"discarded":"private-payload-secret"}`
			publish := func(packetID byte) map[string]any {
				t.Helper()
				body := append(mqttString("iolink/up/one/properties"), 0, packetID)
				if _, err := connection.Write(mqttPacket(0x32, append(body, payload...))); err != nil {
					t.Fatal(err)
				}
				select {
				case record := <-f.done:
					return record
				case <-time.After(5 * time.Second):
					t.Fatal("MQTT dispatch produced no terminal diagnostic")
					return nil
				}
			}
			if scenario == "duplicate" {
				first := publish(1)
				if first["outcome"] != "accepted" {
					t.Fatal("duplicate fixture initial publish was rejected")
				}
				if kind, _, err := readMQTT(connection); err != nil || kind != 0x40 {
					t.Fatal("initial publish had no PUBACK")
				}
			}

			// When
			terminal := publish(2)

			// Then
			required := []string{"mqtt_ingress/started", "mqtt_dispatch/started", "core_validation/accepted", "database/started", "database/completed"}
			wantRows := 1
			if scenario == "database_failure" {
				wantRows = 0
				required = append(required, "database/failed", "ingestion/failed", "mqtt_dispatch/rejected")
			} else {
				required = append(required, "ingestion/completed", "mqtt_dispatch/accepted")
				if scenario == "duplicate" {
					required = append(required, "deduplication/duplicate")
				} else {
					required = append(required, "persistence/committed")
				}
			}
			for _, table := range []string{"sensor_data", "telemetry", "device_shadows", "ingest_messages"} {
				if got := count(t, f.pool, "SELECT count(*) FROM "+table); got != wantRows {
					t.Fatalf("%s rows=%d want=%d", table, got, wantRows)
				}
			}
			id, _ := terminal["ingest_id"].(string)
			assertChainStages(t, f.records(t), chainExpectation{key: "ingest_id", id: id, stages: required})
		})
	}
}

func TestDebugDumpHTTPRegisterDeviceLicenseRejectionCorrelation(t *testing.T) {
	// Given
	f := newDebugChainFixture(t)
	router := gin.New()
	router.Use(operations.RequestLogging(f.logger))
	var registerErr error
	router.POST("/devices", func(c *gin.Context) {
		_, _, registerErr = f.service.RegisterDevice(c.Request.Context(), 1, "private-device-name", "private-device-model", 60)
		c.Status(http.StatusForbidden)
	})
	request := httptest.NewRequest(http.MethodPost, "/devices?private-http-query=secret", strings.NewReader("private-http-body"))
	request.Header.Set("X-Request-ID", "private-header")
	response := httptest.NewRecorder()

	// When
	router.ServeHTTP(response, request)

	// Then
	if !errors.Is(registerErr, license.ErrUnavailable) || response.Code != http.StatusForbidden || count(t, f.pool, "SELECT count(*) FROM devices") != 1 {
		t.Fatalf("license rejection error=%v status=%d", registerErr, response.Code)
	}
	id := response.Header().Get("X-Request-ID")
	if id == "private-header" {
		t.Fatal("caller-controlled request ID was retained")
	}
	records := f.records(t)
	assertChainStages(t, records, chainExpectation{key: "request_id", id: id, stages: []string{"http/started", "database/started", "database/completed", "license/rejected"}})
	for _, record := range records {
		if record["request_id"] == id && record["stage"] == "license" && record["operation"] != "verifier" {
			t.Fatal("license rejection did not identify missing verifier")
		}
	}
}
