# envref — Project Goal

CLI tool that separates config from secrets in `.env` files. Secrets live in secure backends (OS keychain, 1Password, Vault); the `.env` file holds only references (`ref://`), making it safe to commit.

## Mental Model

```
.env        = config values + ref:// secret references (committed to git)
secrets     = OS keychain / password manager / cloud vault
envref resolve = config + resolved secrets → direnv / shell env
```

## Goals

- **G1 Zero secrets on disk** — secrets stay in keychain/vault, injected at runtime via direnv
- **G2 Zero-friction onboarding** — `envref status` shows which secrets a dev needs to set up
- **G3 Works with existing tools** — complements direnv/dotenv/IDEs as a resolution layer
- **G4 No infrastructure** — works with OS keychain out of the box; cloud backends optional
- **G5 Safe to commit** — `.env` with `ref://` contains no secret values
- **G6 Fast (<50ms)** — Go binary with instant startup; direnv calls it on every `cd`

## Non-Goals

- Not a secret manager (no ACLs, rotation, audit logs)
- Not a deployment tool (CI/CD has its own secret injection)
- Not a dotenv replacement (extends the convention, doesn't replace it)

## Target Users

Solo devs, small teams (2–20), OSS maintainers — anyone who wants shared env documentation without sharing actual secrets.

## Competitive Position

Fills the gap between "just use `.env`" (insecure) and "deploy a secret manager" (heavy). Lighter than Doppler/Infisical/SOPS, more structured than raw dotenv/direnv.

## Success Metrics

| Metric                   | Target                               |
| ------------------------ | ------------------------------------ |
| Clone-to-working-env     | < 2 min (secrets pre-stored)         |
| `envref resolve` latency | < 50ms / 100 vars                    |
| Secrets on disk          | 0                                    |
| Setup                    | `brew install envref && envref init` |

## Future

Team secret sharing (age keys), CI/CD export, secret health auditing, plugin ecosystem for backends.
