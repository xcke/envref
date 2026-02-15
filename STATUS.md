# Project Status

## Last Completed
- ENV-018: Add variable interpolation support for `${VAR}` and `$VAR` in .env values [iter-10]

## Current State
- Go module `github.com/xcke/envref` initialized with Cobra dependency
- Root command with help text describing envref's purpose
- `envref version` subcommand prints version (set via `-ldflags` at build time)
- `envref get <KEY>` command loads `.env` + `.env.local`, merges, interpolates, prints value
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
- `envref list` command prints all merged and interpolated key-value pairs
  - `--file` / `-f` and `--local-file` flags consistent with other commands
  - `--show-secrets` flag to reveal ref:// values (masked as `ref://***` by default)
- Variable interpolation (`internal/envfile/interpolate.go`):
  - `${VAR}` and `$VAR` syntax supported in unquoted and double-quoted values
  - Single-quoted and backtick-quoted values are treated as literals (no interpolation)
  - Variables resolve against earlier definitions in the same env (order-dependent)
  - Undefined variables expand to empty string; `$$` produces literal `$`
  - Applied automatically in `get` and `list` commands after merge
- `.env` file parser (`internal/parser`) with:
  - Full quote/multiline/comment support (single, double, backtick quotes)
  - UTF-8 BOM detection and stripping, CRLF normalization
  - Trailing whitespace trimming, duplicate key detection with warnings
  - `QuoteStyle` tracking on entries (QuoteNone, QuoteSingle, QuoteDouble, QuoteBacktick)
- `.env` file loader, merger, and writer (`internal/envfile`) with:
  - `Load`, `LoadOptional`, `Merge`, `Write`, `Interpolate`, ordered key storage
  - `Refs()`, `HasRefs()`, `ResolvedRefs()` for ref:// handling
- `ref://` URI parser (`internal/ref`) with Parse, IsRef, Reference type
- Makefile with targets: `all`, `build`, `test`, `lint`, `vet`, `check`, `install`, `clean`, `help`
- Directory structure: `cmd/envref/`, `internal/cmd/`, `internal/parser/`, `internal/envfile/`, `internal/ref/`, `pkg/`
- All checks pass: `go build`, `go vet`, `go test`, `golangci-lint`

## Known Issues
- None currently
