# Project Status

## Last Completed
- ENV-005: Added `.goreleaser.yml` for cross-platform binary releases (Linux/macOS/Windows, amd64/arm64) [iter-19]

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
- `envref status` — shows environment overview: project info, file existence, key counts, backend resolution status, validation, actionable hints
- **Profile support:** `.envref.yaml` `active_profile` field, `profiles` map, convention-based naming, `--profile` flag, 3-layer merge
- **Config write support:** `SetActiveProfile()` function for targeted YAML field updates
- Resolution pipeline: `internal/resolve` package with `Resolve()` function, per-key error collection, direct backend matching + fallback chain
- Shell-safe quoting for direnv export output
- Variable interpolation (`${VAR}` and `$VAR` syntax)
- `.envref.yaml` config schema with Viper-based loader and project root discovery
- `.env` file parser with full quote/multiline/comment/BOM/CRLF support
- `.env` file loader, merger, writer, and interpolator with ref:// handling
- `ref://` URI parser, `Backend` interface, `Registry` with fallback chain, `NamespacedBackend`, `KeychainBackend`
- **GoReleaser config** for cross-platform releases (Linux/macOS/Windows × amd64/arm64, tar.gz/zip, checksums, changelog)
- Makefile with build/test/lint/install targets
- Comprehensive test coverage: parser (100+), merge (38+), resolve (50+), integration (50+), profile, validate, status tests
- Directory structure: `cmd/envref/`, `internal/cmd/`, `internal/parser/`, `internal/envfile/`, `internal/config/`, `internal/ref/`, `internal/resolve/`, `internal/backend/`, `pkg/`
- All checks pass: `go build`, `go vet`, `go test`, `golangci-lint`

## Known Issues
- None currently
