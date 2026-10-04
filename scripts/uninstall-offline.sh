#!/bin/sh
set -eu
base=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
cd "$base"
docker compose --env-file .env -f deploy/docker-compose.yaml down
