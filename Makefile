# Beacon — one entry point for the whole system.
#
# Everything you need to run, test or regenerate this project is a target
# here. If a command is worth typing twice, it belongs in this file, because
# a README that lists commands goes stale and a Makefile that runs them does
# not.
#
# Run `make` with no arguments to see what is available.

.DEFAULT_GOAL := help
SHELL := /bin/bash

# ---------------------------------------------------------------------------
# Running
# ---------------------------------------------------------------------------

.PHONY: up
up: ## Start Postgres, Redis, Meilisearch, MinIO, Prometheus (docker)
	$(MAKE) -C api db-up

.PHONY: down
down: ## Stop the local dependencies
	$(MAKE) -C api db-down

.PHONY: api
api: ## Run the Go API on :8080 (migrates on boot, runs worker + cron)
	$(MAKE) -C api run

.PHONY: web
web: ## Run the Vite dev server on :5180
	cd web && npm run dev

.PHONY: dev
dev: ## Print how to run both halves (they need two terminals)
	@echo "Beacon needs two processes. In separate terminals:"
	@echo "  make up && make api     # :8080, plus its dependencies"
	@echo "  make web                # :5180"

# ---------------------------------------------------------------------------
# The contract
#
# This is the part that makes the monorepo worth having. The OpenAPI document
# is EMITTED from the Go handlers, and the TypeScript SDK is generated from
# that document. Neither is hand-written, so neither can drift from the server.
# ---------------------------------------------------------------------------

.PHONY: sqlc
sqlc: ## Regenerate internal/db from the migrations and the query files
	$(MAKE) -C api sqlc

.PHONY: spec
spec: ## Emit openapi.json from the Go handlers
	$(MAKE) -C api spec

.PHONY: sdk
sdk: spec ## Regenerate the TypeScript SDK from the emitted spec
	cd sdk && npm run generate

.PHONY: contract
contract: sdk ## Regenerate the contract and fail if it differs from what is committed
	@# Only the GENERATED paths are checked. sdk/ also holds hand-written
	@# behaviour — client.ts, errors.ts, index.ts — which nothing regenerates,
	@# so watching the whole directory reported an ordinary uncommitted edit to
	@# client.ts as "the contract drifted". That is a false alarm that teaches
	@# people to ignore a real one.
	@if [ -n "$$(git status --porcelain sdk/src/generated api/openapi.json)" ]; then \
		echo ""; \
		echo "The generated contract differs from what is committed."; \
		echo "A handler changed and the SDK was not regenerated. Run:"; \
		echo "    make sdk && git add sdk/src/generated api/openapi.json"; \
		echo ""; \
		git --no-pager diff --stat sdk/src/generated api/openapi.json; \
		exit 1; \
	fi
	@echo "contract is in step with the handlers"

# ---------------------------------------------------------------------------
# Checking
# ---------------------------------------------------------------------------

.PHONY: test
test: test-api test-web ## Run every test that does not need a browser

.PHONY: test-api
test-api: ## Go unit tests (integration tests skip without TEST_DATABASE_URL)
	$(MAKE) -C api test

.PHONY: test-web
test-web: ## Vitest unit and component tests
	cd web && npm run test

.PHONY: e2e
e2e: ## Playwright against a REAL Beacon — needs `make up && make api` first
	cd web && npm run e2e

.PHONY: visual
visual: ## Visual regression: the built image vs the committed screenshots
	./web/deploy/visual.sh

.PHONY: visual-update
visual-update: ## Re-record the visual baselines after an intended change
	./web/deploy/visual.sh --update

.PHONY: images
images: ## Build both production container images
	$(MAKE) -C api docker-build
	docker build -f web/Dockerfile -t beacon-web:local .

.PHONY: lint
lint: ## Vet the Go, lint and typecheck the TypeScript
	$(MAKE) -C api vet
	cd web && npm run lint && npm run typecheck

.PHONY: build
build: ## Build both halves, enforcing the frontend performance budget
	$(MAKE) -C api build
	cd web && npm run build

.PHONY: ci
ci: lint contract test build ## Everything CI runs, in the same order

# ---------------------------------------------------------------------------

.PHONY: help
help: ## Show this help
	@echo "Beacon — a multi-tenant task board, and its client."
	@echo ""
	@grep -hE '^[a-z0-9-]+:.*?## ' $(MAKEFILE_LIST) \
	  | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-12s\033[0m %s\n", $$1, $$2}'
	@echo ""
	@echo "New here? Read ARCHITECTURE.md, then api/READING.md."
