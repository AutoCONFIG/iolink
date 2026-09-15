GIT_BASE := https://git.hyhy.fun/rsplab

.PHONY: bootstrap dev test verify clean

bootstrap: ## clone+init submodules, resolve go deps
	git submodule update --init --recursive
	go mod tidy
	cd contracts && go build ./...

dev: ## run db + iolinkd locally
	docker compose -f deploy/docker-compose.yml up -d
	go run ./cmd/iolinkd

test: verify
	go test ./internal/...
	cd $(shell git submodule status | awk '{print $$2}' | grep access | xargs -I{} echo {}) 2>/dev/null; true

verify: ## submodules must be clean and tagged (CI guard)
	@dirty=$$(git submodule status | grep -E '^\+|^U' || true); \
	  if [ -n "$$dirty" ]; then echo "UNCOMMITTED submodule changes:"; echo "$$dirty"; exit 1; fi; \
	  echo submodules OK

clean:
	docker compose -f deploy/docker-compose.yml down -v
