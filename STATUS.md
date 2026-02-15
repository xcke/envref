# Project Status

## Last Completed
- ENV-016: Handle edge cases — BOM stripping, CRLF normalization, duplicate key warnings, trailing whitespace [iter-8]

## Current State
- Go module `github.com/xcke/envref` initialized with Cobra dependency
- Root command with help text describing envref's purpose
- `envref version` subcommand prints version (set via `-ldflags` at build time)
- `envref get <KEY>` command loads `.env` + `.env.local`, merges, prints value for key
  - `--file` / `-f` flag to specify custom .env path (default `.env`)
  - `--local-file` flag to specify override file path (default `.env.local`)
  - Prints parser warnings (e.g., duplicate keys) to stderr
- `envref set <KEY>=<VALUE>` command writes key-value pairs to .env files
  - `--file` / `-f` flag to specify target .env path (default `.env`)
  - `--local` flag to write to `.env.local` instead of `.env`
  - `--local-file` flag to specify .env.local path (default `.env.local`)
  - Updates existing keys in place; appends new keys
  - Creates the target file if it doesn't exist
  - Auto-quotes values containing spaces, newlines, or special characters
  - Prints parser warnings to stderr
- `.env` file parser (`internal/parser`) with:
  - Full quote/multiline/comment support (single, double, backtick quotes)
  - UTF-8 BOM detection and stripping on first line
  - CRLF line ending normalization (strips trailing `\r`)
  - Trailing whitespace trimming for unquoted values
  - Duplicate key detection with warnings (last value wins, emits `Warning`)
  - `Warning` type for non-fatal parse issues
  - `Parse()` returns `([]Entry, []Warning, error)`
- `.env` file loader, merger, and writer (`internal/envfile`) with:
  - `Load(path)` — returns `(*Env, []Warning, error)`
  - `LoadOptional(path)` — same but returns empty Env if file missing
  - `Merge(base, overlays...)` — merge multiple Env layers, later overlays win
  - `Write(path)` — serialize Env to .env file with proper quoting
  - `Env` type with ordered key storage, Get/Set/Keys/All/Len methods
  - `Refs()`, `HasRefs()`, `ResolvedRefs()` for ref:// handling
- `ref://` URI parser (`internal/ref`) with Parse, IsRef, Reference type
- Makefile with targets: `all`, `build`, `test`, `lint`, `vet`, `check`, `install`, `clean`, `help`
- Directory structure: `cmd/envref/`, `internal/cmd/`, `internal/parser/`, `internal/envfile/`, `internal/ref/`, `pkg/`
- All checks pass: `go build`, `go vet`, `go test`, `golangci-lint`

## Known Issues
- None currently
