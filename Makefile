.PHONY: backend-fmt backend-lint backend-test backend-vet backend-tidy backend-ci backend-tools frontend-install frontend-lint frontend-format frontend-format-check frontend-build frontend-ci ci

BACKEND_DIR ?= backend
FRONTEND_DIR ?= frontend

backend-%:
	@$(MAKE) -C $(BACKEND_DIR) $*

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

ci: backend-ci frontend-ci
