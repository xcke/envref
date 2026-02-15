# envref — Project Goals

## One-liner

**envref** is a CLI tool that separates config from secrets in `.env` files, so teams never store plaintext secrets on disk or in git again.

---

## The Problem

Every developer has done it: committed a `.env` file with secrets, shared API keys over Slack, or spent an hour onboarding a new teammate by manually copying secrets one by one. The current landscape forces a painful choice:

- **Plaintext `.env` files** — convenient but insecure. Secrets leak through git history, laptop theft, or accidental commits.
- **Encrypted `.env` files** — secure but painful. Key management overhead, merge conflicts on encrypted blobs, and a decryption step that breaks flow.
- **Cloud secret managers** (Vault, Doppler, AWS SSM) — robust but heavy. Require infrastructure, accounts, and onboarding that most small-to-mid teams skip entirely.

The result: most teams default to plaintext `.env` and hope for the best. The tools that exist either solve the wrong problem (encryption at rest) or require too much setup (full secret management platforms).

---

## The Insight

**The `.env` file should be a manifest, not a secret store.**

Secrets don't belong in files — they belong in stores designed for secrets (OS keychain, password managers, cloud vaults). The `.env` file should declare *what* your app needs, not hold the actual values. By replacing secret values with references (`ref://`), the file becomes safe to commit, easy to read, and trivial to validate.

---

## The Solution

envref introduces a simple mental model:

```
.env = config (committed) + secret references (ref://)
secrets = OS keychain / password manager / cloud vault
envref resolve = merge config + resolved secrets → direnv / shell
```

No encryption dance. No plaintext secrets on disk. No new infrastructure. It works with what developers already have: a keychain on their OS and direnv in their shell.

---

## Goals

### G1 — Zero-secret disk footprint

Secrets never exist as plaintext files on disk. They live in the OS keychain (or another secure backend) and are injected into the shell environment at runtime via direnv. There is no `.env` file containing real secret values to leak.

### G2 — Zero-friction onboarding

A new teammate clones the repo, runs `envref status`, and sees exactly which secrets they need to set up. No Slack messages, no shared documents, no "ask Dave for the Stripe key." The `.env` file *is* the documentation.

### G3 — Works with existing tools, not against them

envref doesn't replace direnv, dotenv, or your IDE's env support. It sits underneath them as a resolution layer. If your tooling expects a `.env` file, `envref resolve` can produce one (temporarily, in memory). The goal is integration, not migration.

### G4 — No infrastructure required

envref works out of the box with the OS keychain. No server to deploy, no account to create, no SaaS to subscribe to. Cloud backends (1Password, AWS SSM, Vault) are optional additions, not prerequisites.

### G5 — Safe to commit

The `.env` file with `ref://` references is safe to commit to git. It contains configuration (ports, feature flags, hostnames) and secret *references* — never actual secret values. This means:

- Full git history of config changes
- Code review for env changes
- No `.env` in `.gitignore` (only `.env.local` for personal overrides)

### G6 — Fast enough to be invisible

direnv calls `envref resolve` on every `cd`. If it takes more than 50ms, developers will notice and hate it. Performance is a feature, not an afterthought. This is why envref is written in Go — a single static binary with instant startup.

---

## Non-Goals

- **envref is not a secret manager.** It doesn't manage access control, audit logs, or rotation policies. It's a *resolution layer* that connects `.env` files to secret stores.
- **envref is not a deployment tool.** It solves the local development secret problem. CI/CD and production environments have their own solutions (GitHub Secrets, cloud-native secret injection). envref can export to those formats, but it doesn't manage them.
- **envref is not a dotenv replacement.** It extends the `.env` convention with references. Projects that don't need secrets can ignore envref entirely — their `.env` files still work as before.

---

## Success Metrics

| Metric | Target |
|--------|--------|
| Time from `git clone` to working env | < 2 minutes (with secrets pre-stored in keychain) |
| `envref resolve` latency | < 50ms for 100 variables |
| Secrets on disk | 0 (enforced by architecture) |
| Setup complexity | `brew install envref && envref init` |
| External dependencies | None required (OS keychain is built-in) |

---

## Target Users

1. **Solo developers** tired of accidentally committing `.env` files or managing encrypted copies.
2. **Small teams (2–20)** who need a shared understanding of what env vars exist without sharing actual values.
3. **Open source maintainers** who want contributors to know exactly what env setup is needed without exposing project secrets.

---

## Competitive Landscape

| Tool | Approach | Why envref is different |
|------|----------|----------------------|
| dotenv | Load `.env` into process | No secret management at all |
| dotenv-vault | Encrypted `.env` in git | Encryption dance, key management overhead |
| direnv | Load env per directory | No secret resolution — envref complements it |
| Doppler / Infisical | Cloud secret manager | Requires infrastructure, accounts, SaaS dependency |
| SOPS | Encrypt files with KMS | File-level encryption, not per-value references |
| 1Password CLI | Secret retrieval | Backend-specific, not a general `.env` framework |

envref occupies the gap between "just use `.env`" and "deploy a secret manager." It's the lightest possible solution that eliminates plaintext secrets from developer machines.

---

## Long-term Vision

envref starts as a local dev tool. Over time it can grow into:

1. **Team secret sharing** — encrypted secret exchange between developers using age/public keys
2. **CI/CD bridge** — `envref export --github-actions` to push references to CI secret stores
3. **Secret health dashboard** — `envref audit` to detect leaked, stale, or weak secrets
4. **Plugin ecosystem** — community-contributed backends for any secret store

But the core promise stays the same: **your `.env` file is a manifest, not a secret store.**
