# Project Status

## Last Completed
- ENV-033/034/035: Implemented OS keychain backend via go-keyring (macOS Keychain, Linux libsecret, Windows Credential Manager) [iter-4]

## Current State
- Go module `github.com/xcke/envref` initialized with Cobra + Viper + go-keyring dependencies
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
- `Backend` interface with `Name()`, `Get()`, `Set()`, `Delete()`, `List()` methods
- `Registry` type with ordered fallback chain
- `NamespacedBackend` wrapper for per-project secret isolation
- **`KeychainBackend`** — cross-platform OS keychain backend using `go-keyring` (macOS Keychain, Linux libsecret/Secret Service, Windows Credential Manager); maintains JSON key index for List() support
- `ErrNotFound` sentinel error and `KeyError` structured error type
- Makefile with build/test/lint/install targets
- Directory structure: `cmd/envref/`, `internal/cmd/`, `internal/parser/`, `internal/envfile/`, `internal/config/`, `internal/ref/`, `internal/backend/`, `pkg/`
- All checks pass: `go build`, `go vet`, `go test`, `golangci-lint`

## Known Issues
- None currently
