# AUR Package — envref-bin

Arch Linux AUR package for `envref`.

## How It Works

GoReleaser automatically publishes the `envref-bin` package to the AUR on each tagged release. The `aurs` section in `.goreleaser.yml` handles:

1. Generating the PKGBUILD with correct version, checksums, and source URLs
2. Committing and pushing to the AUR git repo (`aur.archlinux.org/envref-bin.git`)

The PKGBUILD in this directory is a **reference copy** for local testing. The canonical version is managed by GoReleaser.

## Setup

### 1. Create the AUR Package

Register `envref-bin` at https://aur.archlinux.org/pkgbase/envref-bin/

### 2. Generate an SSH Key for AUR

```bash
ssh-keygen -t ed25519 -f ~/.ssh/aur -N ""
```

Add the public key to your AUR account at https://aur.archlinux.org/account/

### 3. Add the Private Key as a GitHub Secret

Add the contents of `~/.ssh/aur` as the `AUR_SSH_KEY` secret in the GitHub repository settings.

## Installation (End Users)

With an AUR helper like `yay`:

```bash
yay -S envref-bin
```

Or manually:

```bash
git clone https://aur.archlinux.org/envref-bin.git
cd envref-bin
makepkg -si
```

## Local Testing

To test the PKGBUILD locally before publishing:

```bash
cd aur/envref-bin
# Update pkgver to a valid released version
makepkg -si
```
