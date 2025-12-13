GO ?= go
GOLANGCI_LINT_VERSION ?= v1.64.8
GOIMPORTS_VERSION ?= v0.40.0
BIN_DIR := $(CURDIR)/bin
GOLANGCI_LINT := $(BIN_DIR)/golangci-lint
GOIMPORTS := $(BIN_DIR)/goimports
GOFILES := $(shell find . -name '*.go' -not -path "./vendor/*" -not -path "./.git/*" -not -path "./bin/*")

.PHONY: fmt lint test vet tidy ci tools clean

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

ci: fmt vet lint test

clean:
	@rm -rf bin
