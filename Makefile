GIT_BASE := https://git.hyhy.fun/rsplab

.PHONY: bootstrap dev work test verify clean

bootstrap: ## resolve all Go module deps (access/appapi/contracts pulled by version)
	go mod download all
	go build ./...

work: ## optional: local cross-repo dev workspace (expects sibling clones)
	@repos="."; \
	[ -d ../iolink-access ]  && repos="$$repos ../iolink-access"; \
	[ -d ../iolink-appapi ]  && repos="$$repos ../iolink-appapi"; \
	go work init $${repos}; \
	echo "go.work created for: $$repos"

dev: ## run db + iolinkd locally
	docker compose -f deploy/docker-compose.yml up -d
	go run ./cmd/iolinkd

test:
	go vet ./...
	go test ./internal/...
	cd contracts && go test ./...

verify: ## release gate: everything builds/tests without local replaces
	go build ./...
	go vet ./...
	go test ./internal/...
	cd contracts && go test ./...

clean:
	docker compose -f deploy/docker-compose.yml down -v
