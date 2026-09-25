package core_test

import (
	"context"
	"encoding/json"
	"fmt"
	"git.hyhy.fun/rsplab/iolink/internal/core"
	"git.hyhy.fun/rsplab/iolink/internal/domain"
	"git.hyhy.fun/rsplab/iolink/internal/event"
	"git.hyhy.fun/rsplab/iolink/internal/migrate"
	"git.hyhy.fun/rsplab/iolink/internal/testdb"
	"github.com/jackc/pgx/v5/pgxpool"
	dto "github.com/prometheus/client_model/go"
	"io"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"
)

func setup(t *testing.T) (*core.Service, *pgxpool.Pool) {
	t.Helper()
	p := testdb.New(t)
	ctx := context.Background()
	if err := migrate.Up(ctx, p); err != nil {
		t.Fatal(err)
	}
	execute(t, p, `INSERT INTO users(id,open_id) VALUES(1,'test-user');INSERT INTO farms(id,owner_id,name) VALUES(1,1,'farm');INSERT INTO ponds(id,farm_id,name) VALUES(1,1,'A'),(2,1,'B')`)
	execute(t, p, `INSERT INTO devices(pond_id,device_no,secret_hash) VALUES(1,'one',$1)`, core.HashDeviceSecret("test-secret"))
	s, err := core.New(ctx, p, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	return s, p
}
func execute(t *testing.T, p *pgxpool.Pool, q string, args ...any) {
	t.Helper()
	if _, err := p.Exec(context.Background(), q, args...); err != nil {
		t.Fatal(err)
	}
}
func count(t *testing.T, p *pgxpool.Pool, q string) int {
	t.Helper()
	var n int
	if err := p.QueryRow(context.Background(), q).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}
func sample(ts time.Time, id string, props map[string]float64) event.Event {
	return event.Event{Kind: event.KindProperties, DeviceNo: "one", Ts: ts, MessageID: id, Properties: props}
}
func TestTelemetryShadowDedupAndPondSnapshot(t *testing.T) {
	s, p := setup(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond)
	events := []event.Event{sample(now, "first", map[string]float64{"temperature": 26, "signal": -65}), sample(now.Add(time.Second), "second", map[string]float64{"ph": 7}), sample(now.Add(-time.Second), "older", map[string]float64{"temperature": 20})}
	for _, e := range events {
		if err := s.HandleEvent(e); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.HandleEvent(events[0]); err != nil {
		t.Fatal(err)
	}
	if count(t, p, "SELECT count(*) FROM sensor_data") != 3 {
		t.Fatal("message_id dedup failed")
	}
	rd, err := s.Telemetry().Latest(ctx, "one")
	if err != nil || rd.Temperature == nil || *rd.Temperature != 26 || rd.PH == nil || *rd.PH != 7 || rd.Signal == nil || *rd.Signal != -65 || !rd.Timestamps["temperature"].Equal(now) || !rd.Timestamps["ph"].Equal(now.Add(time.Second)) || rd.PondID != 1 || rd.ReportInterval != 60 {
		t.Fatalf("shadow %+v %v", rd, err)
	}
	execute(t, p, "UPDATE ingest_messages SET received_at=now()-interval '25 hours' WHERE message_id='first'")
	if err := s.HandleEvent(events[0]); err != nil {
		t.Fatal(err)
	}
	if count(t, p, "SELECT count(*) FROM sensor_data") != 4 {
		t.Fatal("expired id not accepted")
	}
	for range 2 {
		if err := s.HandleEvent(sample(now, "", map[string]float64{"battery": 90})); err != nil {
			t.Fatal(err)
		}
	}
	if count(t, p, "SELECT count(*) FROM sensor_data") != 6 {
		t.Fatal("legacy at-least-once changed")
	}
	execute(t, p, "UPDATE devices SET pond_id=2 WHERE device_no='one'")
	if _, err := s.Telemetry().Latest(ctx, "one"); err == nil {
		t.Fatal("old pond shadow leaked")
	}
	if err := s.HandleEvent(sample(now.Add(2*time.Second), "new-pond", map[string]float64{"ph": 8})); err != nil {
		t.Fatal(err)
	}
	rd, err = s.Telemetry().Latest(ctx, "one")
	if err != nil || rd.PondID != 2 || rd.Temperature != nil || *rd.PH != 8 {
		t.Fatal("new pond shadow", rd, err)
	}
	if count(t, p, "SELECT count(*) FROM sensor_data WHERE pond_id=1") != 6 {
		t.Fatal("historical pond overwritten")
	}
}
func TestAlarmThresholdConcurrencyConfirmationAndMove(t *testing.T) {
	s, p := setup(t)
	ctx := context.Background()
	min, max := 4.0, 8.0
	for _, pond := range []int64{1, 2} {
		if _, err := s.CreateRule(ctx, domain.AlarmRule{PondID: pond, Metric: "dissolved_oxygen", Min: &min, Max: &max, Level: domain.AlarmCritical}); err != nil {
			t.Fatal(err)
		}
	}
	now := time.Now().UTC()
	for _, v := range []float64{4, 8} {
		if err := s.HandleEvent(sample(now, "", map[string]float64{"dissolved_oxygen": v})); err != nil {
			t.Fatal(err)
		}
	}
	if count(t, p, "SELECT count(*) FROM alarms") != 0 {
		t.Fatal("equal threshold alarm")
	}
	errs := make(chan error, 100)
	var wg sync.WaitGroup
	for i := range 100 {
		wg.Go(func() {
			errs <- s.HandleEvent(sample(now, fmt.Sprintf("c-%d", i), map[string]float64{"dissolved_oxygen": 3}))
		})
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	if count(t, p, "SELECT count(*) FROM alarms") != 1 || count(t, p, "SELECT count(*) FROM notification_outbox") != 1 {
		t.Fatal("concurrent duplicate alarms/outbox")
	}
	var id int64
	var threshold float64
	var message string
	if err := p.QueryRow(ctx, "SELECT id,threshold,message FROM alarms").Scan(&id, &threshold, &message); err != nil || threshold != 4 || !strings.Contains(message, "低于") {
		t.Fatal(threshold, message, err)
	}
	execute(t, p, "UPDATE devices SET pond_id=2 WHERE device_no='one'")
	if err := s.HandleEvent(sample(now, "new", map[string]float64{"dissolved_oxygen": 9})); err != nil {
		t.Fatal(err)
	}
	if count(t, p, "SELECT count(*) FROM alarms WHERE confirmed_at IS NULL") != 2 {
		t.Fatal("old pond suppresses new pond")
	}
	if err := p.QueryRow(ctx, "SELECT threshold,message FROM alarms WHERE pond_id=2").Scan(&threshold, &message); err != nil || threshold != 8 || !strings.Contains(message, "高于") {
		t.Fatal(threshold, message, err)
	}
	for range 2 {
		if err := s.ConfirmAlarm(ctx, id); err != nil {
			t.Fatal(err)
		}
	}
	execute(t, p, "UPDATE devices SET pond_id=1 WHERE device_no='one'")
	if err := s.HandleEvent(sample(now, "after-confirm", map[string]float64{"dissolved_oxygen": 2})); err != nil {
		t.Fatal(err)
	}
	if count(t, p, "SELECT count(*) FROM alarms") != 3 {
		t.Fatal("confirmed alarm prevents new one")
	}
}
func TestRollbackAcrossReadingShadowAlarmOutbox(t *testing.T) {
	s, p := setup(t)
	ctx := context.Background()
	max := 25.0
	if _, err := s.CreateRule(ctx, domain.AlarmRule{PondID: 1, Metric: "temperature", Max: &max, Level: domain.AlarmWarning}); err != nil {
		t.Fatal(err)
	}
	execute(t, p, `CREATE FUNCTION fail_outbox() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'injected failure'; END $$; CREATE TRIGGER test_fail BEFORE INSERT ON notification_outbox FOR EACH ROW EXECUTE FUNCTION fail_outbox()`)
	e := sample(time.Now(), "retry-me", map[string]float64{"temperature": 26})
	if s.HandleEvent(e) == nil {
		t.Fatal("injected failure ignored")
	}
	for _, table := range []string{"sensor_data", "device_shadows", "alarms", "notification_outbox", "ingest_messages"} {
		if count(t, p, "SELECT count(*) FROM "+table) != 0 {
			t.Fatal("partial commit", table)
		}
	}
	execute(t, p, "DROP TRIGGER test_fail ON notification_outbox")
	if err := s.HandleEvent(e); err != nil {
		t.Fatal(err)
	}
	if count(t, p, "SELECT count(*) FROM notification_outbox") != 1 {
		t.Fatal("retry failed")
	}
}
func TestHistoryBoundedAndInvalidRules(t *testing.T) {
	s, p := setup(t)
	ctx := context.Background()
	from := time.Now().UTC().Truncate(time.Hour).Add(-24 * time.Hour)
	to := from.Add(24 * time.Hour)
	execute(t, p, `INSERT INTO sensor_data(ts,device_no,pond_id,temperature) SELECT $1::timestamptz+i*interval '1 minute','one',1,20+i%10 FROM generate_series(0,1439) i`, from)
	for _, limit := range []int{1, 200} {
		points, err := s.Telemetry().History(ctx, "one", "temperature", from, to, limit)
		if err != nil || len(points) != limit {
			t.Fatalf("limit %d got %d %v", limit, len(points), err)
		}
		for i, p := range points {
			if p.Ts.Before(from) || !p.Ts.Before(to) || (i > 0 && !p.Ts.After(points[i-1].Ts)) {
				t.Fatal("bucket timestamps")
			}
		}
	}
	points, err := s.Telemetry().History(ctx, "none", "temperature", from, to, 200)
	if err != nil || points == nil || len(points) != 0 {
		t.Fatal("empty", points, err)
	}
	if _, err = s.Telemetry().History(ctx, "one", "temperature);DROP TABLE users;--", from, to, 200); err != domain.ErrUnknownMetric {
		t.Fatal(err)
	}
	if _, err = s.Telemetry().History(ctx, "one", "temperature", from, to, 0); err != domain.ErrInvalidRange {
		t.Fatal(err)
	}
	min, max := 8.0, 4.0
	if _, err = s.CreateRule(ctx, domain.AlarmRule{PondID: 1, Metric: "ph", Min: &min, Max: &max, Level: domain.AlarmWarning}); err != domain.ErrInvalidRule {
		t.Fatal("inverted rule accepted", err)
	}
}
func TestAuthenticationStatusAndRestart(t *testing.T) {
	s, p := setup(t)
	ctx := context.Background()
	if !s.Authenticate("one", "test-secret") || s.Authenticate("one", "bad") {
		t.Fatal("device credential check")
	}
	e := event.Event{Kind: event.KindStatusChange, DeviceNo: "one", Ts: time.Now(), Online: true}
	for range 2 {
		if err := s.HandleEvent(e); err != nil {
			t.Fatal(err)
		}
	}
	if onlineGauge() != 1 {
		t.Fatal("duplicate online counted")
	}
	e.Online = false
	for range 2 {
		if err := s.HandleEvent(e); err != nil {
			t.Fatal(err)
		}
	}
	if onlineGauge() != 0 {
		t.Fatal("negative online gauge")
	}
	e.Online = true
	if err := s.HandleEvent(e); err != nil {
		t.Fatal(err)
	}
	if _, err := core.New(ctx, p, slog.New(slog.NewTextHandler(io.Discard, nil))); err != nil {
		t.Fatal(err)
	}
	if count(t, p, "SELECT count(*) FROM devices WHERE status='online'") != 0 || onlineGauge() != 0 {
		t.Fatal("restart stale online")
	}
	execute(t, p, "UPDATE devices SET disabled_at=now()")
	if s.Authenticate("one", "test-secret") {
		t.Fatal("disabled device accepted")
	}
}

func onlineGauge() float64 {
	m := &dto.Metric{}
	_ = core.MetricDevicesOnline.Write(m)
	return m.GetGauge().GetValue()
}

func TestDeviceSecretIsRandomAndReturnedOnlyOnCreation(t *testing.T) {
	s, p := setup(t)
	ctx := context.Background()
	one, secret, err := s.RegisterDevice(ctx, 1, "audit", "audit", 0)
	if err != nil || len(secret) != 64 {
		t.Fatal("registration secret", err)
	}
	_, other, err := s.RegisterDevice(ctx, 1, "audit", "audit", 0)
	if err != nil || other == secret {
		t.Fatal("randomness", err)
	}
	var stored string
	if err = p.QueryRow(ctx, "SELECT secret_hash FROM devices WHERE device_no=$1", one.DeviceNo).Scan(&stored); err != nil || stored == secret || stored != core.HashDeviceSecret(secret) {
		t.Fatal("secret storage", err)
	}
	devices, err := s.ListDevices(ctx, false, 0, 50, 0)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(devices)
	if err != nil || strings.Contains(string(raw), secret) || strings.Contains(string(raw), stored) {
		t.Fatal("secret leaked from list")
	}
	execute(t, p, "UPDATE devices SET secret_hash='corrupt' WHERE device_no=$1", one.DeviceNo)
	if s.Authenticate(one.DeviceNo, secret) {
		t.Fatal("corrupt hash accepted")
	}
}

func TestConfiguredAndDeviceIntervals(t *testing.T) {
	_, p := setup(t)
	s, err := core.New(context.Background(), p, slog.New(slog.NewTextHandler(io.Discard, nil)), 5*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if s.ReportInterval("one") != 5*time.Minute {
		t.Fatal("global interval not inherited")
	}
	execute(t, p, "UPDATE devices SET report_interval=60 WHERE device_no='one'")
	if s.ReportInterval("one") != time.Minute {
		t.Fatal("device interval not applied")
	}
}

func TestSingleAndDoubleThresholdRules(t *testing.T) {
	for _, tt := range []struct {
		name        string
		min, max    *float64
		value, want float64
		direction   string
	}{
		{"min-only", number(4), nil, 3, 4, "低于"}, {"max-only", nil, number(8), 9, 8, "高于"}, {"both-low", number(4), number(8), 3, 4, "低于"}, {"both-high", number(4), number(8), 9, 8, "高于"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			s, p := setup(t)
			if _, err := s.CreateRule(context.Background(), domain.AlarmRule{PondID: 1, Metric: "dissolved_oxygen", Min: tt.min, Max: tt.max, Level: domain.AlarmWarning}); err != nil {
				t.Fatal(err)
			}
			if err := s.HandleEvent(sample(time.Now(), "", map[string]float64{"temperature": 25})); err != nil {
				t.Fatal(err)
			}
			if count(t, p, "SELECT count(*) FROM alarms") != 0 {
				t.Fatal("missing metric alarm")
			}
			if err := s.HandleEvent(sample(time.Now(), "", map[string]float64{"dissolved_oxygen": tt.value})); err != nil {
				t.Fatal(err)
			}
			var threshold float64
			var message, level string
			if err := p.QueryRow(context.Background(), "SELECT threshold,message,level FROM alarms").Scan(&threshold, &message, &level); err != nil || threshold != tt.want || !strings.Contains(message, tt.direction) || level != "warning" {
				t.Fatal(threshold, message, level, err)
			}
		})
	}
}
func number(v float64) *float64 { return &v }
