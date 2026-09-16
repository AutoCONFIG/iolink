#!/usr/bin/env bash
# 每日备份业务表 + 归档时序表。crontab 示例:
#   30 2 * * * /opt/iolink/deploy/backup.sh >> /var/log/iolink-backup.log 2>&1
set -euo pipefail
BACKUP_DIR=${BACKUP_DIR:-/var/backups/iolink}
KEEP_DAYS=${KEEP_DAYS:-14}
STAMP=$(date +%Y%m%d-%H%M)
mkdir -p "$BACKUP_DIR"

docker exec deploy-db-1 pg_dump -U iolink -d iolink \
  --exclude-table-data='sensor_data' | gzip > "$BACKUP_DIR/business-$STAMP.sql.gz"

docker exec deploy-db-1 pg_dump -U iolink -d iolink -t sensor_data | gzip > "$BACKUP_DIR/telemetry-$STAMP.sql.gz"

find "$BACKUP_DIR" -name '*.sql.gz' -mtime +$KEEP_DAYS -delete
echo "[$(date)] backup ok: business-$STAMP.sql.gz telemetry-$STAMP.sql.gz"
