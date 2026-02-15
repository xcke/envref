# Project Status

## Last Completed
- ENV-013: Added ref:// detection and tagging for secret references [iter-5]

## Current State
- Go module `github.com/xcke/envref` initialized with Cobra dependency
- Root command with help text describing envref's purpose
- `envref version` subcommand prints version (set via `-ldflags` at build time)
- Unit tests for root command and version subcommand
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
