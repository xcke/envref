# Project Status

## Last Completed
- ENV-020: Defined .envref.yaml schema with Config, BackendConfig, ProfileConfig types and Viper-based loader [iter-11]

## Current State
- Go module `github.com/xcke/envref` initialized with Cobra + Viper dependencies
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
- `.envref.yaml` config schema (`internal/config`):
  - `Config` struct: project name, env_file, local_file, backends, profiles
  - `BackendConfig`: name, type, config map for backend-specific settings
  - `ProfileConfig`: per-profile env_file override
  - Viper-based loader with defaults (env_file=".env", local_file=".env.local")
  - Project root discovery: walks up directory tree to find `.envref.yaml`
  - Validation: required project name, no duplicate backends, non-empty paths
- `.env` file parser (`internal/parser`) with:
  - Full quote/multiline/comment support (single, double, backtick quotes)
  - UTF-8 BOM detection and stripping, CRLF normalization
  - Trailing whitespace trimming, duplicate key detection with warnings
  - `QuoteStyle` tracking on entries
- `.env` file loader, merger, and writer (`internal/envfile`) with:
  - `Load`, `LoadOptional`, `Merge`, `Write`, `Interpolate`, ordered key storage
  - `Refs()`, `HasRefs()`, `ResolvedRefs()` for ref:// handling
- `ref://` URI parser (`internal/ref`) with Parse, IsRef, Reference type
- Makefile with targets: `all`, `build`, `test`, `lint`, `vet`, `check`, `install`, `clean`, `help`
- Directory structure: `cmd/envref/`, `internal/cmd/`, `internal/parser/`, `internal/envfile/`, `internal/config/`, `internal/ref/`, `pkg/`
- All checks pass: `go build`, `go vet`, `go test`, `golangci-lint`

## Known Issues
- None currently
