# DeltaScope v0.42.0 Release Notes

## Summary

DeltaScope v0.42.0 adds PostgreSQL NOT VALID constraint validation pairing. DeltaScope now warns when a named `ALTER TABLE ... ADD CONSTRAINT ... NOT VALID` CHECK or FOREIGN KEY constraint is not followed by a later matching `ALTER TABLE ... VALIDATE CONSTRAINT ...` statement in the same audited SQL batch.

## Added

- New PostgreSQL-only GlobalRule: `ddl.pg.alter.not_valid_constraint.validate.require`
- Default level: `warning`
- Scope: named CHECK / FOREIGN KEY `NOT VALID` constraint additions
- Matching key: same schema + table + constraint name
- Behavior: a later matching `VALIDATE CONSTRAINT` suppresses the warning
- Surfaces: CLI, HTTP, MCP, and `pkg/deltascope` expose the result as a global finding
- Confidence: SQL corpus coverage and Docker-backed PostgreSQL e2e lock the contract

## Example SQL

Problematic batch:

```sql
ALTER TABLE orders ADD CONSTRAINT chk_orders_amount CHECK (amount >= 0) NOT VALID;
```

Paired clean batch:

```sql
ALTER TABLE orders ADD CONSTRAINT chk_orders_amount CHECK (amount >= 0) NOT VALID;
ALTER TABLE orders VALIDATE CONSTRAINT chk_orders_amount;
```

## CLI Example

```bash
deltascope audit \
  --dialect postgresql \
  --format json \
  --sql "ALTER TABLE orders ADD CONSTRAINT chk_orders_amount CHECK (amount >= 0) NOT VALID;"
```

Example JSON excerpt:

```json
{
  "global_findings": [
    {
      "rule_id": "ddl.pg.alter.not_valid_constraint.validate.require",
      "level": "warning",
      "message": "NOT VALID constraint \"chk_orders_amount\" on table \"orders\" should be followed by VALIDATE CONSTRAINT in the audited migration batch"
    }
  ]
}
```

## Rule Contract

| Field | Value |
|------|-------|
| Rule ID | `ddl.pg.alter.not_valid_constraint.validate.require` |
| Kind | PostgreSQL-only GlobalRule |
| Default level | `warning` |
| Applies to | Named CHECK / FOREIGN KEY `NOT VALID` constraints |
| Suppression | Later matching `VALIDATE CONSTRAINT` in the same audited SQL batch |

## Non-Goals

- First-time `VALIDATE CONSTRAINT` support
- Live database validation-state lookup
- Cross-file or cross-deployment validation tracking
- Matching unnamed constraints
- Validating CHECK expression correctness
- Validating FK referenced-table correctness
- Changing MySQL/TiDB behavior
- Adding a new public API contract

## Install

```bash
curl -fsSL https://raw.githubusercontent.com/Fanduzi/DeltaScope/v0.42.0/install.sh | \
  DELTASCOPE_VERSION=v0.42.0 sh
```
