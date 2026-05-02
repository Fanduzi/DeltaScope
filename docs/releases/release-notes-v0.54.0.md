# DeltaScope v0.54.0 Release Notes

## Summary

v0.54.0 closes the remaining high-value PostgreSQL ALTER TABLE residual coverage around trigger-scope operations and replica identity configuration. DeltaScope now normalizes `ENABLE/DISABLE TRIGGER ALL|USER` and `REPLICA IDENTITY` variants, adds three PostgreSQL-only replica identity findings, and keeps `REPLICA IDENTITY DEFAULT` as a clean normalized pass.

## Normalized Forms

| SQL | Normalized Operation |
|-----|---------------------|
| `ALTER TABLE t ENABLE TRIGGER ALL` | `enable_trigger` (scope=`all`) |
| `ALTER TABLE t ENABLE TRIGGER USER` | `enable_trigger` (scope=`user`) |
| `ALTER TABLE t DISABLE TRIGGER ALL` | `disable_trigger` (scope=`all`) |
| `ALTER TABLE t DISABLE TRIGGER USER` | `disable_trigger` (scope=`user`) |
| `ALTER TABLE t REPLICA IDENTITY DEFAULT` | `replica_identity` (identity=`default`) |
| `ALTER TABLE t REPLICA IDENTITY FULL` | `replica_identity` (identity=`full`) |
| `ALTER TABLE t REPLICA IDENTITY NOTHING` | `replica_identity` (identity=`nothing`) |
| `ALTER TABLE t REPLICA IDENTITY USING INDEX idx` | `replica_identity` (identity=`using_index`, index=`idx`) |

## New PostgreSQL-Only Rules

| Rule ID | Trigger | Default Level |
|---------|---------|---------------|
| `ddl.pg.alter.replica_identity_full.warn` | `ALTER TABLE ... REPLICA IDENTITY FULL` | warning |
| `ddl.pg.alter.replica_identity_nothing.warn` | `ALTER TABLE ... REPLICA IDENTITY NOTHING` | warning |
| `ddl.pg.alter.replica_identity_using_index.notice` | `ALTER TABLE ... REPLICA IDENTITY USING INDEX ...` | notice |

- Trigger-scope forms (`ALL`/`USER`) reuse existing `ddl.pg.alter.enable_trigger.notice` and `ddl.pg.alter.disable_trigger.warn` rules.
- `REPLICA IDENTITY DEFAULT` is normalized and intentionally silent — no rule fires.

## Test Coverage

- AST census tests documenting stable parser facts for all eight residual forms.
- Parser/extractor normalization tests for trigger-scope and replica identity variants.
- Corpus fixtures covering all three new rules' trigger forms.
- Service-level tests through `AuditSQL` for replica identity and trigger-scope variants.
- Public surface tests across all four surfaces: `pkg/deltascope` Audit, CLI Execute, HTTP handler, and MCP audit_sql tool.

## Non-Goals

- DeltaScope does not inspect live trigger state or validate trigger definitions or functions.
- DeltaScope does not verify whether `REPLICA IDENTITY USING INDEX` names a valid, unique, or non-partial index.
- This is not full PostgreSQL ALTER TABLE grammar support.
- No MySQL/TiDB behavior changes.
- No default policy changes beyond the three new PostgreSQL-only rule entries.

## Install

**macOS (recommended):**

```bash
brew tap fanduzi/deltascope
brew install --cask deltascope
```

**Generic installer:**

```bash
curl -fsSL https://raw.githubusercontent.com/Fanduzi/DeltaScope/v0.54.0/install.sh | \
  DELTASCOPE_VERSION=v0.54.0 sh
```
