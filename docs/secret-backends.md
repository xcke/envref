# Secret Backends

envref resolves `ref://` secret references through configurable backends. Backends are tried in order — the first one that has the requested key wins.

## Built-in backends

envref ships with two backends:

| Backend | Storage | Encryption | Setup | Use case |
|---------|---------|------------|-------|----------|
| `keychain` | OS keychain (macOS Keychain, Linux Secret Service, Windows Credential Manager) | OS-managed | None (default) | Development machines with a desktop environment |
| `vault` | Local SQLite at `~/.config/envref/vault.db` | age scrypt per-value | `envref vault init` | Headless servers, containers, CI |

### Keychain backend

The keychain backend uses your operating system's native credential store via [go-keyring](https://github.com/zalando/go-keyring):

- **macOS**: Keychain Access
- **Linux**: Secret Service API (GNOME Keyring, KWallet)
- **Windows**: Windows Credential Manager

No configuration is needed — it works out of the box on systems with a desktop environment. Secrets are stored under the service name `envref` with namespaced keys.

### Vault backend

The vault backend is a local encrypted store for environments where OS keychain is unavailable (SSH servers, Docker containers, CI runners).

Each secret is individually encrypted using [age](https://age-encryption.org/) with scrypt-based key derivation from a master passphrase. Secrets are stored in a SQLite database at `~/.config/envref/vault.db`.

## Configuration

Backends are configured in `.envref.yaml`:

```yaml
project: my-app

backends:
  - name: keychain
    type: keychain
  - name: vault
    type: encrypted-vault
    config:
      path: ~/.config/envref/vault.db
```

The `backends` list defines the fallback chain — backends are tried in order when resolving a `ref://` reference. If the first backend doesn't have the key, the next one is tried.

### Default configuration

If no `backends` section is present in `.envref.yaml`, envref uses the keychain backend by default.

### Backend-specific options

#### Keychain

No additional configuration options. Uses the default OS keychain.

#### Vault

| Option | Description | Default |
|--------|-------------|---------|
| `path` | Path to the SQLite database file | `~/.config/envref/vault.db` |

The passphrase is provided interactively (prompted at use time) or via the `ENVREF_VAULT_PASSPHRASE` environment variable for non-interactive use.

## Setting up the vault

### Initialize

```bash
envref vault init
```

This creates the SQLite database and sets up the encryption. You'll be prompted to create a master passphrase (with confirmation).

For non-interactive initialization (CI/scripts):

```bash
ENVREF_VAULT_PASSPHRASE=your-passphrase envref vault init
```

### Lock and unlock

The vault can be locked to prevent all secret access:

```bash
# Lock the vault (requires passphrase verification)
envref vault lock

# Unlock the vault (requires passphrase verification)
envref vault unlock
```

Lock state persists across CLI invocations. When locked, all secret get/set/delete/list operations against the vault backend will fail.

## How secret lookup works

### Namespace format

Secrets are stored with a project namespace to prevent collisions between projects:

```
Without profile:  <project>/<key>
With profile:     <project>/<profile>/<key>
```

For example, with `project: my-app` in `.envref.yaml`:

```
envref secret set api_key              -> stored as: my-app/api_key
envref secret set api_key --profile staging -> stored as: my-app/staging/api_key
```

### Resolution order

When `envref resolve` encounters a `ref://secrets/api_key` reference:

1. **Parse** the `ref://` URI to extract the key name
2. **Try each backend** in order (as configured in `backends`)
3. **With profile**: try `<project>/<profile>/<key>` first, fall back to `<project>/<key>`
4. **Without profile**: look up `<project>/<key>` directly
5. **First hit wins** — stop at the first backend that returns a value

### Caching

During a single `envref resolve` call, resolved values are cached in memory to avoid hitting the backend multiple times for the same key. The cache is not persisted between invocations.

## Storing secrets

### Interactive mode

```bash
envref secret set database_url
# Enter secret value: (hidden input)
```

### Non-interactive mode

```bash
envref secret set database_url --value "postgres://user:pass@host/db"
```

### Specifying a backend

By default, secrets are stored in the first configured backend. Use `--backend` to target a specific one:

```bash
envref secret set api_key --backend vault
```

### Generating random secrets

```bash
# Default: 32 characters, alphanumeric
envref secret generate session_secret

# Custom length and charset
envref secret generate api_key --length 64 --charset hex

# Print the generated value
envref secret generate api_key --print
```

Available character sets:

| Charset | Characters |
|---------|-----------|
| `alphanumeric` (default) | a-z, A-Z, 0-9 |
| `ascii` | alphanumeric + common symbols |
| `hex` | 0-9, a-f |
| `base64` | standard base64 encoding |

Length range: 1-1024 characters. Uses cryptographic RNG (`crypto/rand`).

## Managing secrets

### Retrieve a secret

```bash
envref secret get api_key
```

With profile scope:

```bash
envref secret get api_key --profile staging
```

Profile lookup tries the profile-scoped key first, then falls back to the project-scoped key.

### List secrets

```bash
# List all project secrets
envref secret list

# List profile-scoped secrets
envref secret list --profile staging
```

Lists key names only — values are never printed by `list`.

### Delete a secret

```bash
envref secret delete api_key
# Confirm deletion? (y/N)

# Skip confirmation
envref secret delete api_key --force
```

### Copy between projects

```bash
envref secret copy api_key --from other-project
```

This reads `other-project/api_key` and writes it to `<current-project>/api_key`.

Copy with profile scopes:

```bash
envref secret copy api_key --from other-project --from-profile production --profile staging
```

## Using ref:// in .env files

Reference syntax:

```dotenv
# Simple secret reference
API_KEY=ref://secrets/api_key

# Variable interpolation works alongside references
DATABASE_URL=ref://secrets/database_url
DB_DISPLAY=${DATABASE_URL}
```

The `ref://secrets/<key>` format is the standard reference syntax. The `secrets` segment indicates the secret backend system.

## Choosing a backend

| Scenario | Recommended backend |
|----------|-------------------|
| Local development with desktop | `keychain` (default, zero setup) |
| Headless server / SSH | `vault` |
| Docker container | `vault` with `ENVREF_VAULT_PASSPHRASE` |
| CI/CD pipeline | `vault` with `ENVREF_VAULT_PASSPHRASE` |
| Team with shared secrets | `keychain` per-developer + team sync |

For most development workflows, the default keychain backend is sufficient. Use the vault backend when the OS keychain is not available or when you need a portable, file-based secret store.

## Troubleshooting

### "keychain: secret not found"

The secret hasn't been stored yet. Run:

```bash
envref secret set <key>
```

### "keychain: exec: dbus-launch: not found" (Linux)

The Secret Service API requires a D-Bus session. This is common on headless Linux systems. Switch to the vault backend:

```bash
envref vault init
```

Then update `.envref.yaml` to use vault as the primary backend.

### "vault: locked"

The vault has been locked. Unlock it:

```bash
envref vault unlock
```

### "vault: not initialized"

Run `envref vault init` to create the vault database and set a master passphrase.

### Checking overall secret status

```bash
envref status
```

This shows which references are resolved, which are missing, and provides actionable hints for fixing issues.

## See also

- [Getting Started](getting-started.md) — basic envref setup
- [Profiles](profiles.md) — profile-scoped secrets
- [direnv Integration](direnv-integration.md) — automatic environment loading
