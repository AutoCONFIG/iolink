GO ?= go
PYTHON ?= python3
DOCS_PYTHON ?= .venv/contracts/bin/python
DEV_COMPOSE = docker compose -p iolink-dev -f deploy/docker-compose.yml

.PHONY: bootstrap build test verify verify-contracts docs-tools integration dev migrate admin-init stop clean

bootstrap:
	git submodule update --init
	$(GO) mod download
	$(GO) build ./...

build:
	$(GO) build ./...

test:
	$(GO) test ./... -count=1

verify: build
	$(GO) vet ./...
	$(GO) test ./... -count=1

# R02.a; kept separate so a Go-only build does not install Python dependencies.
docs-tools:
	$(PYTHON) -m venv .venv/contracts
	$(DOCS_PYTHON) -m pip install -r scripts/requirements-docs.txt

verify-contracts:
	$(DOCS_PYTHON) scripts/check_contracts.py

integration:
	@test -n "$(IOLINK_TEST_PG_DSN)" || (echo 'Set IOLINK_TEST_PG_DSN to an isolated test instance'; exit 1)
	$(GO) test ./internal/migrate ./internal/core -count=1 -v

# Export IOLINK_PG_DSN and a random IOLINK_SECRET_KEY before serving.
# The first start requires: make migrate; make admin-init ADMIN_USERNAME=... < protected-password-file.
dev:
	$(DEV_COMPOSE) up -d --wait
	$(GO) run ./cmd/iolinkd migrate up
	$(GO) run ./cmd/iolinkd serve

migrate:
	$(GO) run ./cmd/iolinkd migrate up

admin-init:
	@test -n "$(ADMIN_USERNAME)" || (echo 'Set ADMIN_USERNAME'; exit 1)
	$(GO) run ./cmd/iolinkd admin init "$(ADMIN_USERNAME)"

stop:
	$(DEV_COMPOSE) down

# Intentionally preserves the database volume; data removal is an explicit operator action.
clean: stop
