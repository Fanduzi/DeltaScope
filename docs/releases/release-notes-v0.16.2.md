# DeltaScope v0.16.2 Release Notes

Release date: 2026-04-07

## Overview

DeltaScope `v0.16.2` is a release-validation hotfix for the PostgreSQL surface-unification release train. It does not change the product contract introduced in `v0.16.0`; it fixes the VCS-stamping failure in the containerized GoReleaser PostgreSQL smoke build that blocked Linux amd64 release-archive verification in `v0.16.1`.

## What's Changed

### Release Workflow Hotfix

This release updates the PostgreSQL smoke GoReleaser config used in CI:

- adds `-buildvcs=false` to the Linux amd64 PostgreSQL smoke builds
- keeps the same containerized GoReleaser verification path for the converged Linux amd64 main archive
- preserves the same Linux amd64 PostgreSQL release matrix and compatibility-artifact policy

### Product Surface

No product-surface behavior changes are introduced in this patch release:

- PostgreSQL offline support remains unified across `deltascope`, `deltascope-server`, and `deltascope-mcp`
- Linux amd64 remains the converged PG-capable main-asset lane
- `deltascope-pg` remains a transitional CLI compatibility artifact
- PostgreSQL metadata-aware audit remains unsupported

## Install / Upgrade

```bash
curl -fsSL https://raw.githubusercontent.com/Fanduzi/DeltaScope/v0.16.2/install.sh | \
  DELTASCOPE_VERSION=v0.16.2 sh
```

On `linux/amd64`, the main DeltaScope archive continues to provide PostgreSQL offline support. `deltascope-pg_0.16.2_linux_amd64.tar.gz` remains available only for transitional CLI compatibility workflows.

## Compatibility

No breaking changes. `v0.16.2` is a patch release over `v0.16.1` and exists only to restore the tag-driven release pipeline for the already-approved PostgreSQL surface-unification contract.
