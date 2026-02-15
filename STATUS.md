# Project Status

## Last Completed
- ENV-017: Add `envref list` command — print all key-value pairs, mask ref:// secrets by default [iter-9]

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
- `envref list` command prints all merged key-value pairs
  - `--file` / `-f` and `--local-file` flags consistent with other commands
  - `--show-secrets` flag to reveal ref:// values (masked as `ref://***` by default)
- `.env` file parser (`internal/parser`) with:
  - Full quote/multiline/comment support (single, double, backtick quotes)
  - UTF-8 BOM detection and stripping, CRLF normalization
  - Trailing whitespace trimming, duplicate key detection with warnings
- `.env` file loader, merger, and writer (`internal/envfile`) with:
  - `Load`, `LoadOptional`, `Merge`, `Write`, ordered key storage
  - `Refs()`, `HasRefs()`, `ResolvedRefs()` for ref:// handling
- `ref://` URI parser (`internal/ref`) with Parse, IsRef, Reference type
- Makefile with targets: `all`, `build`, `test`, `lint`, `vet`, `check`, `install`, `clean`, `help`
- Directory structure: `cmd/envref/`, `internal/cmd/`, `internal/parser/`, `internal/envfile/`, `internal/ref/`, `pkg/`
- All checks pass: `go build`, `go vet`, `go test`, `golangci-lint`

## Known Issues
- None currently
