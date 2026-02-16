# Profiles

Profiles let you maintain separate configurations for different environments — development, staging, production, or any custom setup. Each profile has its own `.env.<name>` file that overrides the base `.env`.

## How profiles work

envref uses a three-layer merge strategy:

```
.env  <-  .env.<profile>  <-  .env.local
```

1. **`.env`** — Base configuration, committed to git. Contains shared defaults and `ref://` secret references.
2. **`.env.<profile>`** — Profile-specific overrides (e.g., `.env.staging`). Committed to git.
3. **`.env.local`** — Personal overrides. Gitignored, never committed.

Each layer overrides keys from the previous layer. The last value wins.

### Example

```dotenv
# .env (base)
APP_NAME=my-app
APP_PORT=3000
LOG_LEVEL=info
DATABASE_URL=ref://secrets/database_url
```

```dotenv
# .env.staging
APP_PORT=8080
LOG_LEVEL=debug
DATABASE_URL=ref://secrets/database_url
```

```dotenv
# .env.local (personal)
APP_PORT=9999
```

With the `staging` profile active, `envref resolve` produces:

```
APP_NAME=my-app           <- from .env (no override)
APP_PORT=9999             <- from .env.local (overrides staging)
LOG_LEVEL=debug           <- from .env.staging (overrides base)
DATABASE_URL=<resolved>   <- ref:// resolved from backend
```

## Managing profiles

### Create a profile

```bash
envref profile create staging
```

This creates `.env.staging` with a starter template. You can also:

```bash
# Copy from an existing file as a starting point
envref profile create staging --from .env

# Register the profile in .envref.yaml
envref profile create staging --register

# Overwrite an existing profile file
envref profile create staging --force

# Use a custom file path
envref profile create staging --env-file configs/staging.env
```

Profile names cannot contain dots or slashes, and `local` is reserved (used by `.env.local`).

### List available profiles

```bash
envref profile list
```

Output shows all profiles with their status:

```
* development  .env.development      (config, file)
  staging      .env.staging          (config, file)
  production   .env.production       (no file)
```

- `*` marks the active profile
- `config` means the profile is registered in `.envref.yaml`
- `file` means the `.env.<name>` file exists on disk
- `no file` means the profile is configured but the file hasn't been created

### Set the active profile

```bash
envref profile use staging
```

This updates `active_profile` in `.envref.yaml`. The active profile is used by default in `envref resolve`, `envref status`, and other commands.

To deactivate the current profile:

```bash
envref profile use --clear
```

### Compare profiles

```bash
envref profile diff staging production
```

Output shows differences between two profiles:

```
+ API_URL=https://api.prod.example.com   (only in production)
- DEBUG=true                              (only in staging)
~ LOG_LEVEL: debug -> warn                (changed)
```

Markers:
- `+` — key only in the second profile
- `-` — key only in the first profile
- `~` — key in both but with different values

Supports `--format json` and `--format table` for alternative output.

## Profiles in configuration

Register profiles in `.envref.yaml`:

```yaml
project: my-app

profiles:
  development:
    env_file: .env.development
  staging:
    env_file: .env.staging
  production:
    env_file: .env.production

active_profile: development
```

You can also use convention-based discovery — envref detects `.env.<name>` files on disk even if they're not registered in config.

## Profile-scoped secrets

Secrets can be scoped to a specific profile so that different environments use different secret values for the same key.

### Storing profile-scoped secrets

```bash
# Store a staging-specific database password
envref secret set db_password --profile staging

# Store a production-specific database password
envref secret set db_password --profile production
```

These are stored under separate namespaces in the backend:

```
my-app/staging/db_password
my-app/production/db_password
```

### How profile-scoped lookup works

When resolving `ref://secrets/db_password` with `--profile staging`:

1. First, envref looks up `my-app/staging/db_password` (profile-scoped)
2. If not found, falls back to `my-app/db_password` (project-scoped)

This lets you have a default secret at the project level and override it per profile only when needed.

### Managing profile-scoped secrets

All secret commands support the `--profile` flag:

```bash
envref secret set api_key --profile staging
envref secret get api_key --profile staging
envref secret delete api_key --profile staging
envref secret list --profile staging
envref secret generate api_key --profile staging
```

## Using profiles with resolve

```bash
# Use the active profile (set via envref profile use)
envref resolve

# Override with a specific profile
envref resolve --profile staging

# Combine with direnv output
envref resolve --profile staging --direnv

# Strict mode — fail if any reference can't resolve
envref resolve --profile production --strict
```

## Using profiles with other commands

```bash
# Get a value with profile merge
envref get APP_PORT --profile-file .env.staging

# Show status for a specific profile
envref status --profile staging

# Validate with a profile
envref validate --profile-file .env.staging

# Run a command with a specific profile
envref run --profile staging -- ./deploy.sh
```

## Recommended workflow

1. **Create profiles** for each environment:
   ```bash
   envref profile create development --register
   envref profile create staging --register
   envref profile create production --register
   ```

2. **Set defaults** in `.env` (shared across all environments)

3. **Override per profile** in `.env.<name>` files

4. **Store profile-scoped secrets** where they differ:
   ```bash
   envref secret set db_password --profile staging
   envref secret set db_password --profile production
   ```

5. **Switch profiles** during development:
   ```bash
   envref profile use staging
   envref resolve  # uses staging profile automatically
   ```

6. **Use in CI/CD** with explicit `--profile`:
   ```bash
   envref resolve --profile production --strict
   ```

## See also

- [Getting Started](getting-started.md) — basic envref setup
- [direnv Integration](direnv-integration.md) — automatic environment loading with profiles
- [Secret Backends](secret-backends.md) — configure where secrets are stored
