# Project Status

## Last Completed
- ENV-114: Added `envref run -- <command>` to execute commands with resolved env vars injected [iter-23]

## Current State
- Go module `github.com/xcke/envref` initialized with Cobra + Viper + go-keyring + testify dependencies
- Root command with help text describing envref's purpose
- `envref version` subcommand prints version (set via `-ldflags` at build time)
- `envref init` command scaffolds new envref projects (`.envref.yaml`, `.env`, `.env.local`, optional `.envrc`)
- `envref get <KEY>` command loads `.env` + optional profile + `.env.local`, merges, interpolates, prints value
- `envref set <KEY>=<VALUE>` command writes key-value pairs to .env files
- `envref list` command prints all merged and interpolated key-value pairs
- `envref secret set/get/delete/list` — full secret CRUD via configured backends with project namespace
- `envref resolve` — loads .env + optional profile + .env.local, merges, interpolates, resolves `ref://` references
- `envref resolve --profile <name>` — uses a named profile's env file in the merge chain
- `envref resolve --direnv` — outputs `export KEY=VALUE` format for shell integration
- `envref resolve --strict` — fails with no output if any reference cannot be resolved (CI-safe)
- `envref run -- <command>` — resolves env vars and executes a subprocess with them injected (alternative to direnv)
  - Supports `--profile` and `--strict` flags
  - Inherits current process environment, overlays resolved vars
  - Forwards signals (SIGINT, SIGTERM) to child process
  - Propagates child process exit code
- `envref profile list` — shows available profiles from config and convention-based `.env.*` files, marks active profile
- `envref profile use <name>` — sets active profile in `.envref.yaml`, validates against config and disk, supports `--clear`
- `envref validate` — checks .env against .env.example schema, reports missing/extra keys, supports `--example`, `--profile-file` flags
- `envref status` — shows environment overview: project info, file existence, key counts, backend resolution status, validation, actionable hints
- **Output format support:** `--format` flag on `get`, `list`, and `resolve` commands (plain, json, shell, table)
- **Profile support:** `.envref.yaml` `active_profile` field, `profiles` map, convention-based naming, `--profile` flag, 3-layer merge
- **Config write support:** `SetActiveProfile()` function for targeted YAML field updates
- Resolution pipeline: `internal/resolve` package with `Resolve()` function, per-key error collection, partial resolution, direct backend matching + fallback chain
- Shell-safe quoting for direnv export output
- Variable interpolation (`${VAR}` and `$VAR` syntax)
- `.envref.yaml` config schema with Viper-based loader and project root discovery
- `.env` file parser with full quote/multiline/comment/BOM/CRLF support
- `.env` file loader, merger, writer, and interpolator with ref:// handling
- `ref://` URI parser, `Backend` interface, `Registry` with fallback chain, `NamespacedBackend`, `KeychainBackend`
- **GoReleaser config** for cross-platform releases (Linux/macOS/Windows × amd64/arm64, tar.gz/zip, checksums, changelog)
- **GitHub Actions CI pipeline** with test (ubuntu/macos/windows matrix), lint (go vet + golangci-lint), and build jobs
- Makefile with build/test/lint/install targets
- Comprehensive test coverage: parser (100+), merge (38+), resolve (50+), integration (55+), profile, validate, status, format, strict mode, run command tests
- Directory structure: `cmd/envref/`, `internal/cmd/`, `internal/parser/`, `internal/envfile/`, `internal/config/`, `internal/ref/`, `internal/resolve/`, `internal/backend/`, `pkg/`
- All checks pass: `go build`, `go vet`, `go test`, `golangci-lint`

## Known Issues
- None currently
