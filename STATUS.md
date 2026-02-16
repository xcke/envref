# Project Status

## Last Completed
- ENV-041: Added vault init command with master passphrase setup flow and verification [iter-45]

## Current State
- Go module `github.com/xcke/envref` initialized with Cobra + Viper + go-keyring + age + sqlite + testify + x/term dependencies
- Root command with help text describing envref's purpose
- `envref version` subcommand prints version (set via `-ldflags` at build time)
- `envref init` command scaffolds new envref projects (`.envref.yaml`, `.env`, `.env.local`, optional `.envrc`)
- `envref init --direnv` auto-runs `direnv allow` if direnv is installed; provides install guidance if not
- `envref get <KEY>` command loads `.env` + optional profile + `.env.local`, merges, interpolates, prints value
- `envref set <KEY>=<VALUE>` command writes key-value pairs to .env files
- `envref list` command prints all merged and interpolated key-value pairs
- `envref secret set/get/delete/list/generate/copy` — full secret CRUD + generation + cross-project copy via configured backends with project namespace
- **Profile-scoped secrets:** `--profile` flag on all secret subcommands stores/retrieves secrets as `<project>/<profile>/<key>`; `secret get` falls back from profile to project scope; resolve pipeline tries profile-scoped first then project-scoped
- `envref resolve` — loads .env + optional profile + .env.local, merges, interpolates, resolves `ref://` references
- `envref resolve --profile <name>` — uses a named profile's env file in the merge chain and resolves profile-scoped secrets
- `envref resolve --direnv` — outputs `export KEY=VALUE` format for shell integration
- `envref resolve --strict` — fails with no output if any reference cannot be resolved (CI-safe)
- `envref run -- <command>` — resolves env vars and executes a subprocess with them injected
- `envref profile list/use/create/diff` — full profile management commands
- `envref validate` — checks .env against .env.example schema with `--ci` and `--schema` modes
- `envref status` — shows environment overview with actionable hints
- `envref doctor` — scans .env files for common issues
- `envref config show` — prints resolved effective config (plain, JSON, table formats)
- `envref completion <shell>` — generates shell completion scripts (bash, zsh, fish, powershell)
- `envref edit` — opens .env files in `$VISUAL`/`$EDITOR`
- `envref vault init` — initialize vault with master passphrase (interactive prompt with confirmation, or via env var)
- **Two secret backends:** `KeychainBackend` (OS keychain via go-keyring) and `VaultBackend` (local SQLite + age encryption)
- **VaultBackend:** per-value age scrypt encryption, passphrase via `ENVREF_VAULT_PASSPHRASE` env var or `config.passphrase` or interactive terminal prompt, configurable DB path, lazy connection, WAL mode
- **Vault setup flow:** `vault init` stores encrypted verification token; subsequent access verifies passphrase against token; wrong passphrase returns clear error
- **Interactive passphrase prompt:** secret/resolve/run/status commands prompt for vault passphrase at terminal when not configured via env var or config
- **Global verbosity flags, colorized output, fuzzy key matching, resolution cache**
- **README.md** with project overview, install instructions, quickstart, and command reference
- `.env` file parser with full quote/multiline/comment/BOM/CRLF support
- `ref://` URI parser, `Backend` interface, `Registry`, `NamespacedBackend`
- GoReleaser config, GitHub Actions CI pipeline, Makefile
- Comprehensive test coverage across all packages
- All checks pass: `go build`, `go vet`, `go test`, `golangci-lint`

## Known Issues
- None currently
