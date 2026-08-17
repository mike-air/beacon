.PHONY: help db-up db-down run worker build tidy vet test test-integration cover docker-build fmt sqlc spec

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-12s %s\n", $$1, $$2}'

db-up: ## Start Postgres (and future deps) in Docker
	docker compose -f deploy/docker-compose.yml up -d

db-down: ## Stop the local dependencies
	docker compose -f deploy/docker-compose.yml down

run: ## Run the API (migrates on boot; runs an in-process worker + cron)
	go run ./cmd/beacon-api

worker: ## Run only the background worker + cron (no HTTP server)
	go run ./cmd/beacon-worker

build: ## Compile both binaries into ./bin
	go build -o bin/beacon-api ./cmd/beacon-api
	go build -o bin/beacon-worker ./cmd/beacon-worker

tidy: ## Resolve and lock dependencies
	go mod tidy

vet: ## Static checks
	go vet ./...

fmt: ## Format all Go files
	gofmt -w .

test: ## Run unit tests (integration/e2e tests auto-skip without TEST_DATABASE_URL)
	@go test ./...
	@# Say out loud what was skipped. Without TEST_DATABASE_URL every
	@# integration and e2e test skips itself and the run still prints "ok" for
	@# every package — which is how three real bugs (empty response bodies, a
	@# lost error code, and a dead Idempotency-Key) sat behind a green suite.
	@# A skip you cannot see is indistinguishable from a pass.
	@if [ -z "$$TEST_DATABASE_URL" ]; then \
		echo ""; \
		echo "  NOTE: TEST_DATABASE_URL is unset, so every integration and e2e"; \
		echo "        test SKIPPED. The ok lines above cover unit tests only."; \
		echo "        For the real thing:  make db-up && make test-integration"; \
		echo ""; \
	fi

test-integration: ## Run the full suite incl. integration/e2e (needs TEST_DATABASE_URL)
	@test -n "$$TEST_DATABASE_URL" || { echo "set TEST_DATABASE_URL, e.g. postgres://beacon:beacon@localhost:5432/beacon_test?sslmode=disable"; exit 1; }
	go test ./...

cover: ## Run tests with a coverage profile and print the summary
	go test -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out | tail -1

docker-build: ## Build the production container image
	docker build -t beacon-api:local .

sqlc: ## Regenerate internal/db from migrations/ + internal/db/queries/*.sql
	sqlc generate

spec: ## Emit openapi.json from the registered huma operations
	go run ./cmd/beacon-spec openapi.json
