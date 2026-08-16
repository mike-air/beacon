.PHONY: help db-up db-down run worker build tidy vet test test-integration cover docker-build fmt

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
	go test ./...

test-integration: ## Run the full suite incl. integration/e2e (needs TEST_DATABASE_URL)
	@test -n "$$TEST_DATABASE_URL" || { echo "set TEST_DATABASE_URL, e.g. postgres://beacon:beacon@localhost:5432/beacon_test?sslmode=disable"; exit 1; }
	go test ./...

cover: ## Run tests with a coverage profile and print the summary
	go test -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out | tail -1

docker-build: ## Build the production container image
	docker build -t beacon-api:local .
