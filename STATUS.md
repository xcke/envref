# Project Status

## Last Completed
- ENV-100: Added `envref validate` command — compares merged .env against .env.example schema, reports missing/extra keys, non-zero exit on missing keys for CI [iter-17]

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
- `envref profile list` — shows available profiles from config and convention-based `.env.*` files, marks active profile
- `envref profile use <name>` — sets active profile in `.envref.yaml`, validates against config and disk, supports `--clear`
- `envref validate` — checks .env against .env.example schema, reports missing/extra keys, supports `--example`, `--profile-file` flags
- **Profile support:** `.envref.yaml` `active_profile` field, `profiles` map with custom `env_file` paths, convention-based naming (`.env.<name>`), `--profile` flag overrides config, 3-layer merge: base ← profile ← local
- **Config write support:** `SetActiveProfile()` function for targeted YAML field updates preserving file formatting
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
- **Parser test coverage: 100+ test cases**
- **Merge test coverage: 38+ test cases**
- **Resolve test coverage: 50+ test cases**
- **Integration test coverage: 50+ test cases** (including profile tests)
- **Profile list test coverage: 11 test cases**
- **Profile use test coverage: 10 test cases**
- **Config SetActiveProfile test coverage: 5 test cases**
- **Validate test coverage: 11 test cases**
- Directory structure: `cmd/envref/`, `internal/cmd/`, `internal/parser/`, `internal/envfile/`, `internal/config/`, `internal/ref/`, `internal/resolve/`, `internal/backend/`, `pkg/`
- All checks pass: `go build`, `go vet`, `go test`, `golangci-lint`

## Known Issues
- None currently
