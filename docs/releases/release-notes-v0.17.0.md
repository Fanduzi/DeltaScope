# DeltaScope v0.17.0 Release Notes

Release date: 2026-04-08

## Overview

DeltaScope `v0.17.0` completes the cross-platform PG-capable release convergence work. PostgreSQL offline support now ships through the supported macOS and Linux main archives for `deltascope`, `deltascope-server`, and `deltascope-mcp`, so the primary install story no longer depends on a separate PG-only CLI artifact.

## What's Changed

### Cross-Platform Main Archives

This release converges PostgreSQL-capable main archives across the supported public platforms:

- `darwin/amd64`
- `darwin/arm64`
- `linux/amd64`
- `linux/arm64`

For those platforms, the main `deltascope_<version>_<os>_<arch>.tar.gz` archives now carry PostgreSQL offline support across all three binaries:

- `deltascope`
- `deltascope-server`
- `deltascope-mcp`

### Release and Installer Convergence

The release pipeline, archive smoke checks, installer, Homebrew story, and MCP launcher contract are now aligned with the same main-archive contract:

- native macOS archive smoke validates the PG-capable host archives
- manylinux and archive-shape validation cover both Linux amd64 and Linux arm64
- `install.sh`, Homebrew Cask, and `@fanduzi/deltascope-mcp` all resolve the same main release assets

### Compatibility Story

`deltascope-pg_<version>_linux_amd64.tar.gz` may still appear temporarily as a legacy compatibility download for older CLI-only workflows, but it is no longer part of the primary install path or product narrative.

### Internal Hardening

This release also includes test hardening for CLI and TiDB parser extraction coverage, helping keep the converged product and release contract stable.

## Install / Upgrade

```bash
curl -fsSL https://raw.githubusercontent.com/Fanduzi/DeltaScope/v0.17.0/install.sh | \
  DELTASCOPE_VERSION=v0.17.0 sh
```

macOS users can continue to install with Homebrew:

```bash
brew tap Fanduzi/deltascope
brew install --cask deltascope
```

## Compatibility

No breaking changes. `v0.17.0` changes the release and install story, not the public SQL-audit contract:

- PostgreSQL remains offline-first
- PostgreSQL metadata-aware audit remains unsupported
- `drop_primary_key` remains deferred
