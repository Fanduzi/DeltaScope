# DeltaScope v0.30.0 Release Notes

Release date: 2026-04-14

## Overview

DeltaScope `v0.30.0` is the **PostgreSQL ALTER TABLE GENERATED Boundary Pack**. It tightens the PostgreSQL unsupported boundary contract for `ALTER TABLE ... ADD COLUMN` forms that carry generated stored or identity semantics. It does not represent generated-column support, identity-column support, or broad PostgreSQL `ALTER TABLE` support.

## What Changed

DeltaScope now returns explicit unsupported outcomes for these PostgreSQL `ALTER TABLE ... ADD COLUMN` forms:

1. `GENERATED ALWAYS AS (...) STORED` → `generated_column`
2. `GENERATED ALWAYS AS IDENTITY` → `generated_as_identity`

```sql
ALTER TABLE users ADD COLUMN full_name text GENERATED ALWAYS AS (first_name || ' ' || last_name) STORED;
ALTER TABLE users ADD COLUMN id bigint GENERATED ALWAYS AS IDENTITY;
```

These forms no longer look like ordinary supported add-column paths.

## Unsupported Contract

| Shape | Unsupported feature |
|-------|---------------------|
| `ALTER TABLE ... ADD COLUMN ... GENERATED ALWAYS AS (...) STORED` | `generated_column` |
| `ALTER TABLE ... ADD COLUMN ... GENERATED ALWAYS AS IDENTITY` | `generated_as_identity` |

Surface behavior remains transport-specific and aligned with the existing unsupported contract:

- **CLI** and **`pkg/deltascope`** return partial results with an `unsupported` array and `ErrUnsupportedStatement`.
- **HTTP** and **MCP** expose unsupported statements as transport-level errors because the underlying audit call returns an error.

## Confidence Surface

This release locks the boundary contract across all relevant confidence layers:

- SQL corpus coverage
- service-level checks
- surface parity for CLI, HTTP, MCP, and `pkg/deltascope`

The release does not add new rule IDs, new CLI flags, or new public API contracts.

## What Did Not Change

- Adjacent PostgreSQL generated / identity alteration forms such as `DROP EXPRESSION`, `SET GENERATED`, and `DROP IDENTITY` remain generic unsupported boundaries.
- DeltaScope still does not model generated expressions or identity semantics as supported shared-contract features.
- MySQL and TiDB audit behavior is unchanged.
- `v0.26.0` PostgreSQL `CREATE TABLE` unsupported boundaries remain in place for generated stored columns, identity columns, exclusion constraints, and partitioned tables.

## Follow-up

- Decide whether any additional PostgreSQL `ALTER TABLE` generated / identity forms should receive stable explicit unsupported subtypes.
- Decide later whether these explicit unsupported boundaries should ever become real semantic support.

## Install / Upgrade

```bash
# macOS (recommended)
brew tap Fanduzi/deltascope
brew install --cask deltascope

# Generic installer
curl -fsSL https://raw.githubusercontent.com/Fanduzi/DeltaScope/v0.30.0/install.sh | \
  DELTASCOPE_VERSION=v0.30.0 sh
```
