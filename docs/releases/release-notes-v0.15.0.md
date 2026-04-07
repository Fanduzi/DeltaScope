# DeltaScope v0.15.0 Release Notes

Release date: 2026-04-06

## Overview

DeltaScope `v0.15.0` is the PostgreSQL foundation release. It extends DeltaScope from a MySQL/TiDB-only audit story into a multi-dialect product, and converges the public Linux amd64 main assets onto PG-capable `deltascope`, `deltascope-server`, and `deltascope-mcp` binaries for PostgreSQL offline audit while keeping the broader release matrix honest per platform.

## What's Changed

### PostgreSQL Foundation Support

This release adds the first production PostgreSQL foundation path across the audit engine:

- PostgreSQL parsing and extraction for supported offline audit flows
- normalized PostgreSQL DDL/spec mapping
- PostgreSQL-aware DDL rule registration and semantic checks
- CLI, Go API, and audit-surface capability exposure for PG-capable builds

The current PostgreSQL contract is intentionally narrow and explicit: PostgreSQL support is offline-first, and public release convergence currently applies to the Linux amd64 main assets only rather than every published platform.

### Public Release Shape

PostgreSQL offline support is now published on the Linux amd64 main release assets:

- `deltascope_<version>_linux_amd64.tar.gz`

That main archive contains PG-capable `deltascope`, `deltascope-server`, and `deltascope-mcp` binaries. `deltascope-pg_<version>_linux_amd64.tar.gz` remains available only as a transitional CLI compatibility artifact, and does not redefine the main product surface.

### Linux Release Validation For PG

The PostgreSQL release path now includes layered validation:

- local PG smoke
- Linux/Ubuntu smoke
- manylinux2014 build validation
- glibc baseline gate
- main Linux amd64 PG archive shape verification
- transitional `deltascope-pg` compatibility packaging and upload wiring

This keeps the PG release boundary auditable while letting the Linux amd64 main assets carry the converged PostgreSQL offline story.

### Post-Milestone Correctness And Test Cleanup

Before release cut, the branch also landed narrow cleanup items around the PostgreSQL path:

- fixed PostgreSQL `INSERT ... ON CONFLICT ...` mis-mapping into MySQL-specific DML findings
- restored clean non-tagged test boundaries for PostgreSQL-only extraction coverage
- tightened PG negative regression coverage in the parse, CLI, and public API layers

These changes do not expand the approved product boundary; they make the released PostgreSQL foundation path more correct and easier to verify.

## Install / Upgrade

**Core DeltaScope binaries (CLI, server, MCP):**

```bash
curl -fsSL https://raw.githubusercontent.com/Fanduzi/DeltaScope/v0.15.0/install.sh | \
  DELTASCOPE_VERSION=v0.15.0 sh
```

**PostgreSQL offline support:**

On `linux/amd64`, install the normal DeltaScope release and use the main archive binaries for PostgreSQL offline audit. `deltascope-pg_0.15.0_linux_amd64.tar.gz` remains available only for transitional CLI compatibility workflows.

## Compatibility

No breaking changes to the existing MySQL/TiDB core surfaces. `v0.15.0` adds PostgreSQL foundation support through the converged Linux amd64 main assets plus additive public API routing, while retaining `deltascope-pg` only as a transitional CLI compatibility artifact. `drop_primary_key` remains deferred until DeltaScope has a metadata-aware PostgreSQL constraint-classification path.
