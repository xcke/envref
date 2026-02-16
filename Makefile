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

.PHONY: all build test lint vet check install clean help cover cover-html cover-func stats

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

## stats: Show codebase statistics
stats:
	@echo "── envref codebase stats ──"
	@echo ""
	@printf "%-24s %s\n" "Go source files:" "$$(find . -name '*.go' -not -path './vendor/*' | wc -l | tr -d ' ')"
	@printf "%-24s %s\n" "Go lines (total):" "$$(find . -name '*.go' -not -path './vendor/*' -exec cat {} + | wc -l | tr -d ' ')"
	@printf "%-24s %s\n" "Go lines (no blank/comment):" "$$(find . -name '*.go' -not -path './vendor/*' -exec cat {} + | grep -cvE '^\s*$$|^\s*//' | tr -d ' ')"
	@printf "%-24s %s\n" "Test files:" "$$(find . -name '*_test.go' -not -path './vendor/*' | wc -l | tr -d ' ')"
	@printf "%-24s %s\n" "Test lines:" "$$(find . -name '*_test.go' -not -path './vendor/*' -exec cat {} + | wc -l | tr -d ' ')"
	@echo ""
	@echo "By package:"
	@find . -name '*.go' -not -path './vendor/*' -not -name '*_test.go' | \
		sed 's|/[^/]*$$||' | sort | uniq -c | sort -rn | \
		while read count dir; do \
			lines=$$(cat "$$dir"/*.go 2>/dev/null | grep -cvE '^\s*$$|^\s*//' || echo 0); \
			printf "  %-36s %3d files  %5d lines\n" "$$dir" "$$count" "$$lines"; \
		done
	@echo ""
	@printf "%-24s %s\n" "Packages:" "$$(find . -name '*.go' -not -path './vendor/*' -exec dirname {} \; | sort -u | wc -l | tr -d ' ')"
	@printf "%-24s %s\n" "Dependencies (direct):" "$$(grep -c '^\t[^/]*/' go.mod 2>/dev/null || echo 0)"
	@printf "%-24s %s\n" "Git commits:" "$$(git rev-list --count HEAD 2>/dev/null || echo '?')"
	@printf "%-24s %s\n" "Version:" "$(VERSION)"

## help: Show this help message
help:
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/^## /  /'
