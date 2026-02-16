# Project Status

## Last Completed
- ENV-115: Added `envref edit` command to open .env files in $VISUAL/$EDITOR [iter-41]

## Current State
- Go module `github.com/xcke/envref` initialized with Cobra + Viper + go-keyring + testify dependencies
- Root command with help text describing envref's purpose
- `envref version` subcommand prints version (set via `-ldflags` at build time)
- `envref init` command scaffolds new envref projects (`.envref.yaml`, `.env`, `.env.local`, optional `.envrc`)
- `envref get <KEY>` command loads `.env` + optional profile + `.env.local`, merges, interpolates, prints value
- `envref set <KEY>=<VALUE>` command writes key-value pairs to .env files
- `envref list` command prints all merged and interpolated key-value pairs
- `envref secret set/get/delete/list/generate/copy` — full secret CRUD + generation + cross-project copy via configured backends with project namespace
- **Profile-scoped secrets:** `--profile` flag on all secret subcommands stores/retrieves secrets as `<project>/<profile>/<key>`; `secret get` falls back from profile to project scope; resolve pipeline tries profile-scoped first then project-scoped
- `envref resolve` — loads .env + optional profile + .env.local, merges, interpolates, resolves `ref://` references
- `envref resolve --profile <name>` — uses a named profile's env file in the merge chain and resolves profile-scoped secrets
- `envref resolve --direnv` — outputs `export KEY=VALUE` format for shell integration
- `envref resolve --strict` — fails with no output if any reference cannot be resolved (CI-safe)
- `envref run -- <command>` — resolves env vars and executes a subprocess with them injected
- `envref profile list` — shows available profiles from config and convention-based `.env.*` files
- `envref profile use <name>` — sets active profile in `.envref.yaml`
- `envref profile create <name>` — scaffolds `.env.<name>` file with optional `--from`, `--register`, `--env-file`, `--force` flags
- `envref profile diff <a> <b>` — compares effective environments between two profiles
- `envref validate` — checks .env against .env.example schema
- `envref validate --ci` — CI mode with exit code 1 on failure
- `envref validate --schema` — validates values against `.env.schema.json` with type constraints (string, number, boolean, url, enum, email, port), regex patterns, required/optional declarations
- `envref status` — shows environment overview with actionable hints
- `envref doctor` — scans .env files for common issues
- `envref config show` — prints resolved effective config (plain, JSON, table formats)
- `envref completion <shell>` — generates shell completion scripts (bash, zsh, fish, powershell)
- `envref edit` — opens .env files in `$VISUAL`/`$EDITOR` (default `vi`); supports `--local`, `--config`, `--profile` flags and explicit file argument
- **Global verbosity flags:** `--quiet`/`-q`, `--verbose`, `--debug`; mutually exclusive
- **Colorized output:** auto-detected via TTY; disabled with `--no-color` flag or `NO_COLOR` env var
- **`internal/output` package:** `Writer` type with verbosity and color support
- **`internal/schema` package:** JSON schema loader, type validators, pattern matching, required/optional enforcement
- **Fuzzy key matching:** `internal/suggest` package with "did you mean?" suggestions
- **Resolution cache:** Duplicate `ref://` URIs resolved once per resolve call
- **Config validation on load:** `Load()` calls `Validate()` automatically
- **Config inheritance:** Global config at `~/.config/envref/config.yaml` merged with project `.envref.yaml`
- **Output format support:** `--format` flag on `get`, `list`, `resolve`, and `config show` commands
- **Profile support:** 3-layer merge with `--profile` flag
- **Keychain error handling:** `KeychainError` type with platform-specific hints
- Resolution pipeline with per-key error collection, partial resolution, direct backend matching + fallback chain
- `.env` file parser with full quote/multiline/comment/BOM/CRLF support
- `ref://` URI parser, `Backend` interface, `Registry`, `NamespacedBackend`, `KeychainBackend`
- **GoReleaser config** for cross-platform releases
- **GitHub Actions CI pipeline** with test, lint, build jobs
- Makefile with build/test/lint/install targets
- Comprehensive test coverage across all packages
- Directory structure: `cmd/envref/`, `internal/cmd/`, `internal/parser/`, `internal/envfile/`, `internal/config/`, `internal/ref/`, `internal/resolve/`, `internal/backend/`, `internal/output/`, `internal/suggest/`, `internal/schema/`, `pkg/`
- All checks pass: `go build`, `go vet`, `go test`, `golangci-lint`

## Known Issues
- None currently
