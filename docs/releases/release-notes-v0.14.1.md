# DeltaScope v0.14.1 Release Notes

Release date: 2026-04-04

## Overview

DeltaScope `v0.14.1` is a backward-compatible patch release focused on interface parity and release-surface alignment. It makes the shipped HTTP and CLI entrypoints easier to discover and automate without changing the core audit engine contract.

## What's Changed

### HTTP Discovery Endpoints

The HTTP adapter now exposes read-only discovery endpoints alongside audit execution:

- `GET /v1/rules`
- `GET /v1/rules/{rule_id}`
- `GET /v1/capabilities`

These endpoints align the HTTP surface more closely with the existing CLI and MCP discovery flows.

### Safer CLI Secret Inputs

The CLI now supports:

- `--password-env`
- `--password-file`

This lets scripts and local automation pass database passwords without placing secrets directly in process arguments.

### Release And Support Metadata Sync

This patch also updates versioned install snippets, package metadata, landing-page latest-release references, and the support statement in `SECURITY.md` so the published release path stays internally consistent.

## Install / Upgrade

**macOS (recommended):**

```bash
brew tap Fanduzi/deltascope
brew install --cask deltascope
```

Or upgrade:

```bash
brew upgrade --cask deltascope
```

**Linux / other environments:**

```bash
curl -fsSL https://raw.githubusercontent.com/Fanduzi/DeltaScope/v0.14.1/install.sh | \
  DELTASCOPE_VERSION=v0.14.1 sh
```

## Compatibility

No breaking changes. `v0.14.1` is a backward-compatible patch release on top of `v0.14.0`.
