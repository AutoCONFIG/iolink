#!/bin/sh
set -eu
base=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
cd "$base"
test -f SHA256SUMS || { echo 'missing SHA256SUMS' >&2; exit 1; }
awk 'NF != 2 || $2 !~ /^\.\// || $2 ~ /(^|\/)\.\.(\/|$)/ {bad=1} END {exit bad}' SHA256SUMS || { echo 'invalid checksum paths' >&2; exit 1; }
sha256sum -c SHA256SUMS
docker load <images/iolinkd.tar.gz
docker load <images/db.tar.gz
umask 077
if test ! -f .env; then
  . ./images/refs.env
  printf 'IOLINKD_IMAGE=%s\nIOLINK_DB_IMAGE=%s\nIOLINK_PG_PASSWORD=%s\nIOLINK_SECRET_KEY=%s\n' "$IOLINKD_IMAGE" "$IOLINK_DB_IMAGE" "$(od -An -N24 -tx1 /dev/urandom | tr -d ' \n')" "$(od -An -N48 -tx1 /dev/urandom | tr -d ' \n')" >.env
fi
docker compose --env-file .env -f deploy/docker-compose.yaml up -d --wait
