# Project Status

## Last Completed
- ENV-138: Created animated SVG terminal demo showing the 4-step envref workflow (init, ref://, store secret, resolve); embedded in README.md [iter-77]

## Current State
- Go module `github.com/xcke/envref` initialized with Cobra + Viper + go-keyring + age + sqlite + testify + x/term dependencies
- Root command with help text describing envref's purpose
- `envref version` subcommand prints version (set via `-ldflags` at build time)
- `envref init` command scaffolds new envref projects (`.envref.yaml`, `.env`, `.env.local`, optional `.envrc`)
- `envref init --direnv` auto-runs `direnv allow` if direnv is installed; provides install guidance if not
- `envref get <KEY>` command loads `.env` + optional profile + `.env.local`, merges, interpolates, prints value
- `envref set <KEY>=<VALUE>` command writes key-value pairs to .env files
- `envref list` command prints all merged and interpolated key-value pairs
- `envref secret set/get/delete/list/generate/copy/rotate/share` — full secret CRUD + generation + cross-project copy + rotation + sharing via configured backends
- `envref secret rotate <KEY>` — generates new random value, archives old value as `<KEY>.__history.<N>`, supports `--keep` for configurable history retention
- `envref secret share <KEY> --to <age-key>` — retrieves secret from backend, encrypts with recipient's age X25519 public key, outputs ASCII-armored ciphertext; supports `--to-file` for reading key from file
- **Profile-scoped secrets:** `--profile` flag on all secret subcommands stores/retrieves secrets as `<project>/<profile>/<key>`
- `envref resolve` — loads .env + optional profile + .env.local, merges, interpolates, resolves `ref://` references
- `envref resolve --direnv` — outputs `export KEY=VALUE` format for shell integration
- `envref resolve --strict` — fails with no output if any reference cannot be resolved (CI-safe)
- `envref resolve --watch` / `-w` — watches .env files via fsnotify and re-resolves on changes with debouncing
- `envref run -- <command>` — resolves env vars and executes a subprocess with them injected
- `envref profile list/use/create/diff/export` — full profile management commands
- `envref profile export <name>` — exports merged profile environment as JSON (default), plain, shell, or table format for CI/CD integration
- `envref validate` — checks .env against .env.example schema with `--ci` and `--schema` modes
- `envref status` — shows environment overview with actionable hints
- `envref doctor` — scans .env files for common issues
- `envref audit` — warns about plaintext secrets via pattern matching, key name heuristics, and Shannon entropy analysis
- **`envref audit-log`** — displays append-only JSON-lines audit log of secret operations; supports `--last N`, `--key`, `--json` flags
- `envref config show` — prints resolved effective config (plain, JSON, table formats)
- `envref completion <shell>` — generates shell completion scripts (bash, zsh, fish, powershell)
- `envref edit` — opens .env files in `$VISUAL`/`$EDITOR`
- `envref vault init/lock/unlock/export/import` — full vault management
- **`envref sync push/pull`** — sync secrets via shared age-encrypted git file; push exports all secrets from a backend encrypted for multiple recipients; pull decrypts and imports into a backend with skip/force semantics
- **`envref sync push --to-team`** — encrypts for all team members defined in `.envref.yaml`
- **`envref team list/add/remove`** — manage team members and their age public keys in `.envref.yaml`
- **`envref onboard`** — interactive setup for new team members; identifies missing/unresolved secrets, prompts for values, stores in backend; supports `--dry-run`, `--profile`, `--backend` flags; also checks `.env.example` for missing keys
- **Seven backend types:** `KeychainBackend` (OS keychain via go-keyring), `VaultBackend` (local SQLite + age encryption), `OnePasswordBackend` (1Password via `op` CLI), `AWSSSMBackend` (AWS SSM Parameter Store via `aws` CLI), `OCIVaultBackend` (Oracle OCI Vault via `oci` CLI), `HashiVaultBackend` (HashiCorp Vault via `vault` CLI), and `PluginBackend` (external executables via JSON protocol)
- **Backend documentation:** Comprehensive docs for all 7 backend types with configuration tables, prerequisites, usage examples, multi-backend patterns, plugin protocol spec, and troubleshooting
- **Audit log:** All secret mutations (set, delete, generate, rotate, copy, import) are logged to `.envref.audit.log` with timestamp, user, operation, key, backend, project, and profile; `AuditBackend` wrapper available for programmatic use
- **Team config:** `.envref.yaml` supports a `team` section with member names and age public keys; validated on load (unique names/keys, age1... format)
- **Nested references:** `${ref://secrets/key}` in values and embedded `ref://` URIs after interpolation are resolved via a second pass in the resolution pipeline
- **Security hardening:** Vault passphrase stored as `[]byte` (clearable), zeroed on Close; decrypted plaintext bytes cleared after use
- **Comprehensive README** with architecture diagram, resolution pipeline, project structure, vault docs, benchmarks, and animated SVG terminal demo
- **docs/ directory** with four usage guides: getting-started, direnv-integration, profiles, secret-backends
- **Distribution:** Homebrew tap via GoReleaser `brews` config; Nix flake with `buildGoModule` package and dev shell
- **Project website:** Single-page landing page in `site/` with terminal-inspired dark theme, feature overview, backend comparison, CLI reference, installation methods, and documentation links; GitHub Pages deploy workflow in `.github/workflows/pages.yml`
- **Animated SVG demo:** `site/demo.svg` with 4-step animated terminal showing init, ref://, secret store, and resolve workflow; embedded in README.md
- **MIT LICENSE** file included
- `.env` file parser with full quote/multiline/comment/BOM/CRLF support
- `ref://` URI parser with `FindAll` for embedded ref discovery, `Backend` interface, `Registry`, `NamespacedBackend`
- GoReleaser config, GitHub Actions CI + release pipelines, Makefile with coverage targets
- Comprehensive test coverage across all packages
- Fuzz tests for .env parser, ref:// URI parser, and variable interpolation
- **Integration tests** for keychain backend (12 tests) with `//go:build integration` tag
- **Direnv integration tests** (22 tests) with `//go:build integration` tag
- **CI jobs** for keychain integration tests (macOS) and direnv integration tests (ubuntu + macOS with direnv installed)
- All checks pass: `go build`, `go vet`, `go test`, `golangci-lint`

## Known Issues
- None currently
