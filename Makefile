# envref — CLI tool for separating config from secrets in .env files

# Build variables
BINARY      := envref
MODULE      := github.com/xcke/envref
CMD_PATH    := ./cmd/envref
BUILD_DIR   := ./build
VERSION     ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS     := -ldflags "-s -w -X '$(MODULE)/internal/cmd.version=$(VERSION)'"

# Coverage
COVER_DIR   := ./coverage
COVER_PROFILE := $(COVER_DIR)/coverage.out

# Tool paths
GO          := go
LINT        := golangci-lint

.PHONY: all build test lint vet check install clean help cover cover-html cover-func

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

## cover: Run tests with coverage and generate profile
cover:
	@mkdir -p $(COVER_DIR)
	$(GO) test -race -count=1 -coverprofile=$(COVER_PROFILE) -covermode=atomic ./...
	@$(GO) tool cover -func=$(COVER_PROFILE) | tail -1

## cover-html: Generate HTML coverage report and open it
cover-html: cover
	$(GO) tool cover -html=$(COVER_PROFILE) -o $(COVER_DIR)/coverage.html
	@echo "Coverage report: $(COVER_DIR)/coverage.html"

## cover-func: Show per-function coverage breakdown
cover-func: cover
	$(GO) tool cover -func=$(COVER_PROFILE)

## install: Install envref to $GOPATH/bin
install:
	$(GO) install $(LDFLAGS) $(CMD_PATH)

## clean: Remove build artifacts and coverage data
clean:
	rm -rf $(BUILD_DIR) $(COVER_DIR)
	$(GO) clean -cache -testcache

## help: Show this help message
help:
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/^## /  /'
