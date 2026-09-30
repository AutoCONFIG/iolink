#!/usr/bin/env bash
set -euo pipefail

: "${BACKUP_FILE:?BACKUP_FILE is required}"
: "${PGHOST:?PGHOST is required}"
: "${PGUSER:?PGUSER is required}"
: "${PGDATABASE:?PGDATABASE is required}"
: "${PGPASSWORD:?PGPASSWORD is required}"
: "${IOLINK_RESTORE_TARGET:?IOLINK_RESTORE_TARGET=isolated is required}"

if [[ "$IOLINK_RESTORE_TARGET" != isolated || "$PGDATABASE" != iolink_restore_* ]]; then
  printf '%s\n' 'restore target must be explicitly isolated and named iolink_restore_*' >&2
  exit 2
fi

if [[ ! -r "$BACKUP_FILE" ]]; then
  printf '%s\n' 'backup file is not readable' >&2
  exit 2
fi
if [[ -r "$BACKUP_FILE.sha256" ]]; then
  expected=$(awk 'NF {print $1; exit}' "$BACKUP_FILE.sha256")
  actual=$(sha256sum "$BACKUP_FILE" | awk '{print $1}')
  [[ -n "$expected" && "$actual" == "$expected" ]] || { printf '%s\n' 'backup checksum mismatch' >&2; exit 2; }
else
  printf '%s\n' 'backup checksum sidecar is required' >&2
  exit 2
fi
if [[ ! -r "$BACKUP_FILE.summary" ]]; then
  printf '%s\n' 'backup summary sidecar is required' >&2
  exit 2
fi
if [[ ! -r "$BACKUP_FILE.meta" ]]; then
  printf '%s\n' 'backup metadata sidecar is required' >&2
  exit 2
fi
expected_migrations=$(awk -F= '$1 == "migration_rows" {print $2; exit}' "$BACKUP_FILE.meta")
[[ "$expected_migrations" =~ ^[0-9]+$ ]] || { printf '%s\n' 'backup migration metadata is invalid' >&2; exit 2; }
expected_server=$(awk -F= '$1 == "server_version" {print $2; exit}' "$BACKUP_FILE.meta")
expected_timescaledb=$(awk -F= '$1 == "timescaledb_version" {print $2; exit}' "$BACKUP_FILE.meta")
[[ -n "$expected_server" && -n "$expected_timescaledb" ]] || { printf '%s\n' 'backup provider version metadata is invalid' >&2; exit 2; }
actual_server=$(psql --set=ON_ERROR_STOP=1 --tuples-only --no-align --dbname="$PGDATABASE" --command='SELECT current_setting('"'"'server_version'"'"');')
actual_timescaledb=$(psql --set=ON_ERROR_STOP=1 --tuples-only --no-align --dbname="$PGDATABASE" --command="SELECT extversion FROM pg_extension WHERE extname='timescaledb';")
[[ "$actual_server" == "$expected_server" ]] || { printf 'PostgreSQL version differs: expected %s, got %s\n' "$expected_server" "$actual_server" >&2; exit 1; }
[[ "$actual_timescaledb" == "$expected_timescaledb" ]] || { printf 'TimescaleDB version differs: expected %s, got %s\n' "$expected_timescaledb" "$actual_timescaledb" >&2; exit 1; }
target_has_migrations=$(psql --set=ON_ERROR_STOP=1 --tuples-only --no-align --dbname="$PGDATABASE" --command="SELECT to_regclass('public.schema_migrations') IS NOT NULL;")
if [[ "$target_has_migrations" == t ]]; then
  target_migrations=$(psql --set=ON_ERROR_STOP=1 --tuples-only --no-align --dbname="$PGDATABASE" --command='SELECT count(*) FROM schema_migrations;')
else
  target_migrations=-1
fi
[[ "$target_migrations" =~ ^-?[0-9]+$ ]] || { printf '%s\n' 'target migration metadata is invalid' >&2; exit 2; }
if [[ "$target_migrations" != -1 && "$target_migrations" != 0 && "$target_migrations" != "$expected_migrations" ]]; then
  printf 'target migration count differs before restore: expected %s or empty target, got %s\n' "$expected_migrations" "$target_migrations" >&2
  exit 1
fi
psql --set=ON_ERROR_STOP=1 --dbname="$PGDATABASE" --command='SELECT timescaledb_pre_restore();'
pg_restore --clean --if-exists --exit-on-error --no-owner --no-acl --dbname="$PGDATABASE" "$BACKUP_FILE"
psql --set=ON_ERROR_STOP=1 --dbname="$PGDATABASE" --command='SELECT timescaledb_post_restore();'
actual_migrations=$(psql --set=ON_ERROR_STOP=1 --tuples-only --no-align --dbname="$PGDATABASE" --command='SELECT count(*) FROM schema_migrations;')
[[ "$actual_migrations" == "$expected_migrations" ]] || { printf 'schema migration count differs: expected %s, got %s\n' "$expected_migrations" "$actual_migrations" >&2; exit 1; }
actual=$(mktemp)
trap 'rm -f "$actual"' EXIT
psql --set=ON_ERROR_STOP=1 --tuples-only --no-align --field-separator='|' --command="SELECT 'farms',count(*),coalesce(min(created_at)::text,''),coalesce(max(created_at)::text,'') FROM farms UNION ALL SELECT 'ponds',count(*),coalesce(min(created_at)::text,''),coalesce(max(created_at)::text,'') FROM ponds UNION ALL SELECT 'devices',count(*),coalesce(min(created_at)::text,''),coalesce(max(created_at)::text,'') FROM devices UNION ALL SELECT 'sensor_data',count(*),coalesce(min(ts)::text,''),coalesce(max(ts)::text,'') FROM sensor_data UNION ALL SELECT 'device_shadows',count(*),coalesce(min(ts)::text,''),coalesce(max(ts)::text,'') FROM device_shadows UNION ALL SELECT 'alarms',count(*),coalesce(min(created_at)::text,''),coalesce(max(created_at)::text,'') FROM alarms UNION ALL SELECT 'indexes',count(*),'','' FROM pg_indexes WHERE schemaname='public' UNION ALL SELECT 'retention_jobs',count(*),'','' FROM timescaledb_information.jobs WHERE hypertable_name='sensor_data' AND proc_name='policy_retention' ORDER BY 1;" "$PGDATABASE" > "$actual"
if ! cmp -s "$BACKUP_FILE.summary" "$actual"; then
  printf '%s\n' 'restored database summary differs from backup' >&2
  diff -u "$BACKUP_FILE.summary" "$actual" >&2 || true
  exit 1
fi
printf '%s\n' 'restore verified'
