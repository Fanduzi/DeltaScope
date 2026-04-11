# DeltaScope v0.21.0 Release Notes

Release date: 2026-04-11

## Overview

DeltaScope `v0.21.0` is the **PostgreSQL DDL Coverage Pack** — a release that expands PostgreSQL DDL normalization so that common migration follow-up statements are processed through the shared audit pipeline instead of returning capability-boundary errors.

No new rules are added. No existing rule IDs, levels, or trigger conditions change. The value is that existing shared rule families and metadata-aware semantics now cover more PostgreSQL DDL actions.

## What's Changed

### PostgreSQL ALTER TABLE Coverage Expansion

Six common PostgreSQL `ALTER TABLE` forms that previously returned capability-boundary errors are now normalized into the shared `spec.Alter` contract:

| PostgreSQL DDL | Normalized Action | What This Means |
|----------------|-------------------|-----------------|
| `ALTER TABLE ... ALTER COLUMN ... SET DEFAULT` | `set_default` | Column default assignment during phased rollout is now auditable |
| `ALTER TABLE ... ALTER COLUMN ... DROP DEFAULT` | `drop_default` | Column default removal is now auditable |
| `ALTER TABLE ... ALTER COLUMN ... SET NOT NULL` | `set_not_null` | Nullability enforcement after backfill is now auditable |
| `ALTER TABLE ... ALTER COLUMN ... DROP NOT NULL` | `drop_not_null` | Nullability relaxation is now auditable |
| `ALTER TABLE ... VALIDATE CONSTRAINT` | `validate_constraint` | Constraint validation step in the recommended `NOT VALID` → `VALIDATE` pattern is now auditable |
| `ALTER TABLE ... DROP CONSTRAINT` | `drop_constraint` | Constraint removal is now auditable; primary-key drops map to existing `ddl.alter.drop_primary_key` rules when metadata is available |

### Shared Rule Reuse

This release does not introduce new rules. Instead, the newly normalized PostgreSQL DDL actions flow through existing shared rule families:

- **Alter semantic rules** apply to `set_default`, `drop_default`, `set_not_null`, and `drop_not_null` as standard alter actions.
- **Metadata-aware primary-key rules** apply when `DROP CONSTRAINT` targets a primary key and metadata is available.
- **`VALIDATE CONSTRAINT`** is supported and auditable but has no dedicated rule. It produces a clean audit result unless other findings apply.

### Surface Parity

All newly normalized PostgreSQL DDL actions are confirmed across four surfaces:

- **CLI**: `deltascope audit --dialect postgresql --sql "..."`
- **HTTP**: `POST /v1/audit` with `"dialect": "postgresql"`
- **MCP**: `audit_sql` tool with `"dialect": "postgresql"`
- **Public Go API**: `deltascope.Audit(ctx, deltascope.Request{Dialect: deltascope.DialectPostgreSQL, ...})`

### Examples

Audit a phased migration follow-up:

```bash
deltascope audit \
  --dialect postgresql \
  --sql "alter table users alter column status set default 'active';"
```

Audit a constraint lifecycle step:

```bash
deltascope audit \
  --dialect postgresql \
  --sql "alter table users validate constraint chk_amount;"
```

Audit a constraint removal:

```bash
deltascope audit \
  --dialect postgresql \
  --sql "alter table orders drop constraint orders_pkey;"
```

## Important Notes

- **`VALIDATE CONSTRAINT`** is supported and auditable but does not have a dedicated rule. It produces a clean audit result unless other findings apply to the same statement. It is not "guaranteed to produce a finding."
- **`DROP CONSTRAINT` targeting a primary key** (e.g., `DROP CONSTRAINT users_pkey`) triggers existing `ddl.alter.drop_primary_key` rules only in metadata-aware mode. In offline mode, it passes through as a normal alter action.
- This release narrows the unsupported PostgreSQL DDL surface. It does not claim full PostgreSQL DDL support.

## Install / Upgrade

```bash
curl -fsSL https://raw.githubusercontent.com/Fanduzi/DeltaScope/v0.21.0/install.sh | \
  DELTASCOPE_VERSION=v0.21.0 sh
```

macOS users can install with Homebrew:

```bash
brew tap Fanduzi/deltascope
brew install --cask deltascope
```

## Compatibility

No breaking changes. `v0.21.0` extends the existing audit contract with additive coverage:

- All existing MySQL/TiDB/PostgreSQL offline and metadata-aware behavior is unchanged
- No new rule IDs, severity levels, or trigger conditions are introduced
- The newly normalized actions flow through existing rule families; no policy YAML changes are required
- CLI, HTTP, MCP, and `pkg/deltascope` public API contracts are unchanged — only the set of PostgreSQL DDL forms that produce normal audit results (instead of capability-boundary errors) has expanded
