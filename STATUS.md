# Project Status

## Last Completed
- ENV-014: Added `envref get <KEY>` command to look up and print a single value [iter-6]

## Current State
- Go module `github.com/xcke/envref` initialized with Cobra dependency
- Root command with help text describing envref's purpose
- `envref version` subcommand prints version (set via `-ldflags` at build time)
- `envref get <KEY>` command loads `.env` + `.env.local`, merges, prints value for key
  - `--file` / `-f` flag to specify custom .env path (default `.env`)
  - `--local-file` flag to specify override file path (default `.env.local`)
  - Errors on missing key or missing base .env file; .env.local is optional
- Unit tests for root command, version subcommand, and get command
- Makefile with targets: `all`, `build`, `test`, `lint`, `vet`, `check`, `install`, `clean`, `help`
- Build output goes to `build/` directory with embedded version from `git describe`
- `.env` file parser (`internal/parser`) with full quote/multiline/comment support
- `.env` file loader and merger (`internal/envfile`) with:
  - `Load(path)` — parse .env file from disk into ordered key-value map
  - `LoadOptional(path)` — same as Load but returns empty Env if file missing
  - `Merge(base, overlays...)` — merge multiple Env layers, later overlays win
  - `Env` type with ordered key storage, Get/Set/Keys/All/Len methods
  - `Refs()` — return all entries with ref:// values
  - `HasRefs()` — check if env contains any ref:// references
  - `ResolvedRefs()` — parse ref:// URIs into structured Reference objects
- `ref://` URI parser (`internal/ref`) with:
  - `IsRef(value)` — check if a value is a ref:// reference
  - `Parse(value)` — parse ref:// URI into backend + path components
  - `Reference` type with Raw, Backend, Path fields
- Parser `Entry.IsRef` field auto-set during parsing when value starts with `ref://`
- Directory structure: `cmd/envref/`, `internal/cmd/`, `internal/parser/`, `internal/envfile/`, `internal/ref/`, `pkg/`
- All checks pass: `go build`, `go vet`, `go test`, `golangci-lint`

## Known Issues
- None currently
