# envref — CLI tool for separating config from secrets in .env files

# Build variables
BINARY      := envref
MODULE      := github.com/xcke/envref
CMD_PATH    := ./cmd/envref
BUILD_DIR   := ./build
VERSION     ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS     := -ldflags "-s -w -X '$(MODULE)/internal/cmd.version=$(VERSION)'"

# Tool paths
GO          := go
LINT        := golangci-lint

.PHONY: all build test lint vet check install clean help

## all: Run lint, test, and build (default target)
all: check build

## build: Compile the envref binary into build/
build:
	@mkdir -p $(BUILD_DIR)
	$(GO) build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY) $(CMD_PATH)
	@echo "Built $(BUILD_DIR)/$(BINARY) ($(VERSION))"

## test: Run all tests with race detector
test:
	$(GO) test -race -count=1 ./...

## lint: Run golangci-lint
lint:
	$(LINT) run ./...

## vet: Run go vet
vet:
	$(GO) vet ./...

## check: Run vet, lint, and tests
check: vet lint test

## install: Install envref to $GOPATH/bin
install:
	$(GO) install $(LDFLAGS) $(CMD_PATH)

## clean: Remove build artifacts
clean:
	rm -rf $(BUILD_DIR)
	$(GO) clean -cache -testcache

## help: Show this help message
help:
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/^## /  /'
