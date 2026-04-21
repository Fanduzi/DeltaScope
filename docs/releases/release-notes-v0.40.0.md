# DeltaScope v0.40.0 Release Notes

## Summary

DeltaScope v0.40.0 preserves statement-local foreign key facts for approved PostgreSQL `ALTER TABLE ... ADD CONSTRAINT FOREIGN KEY` forms, allowing existing FK rules to produce findings across CLI, HTTP, MCP, and `pkg/deltascope` surfaces.

## What Changed

### PostgreSQL ALTER TABLE Foreign Key Fact Support

PostgreSQL `ALTER TABLE ... ADD CONSTRAINT ... FOREIGN KEY` forms now preserve statement-local FK facts through the PostgreSQL extractor. The `DDL.Constraints` projection allows existing FK rules to trigger on ALTER TABLE FK additions.

| Preserved Fact | Description |
|----------------|-------------|
| Local columns | Columns in the owning table that participate in the FK |
| Referenced table | Target table of the foreign key reference |
| Referenced columns | Columns in the referenced table |
| Referenced schema | Schema qualifier when the reference uses `schema.table` form |

### Rule Coverage Unlocked

| Rule ID | What It Flags |
|---------|---------------|
| `ddl.table.foreign_key.forbid` | Foreign key constraints are forbidden under the default policy |
| `ddl.pg.table.foreign_key.cross_schema.advisory` | Owning table schema and referenced schema are both explicit and different (notice) |

### Public Surfaces

All four product surfaces produce explicit `rule_id` findings:

| Surface | Behavior |
|---------|----------|
| CLI | Normal audit output with `rule_id` findings |
| HTTP (`POST /v1/audit`) | Normal audit response with `rule_id` findings |
| MCP (`audit_sql`) | Normal tool result with `rule_id` findings |
| `pkg/deltascope` | `Audit()` returns result with findings containing explicit `rule_id` |

### Docker-backed E2E Coverage

PostgreSQL CLI e2e covers `ddl.table.foreign_key.forbid` for statement-local `ALTER TABLE ... ADD CONSTRAINT ... FOREIGN KEY` audit through the Docker-backed test path.

## Supported Forms

- Named FK: `ALTER TABLE orders ADD CONSTRAINT fk_orders_user FOREIGN KEY (user_id) REFERENCES users(id)`
- Composite FK facts (tested forms)
- Schema-qualified references preserve `referenced_schema`

## Non-goals

- No live schema FK existence validation.
- No new FK rule IDs — existing rules cover ALTER TABLE FK additions through extended applicability and the `DDL.Constraints` projection.
- No deferrable constraint support or MATCH FULL policy expansion.
- No full constraint/index parity claim.
- No MySQL/TiDB behavior changes.

## Upgrade Notes

No configuration or policy changes are required. Existing FK rules that were already active for `CREATE TABLE` FK declarations now also apply to `ALTER TABLE ... ADD CONSTRAINT ... FOREIGN KEY` statements when `--dialect postgresql` is set.

## Install

```bash
curl -fsSL https://raw.githubusercontent.com/Fanduzi/DeltaScope/v0.40.0/install.sh | \
  DELTASCOPE_VERSION=v0.40.0 sh
```
