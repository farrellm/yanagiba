# yanagiba -- single entry point for building, testing and checking.
# CI invokes these same targets, so a green `make check` locally means a green
# pipeline.

.DEFAULT_GOAL := build

BINARY      := yanagiba
PKG         := ./cmd/yanagiba
VERSION     := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS     := -X main.version=$(VERSION)

# Developer tooling lives in the repo so it never pollutes the user's GOPATH.
TOOLS_BIN   := $(CURDIR)/bin
GOLANGCI    := $(TOOLS_BIN)/golangci-lint
GOLANGCI_VERSION := v2.12.2

# `make install` targets the user's real bin directory, which is deliberately
# not TOOLS_BIN -- installing into a gitignored directory would be useless.
INSTALL_DIR ?= $(or $(shell go env GOBIN),$(shell go env GOPATH)/bin)

.PHONY: build
build: ## Build the binary into bin/
	go build -ldflags '$(LDFLAGS)' -o $(TOOLS_BIN)/$(BINARY) $(PKG)

.PHONY: install
install: ## Install the binary into $(INSTALL_DIR)
	GOBIN=$(INSTALL_DIR) go install -ldflags '$(LDFLAGS)' $(PKG)
	@echo "installed $(INSTALL_DIR)/$(BINARY)"
	@case ":$$PATH:" in \
		*":$(INSTALL_DIR):"*) ;; \
		*) echo "warning: $(INSTALL_DIR) is not on your PATH" ;; \
	esac

.PHONY: uninstall
uninstall: ## Remove the installed binary
	rm -f $(INSTALL_DIR)/$(BINARY)

# Autoloading requires the file to be named after the function it defines.
FISH_FUNCTION_DIR ?= $(HOME)/.config/fish/functions

.PHONY: install-fish-function
install-fish-function: ## Install the fish wrapper that reloads history after editing
	mkdir -p $(FISH_FUNCTION_DIR)
	cp contrib/$(BINARY).fish $(FISH_FUNCTION_DIR)/$(BINARY).fish
	@echo "installed $(FISH_FUNCTION_DIR)/$(BINARY).fish"
	@echo "start a new fish shell, or run: source $(FISH_FUNCTION_DIR)/$(BINARY).fish"

.PHONY: uninstall-fish-function
uninstall-fish-function: ## Remove the fish wrapper
	rm -f $(FISH_FUNCTION_DIR)/$(BINARY).fish

.PHONY: run
run: ## Run against ARGS, e.g. make run ARGS="--file testdata/sample_history"
	go run -ldflags '$(LDFLAGS)' $(PKG) $(ARGS)

.PHONY: demo
demo: ## Run against the bundled sample history, never your real one
	go run $(PKG) --file internal/fishhist/testdata/sample_history

.PHONY: test
test: ## Run the test suite with the race detector
	go test -race -covermode=atomic -coverprofile=coverage.out ./...

.PHONY: cover
cover: test ## Open the coverage report in a browser
	go tool cover -html=coverage.out

.PHONY: vet
vet: ## Run go vet
	go vet ./...

.PHONY: lint
lint: $(GOLANGCI) ## Run golangci-lint
	$(GOLANGCI) run

.PHONY: fmt
fmt: $(GOLANGCI) ## Format the code
	$(GOLANGCI) fmt

.PHONY: fmt-check
fmt-check: $(GOLANGCI) ## Fail if the code is not formatted
	$(GOLANGCI) fmt --diff

.PHONY: tidy
tidy: ## Tidy go.mod and go.sum
	go mod tidy

.PHONY: tidy-check
tidy-check: ## Fail if go.mod or go.sum are not tidy
	go mod tidy
	git diff --exit-code go.mod go.sum

.PHONY: check-fish
check-fish: ## Syntax-check the shipped fish function (skipped if fish is absent)
	@if command -v fish >/dev/null 2>&1; then \
		fish --no-execute contrib/$(BINARY).fish && echo "contrib/$(BINARY).fish: ok"; \
	else \
		echo "fish not installed, skipping"; \
	fi

.PHONY: check
check: fmt-check vet lint check-fish test ## Everything CI runs

.PHONY: check-history
check-history: ## Verify the codec round-trips your real history, read-only
	go run $(PKG) --check

.PHONY: tools
tools: $(GOLANGCI) ## Install pinned developer tooling into bin/

$(GOLANGCI):
	GOBIN=$(TOOLS_BIN) go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_VERSION)

.PHONY: clean
clean: ## Remove build output
	rm -rf $(TOOLS_BIN) coverage.out

.PHONY: help
help: ## List available targets
	@grep -hE '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) \
		| awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-16s\033[0m %s\n", $$1, $$2}'
