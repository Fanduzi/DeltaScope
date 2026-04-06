# DeltaScope v0.15.0 Release Notes

Release date: 2026-04-06

## Overview

DeltaScope `v0.15.0` is the PostgreSQL foundation release. It extends DeltaScope from a MySQL/TiDB-only audit story into a multi-dialect product with a dedicated PostgreSQL-capable CLI path, while keeping the existing core release contract stable.

## What's Changed

### PostgreSQL Foundation Support

This release adds the first production PostgreSQL foundation path across the audit engine:

- PostgreSQL parsing and extraction for supported offline audit flows
- normalized PostgreSQL DDL/spec mapping
- PostgreSQL-aware DDL rule registration and semantic checks
- CLI, Go API, and audit-surface capability exposure for PG-capable builds

The current PostgreSQL contract is intentionally narrow and explicit: it is an offline-first, CLI-first foundation rather than a full parity release across every existing product surface.

### Dedicated Public PG Artifact

PostgreSQL is published through a dedicated public v1 artifact:

- `deltascope-pg_<version>_linux_amd64.tar.gz`

This archive contains the PG-capable CLI only. It does **not** publish `deltascope-server-pg` or `deltascope-mcp-pg`, and it does not change the existing installer, Homebrew Cask, or npm MCP launcher contract for the core DeltaScope binaries.

### Linux Release Validation For PG

The PostgreSQL release path now includes layered validation:

- local PG smoke
- Linux/Ubuntu smoke
- manylinux2014 build validation
- glibc baseline gate
- dedicated packaging and upload wiring for `deltascope-pg`

This keeps the PG release boundary auditable without expanding the core pure-Go release path.

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

**PostgreSQL-capable CLI:**

Download `deltascope-pg_v0.15.0_linux_amd64.tar.gz` from the GitHub Release assets when you need PostgreSQL offline audit support.

## Compatibility

No breaking changes to the existing MySQL/TiDB core surfaces. `v0.15.0` adds PostgreSQL foundation support through a dedicated PG-capable CLI artifact and additive public API routing. `drop_primary_key` remains deferred until DeltaScope has a metadata-aware PostgreSQL constraint-classification path.
