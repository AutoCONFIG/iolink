#!/bin/sh
set -eu
base=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
cd "$base"
test -f .env || { echo 'missing .env' >&2; exit 1; }
docker compose --env-file .env -f deploy/docker-compose.yaml exec -T iolinkd wget --spider --timeout=3 http://127.0.0.1:8080/healthz
docker compose --env-file .env -f deploy/docker-compose.yaml exec -T iolinkd wget --spider --timeout=3 http://127.0.0.1:8080/readyz
echo 'offline health and readiness smoke passed'
