# DeltaScope v0.28.0 Release Notes

Release date: 2026-04-13

## Overview

DeltaScope `v0.28.0` is the **Referenced-Object Metadata Surface Pack**. It exposes PostgreSQL referenced-object facts that already existed in the shared semantic contract (`ReferencedSchema`, `ReferencedTable`, `ReferencedColumns`) as additive finding metadata on the FK forbid rule, across all four transport surfaces (CLI, HTTP, MCP, `pkg/deltascope`). This release does not add new rules, new CLI flags, or new public API contracts, and it does not represent schema-aware FK policy support or full PostgreSQL foreign key support.

## What Changed

The `ddl.table.foreign_key.forbid` finding metadata now includes referenced-object fields when the underlying constraint carries those facts:

```sql
-- Schema-qualified FK triggers finding with referenced-object metadata
CREATE TABLE orders (
    id bigint PRIMARY KEY,
    approver_id bigint,
    CONSTRAINT fk_orders_approver FOREIGN KEY (approver_id) REFERENCES public.users(id)
);
```

Finding metadata now contains:

| Field | Value |
|-------|-------|
| `referenced_schema` | `"public"` |
| `referenced_table` | `"users"` |
| `referenced_columns` | `["id"]` |

`referenced_table` is never concatenated into `"public.users"`. The schema and table are always separate fields.

## Metadata Contract

The metadata widening is additive and conditional:

- `referenced_schema` — present when the FK references a schema-qualified table (e.g., `public.users`). Omitted when no schema qualifier is present.
- `referenced_table` — present when the FK constraint has a referenced table (standard for all FK constraints).
- `referenced_columns` — present when the FK constraint has referenced columns (standard for all FK constraints).

Existing metadata fields (`table`, `constraint`, `columns`) are unchanged.

## What Did Not Change

- No new rule IDs. The `ddl.table.foreign_key.forbid` rule is unchanged; only its finding metadata is wider.
- No new CLI flags, HTTP payload contracts, MCP tool contracts, or public Go API contracts.
- No changes to the PostgreSQL parser or extractor; `ReferencedSchema` preservation was shipped in `v0.27.0`.
- No changes to the unsupported boundary contracts.
- No changes to the MySQL or TiDB audit surfaces.
- No schema-aware FK policy decisions or cross-schema validation.

## Follow-up

- Schema-aware FK policy/rule work remains a future decision point.
- `ALTER TABLE ... GENERATED` boundary coverage remains uncommitted.

## Install / Upgrade

```bash
# macOS (recommended)
brew tap Fanduzi/deltascope
brew install --cask deltascope

# Generic installer
curl -fsSL https://raw.githubusercontent.com/Fanduzi/DeltaScope/v0.28.0/install.sh | \
  DELTASCOPE_VERSION=v0.28.0 sh
```
