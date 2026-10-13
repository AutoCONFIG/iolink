#!/usr/bin/env bash
set -u
: "${IOLINK_TEST_PG_DSN:?provide the isolated PostgreSQL test DSN}"
export IOLINK_CONTRACT_PYTHON='/media/yun/706bc403-c76c-4fdd-8a3f-d954b6189048/iolink/.venv/contracts/bin/python'
export GIN_MODE=release
go test -race -shuffle=on -count=1 -timeout=3m -v ./internal/core -run '^TestAPIKeyNameContract_countsCharactersAndPreservesAtomicity$'
