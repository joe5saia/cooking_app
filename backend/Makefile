GO ?= go
GOLANGCI_LINT_VERSION ?= v1.64.8
GOIMPORTS_VERSION ?= v0.40.0
GOOSE_VERSION ?= v3.26.0
SQLC_VERSION ?= v1.30.0
BIN_DIR := $(CURDIR)/bin
GOLANGCI_LINT := $(BIN_DIR)/golangci-lint
GOIMPORTS := $(BIN_DIR)/goimports
GOOSE := $(BIN_DIR)/goose
SQLC := $(BIN_DIR)/sqlc
MIGRATIONS_DIR := $(CURDIR)/migrations
GOFILES := $(shell find . -name '*.go' -not -path "./vendor/*" -not -path "./.git/*" -not -path "./bin/*")

.PHONY: fmt lint test vet tidy ci tools clean
.PHONY: run-api
.PHONY: migrate-create migrate-up migrate-down migrate-status migrate-version
.PHONY: sqlc-generate sqlc-version

fmt:
	@$(GO) fmt ./...
	@$(MAKE) tools
	@$(GOIMPORTS) -w $(GOFILES)

lint:
	@$(MAKE) tools
	@$(GOLANGCI_LINT) run ./...

test:
	@$(GO) test ./...

vet:
	@$(GO) vet ./...

tidy:
	@$(GO) mod tidy

tools:
	@mkdir -p $(BIN_DIR)
	@GOBIN=$(BIN_DIR) $(GO) install golang.org/x/tools/cmd/goimports@$(GOIMPORTS_VERSION)
	@GOBIN=$(BIN_DIR) $(GO) install github.com/golangci/golangci-lint/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)
	@GOBIN=$(BIN_DIR) $(GO) install github.com/pressly/goose/v3/cmd/goose@$(GOOSE_VERSION)
	@GOBIN=$(BIN_DIR) $(GO) install github.com/sqlc-dev/sqlc/cmd/sqlc@$(SQLC_VERSION)

ci: fmt vet lint test

run-api:
	@DATABASE_URL=$${DATABASE_URL:?DATABASE_URL is required} HTTP_ADDR=$${HTTP_ADDR:-:8080} LOG_LEVEL=$${LOG_LEVEL:-info} $(GO) run ./cmd/api

migrate-version:
	@$(MAKE) tools
	@$(GOOSE) -version

migrate-create:
	@$(MAKE) tools
	@name=$${name:?name is required (example: make -C backend migrate-create name=create_users)}; $(GOOSE) -dir $(MIGRATIONS_DIR) create $$name sql

migrate-up:
	@$(MAKE) tools
	@DATABASE_URL=$${DATABASE_URL:?DATABASE_URL is required}; $(GOOSE) -dir $(MIGRATIONS_DIR) postgres "$$DATABASE_URL" up

migrate-down:
	@$(MAKE) tools
	@DATABASE_URL=$${DATABASE_URL:?DATABASE_URL is required}; $(GOOSE) -dir $(MIGRATIONS_DIR) postgres "$$DATABASE_URL" down

migrate-status:
	@$(MAKE) tools
	@DATABASE_URL=$${DATABASE_URL:?DATABASE_URL is required}; $(GOOSE) -dir $(MIGRATIONS_DIR) postgres "$$DATABASE_URL" status

sqlc-version:
	@$(MAKE) tools
	@$(SQLC) version

sqlc-generate:
	@$(MAKE) tools
	@$(SQLC) generate -f $(CURDIR)/sqlc.yaml

clean:
	@rm -rf bin
