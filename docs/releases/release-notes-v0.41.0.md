# DeltaScope v0.41.0 Release Notes

## Summary

DeltaScope v0.41.0 preserves statement-local check constraint facts for approved PostgreSQL `ALTER TABLE ... ADD CONSTRAINT CHECK` forms, allowing existing check constraint naming rules and the PostgreSQL `NOT VALID` advisory rule to produce findings across CLI, HTTP, MCP, and `pkg/deltascope` surfaces.

## What Changed

### PostgreSQL ALTER TABLE ADD CONSTRAINT CHECK Fact Support

PostgreSQL `ALTER TABLE ... ADD CONSTRAINT ... CHECK` forms now preserve statement-local check constraint metadata through the PostgreSQL extractor. The `DDL.Constraints` projection allows existing check naming rules and the PostgreSQL `NOT VALID` advisory to trigger on ALTER TABLE CHECK additions.

| Preserved Fact | Description |
|----------------|-------------|
| Constraint name | Explicitly named CHECK constraint identifier |
| Check expression | The boolean expression defining the constraint |

### Rule Coverage Unlocked

| Rule ID | What It Flags |
|---------|---------------|
| `ddl.pg.alter.add_check.not_valid.require` | `ADD CHECK` constraint should use `NOT VALID` to avoid a full table scan with `ACCESS EXCLUSIVE` lock |
| `ddl.constraint.check.name.prefix.require` | Explicitly named check constraint does not start with the required structured naming prefix (when configured) |
| `ddl.constraint.check.name.suffix.require` | Explicitly named check constraint does not end with the required structured naming suffix (when configured) |
| `ddl.constraint.check.name.contains.require` | Explicitly named check constraint does not contain any configured structured naming token (when configured) |

### Public Surfaces

All four product surfaces produce explicit `rule_id` findings:

| Surface | Behavior |
|---------|----------|
| CLI | Normal audit output with `rule_id` findings |
| HTTP (`POST /v1/audit`) | Normal audit response with `rule_id` findings |
| MCP (`audit_sql`) | Normal tool result with `rule_id` findings |
| `pkg/deltascope` | `Audit()` returns result with findings containing explicit `rule_id` |

### Docker-backed E2E Coverage

PostgreSQL CLI e2e covers `ddl.pg.alter.add_check.not_valid.require` and `ddl.constraint.check.name.prefix.require` for statement-local `ALTER TABLE ... ADD CONSTRAINT ... CHECK` audit through the Docker-backed test path.

## Supported Forms

- Named CHECK: `ALTER TABLE orders ADD CONSTRAINT amount_positive CHECK (amount >= 0)`
- Config-driven naming governance: `ddl.constraint.check.name.prefix.require` with prefix `ck_`

## Non-goals

- No live schema CHECK existence validation.
- No new rule IDs — `ddl.pg.alter.add_check.not_valid.require` was already registered; check naming rules cover ALTER CHECK through extended applicability.
- No `NOT VALID` validation enforcement or deferred constraint support.
- No MySQL/TiDB behavior changes.

## Upgrade Notes

No configuration or policy changes are required. The `ddl.pg.alter.add_check.not_valid.require` rule fires by default on `ALTER TABLE ... ADD CONSTRAINT ... CHECK` statements when `--dialect postgresql` is set. Check naming rules require explicit configuration (`prefix`, `suffix`, or `contains` parameters) to produce findings.

## Install

```bash
curl -fsSL https://raw.githubusercontent.com/Fanduzi/DeltaScope/v0.41.0/install.sh | \
  DELTASCOPE_VERSION=v0.41.0 sh
```
