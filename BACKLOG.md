# Backlog

<!-- Format: - [STATUS] PRIORITY | ID | Title -->
<!-- Status: TODO, IN_PROGRESS, DONE, BLOCKED -->
<!-- Priority: P0 (hotfix), P1 (high), P2 (normal), P3 (low) -->
<!-- Agent picks highest-priority TODO each iteration -->

---

## Phase 0 — Project Bootstrap

- [DONE] P1 | ENV-001 | Initialize Go module (`go mod init github.com/xcke/envref`)
- [DONE] P1 | ENV-002 | Set up project directory structure (`cmd/`, `internal/`, `pkg/`)
- [DONE] P1 | ENV-003 | Add Cobra CLI scaffold with root command and version flag
- [DONE] P1 | ENV-004 | Set up Makefile with `build`, `test`, `lint`, `install` targets
- [DONE] P2 | ENV-005 | Add `.goreleaser.yml` for cross-platform binary releases
- [DONE] P2 | ENV-006 | Set up CI pipeline (GitHub Actions: test, lint, build)
- [DONE] P2 | ENV-007 | Add `README.md` with project overview, install instructions, and quickstart
- [DONE] P3 | ENV-008 | Add `LICENSE` (MIT or Apache 2.0)
- [DONE] P3 | ENV-009 | Add `.gitignore` for Go projects

---

## Phase 1 — Core .env Parsing & Merging

- [DONE] P1 | ENV-010 | Implement `.env` file parser (handle quotes, multiline, comments, empty lines)
- [DONE] P1 | ENV-011 | Implement `.env.local` parser (same format, gitignored overrides)
- [DONE] P1 | ENV-012 | Implement merge logic: `.env` ← `.env.local` (local wins on conflict)
- [DONE] P1 | ENV-013 | Detect and tag `ref://` values as unresolved secret references
- [DONE] P1 | ENV-014 | Add `envref get <KEY>` command — print single resolved value
- [DONE] P1 | ENV-015 | Add `envref set <KEY>=<VALUE>` command — write to `.env` or `.env.local`
- [DONE] P2 | ENV-016 | Handle edge cases: duplicate keys (last wins + warn), BOM, CRLF, trailing whitespace
- [DONE] P2 | ENV-017 | Add `envref list` command — print all key-value pairs (mask secrets by default)
- [DONE] P2 | ENV-018 | Support variable interpolation within `.env` (`DB_URL=postgres://${DB_HOST}:${DB_PORT}/app`)
- [DONE] P3 | ENV-019 | Add `--format` flag to output commands (`json`, `shell`, `table`)

---

## Phase 2 — Config File (`.envref.yaml`)

- [DONE] P1 | ENV-020 | Define `.envref.yaml` schema (project name, secret backends, profiles)
- [DONE] P1 | ENV-021 | Implement config loader using Viper (project root discovery via `.envref.yaml`)
- [DONE] P1 | ENV-022 | Add `envref init` command — scaffold `.envref.yaml`, `.env`, `.envrc`, `.gitignore` entries
- [DONE] P2 | ENV-023 | Support config inheritance / defaults (global `~/.config/envref/config.yaml` + project-level)
- [DONE] P2 | ENV-024 | Validate config on load — clear errors for missing/malformed fields
- [DONE] P3 | ENV-025 | Add `envref config show` command — print resolved effective config

---

## Phase 3 — Secret Backends

### 3a — Backend Interface

- [DONE] P1 | ENV-030 | Define `SecretBackend` interface (`Get(key) → value`, `Set(key, value)`, `Delete(key)`, `List()`)
- [DONE] P1 | ENV-031 | Implement backend registry with ordered fallback chain (try backend 1 → 2 → 3)
- [DONE] P1 | ENV-032 | Namespace secrets per project (`<project>/<key>`) to avoid collisions

### 3b — OS Keychain Backend

- [DONE] P1 | ENV-033 | Implement macOS Keychain backend via `go-keyring`
- [DONE] P1 | ENV-034 | Implement Linux `libsecret` / `secret-service` backend via `go-keyring`
- [DONE] P1 | ENV-035 | Implement Windows Credential Manager backend via `go-keyring`
- [DONE] P2 | ENV-036 | Handle keychain access errors gracefully (locked keychain, permissions, missing daemon)

### 3c — Local Encrypted Vault Backend (fallback)

- [DONE] P2 | ENV-040 | Implement local vault backend (SQLite + age encryption at `~/.config/envref/vault.db`)
- [DONE] P2 | ENV-041 | Add master password or age key setup flow on first use
- [DONE] P2 | ENV-042 | Add vault `lock` / `unlock` commands
- [DONE] P3 | ENV-043 | Add vault export/import for backup/migration

### 3d — External Backends (plugins)

- [DONE] P3 | ENV-050 | Implement 1Password CLI backend (`op` CLI wrapper)
- [TODO] P3 | ENV-052 | Implement AWS SSM Parameter Store backend and Oracle OCI Vault store backend
- [TODO] P3 | ENV-053 | Implement HashiCorp Vault backend
- [DONE] P3 | ENV-054 | Define plugin interface for community-contributed backends

---

## Phase 4 — Secret Management Commands

- [DONE] P1 | ENV-060 | Add `envref secret set <key>` — prompt for value (hidden input), store in backend
- [DONE] P1 | ENV-061 | Add `envref secret get <key>` — retrieve and print from backend
- [DONE] P1 | ENV-062 | Add `envref secret delete <key>` — remove from backend with confirmation
- [DONE] P1 | ENV-063 | Add `envref secret list` — list stored secret keys (no values) for current project
- [DONE] P2 | ENV-064 | Add `envref secret set <key> --value <val>` — non-interactive mode for scripting
- [DONE] P2 | ENV-065 | Add `envref secret generate <key>` — generate random secret (configurable length, charset)
- [DONE] P2 | ENV-066 | Add `envref secret copy <key> --from <project>` — copy secret between projects
- [DONE] P3 | ENV-067 | Add `envref secret rotate <key>` — generate new value, store old in history

---

## Phase 5 — Reference Resolution Engine

- [DONE] P1 | ENV-070 | Implement `ref://` URI parser (`ref://secrets/<key>`, `ref://keychain/<key>`, `ref://ssm/<path>`)
- [DONE] P1 | ENV-071 | Implement resolution pipeline: parse → resolve refs → merge → output
- [DONE] P1 | ENV-072 | Add `envref resolve` command — output fully resolved KEY=VALUE pairs to stdout
- [DONE] P1 | ENV-073 | Add `--direnv` output format (`export KEY=VALUE` lines for `eval`)
- [DONE] P2 | ENV-074 | Handle resolution failures gracefully — partial resolve with clear error per failed key
- [DONE] P2 | ENV-075 | Add `--strict` flag — fail entirely if any ref can't be resolved
- [DONE] P2 | ENV-076 | Cache resolved values in memory during a single resolve call (avoid duplicate backend hits)
- [TODO] P3 | ENV-077 | Support nested references (`DB_URL=postgres://${ref://secrets/db_user}:${ref://secrets/db_pass}@localhost/app`)

---

## Phase 6 — direnv Integration

- [DONE] P1 | ENV-080 | Add `envref init --direnv` — generate `.envrc` with `eval "$(envref resolve --direnv)"`
- [DONE] P1 | ENV-081 | Ensure `envref resolve --direnv` output is compatible with direnv `eval`
- [DONE] P2 | ENV-082 | Handle direnv trust/allow flow — prompt user to run `direnv allow` after init
- [DONE] P2 | ENV-083 | Add `envref resolve --watch` — output on file changes (for direnv reload triggers)
- [DONE] P2 | ENV-084 | Ensure fast startup (<50ms) for resolve command — benchmark and optimize
- [TODO] P3 | ENV-085 | Add documentation: direnv integration guide with examples

---

## Phase 7 — Profiles (Environment Switching)

- [DONE] P1 | ENV-090 | Define profile structure (`.env.development`, `.env.staging`, `.env.production`)
  Plan: Added ActiveProfile field to Config, ProfileEnvFile/HasProfile/EffectiveProfile methods,
  --profile flag on resolve, --profile-file on get/list, updated loadAndMergeEnv for 3-layer merge.
- [DONE] P1 | ENV-091 | Add `envref profile list` — show available profiles
- [DONE] P1 | ENV-092 | Add `envref profile use <name>` — set active profile (stored in `.envref.yaml` or `.envref.local`)
- [DONE] P1 | ENV-093 | Update resolve pipeline to load base `.env` ← profile `.env.<name>` ← `.env.local`
- [DONE] P2 | ENV-094 | Add `envref profile create <name>` — scaffold new profile file
- [DONE] P2 | ENV-095 | Add `envref profile diff <a> <b>` — show key/value differences between profiles
- [DONE] P2 | ENV-096 | Support profile-scoped secrets (`<project>/<profile>/<key>`)
- [TODO] P3 | ENV-097 | Add `envref profile export <name>` — export profile as JSON for CI/CD

---

## Phase 8 — Validation & Health Checks

- [DONE] P1 | ENV-100 | Add `envref validate` — check `.env` against `.env.example` schema (missing/extra keys)
- [DONE] P1 | ENV-101 | Add `envref status` — show resolved/missing/unresolved overview with actionable hints
- [DONE] P2 | ENV-102 | Add `envref doctor` — check for common issues:
  - Duplicate keys
  - Trailing whitespace
  - Unquoted values with spaces
  - Empty values without explicit intent
  - `.env` not in `.gitignore`
  - `.envrc` not trusted by direnv
- [DONE] P2 | ENV-103 | Add `.env.schema.json` support — type checking (string, number, boolean, url, enum)
- [DONE] P2 | ENV-104 | Add `envref validate --ci` — exit code 1 on failure for CI pipelines
- [DONE] P3 | ENV-105 | Add `envref audit` — warn about secrets that might be in plaintext `.env` (entropy analysis, pattern matching for API keys/tokens)

---

## Phase 9 — Developer Experience Polish

- [DONE] P2 | ENV-110 | Add shell completions (`envref completion bash|zsh|fish`)
- [DONE] P2 | ENV-111 | Add colorized, human-friendly terminal output (with `--no-color` flag)
- [DONE] P2 | ENV-112 | Add `--quiet` / `--verbose` / `--debug` global flags
- [DONE] P2 | ENV-113 | Implement fuzzy key matching in error messages ("KEY not found, did you mean API_KEY?")
- [DONE] P2 | ENV-114 | Add `envref run -- <command>` — inject resolved env vars and exec (alternative to direnv)
- [DONE] P2 | ENV-115 | Add `envref edit` — open `.env` in `$EDITOR`
- [DEFER] P3 | ENV-116 | Add `envref import <file|url>` — import env vars from another format (Docker, Vercel, Heroku)
- [DEFER] P3 | ENV-117 | Add `envref export --format <docker|github|gitlab>` — export for CI platforms
- [DEFER] P3 | ENV-118 | Add interactive TUI mode for browsing/editing (Bubble Tea)

---

## Phase 10 — Team & Sharing Features

- [TODO] P3 | ENV-120 | Add `envref secret share <key> --to <teammate>` — encrypt secret for specific recipient (age public key)
- [TODO] P3 | ENV-121 | Add `envref sync push` / `envref sync pull` — sync secrets via shared encrypted git file
- [TODO] P3 | ENV-122 | Add `.envref.team.yaml` — team-level config with member public keys
- [TODO] P3 | ENV-123 | Add `envref onboard` — interactive setup for new team members (walk through all missing secrets)
- [TODO] P3 | ENV-124 | Add audit log — track who set/changed which secrets (local git-backed log)

---

## Phase 11 — Documentation & Distribution

- [DONE] P2 | ENV-130 | Write comprehensive README with architecture diagram
- [DONE] P2 | ENV-131 | Add `docs/` with usage guides per feature (getting started, direnv, profiles, backends)
- [DONE] P2 | ENV-132 | Set up GoReleaser for GitHub Releases (Linux, macOS, Windows binaries)
- [DONE] P2 | ENV-133 | Add Homebrew tap formula
- [TODO] P3 | ENV-134 | Add AUR package
- [TODO] P3 | ENV-135 | Add Scoop manifest (Windows)
- [TODO] P3 | ENV-136 | Add Nix flake
- [TODO] P3 | ENV-137 | Create project website / landing page Shell inspired Github Page landing page for the tool, with excelent documentation
- [TODO] P3 | ENV-138 | Record demo GIF / asciinema for README

---

## Phase 12 — Testing & Quality

- [DONE] P1 | ENV-140 | Unit tests for `.env` parser (edge cases,**** multiline, quotes, comments)
- [DONE] P1 | ENV-141 | Unit tests for merge logic (override precedence, ref detection)
- [DONE] P1 | ENV-142 | Unit tests for resolution pipeline (mock backends)
- [DONE] P1 | ENV-143 | Integration tests for CLI commands (cobra test helpers)
- [DONE] P2 | ENV-144 | Integration tests for keychain backend (platform-specific CI matrix)
- [DONE] P2 | ENV-145 | Integration tests for direnv integration (end-to-end shell test)
- [DONE] P2 | ENV-146 | Benchmark `envref resolve` — target <50ms for 100 vars
- [DONE] P2 | ENV-147 | Set up code coverage reporting
- [DONE] P3 | ENV-148 | Fuzz testing for `.env` parser
- [DONE] P3 | ENV-149 | Security audit of secret handling (memory zeroing, no secrets in logs/errors)
---

## Milestone Summary

| Milestone              | Phases                                  | Goal                                                           |
| ---------------------- | --------------------------------------- | -------------------------------------------------------------- |
| **v0.1.0 — MVP**       | 0, 1, 2, 3a, 3b, 4, 5, 6, 12 (P1 tests) | Core resolve + keychain + direnv. Usable for solo dev.         |
| **v0.2.0 — Profiles**  | 7, 8                                    | Multi-environment support, validation, health checks.          |
| **v0.3.0 — DX Polish** | 9, 12 (P2 tests)                        | Shell completions, run command, fuzzy matching, import/export. |
| **v0.4.0 — Backends**  | 3c, 3d                                  | Local vault fallback, 1Password, Bitwarden, AWS SSM, Vault.    |
| **v0.5.0 — Teams**     | 10                                      | Secret sharing, sync, onboarding, audit log.                   |
| **v1.0.0 — Stable**    | 11, 12 (remaining)                      | Docs, distribution, full test coverage, security audit.        |