# Project Status

## Last Completed
- ENV-052: Implemented AWS SSM Parameter Store backend and Oracle OCI Vault backend with CLI wrappers and mock-based tests [iter-64]

## Current State
- Go module `github.com/xcke/envref` initialized with Cobra + Viper + go-keyring + age + sqlite + testify + x/term dependencies
- Root command with help text describing envref's purpose
- `envref version` subcommand prints version (set via `-ldflags` at build time)
- `envref init` command scaffolds new envref projects (`.envref.yaml`, `.env`, `.env.local`, optional `.envrc`)
- `envref init --direnv` auto-runs `direnv allow` if direnv is installed; provides install guidance if not
- `envref get <KEY>` command loads `.env` + optional profile + `.env.local`, merges, interpolates, prints value
- `envref set <KEY>=<VALUE>` command writes key-value pairs to .env files
- `envref list` command prints all merged and interpolated key-value pairs
- `envref secret set/get/delete/list/generate/copy/rotate` — full secret CRUD + generation + cross-project copy + rotation via configured backends with project namespace
- `envref secret rotate <KEY>` — generates new random value, archives old value as `<KEY>.__history.<N>`, supports `--keep` for configurable history retention
- **Profile-scoped secrets:** `--profile` flag on all secret subcommands stores/retrieves secrets as `<project>/<profile>/<key>`
- `envref resolve` — loads .env + optional profile + .env.local, merges, interpolates, resolves `ref://` references
- `envref resolve --direnv` — outputs `export KEY=VALUE` format for shell integration
- `envref resolve --strict` — fails with no output if any reference cannot be resolved (CI-safe)
- `envref resolve --watch` / `-w` — watches .env files via fsnotify and re-resolves on changes with debouncing
- `envref run -- <command>` — resolves env vars and executes a subprocess with them injected
- `envref profile list/use/create/diff` — full profile management commands
- `envref validate` — checks .env against .env.example schema with `--ci` and `--schema` modes
- `envref status` — shows environment overview with actionable hints
- `envref doctor` — scans .env files for common issues
- `envref audit` — warns about plaintext secrets via pattern matching, key name heuristics, and Shannon entropy analysis
- `envref config show` — prints resolved effective config (plain, JSON, table formats)
- `envref completion <shell>` — generates shell completion scripts (bash, zsh, fish, powershell)
- `envref edit` — opens .env files in `$VISUAL`/`$EDITOR`
- `envref vault init/lock/unlock/export/import` — full vault management
- **Six backend types:** `KeychainBackend` (OS keychain via go-keyring), `VaultBackend` (local SQLite + age encryption), `OnePasswordBackend` (1Password via `op` CLI), `AWSSSMBackend` (AWS SSM Parameter Store via `aws` CLI), `OCIVaultBackend` (Oracle OCI Vault via `oci` CLI), and `PluginBackend` (external executables via JSON protocol)
- **AWS SSM backend:** Stores secrets as SecureString parameters with configurable prefix; supports region, profile, and command config options; mock-based tests via compiled aws_mock helper
- **OCI Vault backend:** Stores secrets as base64-encoded vault secret bundles; supports vault_id, compartment_id, key_id, profile config; schedule-deletion semantics for Delete; mock-based tests via compiled oci_mock helper
- **1Password backend:** Stores secrets as Secure Note items; supports vault, account, and command config options; edit-or-create semantics for Set; mock-based tests via compiled op_mock helper
- **Plugin interface:** External backends communicate via JSON-over-stdin/stdout protocol; plugins discovered by convention (`envref-backend-<name>` on $PATH) or explicit `command` config; configured as `type: plugin` in `.envref.yaml`
- **Nested references:** `${ref://secrets/key}` in values and embedded `ref://` URIs after interpolation are resolved via a second pass in the resolution pipeline
- **Security hardening:** Vault passphrase stored as `[]byte` (clearable), zeroed on Close; decrypted plaintext bytes cleared after use
- **Comprehensive README** with architecture diagram, resolution pipeline, project structure, vault docs, and benchmarks
- **docs/ directory** with four usage guides: getting-started, direnv-integration, profiles, secret-backends
- **Homebrew tap:** GoReleaser `brews` config auto-publishes to `xcke/homebrew-tap`
- **MIT LICENSE** file included
- `.env` file parser with full quote/multiline/comment/BOM/CRLF support
- `ref://` URI parser with `FindAll` for embedded ref discovery, `Backend` interface, `Registry`, `NamespacedBackend`
- GoReleaser config, GitHub Actions CI + release pipelines, Makefile with coverage targets
- Comprehensive test coverage across all packages
- Fuzz tests for .env parser, ref:// URI parser, and variable interpolation
- **Integration tests** for keychain backend (12 tests) with `//go:build integration` tag
- **Direnv integration tests** (22 tests) with `//go:build integration` tag covering end-to-end shell eval, init --direnv, profile switching, shell quoting safety, strict mode, multiline values, special characters, .envrc sourcing, and real direnv binary tests
- **CI jobs** for keychain integration tests (macOS) and direnv integration tests (ubuntu + macOS with direnv installed)
- All checks pass: `go build`, `go vet`, `go test`, `golangci-lint`

## Known Issues
- None currently
