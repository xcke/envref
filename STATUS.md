# Project Status

## Last Completed
- ENV-104: Added `envref validate --ci` flag for CI pipelines — extra keys are errors, compact output, silent on success [iter-33]

## Current State
- Go module `github.com/xcke/envref` initialized with Cobra + Viper + go-keyring + testify dependencies
- Root command with help text describing envref's purpose
- `envref version` subcommand prints version (set via `-ldflags` at build time)
- `envref init` command scaffolds new envref projects (`.envref.yaml`, `.env`, `.env.local`, optional `.envrc`)
- `envref get <KEY>` command loads `.env` + optional profile + `.env.local`, merges, interpolates, prints value
- `envref set <KEY>=<VALUE>` command writes key-value pairs to .env files
- `envref list` command prints all merged and interpolated key-value pairs
- `envref secret set/get/delete/list/generate/copy` — full secret CRUD + generation + cross-project copy via configured backends with project namespace
- `envref resolve` — loads .env + optional profile + .env.local, merges, interpolates, resolves `ref://` references
- `envref resolve --profile <name>` — uses a named profile's env file in the merge chain
- `envref resolve --direnv` — outputs `export KEY=VALUE` format for shell integration
- `envref resolve --strict` — fails with no output if any reference cannot be resolved (CI-safe)
- `envref run -- <command>` — resolves env vars and executes a subprocess with them injected
- `envref profile list` — shows available profiles from config and convention-based `.env.*` files
- `envref profile use <name>` — sets active profile in `.envref.yaml`
- `envref validate` — checks .env against .env.example schema
- `envref validate --ci` — CI mode: extra keys are errors, compact output, silent on success, exit code 1 on any failure
- `envref status` — shows environment overview with actionable hints
- `envref doctor` — scans .env files for common issues
- `envref config show` — prints resolved effective config (plain, JSON, table formats)
- `envref completion <shell>` — generates shell completion scripts (bash, zsh, fish, powershell)
- **Resolution cache:** Duplicate `ref://` URIs within a single resolve call are resolved once, avoiding redundant backend queries
- **Config validation on load:** `Load()` calls `Validate()` automatically, returning `*ValidationError` for semantic errors
- **Config inheritance:** Global config at `~/.config/envref/config.yaml` merged with project `.envref.yaml`
- **Output format support:** `--format` flag on `get`, `list`, `resolve`, and `config show` commands (plain, json, shell, table)
- **Profile support:** 3-layer merge with `--profile` flag
- **Keychain error handling:** `KeychainError` type classifies raw go-keyring errors into categories with platform-specific actionable hints
- Resolution pipeline with per-key error collection, partial resolution, direct backend matching + fallback chain
- `.env` file parser with full quote/multiline/comment/BOM/CRLF support
- `ref://` URI parser, `Backend` interface, `Registry`, `NamespacedBackend`, `KeychainBackend`
- **GoReleaser config** for cross-platform releases
- **GitHub Actions CI pipeline** with test, lint, build jobs
- Makefile with build/test/lint/install targets
- Comprehensive test coverage across all packages
- Directory structure: `cmd/envref/`, `internal/cmd/`, `internal/parser/`, `internal/envfile/`, `internal/config/`, `internal/ref/`, `internal/resolve/`, `internal/backend/`, `pkg/`
- All checks pass: `go build`, `go vet`, `go test`, `golangci-lint`

## Known Issues
- None currently
