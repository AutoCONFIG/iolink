GO ?= go
PYTHON ?= python3
DOCS_PYTHON ?= .venv/contracts/bin/python
DEV_COMPOSE = docker compose -p iolink-dev -f deploy/docker-compose.yml

.PHONY: bootstrap build test verify verify-contracts docs-tools integration capacity web-verify dev migrate admin-init stop clean embed-front

embed-front:
	@sh scripts/embed-frontend.sh

bootstrap:
	git submodule update --init
	sh scripts/embed-frontend.sh
	$(GO) mod download
	$(GO) build ./...

build: embed-front
	$(GO) build ./...

test: embed-front
	$(GO) test ./... -count=1

verify: build
	$(GO) vet ./...
	$(GO) test -race -shuffle=on ./... -count=1

web-verify:
	npm ci --prefix web
	npm run typecheck --prefix web
	npm run build --prefix web
	npm ci --prefix web-mini
	npm test --prefix web-mini
	npm run typecheck --prefix web-mini
	npm run build --prefix web-mini

# R02.a; kept separate so a Go-only build does not install Python dependencies.
docs-tools:
	$(PYTHON) -m venv .venv/contracts
	$(DOCS_PYTHON) -m pip install -r scripts/requirements-docs.txt

verify-contracts:
	$(DOCS_PYTHON) scripts/check_contracts.py

integration: embed-front
	@test -n "$(IOLINK_TEST_PG_DSN)" || (echo 'Set IOLINK_TEST_PG_DSN to an isolated test instance'; exit 1)
	$(GO) test ./internal/migrate ./internal/core ./internal/notifications ./internal/persistence ./internal/wechat -count=1 -v

capacity:
	$(PYTHON) scripts/capacity_smoke.py --url "$${IOLINK_CAPACITY_URL:-http://127.0.0.1:8080/healthz}" --clients "$${IOLINK_CAPACITY_CLIENTS:-20}" --requests "$${IOLINK_CAPACITY_REQUESTS:-100}"

# Export IOLINK_PG_DSN and a random IOLINK_SECRET_KEY before serving.
# The first start requires: make migrate; make admin-init ADMIN_USERNAME=... < protected-password-file.
dev: embed-front
	$(DEV_COMPOSE) up -d --wait
	$(GO) run ./cmd/iolinkd migrate up
	$(GO) run ./cmd/iolinkd serve

migrate: embed-front
	$(GO) run ./cmd/iolinkd migrate up

admin-init: embed-front
	@test -n "$(ADMIN_USERNAME)" || (echo 'Set ADMIN_USERNAME'; exit 1)
	$(GO) run ./cmd/iolinkd admin init "$(ADMIN_USERNAME)"

stop:
	$(DEV_COMPOSE) down

# Intentionally preserves the database volume; data removal is an explicit operator action.
clean: stop
