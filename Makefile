.PHONY: backend-fmt backend-lint backend-test backend-vet backend-tidy backend-ci backend-tools frontend-install frontend-lint frontend-format frontend-format-check frontend-build frontend-ci frontend-visual ci
.PHONY: backend-run-api
.PHONY: dev-up dev-down dev-logs dev-psql

BACKEND_DIR ?= backend
FRONTEND_DIR ?= frontend

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

ci: backend-ci frontend-ci

dev-up:
	@docker compose up -d --build

dev-down:
	@docker compose down -v

dev-logs:
	@docker compose logs -f --tail=200

dev-psql:
	@docker compose exec -it db psql -U $${POSTGRES_USER:-app} -d $${POSTGRES_DB:-app}
