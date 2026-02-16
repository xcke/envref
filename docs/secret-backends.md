# Secret Backends

envref resolves `ref://` secret references through configurable backends. Backends are tried in order — the first one that has the requested key wins.

## Built-in backends

envref ships with six built-in backends plus a plugin system for custom integrations:

| Backend | Type | Storage | Encryption | Setup | Use case |
|---------|------|---------|------------|-------|----------|
| Keychain | `keychain` | OS keychain (macOS Keychain, Linux Secret Service, Windows Credential Manager) | OS-managed | None (default) | Development machines with a desktop environment |
| Vault | `vault` | Local SQLite at `~/.config/envref/vault.db` | age scrypt per-value | `envref vault init` | Headless servers, containers, CI |
| 1Password | `1password` | 1Password vault via `op` CLI | 1Password-managed | `op signin` | Teams using 1Password |
| AWS SSM | `aws-ssm` | AWS Systems Manager Parameter Store | AWS KMS | AWS CLI configured | AWS-based infrastructure |
| HashiCorp Vault | `hashicorp-vault` | HashiCorp Vault KV v2 secrets engine | Vault-managed | `vault login` | Enterprise secret management |
| OCI Vault | `oci-vault` | Oracle Cloud Infrastructure Vault | OCI-managed | OCI CLI configured | Oracle Cloud workloads |
| Plugin | `plugin` | Custom (external executable) | Custom | Plugin on `$PATH` | Custom or third-party secret stores |

---

## Keychain backend

The keychain backend uses your operating system's native credential store via [go-keyring](https://github.com/zalando/go-keyring):

- **macOS**: Keychain Access
- **Linux**: Secret Service API (GNOME Keyring, KWallet)
- **Windows**: Windows Credential Manager

No configuration is needed — it works out of the box on systems with a desktop environment. Secrets are stored under the service name `envref` with namespaced keys.

**Configuration:**

```yaml
backends:
  - name: keychain
    type: keychain
```

No additional options. This is the default backend when no `backends` section is present in `.envref.yaml`.

**Example — store and retrieve a secret:**

```bash
# Store a database URL in the OS keychain
envref secret set database_url --value "postgres://user:pass@localhost/mydb"

# Retrieve it
envref secret get database_url
# postgres://user:pass@localhost/mydb

# Reference it in .env
echo 'DATABASE_URL=ref://secrets/database_url' >> .env

# Resolve
envref resolve
# DATABASE_URL=postgres://user:pass@localhost/mydb
```

---

## Vault backend

The vault backend is a local encrypted store for environments where the OS keychain is unavailable (SSH servers, Docker containers, CI runners).

Each secret is individually encrypted using [age](https://age-encryption.org/) with scrypt-based key derivation from a master passphrase. Secrets are stored in a SQLite database at `~/.config/envref/vault.db`.

**Configuration:**

```yaml
backends:
  - name: vault
    type: vault
    config:
      path: ~/.config/envref/vault.db   # optional, this is the default
```

| Option | Description | Default |
|--------|-------------|---------|
| `path` | Path to the SQLite database file | `~/.config/envref/vault.db` |

The passphrase is resolved in order:
1. `ENVREF_VAULT_PASSPHRASE` environment variable
2. Interactive terminal prompt

**Example — set up vault for a CI pipeline:**

```bash
# Initialize the vault (interactive — prompts for passphrase)
envref vault init

# Or initialize non-interactively
ENVREF_VAULT_PASSPHRASE=my-secret-passphrase envref vault init

# Store secrets
envref secret set api_key --backend vault --value "sk-abc123"
envref secret set db_password --backend vault

# In CI, set the passphrase via environment variable
export ENVREF_VAULT_PASSPHRASE="my-secret-passphrase"
envref resolve
```

**Example — vault lifecycle management:**

```bash
# Lock the vault (prevents all access)
envref vault lock

# Unlock the vault
envref vault unlock

# Export all secrets as JSON (plaintext — handle with care)
envref vault export > vault-backup.json

# Import secrets from a backup
envref vault import < vault-backup.json
```

---

## 1Password backend

The 1Password backend delegates secret storage to [1Password](https://1password.com/) via the `op` CLI (v2+). Secrets are stored as "Secure Note" items in the configured vault, with the item title as the secret key and the value in the `notesPlain` field.

**Prerequisites:**

1. Install the [1Password CLI](https://developer.1password.com/docs/cli/get-started/):
   ```bash
   brew install 1password-cli
   ```

2. Sign in to your account:
   ```bash
   op signin
   ```

3. (Optional) Enable [biometric unlock](https://developer.1password.com/docs/cli/get-started/#turn-on-biometric-unlock) for passwordless CLI access.

**Configuration:**

```yaml
backends:
  - name: op
    type: 1password
    config:
      vault: Personal              # 1Password vault name (default: "Personal")
      account: my.1password.com    # optional: account shorthand or URL
      command: /usr/local/bin/op   # optional: path to op CLI
```

| Option | Description | Default |
|--------|-------------|---------|
| `vault` | 1Password vault name | `Personal` |
| `account` | Account shorthand or URL (for multi-account setups) | _(none)_ |
| `command` | Path to the `op` CLI executable | `op` (found via `$PATH`) |

**Example — team setup with 1Password:**

```yaml
# .envref.yaml
project: my-saas-app

backends:
  - name: op
    type: 1password
    config:
      vault: Engineering
      account: mycompany.1password.com
```

```bash
# Sign in to 1Password
op signin

# Store secrets via envref (stored in the "Engineering" vault)
envref secret set stripe_api_key --value "sk_live_abc123"
envref secret set sendgrid_key --value "SG.xyz789"

# Reference in .env
echo 'STRIPE_API_KEY=ref://secrets/stripe_api_key' >> .env
echo 'SENDGRID_KEY=ref://secrets/sendgrid_key' >> .env

# Resolve — fetches from 1Password
envref resolve
# STRIPE_API_KEY=sk_live_abc123
# SENDGRID_KEY=SG.xyz789
```

---

## AWS SSM Parameter Store backend

The AWS SSM backend stores secrets in [AWS Systems Manager Parameter Store](https://docs.aws.amazon.com/systems-manager/latest/userguide/systems-manager-parameter-store.html) as SecureString parameters, encrypted with AWS KMS. It delegates all operations to the `aws` CLI.

**Prerequisites:**

1. Install the [AWS CLI v2](https://docs.aws.amazon.com/cli/latest/userguide/getting-started-install.html):
   ```bash
   brew install awscli
   ```

2. Configure credentials:
   ```bash
   aws configure
   # Or use SSO: aws sso login --profile myprofile
   ```

3. Ensure the IAM role/user has `ssm:GetParameter`, `ssm:PutParameter`, `ssm:DeleteParameter`, and `ssm:GetParametersByPath` permissions.

**Configuration:**

```yaml
backends:
  - name: ssm
    type: aws-ssm
    config:
      prefix: /myapp/prod        # parameter name prefix (default: "/envref")
      region: us-east-1           # optional: AWS region
      profile: prod-account       # optional: AWS CLI named profile
      command: /usr/local/bin/aws # optional: path to aws CLI
```

| Option | Description | Default |
|--------|-------------|---------|
| `prefix` | Parameter name prefix (keys stored as `<prefix>/<key>`) | `/envref` |
| `region` | AWS region for Parameter Store | _(AWS CLI default)_ |
| `profile` | AWS CLI named profile | _(AWS CLI default)_ |
| `command` | Path to the `aws` CLI executable | `aws` (found via `$PATH`) |

Secrets are stored as SecureString parameters at `<prefix>/<key>`. For example, with `prefix: /myapp/prod`, a secret named `api_key` is stored at `/myapp/prod/api_key`.

**Example — production secrets in AWS:**

```yaml
# .envref.yaml
project: my-api

backends:
  - name: ssm
    type: aws-ssm
    config:
      prefix: /my-api/production
      region: us-west-2
      profile: production
```

```bash
# Store secrets (written to SSM Parameter Store as SecureString)
envref secret set database_url --backend ssm \
  --value "postgres://admin:s3cret@rds.amazonaws.com:5432/mydb"
envref secret set jwt_secret --backend ssm

# List stored secrets
envref secret list --backend ssm
# database_url
# jwt_secret

# Use in .env
# DATABASE_URL=ref://secrets/database_url
# JWT_SECRET=ref://secrets/jwt_secret

# Resolve (uses AWS credentials to fetch from SSM)
envref resolve
```

**Example — multi-environment with profiles:**

```yaml
# .envref.yaml
project: my-api

backends:
  - name: ssm-staging
    type: aws-ssm
    config:
      prefix: /my-api/staging
      region: us-west-2
      profile: staging
  - name: ssm-prod
    type: aws-ssm
    config:
      prefix: /my-api/production
      region: us-west-2
      profile: production
```

```bash
# Store per-environment secrets
envref secret set api_key --backend ssm-staging --value "sk_test_abc"
envref secret set api_key --backend ssm-prod --value "sk_live_xyz"

# Resolve with specific backend
envref secret get api_key --backend ssm-staging
# sk_test_abc
```

---

## HashiCorp Vault backend

The HashiCorp Vault backend stores secrets in a [HashiCorp Vault](https://www.vaultproject.io/) KV v2 secrets engine. It delegates all operations to the `vault` CLI.

**Prerequisites:**

1. Install the [Vault CLI](https://developer.hashicorp.com/vault/install):
   ```bash
   brew install hashicorp/tap/vault
   ```

2. Authenticate with your Vault server:
   ```bash
   export VAULT_ADDR="https://vault.example.com:8200"
   vault login
   ```

3. Ensure you have read/write access to the target KV v2 mount and path.

**Configuration:**

```yaml
backends:
  - name: hcvault
    type: hashicorp-vault
    config:
      mount: secret              # KV v2 mount path (default: "secret")
      prefix: envref             # path prefix within mount (default: "envref")
      addr: https://vault.example.com:8200  # optional: Vault server URL
      namespace: admin           # optional: Vault Enterprise namespace
      token: hvs.abc123         # optional: auth token (prefer VAULT_TOKEN env var)
      command: /usr/local/bin/vault  # optional: path to vault CLI
```

| Option | Description | Default |
|--------|-------------|---------|
| `mount` | KV v2 secrets engine mount path | `secret` |
| `prefix` | Path prefix within the mount (keys stored at `<mount>/data/<prefix>/<key>`) | `envref` |
| `addr` | Vault server URL (can also use `VAULT_ADDR` env var) | _(vault CLI default)_ |
| `namespace` | Vault Enterprise namespace (can also use `VAULT_NAMESPACE` env var) | _(none)_ |
| `token` | Authentication token (can also use `VAULT_TOKEN` env var) | _(none)_ |
| `command` | Path to the `vault` CLI executable | `vault` (found via `$PATH`) |

Secrets are stored as individual KV v2 entries at `<mount>/data/<prefix>/<key>` with the value in a `value` field.

**Example — centralized secret management:**

```yaml
# .envref.yaml
project: payment-service

backends:
  - name: hcvault
    type: hashicorp-vault
    config:
      mount: secret
      prefix: payment-service
      addr: https://vault.internal:8200
```

```bash
# Authenticate (typically via CI token or app role)
export VAULT_ADDR="https://vault.internal:8200"
vault login -method=token token=hvs.abc123

# Store secrets
envref secret set stripe_key --backend hcvault --value "sk_live_abc123"
envref secret set webhook_secret --backend hcvault

# Reference in .env
# STRIPE_KEY=ref://secrets/stripe_key
# WEBHOOK_SECRET=ref://secrets/webhook_secret

# Resolve
envref resolve
```

**Example — Vault Enterprise with namespaces:**

```yaml
backends:
  - name: hcvault
    type: hashicorp-vault
    config:
      mount: kv
      prefix: team-alpha/myapp
      addr: https://vault.corp.example.com:8200
      namespace: engineering/team-alpha
```

---

## OCI Vault backend

The OCI Vault backend stores secrets in [Oracle Cloud Infrastructure Vault](https://www.oracle.com/security/cloud-security/key-management/). It delegates all operations to the `oci` CLI.

**Prerequisites:**

1. Install the [OCI CLI](https://docs.oracle.com/en-us/iaas/Content/API/SDKDocs/cliinstall.htm):
   ```bash
   brew install oci-cli
   # Or: pip install oci-cli
   ```

2. Configure the CLI:
   ```bash
   oci setup config
   ```

3. Ensure you have a Vault, Compartment, and Master Encryption Key provisioned in OCI.

4. Ensure your IAM policy grants `manage secret-family` permissions in the target compartment.

**Configuration:**

```yaml
backends:
  - name: oci
    type: oci-vault
    config:
      vault_id: ocid1.vault.oc1.iad.abcd1234...         # required: vault OCID
      compartment_id: ocid1.compartment.oc1..abcd5678... # required: compartment OCID
      key_id: ocid1.key.oc1.iad.abcd9012...              # required: master encryption key OCID
      profile: DEFAULT                                    # optional: OCI CLI config profile
      command: /usr/local/bin/oci                         # optional: path to oci CLI
```

| Option | Description | Default |
|--------|-------------|---------|
| `vault_id` | OCI Vault OCID | _(required)_ |
| `compartment_id` | OCI Compartment OCID | _(required)_ |
| `key_id` | Master Encryption Key OCID (required for storing secrets) | _(required)_ |
| `profile` | OCI CLI configuration profile name | _(OCI CLI default)_ |
| `command` | Path to the `oci` CLI executable | `oci` (found via `$PATH`) |

Secret values are base64-encoded before storage. Deletion in OCI is scheduled (not immediate) with a minimum pending period.

**Example — Oracle Cloud workload:**

```yaml
# .envref.yaml
project: oci-microservice

backends:
  - name: oci
    type: oci-vault
    config:
      vault_id: ocid1.vault.oc1.iad.b5re4wdnaabc.abuwcljr...
      compartment_id: ocid1.compartment.oc1..aaaaaaa...
      key_id: ocid1.key.oc1.iad.b5re4wdnaabc.abuwcljr...
```

```bash
# Store secrets
envref secret set db_password --backend oci --value "oracle-secret-123"

# Reference in .env
# DB_PASSWORD=ref://secrets/db_password

# Resolve
envref resolve
```

---

## Plugin backend

The plugin backend enables integration with any secret store by delegating operations to an external executable. Plugins communicate via a simple JSON-over-stdin/stdout protocol.

**Plugin discovery:**

Plugins are found in one of two ways:
1. **By convention**: an executable named `envref-backend-<name>` on `$PATH`
2. **By explicit path**: the `command` option in `.envref.yaml`

**Configuration:**

```yaml
backends:
  - name: my-store
    type: plugin
    config:
      command: /usr/local/bin/envref-backend-my-store  # optional if on $PATH
```

| Option | Description | Default |
|--------|-------------|---------|
| `command` | Path to the plugin executable | `envref-backend-<name>` (found via `$PATH`) |

### Plugin protocol

The plugin receives a JSON request on stdin and must write a JSON response to stdout. Each invocation handles a single operation.

**Request format:**

```json
{
  "operation": "get",
  "key": "api_key"
}
```

| Field | Type | Description |
|-------|------|-------------|
| `operation` | string | One of: `get`, `set`, `delete`, `list` |
| `key` | string | Secret key name (present for `get`, `set`, `delete`) |
| `value` | string | Secret value (present for `set` only) |

**Response format:**

```json
{
  "value": "sk-abc123"
}
```

| Field | Type | Description |
|-------|------|-------------|
| `value` | string | Secret value (returned by `get`) |
| `keys` | []string | List of key names (returned by `list`) |
| `error` | string | Error message (present on failure) |

**Error handling:** If a key is not found, the plugin should return an error message containing "not found" (case-insensitive). envref interprets this as `ErrNotFound` and continues to the next backend in the fallback chain.

### Example — writing a plugin in Bash

```bash
#!/usr/bin/env bash
# envref-backend-mystore — example plugin that stores secrets in a flat file
set -euo pipefail

STORE_FILE="${HOME}/.mystore/secrets.json"
mkdir -p "$(dirname "$STORE_FILE")"
[ -f "$STORE_FILE" ] || echo '{}' > "$STORE_FILE"

REQUEST=$(cat)
OP=$(echo "$REQUEST" | jq -r '.operation')
KEY=$(echo "$REQUEST" | jq -r '.key // empty')

case "$OP" in
  get)
    VALUE=$(jq -r --arg k "$KEY" '.[$k] // empty' "$STORE_FILE")
    if [ -z "$VALUE" ]; then
      echo '{"error": "not found"}'
    else
      echo "{\"value\": $(echo "$VALUE" | jq -Rs .)}"
    fi
    ;;
  set)
    VALUE=$(echo "$REQUEST" | jq -r '.value')
    jq --arg k "$KEY" --arg v "$VALUE" '. + {($k): $v}' "$STORE_FILE" > "${STORE_FILE}.tmp"
    mv "${STORE_FILE}.tmp" "$STORE_FILE"
    echo '{}'
    ;;
  delete)
    jq --arg k "$KEY" 'del(.[$k])' "$STORE_FILE" > "${STORE_FILE}.tmp"
    mv "${STORE_FILE}.tmp" "$STORE_FILE"
    echo '{}'
    ;;
  list)
    KEYS=$(jq '[keys[]]' "$STORE_FILE")
    echo "{\"keys\": $KEYS}"
    ;;
esac
```

```bash
# Make it executable and place on $PATH
chmod +x envref-backend-mystore
mv envref-backend-mystore /usr/local/bin/
```

```yaml
# .envref.yaml
backends:
  - name: mystore
    type: plugin
```

### Example — writing a plugin in Python

```python
#!/usr/bin/env python3
"""envref-backend-redis — plugin that stores secrets in Redis."""
import json
import sys
import redis

r = redis.Redis(host="localhost", port=6379, db=0, decode_responses=True)
PREFIX = "envref:"

request = json.loads(sys.stdin.read())
op = request["operation"]
key = request.get("key", "")

if op == "get":
    value = r.get(f"{PREFIX}{key}")
    if value is None:
        print(json.dumps({"error": "not found"}))
    else:
        print(json.dumps({"value": value}))
elif op == "set":
    r.set(f"{PREFIX}{key}", request["value"])
    print(json.dumps({}))
elif op == "delete":
    r.delete(f"{PREFIX}{key}")
    print(json.dumps({}))
elif op == "list":
    keys = [k.removeprefix(PREFIX) for k in r.keys(f"{PREFIX}*")]
    print(json.dumps({"keys": keys}))
```

---

## Configuration

Backends are configured in `.envref.yaml`:

```yaml
project: my-app

backends:
  - name: keychain
    type: keychain
  - name: vault
    type: vault
    config:
      path: ~/.config/envref/vault.db
```

The `backends` list defines the fallback chain — backends are tried in order when resolving a `ref://` reference. If the first backend doesn't have the key, the next one is tried.

### Default configuration

If no `backends` section is present in `.envref.yaml`, envref uses the keychain backend by default.

### Multi-backend configuration

You can configure multiple backends for different use cases. Common patterns:

**Development with cloud fallback:**

```yaml
project: my-app

backends:
  - name: keychain
    type: keychain
  - name: ssm
    type: aws-ssm
    config:
      prefix: /my-app/shared
      region: us-west-2
```

Secrets in the OS keychain are found first. If a key isn't in the keychain (e.g., shared infrastructure secrets), SSM is checked next.

**Team with 1Password and local vault fallback:**

```yaml
project: my-app

backends:
  - name: op
    type: 1password
    config:
      vault: Engineering
  - name: vault
    type: vault
```

Team secrets live in 1Password. Developer-specific overrides go in the local vault.

**Full enterprise stack:**

```yaml
project: my-app

backends:
  - name: keychain
    type: keychain
  - name: hcvault
    type: hashicorp-vault
    config:
      mount: secret
      prefix: my-app
      addr: https://vault.internal:8200
  - name: ssm
    type: aws-ssm
    config:
      prefix: /shared/secrets
      region: us-east-1
```

---

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

---

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

---

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
envref secret set api_key --backend ssm
envref secret set api_key --backend hcvault
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

---

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

### Rotate a secret

```bash
# Generate a new random value, archive the old one
envref secret rotate api_key

# Keep more history entries (default: 5)
envref secret rotate api_key --keep 10
```

Rotation generates a new random value, stores it as the current value, and archives the old value as `<key>.__history.<N>`.

### Share a secret

```bash
# Encrypt a secret for a specific recipient using their age public key
envref secret share api_key --to age1ql3z7hjy54pw3hyww5ayyfg7zqgvc7w3j2elw8zmrj2kg5sfn9aqmcac8p

# Read the recipient's public key from a file
envref secret share api_key --to-file recipient.pub
```

The output is ASCII-armored age-encrypted ciphertext that only the recipient can decrypt.

---

## Using ref:// in .env files

Reference syntax:

```dotenv
# Simple secret reference
API_KEY=ref://secrets/api_key

# Variable interpolation works alongside references
DATABASE_URL=ref://secrets/database_url
DB_DISPLAY=${DATABASE_URL}

# Nested references (resolved in a second pass)
FULL_URL=postgres://${ref://secrets/db_user}:${ref://secrets/db_pass}@localhost/app
```

The `ref://secrets/<key>` format is the standard reference syntax. The `secrets` segment indicates the secret backend system.

---

## Choosing a backend

| Scenario | Recommended backend | Why |
|----------|-------------------|-----|
| Local development with desktop | `keychain` (default) | Zero setup, OS-native security |
| Headless server / SSH | `vault` | No desktop environment needed |
| Docker container | `vault` with `ENVREF_VAULT_PASSPHRASE` | Portable, single-file store |
| CI/CD pipeline | `vault` or `aws-ssm` | Non-interactive, scriptable |
| Team using 1Password | `1password` | Shared vaults, biometric unlock |
| AWS infrastructure | `aws-ssm` | Native integration, IAM-based access |
| Enterprise with HashiCorp Vault | `hashicorp-vault` | Centralized policy and audit |
| Oracle Cloud workloads | `oci-vault` | OCI-native key management |
| Custom secret store | `plugin` | Any store via JSON protocol |
| Team with shared secrets | `keychain` per-developer + `sync push/pull` | Each dev has own keychain, sync via git |

For most development workflows, the default keychain backend is sufficient. Add cloud backends when secrets need to be shared across infrastructure or managed centrally.

---

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

### "1password: op get: ... isn't an item"

The secret hasn't been stored in 1Password yet. Run:

```bash
envref secret set <key> --backend op
```

### "1password: start op: executable file not found"

The `op` CLI is not installed or not on your `$PATH`. Install it:

```bash
brew install 1password-cli
```

Then sign in: `op signin`.

### "unknown backend type"

The backend type in `.envref.yaml` is not recognized. Recognized types are: `keychain`, `1password`, `aws-ssm`, `oci-vault`, `hashicorp-vault`. For custom backends, use `type: plugin`.

### AWS SSM permission errors

Ensure your IAM role has the required permissions:

```json
{
  "Effect": "Allow",
  "Action": [
    "ssm:GetParameter",
    "ssm:PutParameter",
    "ssm:DeleteParameter",
    "ssm:GetParametersByPath"
  ],
  "Resource": "arn:aws:ssm:*:*:parameter/myapp/*"
}
```

### HashiCorp Vault authentication errors

Ensure `VAULT_ADDR` and `VAULT_TOKEN` are set, or run `vault login`:

```bash
export VAULT_ADDR="https://vault.example.com:8200"
vault login
```

### Checking overall secret status

```bash
envref status
```

This shows which references are resolved, which are missing, and provides actionable hints for fixing issues.

---

## See also

- [Getting Started](getting-started.md) — basic envref setup
- [Profiles](profiles.md) — profile-scoped secrets
- [direnv Integration](direnv-integration.md) — automatic environment loading
