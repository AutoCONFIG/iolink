#!/usr/bin/env bash
set -euo pipefail

: "${PGHOST:?PGHOST is required}"
: "${PGUSER:?PGUSER is required}"
: "${PGDATABASE:?PGDATABASE is required}"
: "${PGPASSWORD:?PGPASSWORD is required}"
BACKUP_DIR=${BACKUP_DIR:-/var/backups/iolink}
KEEP_DAYS=${KEEP_DAYS:-14}
STAMP=$(date -u +%Y%m%dT%H%M%SZ)
mkdir -p "$BACKUP_DIR"
umask 077
backup="$BACKUP_DIR/iolink-$STAMP.dump"

pg_dump --format=custom --no-owner --no-acl --file="$backup" "$PGDATABASE"
(cd "$(dirname "$backup")" && sha256sum "$(basename "$backup")") > "$backup.sha256"
printf 'server_version=' > "$backup.meta"
psql --set=ON_ERROR_STOP=1 --tuples-only --no-align --command='SELECT current_setting('"'"'server_version'"'"');' "$PGDATABASE" >> "$backup.meta"
printf 'timescaledb_version=' >> "$backup.meta"
psql --set=ON_ERROR_STOP=1 --tuples-only --no-align --command="SELECT extversion FROM pg_extension WHERE extname='timescaledb';" "$PGDATABASE" >> "$backup.meta"
printf 'migration_rows=' >> "$backup.meta"
psql --set=ON_ERROR_STOP=1 --tuples-only --no-align --command='SELECT count(*) FROM schema_migrations;' "$PGDATABASE" >> "$backup.meta"
psql --set=ON_ERROR_STOP=1 --tuples-only --no-align --field-separator='|' --command="SELECT 'farms',count(*),coalesce(min(created_at)::text,''),coalesce(max(created_at)::text,'') FROM farms UNION ALL SELECT 'ponds',count(*),coalesce(min(created_at)::text,''),coalesce(max(created_at)::text,'') FROM ponds UNION ALL SELECT 'devices',count(*),coalesce(min(created_at)::text,''),coalesce(max(created_at)::text,'') FROM devices UNION ALL SELECT 'sensor_data',count(*),coalesce(min(ts)::text,''),coalesce(max(ts)::text,'') FROM sensor_data UNION ALL SELECT 'device_shadows',count(*),coalesce(min(ts)::text,''),coalesce(max(ts)::text,'') FROM device_shadows UNION ALL SELECT 'alarms',count(*),coalesce(min(created_at)::text,''),coalesce(max(created_at)::text,'') FROM alarms UNION ALL SELECT 'indexes',count(*),'','' FROM pg_indexes WHERE schemaname='public' UNION ALL SELECT 'retention_jobs',count(*),'','' FROM timescaledb_information.jobs WHERE hypertable_name='sensor_data' AND proc_name='policy_retention' ORDER BY 1;" "$PGDATABASE" > "$backup.summary"
find "$BACKUP_DIR" -type f -name 'iolink-*.dump' -mtime "+$KEEP_DAYS" -delete
find "$BACKUP_DIR" -type f -name 'iolink-*.dump.sha256' -mtime "+$KEEP_DAYS" -delete
find "$BACKUP_DIR" -type f -name 'iolink-*.dump.meta' -mtime "+$KEEP_DAYS" -delete
find "$BACKUP_DIR" -type f -name 'iolink-*.dump.summary' -mtime "+$KEEP_DAYS" -delete
printf '%s\n' "$backup"
