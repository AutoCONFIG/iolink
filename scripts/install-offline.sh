#!/bin/sh
set -eu
base=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
cd "$base"
test -f SHA256SUMS || { echo 'missing SHA256SUMS' >&2; exit 1; }
awk 'NF != 2 || $2 !~ /^\.\// || $2 ~ /(^|\/)\.\.(\/|$)/ {bad=1} END {exit bad}' SHA256SUMS || { echo 'invalid checksum paths' >&2; exit 1; }
sha256sum -c SHA256SUMS
test -s metadata/manifest.json
test -s metadata/sbom.spdx.json
test -s metadata/go-dependencies.txt
test -s metadata/licenses.txt
test -d migrations/sql
grep -Eq '"architecture"[[:space:]]*:[[:space:]]*"linux/amd64"' metadata/manifest.json
grep -Eq '"application_image_id"[[:space:]]*:[[:space:]]*"sha256:' metadata/manifest.json
grep -Eq '"database_image_id"[[:space:]]*:[[:space:]]*"sha256:' metadata/manifest.json
grep -Eq '"spdxVersion"[[:space:]]*:[[:space:]]*"SPDX-2.3"' metadata/sbom.spdx.json
docker load <images/iolinkd.tar.gz
docker load <images/db.tar.gz
umask 077
. ./images/refs.env
test "$(docker image inspect "$IOLINKD_IMAGE" --format '{{.Id}}')" = "$(cat images/iolinkd.id)"
test "$(docker image inspect "$IOLINK_DB_IMAGE" --format '{{.Id}}')" = "$(cat images/db.id)"
if test ! -f .env; then
  printf 'IOLINKD_IMAGE=%s\nIOLINK_DB_IMAGE=%s\nIOLINK_PG_PASSWORD=%s\nIOLINK_SECRET_KEY=%s\n' "$IOLINKD_IMAGE" "$IOLINK_DB_IMAGE" "$(od -An -N24 -tx1 /dev/urandom | tr -d ' \n')" "$(od -An -N48 -tx1 /dev/urandom | tr -d ' \n')" >.env
fi
docker compose --env-file .env -f deploy/docker-compose.yaml up -d --wait db
docker compose --env-file .env -f deploy/docker-compose.yaml run --rm --pull never --no-deps iolinkd migrate up
if test -z "${IOLINK_SETUP_INPUT:-}" || test ! -f "$IOLINK_SETUP_INPUT"; then
  echo 'set IOLINK_SETUP_INPUT to a protected setup JSON file; installation stops before serving' >&2
  exit 2
fi
umask 077
docker compose --env-file .env -f deploy/docker-compose.yaml run --rm --pull never --no-deps -T iolinkd setup init <"$IOLINK_SETUP_INPUT"
docker compose --env-file .env -f deploy/docker-compose.yaml run --rm --pull never --no-deps iolinkd setup status
if test -z "${IOLINK_LICENSE_INPUT:-}" || test ! -f "$IOLINK_LICENSE_INPUT"; then
  echo 'set IOLINK_LICENSE_INPUT to a protected License envelope; installation stops before serving' >&2
  exit 3
fi
docker compose --env-file .env -f deploy/docker-compose.yaml run --rm --pull never --no-deps -T iolinkd license import <"$IOLINK_LICENSE_INPUT"
docker compose --env-file .env -f deploy/docker-compose.yaml up -d --pull never --wait iolinkd
./smoke.sh
