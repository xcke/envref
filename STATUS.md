# Project Status

## Last Completed
- ENV-070, ENV-071, ENV-072, ENV-073: Added `envref resolve` command with full resolution pipeline and `--direnv` output format [iter-9]

## Current State
- Go module `github.com/xcke/envref` initialized with Cobra + Viper + go-keyring + testify dependencies
- Root command with help text describing envref's purpose
- `envref version` subcommand prints version (set via `-ldflags` at build time)
- `envref init` command scaffolds new envref projects (`.envref.yaml`, `.env`, `.env.local`, optional `.envrc`)
- `envref get <KEY>` command loads `.env` + `.env.local`, merges, interpolates, prints value
- `envref set <KEY>=<VALUE>` command writes key-value pairs to .env files
- `envref list` command prints all merged and interpolated key-value pairs
- `envref secret set/get/delete/list` — full secret CRUD via configured backends with project namespace
- **`envref resolve`** — loads .env + .env.local, merges, interpolates variables, resolves all `ref://` references via configured backends, outputs fully resolved KEY=VALUE pairs
- **`envref resolve --direnv`** — outputs `export KEY=VALUE` format for `eval "$(envref resolve --direnv)"` shell integration
- Resolution pipeline: `internal/resolve` package with `Resolve()` function, per-key error collection, direct backend matching + fallback chain
- Shell-safe quoting for direnv export output (single-quote escaping)
- Variable interpolation (`${VAR}` and `$VAR` syntax)
- `.envref.yaml` config schema with Viper-based loader and project root discovery
- `.env` file parser with full quote/multiline/comment/BOM/CRLF support
- `.env` file loader, merger, writer, and interpolator with ref:// handling
- `ref://` URI parser (`ref://<backend>/<path>` format with nested path support)
- `Backend` interface with `Name()`, `Get()`, `Set()`, `Delete()`, `List()` methods
- `Registry` type with ordered fallback chain
- `NamespacedBackend` wrapper for per-project secret isolation
- `KeychainBackend` — cross-platform OS keychain backend using `go-keyring`
- Makefile with build/test/lint/install targets
- Directory structure: `cmd/envref/`, `internal/cmd/`, `internal/parser/`, `internal/envfile/`, `internal/config/`, `internal/ref/`, `internal/resolve/`, `internal/backend/`, `pkg/`
- All checks pass: `go build`, `go vet`, `go test`, `golangci-lint`

## Known Issues
- None currently
