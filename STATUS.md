# Project Status

## Last Completed
- ENV-031: Implemented backend registry with ordered fallback chain, Get/Set/Delete/List by name, and String representation [iter-2]

## Current State
- Go module `github.com/xcke/envref` initialized with Cobra + Viper dependencies
- Root command with help text describing envref's purpose
- `envref version` subcommand prints version (set via `-ldflags` at build time)
- `envref init` command scaffolds new envref projects (`.envref.yaml`, `.env`, `.env.local`, optional `.envrc`)
- `envref get <KEY>` command loads `.env` + `.env.local`, merges, interpolates, prints value
- `envref set <KEY>=<VALUE>` command writes key-value pairs to .env files
- `envref list` command prints all merged and interpolated key-value pairs
- Variable interpolation (`${VAR}` and `$VAR` syntax)
- `.envref.yaml` config schema with Viper-based loader and project root discovery
- `.env` file parser with full quote/multiline/comment/BOM/CRLF support
- `.env` file loader, merger, writer, and interpolator with ref:// handling
- `ref://` URI parser
- **`Backend` interface** in `internal/backend/` with `Name()`, `Get()`, `Set()`, `Delete()`, `List()` methods
- **`Registry`** type with ordered fallback chain: `Get()` tries backends in order; `GetFrom`/`SetIn`/`DeleteFrom`/`ListFrom` target specific backends
- `ErrNotFound` sentinel error and `KeyError` structured error type for backend operations
- Makefile with build/test/lint/install targets
- Directory structure: `cmd/envref/`, `internal/cmd/`, `internal/parser/`, `internal/envfile/`, `internal/config/`, `internal/ref/`, `internal/backend/`, `pkg/`
- All checks pass: `go build`, `go vet`, `go test`, `golangci-lint`

## Known Issues
- None currently
