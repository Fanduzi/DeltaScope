# DeltaScope v0.130.0 Release Notes

## Summary — PostgreSQL ALTER TABLE Residual Coverage / Semantics Depth

v0.130.0 deepens PostgreSQL ALTER TABLE residual coverage with 10 new rules across three semantic families: storage/layout, trigger/rule residual, and reloptions. The PostgreSQL ALTER TABLE residual census now shows 40 of 66 forms `finding_covered` (up from 29), with `unsupported_boundary` reduced from 32 to 21.

## New Rules

### Storage / Layout (2 rules)

| Rule ID | Default Level | Trigger |
|---------|--------------|---------|
| `ddl.pg.alter.set_tablespace.notice` | notice | `ALTER TABLE ... SET TABLESPACE ...` |
| `ddl.pg.alter.set_access_method.warn` | warning | `ALTER TABLE ... SET ACCESS METHOD ...` |

### Trigger / Rule Residual (6 rules)

| Rule ID | Default Level | Trigger |
|---------|--------------|---------|
| `ddl.pg.alter.enable_replica_trigger.notice` | notice | `ALTER TABLE ... ENABLE REPLICA TRIGGER ...` |
| `ddl.pg.alter.enable_always_trigger.notice` | notice | `ALTER TABLE ... ENABLE ALWAYS TRIGGER ...` |
| `ddl.pg.alter.enable_rule.notice` | notice | `ALTER TABLE ... ENABLE RULE ...` |
| `ddl.pg.alter.disable_rule.warn` | warning | `ALTER TABLE ... DISABLE RULE ...` |
| `ddl.pg.alter.enable_replica_rule.notice` | notice | `ALTER TABLE ... ENABLE REPLICA RULE ...` |
| `ddl.pg.alter.enable_always_rule.notice` | notice | `ALTER TABLE ... ENABLE ALWAYS RULE ...` |

### Reloptions (2 rules)

| Rule ID | Default Level | Trigger |
|---------|--------------|---------|
| `ddl.pg.alter.set_reloptions.warn` | warning | `ALTER TABLE ... SET (...)` with reloption keys |
| `ddl.pg.alter.reset_reloptions.notice` | notice | `ALTER TABLE ... RESET (...)` with reloption keys |

## Census Movement

| Metric | Before v0.130.0 | After v0.130.0 |
|--------|----------------|----------------|
| total | 66 | 66 |
| finding_covered | 29 | 40 |
| normalized_silent | 1 | 1 |
| unsupported_boundary | 32 | 21 |
| parser_error | 4 | 4 |
| unclassified | 0 | 0 |

## SQL Corpus

| Metric | Value |
|--------|-------|
| policy_rule_ids | 326 |
| supported_rule_dialect_targets | 517 |
| covered_rule_dialect_targets | 517 |
| coverage_percent | 100.0% |
| expected_yaml_files_total | 225 |

## No-Leak Contract

- **Storage/layout**: findings do not emit raw tablespace names, access method names, or live validation claims.
- **Trigger/rule residual**: findings do not emit trigger function names, trigger body text, rule query text, or rule command text.
- **Reloptions**: findings do not emit option names or values (e.g., `fillfactor`, `autovacuum_enabled`, `70`, `false`).

## Non-Goals

- Not full PostgreSQL ALTER TABLE support.
- No live catalog validation.
- No rewrite duration estimate.
- No runtime behavior validation.
- No DCL expansion.
- Remaining `unsupported_boundary` forms deferred to later milestones.
- No v1.0/stable API contract claim.
