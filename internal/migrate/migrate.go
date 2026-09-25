// Package migrate owns immutable, transactional database migrations.
package migrate

import (
	"context"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed sql/*.sql fingerprint.sql legacy-fingerprint.json
var files embed.FS

const lockID int64 = 0x496f4c696e6b

type migration struct {
	version             int
	name, sql, checksum string
}
type Version struct {
	Version  int    `json:"version"`
	Name     string `json:"name"`
	Checksum string `json:"checksum"`
	Applied  bool   `json:"applied"`
}

func load() ([]migration, error) {
	entries, err := files.ReadDir("sql")
	if err != nil {
		return nil, err
	}
	var out []migration
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		v, err := strconv.Atoi(strings.SplitN(e.Name(), "_", 2)[0])
		if err != nil {
			return nil, err
		}
		raw, err := files.ReadFile("sql/" + e.Name())
		if err != nil {
			return nil, err
		}
		hash := sha256.Sum256(raw)
		out = append(out, migration{v, e.Name(), string(raw), hex.EncodeToString(hash[:])})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].version < out[j].version })
	for i, m := range out {
		if m.version != i+1 {
			return nil, errors.New("migration versions must be contiguous starting at 1")
		}
	}
	return out, nil
}

// Up applies all pending migrations atomically. Existing unmanaged schemas require
// an explicit AdoptLegacy after a backup; startup never silently migrates data.
func Up(ctx context.Context, pool *pgxpool.Pool) error {
	ms, err := load()
	if err != nil {
		return err
	}
	return apply(ctx, pool, ms, false)
}
func AdoptLegacy(ctx context.Context, pool *pgxpool.Pool) error {
	ms, err := load()
	if err != nil {
		return err
	}
	return apply(ctx, pool, ms, true)
}

func apply(ctx context.Context, pool *pgxpool.Pool, ms []migration, adopt bool) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())
	if _, err = tx.Exec(ctx, "SELECT pg_advisory_xact_lock($1)", lockID); err != nil {
		return fmt.Errorf("migration lock: %w", err)
	}
	if _, err = tx.Exec(ctx, "SET LOCAL search_path = public"); err != nil {
		return err
	}
	exists, err := hasHistory(ctx, tx)
	if err != nil {
		return err
	}
	if !exists {
		var count int
		err = tx.QueryRow(ctx, `SELECT count(*) FROM pg_class WHERE relnamespace='public'::regnamespace AND relkind IN ('r','p')`).Scan(&count)
		if err != nil {
			return err
		}
		if adopt {
			if err = validateLegacy(ctx, tx); err != nil {
				return err
			}
		} else if count != 0 {
			return errors.New("unmanaged schema: back up and run migrate adopt-legacy; no changes applied")
		}
		if _, err = tx.Exec(ctx, `CREATE TABLE schema_migrations(version INTEGER PRIMARY KEY,name TEXT NOT NULL,checksum TEXT NOT NULL,applied_at TIMESTAMPTZ NOT NULL DEFAULT now())`); err != nil {
			return err
		}
		if adopt {
			m := ms[0]
			if _, err = tx.Exec(ctx, `INSERT INTO schema_migrations(version,name,checksum) VALUES($1,$2,$3)`, m.version, m.name, m.checksum); err != nil {
				return err
			}
		}
	} else if adopt {
		return errors.New("database already managed; use migrate up")
	}
	applied, err := history(ctx, tx, ms)
	if err != nil {
		return err
	}
	for _, m := range ms[len(applied):] {
		if _, err = tx.Exec(ctx, m.sql); err != nil {
			return fmt.Errorf("migration %s rolled back: %w", m.name, err)
		}
		if _, err = tx.Exec(ctx, `INSERT INTO schema_migrations(version,name,checksum) VALUES($1,$2,$3)`, m.version, m.name, m.checksum); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

type queryer interface {
	QueryRow(context.Context, string, ...any) pgx.Row
	Query(context.Context, string, ...any) (pgx.Rows, error)
}

func hasHistory(ctx context.Context, q queryer) (bool, error) {
	var ok bool
	err := q.QueryRow(ctx, `SELECT to_regclass('public.schema_migrations') IS NOT NULL`).Scan(&ok)
	return ok, err
}
func history(ctx context.Context, q queryer, ms []migration) ([]Version, error) {
	rows, err := q.Query(ctx, `SELECT version,name,checksum FROM public.schema_migrations ORDER BY version`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Version
	for rows.Next() {
		var v Version
		if err = rows.Scan(&v.Version, &v.Name, &v.Checksum); err != nil {
			return nil, err
		}
		i := len(out)
		if i >= len(ms) || v.Version != ms[i].version || v.Name != ms[i].name || v.Checksum != ms[i].checksum {
			return nil, fmt.Errorf("schema history mismatch at version %d (gap, checksum drift or newer database); refuse startup/migration", v.Version)
		}
		v.Applied = true
		out = append(out, v)
	}
	return out, rows.Err()
}
func Status(ctx context.Context, pool *pgxpool.Pool) ([]Version, error) {
	ms, err := load()
	if err != nil {
		return nil, err
	}
	var applied []Version
	ok, err := hasHistory(ctx, pool)
	if err != nil {
		return nil, err
	}
	if ok {
		applied, err = history(ctx, pool, ms)
		if err != nil {
			return nil, err
		}
	}
	out := make([]Version, 0, len(ms))
	out = append(out, applied...)
	for _, m := range ms[len(applied):] {
		out = append(out, Version{m.version, m.name, m.checksum, false})
	}
	return out, nil
}
func CheckLatest(ctx context.Context, pool *pgxpool.Pool) error {
	st, err := Status(ctx, pool)
	if err != nil {
		return err
	}
	for _, v := range st {
		if !v.Applied {
			return fmt.Errorf("database migration %s pending; run iolinkd migrate up before serving", v.Name)
		}
	}
	return nil
}
func validateLegacy(ctx context.Context, tx pgx.Tx) error {
	var tables int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM pg_class WHERE relnamespace='public'::regnamespace AND relkind IN ('r','p')`).Scan(&tables); err != nil {
		return err
	}
	if tables != 8 {
		return errors.New("legacy schema must contain exactly the eight supported business tables; no changes applied")
	}

	raw, _ := files.ReadFile("fingerprint.sql")
	var got string
	if err := tx.QueryRow(ctx, string(raw)).Scan(&got); err != nil {
		return err
	}
	var actual, expected []string
	want, _ := files.ReadFile("legacy-fingerprint.json")
	if err := json.Unmarshal([]byte(got), &actual); err != nil {
		return err
	}
	if err := json.Unmarshal(want, &expected); err != nil {
		return err
	}
	if !reflect.DeepEqual(actual, expected) {
		return errors.New("legacy schema differs from supported baseline (columns, defaults, keys or indexes); no changes applied")
	}
	var ht, retention bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM timescaledb_information.hypertables WHERE hypertable_schema='public' AND hypertable_name='sensor_data'), EXISTS(SELECT 1 FROM timescaledb_information.jobs WHERE hypertable_schema='public' AND hypertable_name='sensor_data' AND proc_name='policy_retention' AND (config->>'drop_after')::interval=interval '13 months')`).Scan(&ht, &retention); err != nil {
		return err
	}
	if !ht || !retention {
		return errors.New("legacy hypertable or 13-month retention missing; no changes applied")
	}
	return nil
}
