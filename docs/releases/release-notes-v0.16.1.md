# DeltaScope v0.16.1 Release Notes

Release date: 2026-04-07

## Overview

DeltaScope `v0.16.1` is a release-validation hotfix for the PostgreSQL surface-unification release train. It does not change the product contract introduced in `v0.16.0`; it fixes the shell compatibility issue that caused the tag-driven release workflow to fail before the Linux amd64 PostgreSQL archive smoke verification could run to completion.

## What's Changed

### Release Workflow Hotfix

This release fixes the `verify-pg-linux-release-archive` wrapper used in CI:

- removes the top-level `pipefail` assumption from the Makefile recipe that GitHub Actions executes via `/bin/sh`
- keeps the inner Linux container validation path unchanged
- preserves the same GoReleaser smoke verification for the converged Linux amd64 PG-capable main archive

### Product Surface

No product-surface behavior changes are introduced in this patch release:

- PostgreSQL offline support remains unified across `deltascope`, `deltascope-server`, and `deltascope-mcp`
- Linux amd64 remains the converged PG-capable main-asset lane
- `deltascope-pg` remains a transitional CLI compatibility artifact
- PostgreSQL metadata-aware audit remains unsupported

## Install / Upgrade

```bash
curl -fsSL https://raw.githubusercontent.com/Fanduzi/DeltaScope/v0.16.1/install.sh | \
  DELTASCOPE_VERSION=v0.16.1 sh
```

On `linux/amd64`, the main DeltaScope archive continues to provide PostgreSQL offline support. `deltascope-pg_0.16.1_linux_amd64.tar.gz` remains available only for transitional CLI compatibility workflows.

## Compatibility

No breaking changes. `v0.16.1` is a patch release over `v0.16.0` and exists only to restore the tag-driven release pipeline for the already-approved PostgreSQL surface-unification contract.
