# Getting Started with envref

This guide walks you through installing envref, initializing a project, storing secrets, and resolving your environment.

## Installation

### Homebrew (macOS / Linux)

```bash
brew install xcke/tap/envref
```

### From source (requires Go 1.24+)

```bash
go install github.com/xcke/envref/cmd/envref@latest
```

### From GitHub Releases

Download the latest binary for your platform from [GitHub Releases](https://github.com/xcke/envref/releases).

### Build from source

```bash
git clone https://github.com/xcke/envref.git
cd envref
make build
# Binary is at ./build/envref
```

Verify the installation:

```bash
envref version
```

## Initialize a project

Run `envref init` inside your project directory:

```bash
cd my-project
envref init --project my-app
```

This creates three files:

| File | Purpose | Committed to git? |
|------|---------|-------------------|
| `.envref.yaml` | Project config (name, backends, profiles) | Yes |
| `.env` | Base environment variables with example entries | Yes |
| `.env.local` | Personal overrides | No (gitignored) |

The `.env.local` file is automatically added to `.gitignore`.

### Flags

| Flag | Description |
|------|-------------|
| `-p`, `--project <name>` | Project name (defaults to current directory name) |
| `--direnv` | Also generate `.envrc` for direnv integration |
| `--force` | Overwrite existing files |
| `--dir <path>` | Target directory (defaults to current working directory) |

## Add secret references

Edit `.env` to replace secret values with `ref://` references:

```dotenv
# .env — safe to commit
APP_NAME=my-app
APP_PORT=3000
DATABASE_URL=ref://secrets/database_url
API_KEY=ref://secrets/api_key
```

The `ref://secrets/<key>` syntax tells envref to look up `<key>` from the configured secret backend at resolve time. The `.env` file never contains actual secret values.

## Store secrets

Use `envref secret set` to store secrets in your OS keychain (the default backend):

```bash
# Interactive — prompts for the value (hidden input)
envref secret set database_url

# Non-interactive — pass the value directly
envref secret set api_key --value sk-abc123
```

You can also generate random secrets:

```bash
envref secret generate session_secret --length 64
```

### Secret commands overview

| Command | Description |
|---------|-------------|
| `envref secret set <key>` | Store a secret (interactive prompt) |
| `envref secret set <key> --value <val>` | Store a secret (non-interactive) |
| `envref secret get <key>` | Retrieve and print a secret value |
| `envref secret delete <key>` | Remove a secret (with confirmation) |
| `envref secret list` | List all secret keys for the current project |
| `envref secret generate <key>` | Generate and store a random secret |
| `envref secret copy <key> --from <project>` | Copy a secret from another project |

## Resolve your environment

The `envref resolve` command merges your `.env` files, interpolates variables, and resolves all `ref://` references:

```bash
# Print resolved KEY=VALUE pairs
envref resolve

# Output as JSON
envref resolve --format json

# Export format for shell eval
envref resolve --direnv
```

### Inject into a running command

Use `envref run` to launch a subprocess with the resolved environment:

```bash
envref run -- node server.js
envref run -- docker compose up
```

The `--` separates envref flags from the command to run.

### Use with direnv

For automatic resolution on `cd`:

```bash
envref init --direnv
direnv allow
```

This generates an `.envrc` containing `eval "$(envref resolve --direnv)"`. See the [direnv integration guide](direnv-integration.md) for details.

## Read and write individual variables

```bash
# Get a single value
envref get APP_PORT

# Set a value in .env
envref set APP_PORT=8080

# Set a value in .env.local (personal override)
envref set DB_HOST=localhost --local

# List all merged variables
envref list
```

The `list` command masks secret references by default (`ref://***`). Use `--show-secrets` to display the full `ref://` URIs.

## Output formats

Most commands support `--format` with these options:

| Format | Output |
|--------|--------|
| `plain` (default) | `KEY=VALUE` one per line |
| `shell` | `export KEY=VALUE` with shell-safe quoting |
| `json` | JSON array of `{"key": ..., "value": ...}` objects |
| `table` | Aligned columns with headers |

## Check your environment

```bash
# Validate .env against .env.example
envref validate

# Show environment overview with actionable hints
envref status

# Scan for common issues (duplicate keys, trailing whitespace, etc.)
envref doctor
```

## Edit environment files

Open `.env` files directly in your editor:

```bash
envref edit                     # edit .env
envref edit --local            # edit .env.local
envref edit --profile staging  # edit .env.staging
envref edit --config           # edit .envref.yaml
```

Uses `$VISUAL`, then `$EDITOR`, then falls back to `vi`.

## Global flags

These flags are available on all commands:

| Flag | Description |
|------|-------------|
| `-q`, `--quiet` | Suppress informational output (errors only) |
| `--verbose` | Show additional detail |
| `--debug` | Show debug information |
| `--no-color` | Disable colorized output (also respects `NO_COLOR`) |

## Shell completions

Generate tab-completion scripts for your shell:

```bash
# Bash (Linux)
envref completion bash > /etc/bash_completion.d/envref

# Bash (macOS with Homebrew)
envref completion bash > $(brew --prefix)/etc/bash_completion.d/envref

# Zsh
envref completion zsh > "${fpath[1]}/_envref"

# Fish
envref completion fish > ~/.config/fish/completions/envref.fish

# PowerShell
envref completion powershell >> $PROFILE
```

## Next steps

- [direnv Integration](direnv-integration.md) — automatic environment loading
- [Profiles](profiles.md) — manage development, staging, and production configs
- [Secret Backends](secret-backends.md) — keychain, vault, and backend configuration
