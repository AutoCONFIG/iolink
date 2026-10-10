package gbcatalogpg_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"git.hyhy.fun/rsplab/iolink/internal/gbcatalog"
	"git.hyhy.fun/rsplab/iolink/internal/gbcatalogpg"
	"git.hyhy.fun/rsplab/iolink/internal/migrate"
	"git.hyhy.fun/rsplab/iolink/internal/testdb"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

const testDevice = "34020000001320001401"

func fixture(t *testing.T) (*pgxpool.Pool, *gbcatalogpg.Store) {
	t.Helper()
	p := testdb.New(t)
	if err := migrate.Up(t.Context(), p); err != nil {
		t.Fatal(err)
	}
	_, err := p.Exec(t.Context(), `INSERT INTO tenants(id,name) VALUES (1401,'gb-catalog-pg') ON CONFLICT (id) DO NOTHING`)
	if err == nil {
		_, err = p.Exec(t.Context(), `INSERT INTO video_gb_devices(id,tenant_id,device_id,name,credential_cipher) VALUES (1401,1401,$1,'catalog device',decode(repeat('aa',29),'hex'))`, testDevice)
	}
	if err != nil {
		t.Fatal(err)
	}
	return p, gbcatalogpg.New(p)
}

func refresh(deadline time.Time, expected int) gbcatalog.Refresh {
	return gbcatalog.Refresh{DeviceID: testDevice, SN: 1, ExpectedCount: expected, Deadline: deadline}
}

func refreshSN(sn int, deadline time.Time, expected int) gbcatalog.Refresh {
	return gbcatalog.Refresh{DeviceID: testDevice, SN: sn, ExpectedCount: expected, Deadline: deadline}
}

func item(id, name string, online bool) gbcatalog.Item {
	return gbcatalog.Item{ChannelID: id, Name: name, Online: online}
}

func TestStartRefreshIsIdempotentAndSerializesSamePeer(t *testing.T) {
	p, store := fixture(t)
	deadline := time.Now().Add(10 * time.Second).Truncate(time.Microsecond)
	const workers = 8
	results := make(chan gbcatalog.Refresh, workers)
	errs := make(chan error, workers)
	var wg sync.WaitGroup
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			got, err := store.StartRefresh(context.Background(), refresh(deadline, 1))
			results <- got
			errs <- err
		}()
	}
	wg.Wait()
	close(results)
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	for got := range results {
		if got.DeviceID != testDevice || got.SN != 1 || got.ExpectedCount != 1 || !got.Deadline.Equal(deadline) {
			t.Fatalf("unexpected refresh: %+v", got)
		}
	}
	var count int
	if err := p.QueryRow(t.Context(), `SELECT count(*) FROM video_gb_catalogs WHERE device_id=1401 AND sn=1`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("catalog rows=%d", count)
	}
}

func TestReplaceCatalogAtomicallyReplacesDirectory(t *testing.T) {
	p, store := fixture(t)
	deadline := time.Now().Add(10 * time.Second).Truncate(time.Microsecond)
	if _, err := store.StartRefresh(t.Context(), refresh(deadline, 2)); err != nil {
		t.Fatal(err)
	}
	snapshot := gbcatalog.Snapshot{DeviceID: testDevice, SN: 1, ObservedAt: time.Now(), Items: []gbcatalog.Item{
		item("34020000001310001401", "one", true),
		item("34020000001310001402", "two", false),
	}}
	if err := store.ReplaceCatalog(t.Context(), snapshot); err != nil {
		t.Fatal(err)
	}
	var state string
	if err := p.QueryRow(t.Context(), `SELECT catalog_state FROM video_gb_devices WHERE id=1401`).Scan(&state); err != nil {
		t.Fatal(err)
	}
	if state != "ready" {
		t.Fatalf("device catalog state=%q", state)
	}
	if _, err := p.Exec(t.Context(), `INSERT INTO video_gb_channels(device_id,tenant_id,channel_id,name,catalog_sn,present) VALUES (1401,1401,'34020000001310001403','old',1,true)`); err != nil {
		t.Fatal(err)
	}
	if _, err := store.StartRefresh(t.Context(), refreshSN(2, time.Now().Add(10*time.Second).Truncate(time.Microsecond), 1)); err != nil {
		t.Fatal(err)
	}
	if err := store.ReplaceCatalog(t.Context(), gbcatalog.Snapshot{DeviceID: testDevice, SN: 2, ObservedAt: time.Now(), Items: []gbcatalog.Item{item("34020000001310001401", "renamed", true)}}); err != nil {
		t.Fatal(err)
	}
	var present, online bool
	var name string
	if err := p.QueryRow(t.Context(), `SELECT present,online,name FROM video_gb_channels WHERE device_id=1401 AND channel_id='34020000001310001403'`).Scan(&present, &online, &name); err != nil {
		t.Fatal(err)
	}
	if present || online || name != "old" {
		t.Fatalf("old channel=%t/%t/%q", present, online, name)
	}
	if err := p.QueryRow(t.Context(), `SELECT name FROM video_gb_channels WHERE device_id=1401 AND channel_id='34020000001310001401'`).Scan(&name); err != nil {
		t.Fatal(err)
	}
	if name != "renamed" {
		t.Fatalf("new channel name=%q", name)
	}
	if _, err := store.StartRefresh(t.Context(), refresh(time.Now().Add(10*time.Second).Truncate(time.Microsecond), 1)); !errors.Is(err, gbcatalog.ErrInvalid) {
		t.Fatalf("completed refresh reopened: %v", err)
	}
}

func TestExpiredRefreshMarksStaleAndPreservesChannels(t *testing.T) {
	p, store := fixture(t)
	if _, err := p.Exec(t.Context(), `INSERT INTO video_gb_channels(device_id,tenant_id,channel_id,name,catalog_sn,present,online) VALUES (1401,1401,'34020000001310001401','old',1,true,true)`); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Exec(t.Context(), `UPDATE video_gb_devices SET catalog_state='ready' WHERE id=1401`); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(2 * time.Second).Truncate(time.Microsecond)
	if _, err := store.StartRefresh(t.Context(), refresh(deadline, 1)); err != nil {
		t.Fatal(err)
	}
	at := deadline.Add(time.Second)
	if err := store.MarkStale(t.Context(), testDevice, at); err != nil {
		t.Fatal(err)
	}
	var state string
	var present bool
	if err := p.QueryRow(t.Context(), `SELECT d.catalog_state,c.present FROM video_gb_devices d JOIN video_gb_channels c ON c.device_id=d.id WHERE d.id=1401 AND c.channel_id='34020000001310001401'`).Scan(&state, &present); err != nil {
		t.Fatal(err)
	}
	if state != "stale" || !present {
		t.Fatalf("state=%q present=%t", state, present)
	}
}

func TestRegistrationAndCatalogConstraints(t *testing.T) {
	p, store := fixture(t)
	if _, err := p.Exec(t.Context(), `UPDATE video_gb_devices SET registration_state='registered' WHERE id=1401`); !isSQLState(err, "23514") {
		t.Fatalf("registered fields error=%v", err)
	}
	now := time.Now().UTC().Truncate(time.Microsecond)
	if _, err := p.Exec(t.Context(), `UPDATE video_gb_devices SET registration_state='registered',registered_at=$1,registered_until=$2,peer_address='192.0.2.1',peer_port=5060,transport='udp' WHERE id=1401`, now, now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Exec(t.Context(), `INSERT INTO video_gb_devices(id,tenant_id,device_id,name,credential_cipher,registration_state,registered_at,registered_until,peer_address,peer_port,transport) VALUES (1402,1401,'34020000001320001402','other',decode(repeat('aa',29),'hex'),'registered',$1,$2,'192.0.2.1',5060,'udp')`, now, now.Add(time.Minute)); !isSQLState(err, "23505") {
		t.Fatalf("peer conflict error=%v", err)
	}
	if _, err := store.StartRefresh(t.Context(), refresh(time.Now().Add(10*time.Second).Truncate(time.Microsecond), 1)); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Exec(t.Context(), `UPDATE video_gb_catalogs SET received_count=1001 WHERE device_id=1401 AND sn=1`); !isSQLState(err, "23514") {
		t.Fatalf("received count error=%v", err)
	}
	if _, err := p.Exec(t.Context(), `UPDATE video_gb_catalogs SET state='complete' WHERE device_id=1401 AND sn=1`); !isSQLState(err, "23514") {
		t.Fatalf("complete timestamp error=%v", err)
	}
}

func TestMigrationIsIdempotent(t *testing.T) {
	p := testdb.New(t)
	if err := migrate.Up(t.Context(), p); err != nil {
		t.Fatal(err)
	}
	if err := migrate.Up(t.Context(), p); err != nil {
		t.Fatalf("second migration run: %v", err)
	}
}

func isSQLState(err error, want string) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == want
}
