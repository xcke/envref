# envref

Separate config from secrets in `.env` files. Secrets stay in your OS keychain (or other backends) — the `.env` file holds only `ref://` references, making it safe to commit.

```
.env        = config values + ref:// secret references (committed to git)
secrets     = OS keychain / password manager / cloud vault
envref resolve = config + resolved secrets → direnv / shell env
```

## Install

### From source

```bash
go install github.com/xcke/envref/cmd/envref@latest
```

### From releases

Download the latest binary from [GitHub Releases](https://github.com/xcke/envref/releases) for your platform (Linux, macOS, Windows; amd64/arm64).

### Build from source

```bash
git clone https://github.com/xcke/envref.git
cd envref
make build
# Binary is at ./build/envref
```

## Quickstart

### 1. Initialize a project

```bash
cd my-project
envref init --project my-app
```

This creates:
- `.envref.yaml` — project config (project name, secret backends)
- `.env` — environment variables with example entries
- `.env.local` — local overrides (gitignored)

### 2. Add secret references to `.env`

```dotenv
# .env — safe to commit
APP_NAME=my-app
APP_PORT=3000
DATABASE_URL=ref://secrets/database_url
API_KEY=ref://secrets/api_key
```

### 3. Store the actual secrets

```bash
envref secret set database_url
# Prompts for the value (hidden input)

envref secret set api_key --value sk-abc123
# Non-interactive mode
```

### 4. Resolve and use

```bash
# Print resolved KEY=VALUE pairs
envref resolve

# Inject into a command
envref run -- node server.js

# Use with direnv
envref init --direnv
# This generates .envrc with: eval "$(envref resolve --direnv)"
```

## How it works

envref uses a layered merge strategy:

```
.env  ←  .env.<profile>  ←  .env.local
```

1. `.env` is your base config — committed to git, contains `ref://` references for secrets
2. `.env.<profile>` (optional) overrides per environment (development, staging, production)
3. `.env.local` is your personal overrides — gitignored, never committed

During resolution, `ref://` URIs are resolved through configured secret backends (OS keychain by default). Variable interpolation (`${VAR}`) is supported within values.

## Commands

| Command | Description |
|---------|-------------|
| `envref init` | Scaffold a new envref project |
| `envref get <KEY>` | Print the value of an environment variable |
| `envref set <KEY>=<VALUE>` | Set a variable in a .env file |
| `envref list` | List all environment variables |
| `envref resolve` | Resolve all references and output KEY=VALUE pairs |
| `envref run -- <cmd>` | Run a command with resolved env vars injected |
| `envref secret set\|get\|delete\|list` | Manage secrets in backends |
| `envref secret generate <key>` | Generate and store a random secret |
| `envref secret copy <key> --from <project>` | Copy a secret between projects |
| `envref profile list\|use\|create\|diff` | Manage environment profiles |
| `envref validate` | Check .env against .env.example schema |
| `envref status` | Show environment overview with actionable hints |
| `envref doctor` | Scan .env files for common issues |
| `envref config show` | Print resolved effective config |
| `envref edit` | Open .env files in your editor |
| `envref completion <shell>` | Generate shell completion scripts |
| `envref version` | Print the version |

## Profiles

Profiles let you maintain per-environment configs:

```bash
# Create a staging profile
envref profile create staging

# Switch to it
envref profile use staging

# Resolve with a specific profile
envref resolve --profile production

# Compare two profiles
envref profile diff staging production
```

Profile-scoped secrets are supported — `envref secret set db_pass --profile staging` stores the secret under `<project>/staging/db_pass`, separate from the default scope.

## Validation

Check your `.env` against `.env.example` to catch missing variables:

```bash
envref validate
```

For type-level validation, create a `.env.schema.json`:

```json
{
  "APP_PORT": { "type": "port", "required": true },
  "API_URL": { "type": "url", "required": true },
  "DEBUG": { "type": "boolean" },
  "LOG_LEVEL": { "type": "enum", "values": ["debug", "info", "warn", "error"] }
}
```

```bash
envref validate --schema
```

Use `--ci` in pipelines for exit code 1 on failure:

```bash
envref validate --ci
```

## direnv integration

envref integrates with [direnv](https://direnv.net) so your environment is automatically resolved when you `cd` into a project:

```bash
envref init --direnv
direnv allow
```

This generates an `.envrc` that runs `eval "$(envref resolve --direnv)"` on directory entry.

## Global flags

| Flag | Description |
|------|-------------|
| `--quiet`, `-q` | Suppress informational output (errors only) |
| `--verbose` | Show additional detail |
| `--debug` | Show debug information |
| `--no-color` | Disable colorized output (also respects `NO_COLOR` env var) |

## Configuration

Project config lives in `.envref.yaml`:

```yaml
project: my-app
secret_backend: keychain
profiles:
  - development
  - staging
  - production
active_profile: development
```

Global defaults can be set at `~/.config/envref/config.yaml` — project config takes precedence.

## Development

Requires Go 1.24+.

```bash
make build     # Compile to ./build/envref
make test      # Run tests with race detector
make lint      # Run golangci-lint
make check     # vet + lint + test
make install   # Install to $GOPATH/bin
```

## License

See [LICENSE](LICENSE) for details.
