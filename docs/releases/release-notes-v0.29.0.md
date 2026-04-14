# DeltaScope v0.29.0 Release Notes

Release date: 2026-04-14

## Overview

DeltaScope `v0.29.0` is the **Schema-Aware FK Policy Pack**. It is the first schema-aware FK policy step: a PostgreSQL-only notice rule, `ddl.pg.table.foreign_key.cross_schema.advisory`, for explicit cross-schema foreign keys. It does not represent a broad PostgreSQL FK engine, a cross-schema validation workflow, or `search_path` modeling.

## What Changed

DeltaScope now emits an extra advisory finding when all of the following are true:

1. The audit dialect is PostgreSQL.
2. The owning table schema is explicit.
3. The referenced schema is explicit.
4. The owning table schema and referenced schema are different.

```sql
-- Emits ddl.pg.table.foreign_key.cross_schema.advisory
CREATE TABLE billing.orders (
    id bigint PRIMARY KEY,
    approver_id bigint,
    CONSTRAINT fk_orders_approver FOREIGN KEY (approver_id) REFERENCES auth.users(id)
);
```

The existing FK forbid rule still applies under the default policy. The new advisory adds schema context; it does not replace `ddl.table.foreign_key.forbid`.

## Rule Contract

| Field | Contract |
|-------|----------|
| Rule ID | `ddl.pg.table.foreign_key.cross_schema.advisory` |
| Default level | `notice` |
| Dialect | PostgreSQL only |
| Trigger | owning table schema and referenced schema are both explicit and different |
| Same-schema FK | does not trigger |
| Bare reference | `REFERENCES users(id)` does not trigger because the referenced schema is unknown |

## Metadata Contract

Advisory findings may include these metadata fields:

| Field | Example |
|-------|---------|
| `table_schema` | `"billing"` |
| `referenced_schema` | `"auth"` |
| `referenced_table` | `"users"` |
| `referenced_columns` | `["id"]` |

`referenced_table` remains normalized as `"users"`, never `"auth.users"`. Schema and table remain separate fields.

## What Did Not Change

- Bare references remain schema unknown. DeltaScope does not infer `public` and does not model PostgreSQL `search_path` semantics.
- Same-schema foreign keys do not trigger the new advisory.
- No parser, extractor, corpus, or public API expansion is implied by this release note.
- MySQL and TiDB audit behavior is unchanged.
- `v0.28.0` metadata widening remains the underlying surface that exposes `referenced_schema`, `referenced_table`, and `referenced_columns` where those facts exist.

## Follow-up

- Decide whether schema-aware FK policy should expand beyond this explicit cross-schema advisory.
- `ALTER TABLE ... GENERATED` boundary coverage remains uncommitted.

## Install / Upgrade

```bash
# macOS (recommended)
brew tap Fanduzi/deltascope
brew install --cask deltascope

# Generic installer
curl -fsSL https://raw.githubusercontent.com/Fanduzi/DeltaScope/v0.29.0/install.sh | \
  DELTASCOPE_VERSION=v0.29.0 sh
```
