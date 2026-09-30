# Developer entry points. Every target works against a local PostgreSQL reachable
# at DATABASE_URL; nothing here is required by the application itself.

-include .env
export

DATABASE_URL ?= postgres://payment_links:payment_links@localhost:5432/payment_links?sslmode=disable
MIGRATE_IMAGE ?= migrate/migrate:v4.17.1
PSQL_IMAGE ?= postgres:17-alpine
DB_CONTAINER ?= payment-links-db
DOCKER_NET ?= payment-links-net
# Same database seen from inside the Docker network, for the migrate container.
INTERNAL_DATABASE_URL ?= postgres://payment_links:payment_links@$(DB_CONTAINER):5432/payment_links?sslmode=disable

.PHONY: help run build test test-race test-local-db test-unit vet fmt tidy db-up db-down migrate-up migrate-down seed reset

help: ## List the available targets
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-14s\033[0m %s\n", $$1, $$2}'

run: ## Start the API
	go run ./cmd/api

build: ## Compile the API binary into bin/api
	go build -ldflags "-X main.version=$$(git rev-parse --short HEAD 2>/dev/null || echo dev)" -o bin/api ./cmd/api

# One command for the whole suite. The integration tests get their database from
# TestMain: TEST_DATABASE_URL when it is set, otherwise a throwaway container started
# with testcontainers-go. With neither Docker nor that variable they skip with an
# explanation, so the unit suite still runs on a machine without Docker.
test: ## Run every test (starts a throwaway PostgreSQL if Docker is available)
	go test ./...

# -race needs cgo, which needs a C toolchain. That is not a given on Windows, so it
# is a separate target instead of the default.
test-race: ## Run every test with the race detector (requires cgo)
	CGO_ENABLED=1 go test ./... -race

# Reuses a database that is already up instead of starting a container, which is
# faster in a tight edit-test loop.
# WARNING: the integration tests TRUNCATE businesses, payment_references and
# webhook_events. Point this at a throwaway database, never at one with data.
test-local-db: ## Run every test against the database at DATABASE_URL (wipes it)
	TEST_DATABASE_URL="$(DATABASE_URL)" go test ./... -count=1

test-unit: ## Run only the tests that need no database
	go test ./internal/domain/... ./internal/application/... ./internal/transport/... ./internal/arch/...

vet: ## Run go vet
	go vet ./...

fmt: ## Format the code
	gofmt -l -w .

tidy: ## Sync go.mod and go.sum
	go mod tidy

db-up: ## Start a local PostgreSQL in Docker
	docker network create $(DOCKER_NET) 2>/dev/null || true
	docker run -d --name $(DB_CONTAINER) --network $(DOCKER_NET) 		-e POSTGRES_USER=payment_links 		-e POSTGRES_PASSWORD=payment_links 		-e POSTGRES_DB=payment_links 		-p 5432:5432 $(PSQL_IMAGE)
	@until docker exec $(DB_CONTAINER) pg_isready -U payment_links -d payment_links >/dev/null 2>&1; do sleep 1; done
	@echo "postgres ready on localhost:5432"

db-down: ## Remove the local PostgreSQL container
	docker rm -f $(DB_CONTAINER)

# The migration and seed containers reach PostgreSQL over the Docker network by
# container name. Going through localhost would only work on Linux, where
# --network host is a real host network; on Docker Desktop it is not.
migrate-up: ## Apply all pending migrations
	docker run --rm --network $(DOCKER_NET) -v "$(CURDIR)/migrations:/migrations" 		$(MIGRATE_IMAGE) -path=/migrations -database "$(INTERNAL_DATABASE_URL)" up

migrate-down: ## Roll back the last migration
	docker run --rm --network $(DOCKER_NET) -v "$(CURDIR)/migrations:/migrations" 		$(MIGRATE_IMAGE) -path=/migrations -database "$(INTERNAL_DATABASE_URL)" down 1

seed: ## Load the demo businesses (idempotent)
	docker cp seeds/001_businesses.sql $(DB_CONTAINER):/tmp/001_businesses.sql
	docker exec $(DB_CONTAINER) psql -U payment_links -d payment_links -v ON_ERROR_STOP=1 -f /tmp/001_businesses.sql

reset: db-down db-up migrate-up seed ## Recreate the database from scratch, migrate and seed
