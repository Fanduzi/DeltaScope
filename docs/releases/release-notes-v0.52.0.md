# DeltaScope v0.52.0 Release Notes

## Summary

v0.52.0 normalizes six previously unsupported PostgreSQL ALTER TABLE actions. DeltaScope now handles `ALTER TABLE ... SET SCHEMA`, `ALTER TABLE ... OWNER TO`, `ALTER TABLE ... ENABLE/DISABLE TRIGGER name`, and `ALTER TABLE ... ATTACH/DETACH PARTITION` through the audit pipeline instead of returning unsupported — with six new PostgreSQL-only rules providing actionable findings.

## Added

- Six new PostgreSQL-only rules:

| Rule ID | Trigger | Default Level |
|---------|---------|---------------|
| `ddl.pg.alter.set_schema.advisory` | `ALTER TABLE ... SET SCHEMA` | notice |
| `ddl.pg.alter.owner.advisory` | `ALTER TABLE ... OWNER TO` | notice |
| `ddl.pg.alter.enable_trigger.notice` | `ALTER TABLE ... ENABLE TRIGGER name` | notice |
| `ddl.pg.alter.disable_trigger.warn` | `ALTER TABLE ... DISABLE TRIGGER name` | warning |
| `ddl.pg.alter.attach_partition.advisory` | `ALTER TABLE ... ATTACH PARTITION` | notice |
| `ddl.pg.alter.detach_partition.warn` | `ALTER TABLE ... DETACH PARTITION` | warning |

- Parser/extractor normalization for all six ALTER TABLE action types.
- Corpus fixtures covering each rule's trigger forms.
- Service-level tests through `AuditSQL` for all six rules.
- Public surface tests across all four surfaces: `pkg/deltascope` Audit, CLI Execute, HTTP handler, and MCP audit_sql tool.
- AST census tests documenting stable parser facts for each action type.

## Non-Goals

- This is not full PostgreSQL ALTER TABLE grammar support. Remaining ALTER TABLE sub-commands (e.g., `ALTER COLUMN TYPE`, `ADD CONSTRAINT ... NOT VALID`, `ENABLE/DISABLE TRIGGER ALL/USER`, `REPLICA IDENTITY`) are explicit boundaries.
- Partition bound semantic analysis is not performed. `ATTACH PARTITION ... FOR VALUES` bounds are not validated against the parent partition scheme.
- `ENABLE/DISABLE TRIGGER ALL` and `ENABLE/DISABLE TRIGGER USER` variants remain deferred.
- No MySQL/TiDB behavior changes.
- No default policy changes beyond the six new PostgreSQL-only rule entries.

## Install

**macOS (recommended):**

```bash
brew tap fanduzi/deltascope
brew install --cask deltascope
```

**Generic installer:**

```bash
curl -fsSL https://raw.githubusercontent.com/Fanduzi/DeltaScope/v0.52.0/install.sh | \
  DELTASCOPE_VERSION=v0.52.0 sh
```
