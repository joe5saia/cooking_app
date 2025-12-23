.PHONY: backend-fmt backend-lint backend-test backend-vet backend-tidy backend-ci backend-tools frontend-install frontend-lint frontend-format frontend-format-check frontend-build frontend-ci frontend-visual frontend-e2e ci
.PHONY: backend-run-api
.PHONY: dev-up dev-down dev-logs dev-psql dev-bootstrap-user

BACKEND_DIR ?= backend
FRONTEND_DIR ?= frontend
DEV_ENV_FILE ?= .env.dev
DEV_COMPOSE ?= docker compose --env-file $(DEV_ENV_FILE)
# Default dev login credentials for local compose bootstrap.
DEV_BOOTSTRAP_USERNAME ?= admin
DEV_BOOTSTRAP_PASSWORD ?= sybil
DEV_BOOTSTRAP_ATTEMPTS ?= 20
DEV_BOOTSTRAP_SLEEP_SECONDS ?= 1
DEV_BOOTSTRAP_GO_BIN ?= /usr/local/go/bin/go

backend-fmt:
	@$(MAKE) -C $(BACKEND_DIR) fmt

backend-lint:
	@$(MAKE) -C $(BACKEND_DIR) lint

backend-test:
	@$(MAKE) -C $(BACKEND_DIR) test

backend-vet:
	@$(MAKE) -C $(BACKEND_DIR) vet

backend-tidy:
	@$(MAKE) -C $(BACKEND_DIR) tidy

backend-ci:
	@$(MAKE) -C $(BACKEND_DIR) ci

backend-tools:
	@$(MAKE) -C $(BACKEND_DIR) tools

backend-run-api:
	@$(MAKE) -C $(BACKEND_DIR) run-api

frontend-install:
	@cd $(FRONTEND_DIR) && if [ -n "$$CI" ]; then npm ci; else npm install; fi

frontend-lint:
	@cd $(FRONTEND_DIR) && npm run lint

frontend-format:
	@cd $(FRONTEND_DIR) && npm run format

frontend-format-check:
	@cd $(FRONTEND_DIR) && npm run format:check

frontend-build:
	@cd $(FRONTEND_DIR) && npm run build

frontend-ci: frontend-install frontend-format-check frontend-lint frontend-build
	@cd $(FRONTEND_DIR) && npm run test

frontend-visual: frontend-install
	@cd $(FRONTEND_DIR) && npm run e2e

frontend-e2e: frontend-install
	@cd $(FRONTEND_DIR) && npm run e2e

ci: backend-ci frontend-ci

dev-up:
	@$(DEV_COMPOSE) up -d --build
	@$(MAKE) dev-bootstrap-user

dev-down:
	@$(DEV_COMPOSE) down -v

dev-logs:
	@$(DEV_COMPOSE) logs -f --tail=200

dev-psql:
	@$(DEV_COMPOSE) exec -it db psql -U $${POSTGRES_USER:-app} -d $${POSTGRES_DB:-app}

dev-bootstrap-user:
	@$(DEV_COMPOSE) exec -T backend sh -lc '\
		username="$(DEV_BOOTSTRAP_USERNAME)"; \
		password="$(DEV_BOOTSTRAP_PASSWORD)"; \
		attempts="$(DEV_BOOTSTRAP_ATTEMPTS)"; \
		sleep_seconds="$(DEV_BOOTSTRAP_SLEEP_SECONDS)"; \
		go_bin="$(DEV_BOOTSTRAP_GO_BIN)"; \
		output=""; \
		i=1; \
		while [ "$$i" -le "$$attempts" ]; do \
			if output=$$($$go_bin run ./cmd/cli bootstrap-user --username "$$username" --password "$$password" 2>&1); then \
				echo "$$output"; \
				exit 0; \
			fi; \
			echo "$$output" | grep -q "bootstrap refused: users table is not empty" && exit 0; \
			i=$$((i + 1)); \
			sleep "$$sleep_seconds"; \
		done; \
		echo "$$output"; \
		exit 1; \
	'
