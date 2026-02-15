# Project Status

## Last Completed
- ENV-021: Config loader already implemented in ENV-020 (Viper + project root discovery) — marked DONE [iter-12]
- ENV-022: Added `envref init` command with project scaffolding [iter-12]

## Current State
- Go module `github.com/xcke/envref` initialized with Cobra + Viper dependencies
- Root command with help text describing envref's purpose
- `envref version` subcommand prints version (set via `-ldflags` at build time)
- `envref init` command scaffolds new envref projects:
  - Creates `.envref.yaml` with project name, defaults, and commented backend/profile examples
  - Creates `.env` with example entries and ref:// usage hint
  - Creates `.env.local` for local overrides
  - Optional `--direnv` flag generates `.envrc` with `eval "$(envref resolve --direnv)"`
  - Appends `.env.local` to `.gitignore` (creates or updates)
  - `--force` flag to overwrite existing files; skips by default
  - `--project` / `-p` flag (defaults to directory name)
  - Generated config is valid and loadable by the config package
- `envref get <KEY>` command loads `.env` + `.env.local`, merges, interpolates, prints value
- `envref set <KEY>=<VALUE>` command writes key-value pairs to .env files
- `envref list` command prints all merged and interpolated key-value pairs
- Variable interpolation (`${VAR}` and `$VAR` syntax)
- `.envref.yaml` config schema with Viper-based loader and project root discovery
- `.env` file parser with full quote/multiline/comment/BOM/CRLF support
- `.env` file loader, merger, writer, and interpolator with ref:// handling
- `ref://` URI parser
- Makefile with build/test/lint/install targets
- Directory structure: `cmd/envref/`, `internal/cmd/`, `internal/parser/`, `internal/envfile/`, `internal/config/`, `internal/ref/`, `pkg/`
- All checks pass: `go build`, `go vet`, `go test`, `golangci-lint`

## Known Issues
- None currently
