# DeltaScope v0.16.3 Release Notes

Release date: 2026-04-07

## Overview

DeltaScope `v0.16.3` is a release-validation hotfix for the PostgreSQL surface-unification release train. It does not change the product contract introduced in `v0.16.0`; it fixes the file-ownership issue in the containerized PostgreSQL archive smoke step that caused the subsequent GoReleaser publish step to fail while cleaning `dist/`.

## What's Changed

### Release Workflow Hotfix

This release updates the PostgreSQL archive smoke wrapper used in CI:

- runs the manylinux smoke container with the host UID/GID instead of root
- installs Go and GoReleaser under `/tmp` so the smoke no longer needs root-only paths
- preserves the same containerized GoReleaser verification path for the converged Linux amd64 main archive

### Product Surface

No product-surface behavior changes are introduced in this patch release:

- PostgreSQL offline support remains unified across `deltascope`, `deltascope-server`, and `deltascope-mcp`
- Linux amd64 remains the converged PG-capable main-asset lane
- `deltascope-pg` remains a transitional CLI compatibility artifact
- PostgreSQL metadata-aware audit remains unsupported

## Install / Upgrade

```bash
curl -fsSL https://raw.githubusercontent.com/Fanduzi/DeltaScope/v0.16.3/install.sh | \
  DELTASCOPE_VERSION=v0.16.3 sh
```

On `linux/amd64`, the main DeltaScope archive continues to provide PostgreSQL offline support. `deltascope-pg_0.16.3_linux_amd64.tar.gz` remains available only for transitional CLI compatibility workflows.

## Compatibility

No breaking changes. `v0.16.3` is a patch release over `v0.16.2` and exists only to restore the tag-driven release pipeline for the already-approved PostgreSQL surface-unification contract.
