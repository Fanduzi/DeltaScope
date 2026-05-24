# DeltaScope v0.170.0 Release Notes

## Summary — PostgreSQL ALTER TABLE Final Parseable Boundary Rules

v0.170.0 adds 4 new PostgreSQL-only ALTER TABLE notice rules covering the final parseable boundary forms: SET EXPRESSION, ADD IDENTITY, ADD EXCLUSION CONSTRAINT, and ALL IN TABLESPACE. The PostgreSQL ALTER TABLE residual census now shows 64 of 66 forms `finding_covered` (up from 60), with `unsupported_boundary` holding at 0. Total PostgreSQL ALTER TABLE rule count rises to 32.

## New Rules

### Final Parseable Boundary (4 rules)

| Rule ID | Default Level | Trigger |
|---------|--------------|---------|
| `ddl.pg.alter.set_expression.notice` | notice | `ALTER TABLE ... ALTER COLUMN ... SET EXPRESSION` sets a generated column expression |
| `ddl.pg.alter.add_identity.notice` | notice | `ALTER TABLE ... ALTER COLUMN ... ADD GENERATED ... AS IDENTITY` adds identity to an existing column |
| `ddl.pg.alter.add_exclusion_constraint.notice` | notice | `ALTER TABLE ... ADD CONSTRAINT ... EXCLUDE USING` adds an exclusion constraint |
| `ddl.pg.alter.move_all_tablespace.notice` | notice | `ALTER TABLE ALL IN TABLESPACE ... SET TABLESPACE ...` moves all tables in a tablespace |

## Census Movement

| Metric | Before v0.170.0 | After v0.170.0 |
|--------|----------------|----------------|
| total | 66 | 66 |
| finding_covered | 60 | 64 |
| normalized_silent | 2 | 2 |
| unsupported_boundary | 0 | 0 |
| parser_error | 4 | 4 |
| unclassified | 0 | 0 |

### Remaining parser_error (4)

Four PostgreSQL 18 grammar forms remain out of scope pending `pg_query_go v7` (blocked on libpg_query 18 support).

## SQL Corpus

| Metric | Value |
|--------|-------|
| policy_rule_ids | 344 |
| supported_rule_dialect_targets | 535 |
| covered_rule_dialect_targets | 535 |
| coverage_percent | 100.0 |
| expected_yaml_files_total | 243 |
| missing_rule_dialect_targets | 0 |

## No-Leak Contract

Final parseable boundary findings emit bounded metadata only:

| Key | Description |
|-----|-------------|
| `operation` | `alter_table` |
| `action` | `set_expression`, `add_identity`, `add_exclusion_constraint`, or `move_all_tablespace` |
| `table` | Target table name |
| `column` | Target column name (for SET EXPRESSION and ADD IDENTITY) |
| `constraint` | Constraint name (for ADD EXCLUSION CONSTRAINT) |

Forbidden metadata keys (never emitted): `raw_sql`, `expression_body`, `sequence_options`, `exclusion_operators`, `exclusion_predicates`, `operator_class`, `catalog_state`, `tablespace_name`.

## Non-Goals

- Not full PostgreSQL ALTER TABLE support.
- No PostgreSQL 18 parser support.
- No runtime/live validation.
- No rewrite duration estimate.
- No v1.0/stable API contract claim.
