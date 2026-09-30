# Payment Links — entry points for the whole stack.
#
# Needs a POSIX shell. On Windows use Git Bash or WSL; the README lists the raw
# commands for anyone without make.

.ONESHELL:
.PHONY: help up down restart logs ps test test-backend test-frontend build webhook-demo psql clean

API_URL ?= http://localhost:8080
WEB_URL ?= http://localhost:3000
COMPOSE ?= docker compose

help: ## List the available targets
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-16s\033[0m %s\n", $$1, $$2}'

up: ## Build and start the whole stack (database, migrations, seed, API, dashboard)
	$(COMPOSE) up --build -d
	@echo
	@echo "Dashboard: $(WEB_URL)"
	@echo "API:       $(API_URL)/health"

down: ## Stop everything and remove the containers (the database volume survives)
	$(COMPOSE) down

clean: ## Stop everything and delete the database volume as well
	$(COMPOSE) down -v

restart: down up ## Recreate the stack

logs: ## Follow the API and dashboard logs
	$(COMPOSE) logs -f api web

ps: ## Show the state of each service
	$(COMPOSE) ps

build: ## Build the images without starting anything
	$(COMPOSE) build

test: test-backend test-frontend ## Run every test

test-backend: ## Backend test suite (starts a throwaway PostgreSQL by itself)
	cd backend && go test ./...

test-frontend: ## Frontend type check
	cd frontend && npm run typecheck

psql: ## Open a psql shell on the running database
	$(COMPOSE) exec db psql -U payment_links -d payment_links

## --------------------------------------------------------------------------------
## The interesting demo lives in a script rather than in a recipe, so it also runs
## where make is not installed (Windows without Git Bash tooling, a bare container).
## --------------------------------------------------------------------------------
webhook-demo: ## Simulate a provider payment, then redeliver the same event_id
	@sh ./scripts/webhook-demo.sh
