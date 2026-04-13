# DeltaScope v0.27.0 Release Notes

Release date: 2026-04-13

## Overview

DeltaScope `v0.27.0` is the **Schema-Qualified Reference Semantics Pack**. It preserves PostgreSQL schema-qualified referenced-object facts in the shared `spec.Constraint` contract, backed by corpus cases and service-level semantic tests. This release does not add new rules, new CLI flags, or new public API contracts, and it does not represent full PostgreSQL foreign key support or schema-aware rule decisions.

## What Changed

The PostgreSQL extractor now preserves the schema portion of schema-qualified `REFERENCES` and `FOREIGN KEY ... REFERENCES` forms:

```sql
-- Both forms now preserve ReferencedSchema = "public", ReferencedTable = "users"
CREATE TABLE orders (
    user_id bigint REFERENCES public.users(id)
);

CREATE TABLE orders (
    user_id bigint,
    FOREIGN KEY (user_id) REFERENCES public.users(id)
);
```

The `spec.Constraint` struct gains an additive `ReferencedSchema` field. The normalized representation always separates schema and table:

| Field | Value |
|-------|-------|
| `ReferencedSchema` | `"public"` |
| `ReferencedTable` | `"users"` |

`ReferencedTable` is never concatenated into `"public.users"`.

## Semantic Contract

The schema-qualified reference semantics are locked by three layers:

1. **Parser/extractor**: the PostgreSQL extractor populates `ReferencedSchema` alongside the existing `ReferencedTable` and `ReferencedColumns` fields.
2. **Corpus-level**: `testdata/sql-corpus/postgresql/` includes dedicated schema-qualified reference cases with `.expected.yaml` assertions on `ReferencedSchema` and `ReferencedTable`.
3. **Service-level**: semantic tests assert that schema-qualified reference facts are preserved through the audit pipeline.

## Surface Contract

Current public finding metadata remains unchanged:

- **CLI**: FK forbid findings do not include `referenced_schema` in the output.
- **HTTP** and **MCP**: finding metadata does not expose `referenced_schema`.
- **`pkg/deltascope`**: the public `Result` type does not carry `referenced_schema` in finding metadata.

The shared semantic contract (`spec.Constraint`) is richer underneath, but the public transport surfaces preserve their existing supported behavior.

## What Did Not Change

- No new rule IDs were added. `ReferencedSchema` is an extractor/shared semantic fact, not a rule ID.
- No new CLI flags, HTTP payload contracts, MCP tool contracts, or public Go API contracts.
- No changes to the PostgreSQL unsupported boundary contracts from `v0.26.0`.
- No changes to the MySQL or TiDB audit surfaces.
- Existing FK forbid rule metadata does not include the `referenced_schema` field.

## Follow-up

- Decide whether public finding metadata should expose referenced-object schema facts.
- Schema-aware FK policy/rule work remains a future decision point, not a committed milestone.
- `ALTER TABLE ... GENERATED` boundary coverage from `v0.26.0` follow-up remains uncommitted.

## Install / Upgrade

```bash
# macOS (recommended)
brew tap Fanduzi/deltascope
brew install --cask deltascope

# Generic installer
curl -fsSL https://raw.githubusercontent.com/Fanduzi/DeltaScope/v0.27.0/install.sh | \
  DELTASCOPE_VERSION=v0.27.0 sh
```
