# DeltaScope v0.16.0 Release Notes

Release date: 2026-04-07

## Overview

DeltaScope `v0.16.0` is the PostgreSQL surface-unification release. It takes the earlier PostgreSQL foundation work and brings PostgreSQL offline audit onto the unified `deltascope`, `deltascope-server`, and `deltascope-mcp` product surfaces, while keeping the public release matrix honest about which published platforms currently ship PG-capable main assets.

## What's Changed

### Unified PostgreSQL Product Surfaces

This release makes PostgreSQL offline audit part of the main DeltaScope product story:

- `deltascope` accepts `--dialect postgresql` on PG-capable builds
- `deltascope-server` accepts PostgreSQL offline audit requests
- `deltascope-mcp` exposes PostgreSQL offline audit through the same MCP tool surface
- `pkg/deltascope` keeps the public Go API additive while preserving the same offline PostgreSQL contract

PostgreSQL metadata-aware audit is still intentionally unsupported. DeltaScope continues to return explicit unsupported responses instead of silently downgrading or over-claiming parity.

### Release Matrix Convergence

The public release story is now more consistent with the actual shipped binaries:

- the Linux amd64 main archive is the converged PG-capable release path
- the main archive still uses the normal `deltascope_<version>_<os>_<arch>.tar.gz` naming
- `deltascope-pg_<version>_linux_amd64.tar.gz` remains only as a transitional CLI compatibility artifact
- other published platforms remain on the current pure-Go matrix until an equivalent PG-capable release baseline is verified

### Release Validation Closure

The Linux amd64 PostgreSQL release path is now backed by the real validation chain used for the shipped assets:

- local and tagged PostgreSQL test lanes across CLI, HTTP, MCP, and `pkg/deltascope`
- manylinux2014 / glibc baseline validation
- GoReleaser smoke verification for the Linux amd64 PG-capable main archive
- transitional `deltascope-pg` compatibility packaging kept separate from the primary product story

### Docs And Install Story Alignment

README, landing page, CLI/library references, MCP launcher docs, and release notes now all tell the same story:

- DeltaScope has one main product surface
- Linux amd64 main assets carry PostgreSQL offline support
- `deltascope-pg` is compatibility-only
- broader platform PostgreSQL parity is not overstated

## Install / Upgrade

**Core DeltaScope binaries (CLI, server, MCP):**

```bash
curl -fsSL https://raw.githubusercontent.com/Fanduzi/DeltaScope/v0.16.0/install.sh | \
  DELTASCOPE_VERSION=v0.16.0 sh
```

**PostgreSQL offline support:**

On `linux/amd64`, install the normal DeltaScope release and use the main archive binaries for PostgreSQL offline audit. `deltascope-pg_0.16.0_linux_amd64.tar.gz` remains available only for transitional CLI compatibility workflows.

## Compatibility

No breaking changes are introduced to the existing MySQL/TiDB product surfaces. `v0.16.0` unifies PostgreSQL offline support across the main DeltaScope surfaces without claiming PostgreSQL metadata-aware parity. `drop_primary_key` remains deferred until DeltaScope has a metadata-aware PostgreSQL constraint-classification path.
