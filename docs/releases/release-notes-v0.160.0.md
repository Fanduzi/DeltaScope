# DeltaScope v0.160.0 Release Notes

## Summary — PostgreSQL ALTER TABLE Constraint Deferrability Rules

v0.160.0 adds 2 new PostgreSQL-only ALTER TABLE rules covering FK constraint deferrability changes, and silently normalizes `SET WITHOUT OIDS`. The PostgreSQL ALTER TABLE residual census now shows 56 of 66 forms `finding_covered` (up from 54), with `unsupported_boundary` reduced from 7 to 4. Total PostgreSQL ALTER TABLE rule count rises to 28.

## New Rules

### Constraint Deferrability (2 rules)

| Rule ID | Default Level | Trigger |
|---------|--------------|---------|
| `ddl.pg.alter.constraint_deferrable.notice` | notice | `ALTER TABLE ... ALTER CONSTRAINT ... DEFERRABLE` marks an FK constraint as deferrable |
| `ddl.pg.alter.constraint_initially_deferred.notice` | notice | `ALTER TABLE ... ALTER CONSTRAINT ... INITIALLY DEFERRED` marks an FK constraint as initially deferred |

## Silent Normalization

| Action | SQL Form | Behavior |
|--------|----------|----------|
| `set_without_oids` | `ALTER TABLE ... SET WITHOUT OIDS` | Normalized silently, no finding emitted. Obsolete since PostgreSQL 12 (OIDs removed from user tables). |

## Census Movement

| Metric | Before v0.160.0 | After v0.160.0 |
|--------|----------------|----------------|
| total | 66 | 66 |
| finding_covered | 54 | 56 |
| normalized_silent | 1 | 2 |
| unsupported_boundary | 7 | 4 |
| parser_error | 4 | 4 |
| unclassified | 0 | 0 |

### Remaining unsupported_boundary (4)

| SQL Form | Reason |
|----------|--------|
| `ALTER TABLE ... SET EXPRESSION AS (...)` | Expression body inseparable from column identity |
| `ALTER TABLE ... ADD GENERATED ... AS IDENTITY` | Sequence options and identity semantics require careful modeling |
| `ALTER TABLE ... EXCLUDE USING gist (...)` | Exclusion constraints involve operator classes and complex predicate expressions |
| `ALTER TABLE ALL IN TABLESPACE ... SET TABLESPACE ...` | Bulk tablespace move is a different statement kind (`AlterTableMoveAllStmt`) |

### Remaining parser_error (4)

Four PostgreSQL 18 grammar forms remain out of scope pending `pg_query_go v7` (blocked on libpg_query 18 support).

## SQL Corpus

| Metric | Value |
|--------|-------|
| policy_rule_ids | 340 |
| supported_rule_dialect_targets | 531 |
| covered_rule_dialect_targets | 531 |
| coverage_percent | 100.0 |
| expected_yaml_files_total | 239 |
| missing_rule_dialect_targets | 0 |

## No-Leak Contract

Constraint deferrability findings emit bounded metadata only:

| Key | Description |
|-----|-------------|
| `operation` | `alter_table` |
| `action` | `alter_constraint_deferrable` or `alter_constraint_initially_deferred` |
| `table` | Target table name |
| `constraint` | FK constraint name |
| `constraint_type` | `foreign_key` |
| `deferrable` | Boolean flag (`"true"` / `"false"`) |
| `initially_deferred` | Boolean flag (`"true"` / `"false"`) |

Forbidden metadata keys (never emitted): `raw_sql`, `expression`, `predicate`, `operator_class`, `exclusions`, `sequence_options`, `catalog_state`, `validation_result`, `dependency_graph`.

## Non-Goals

- Not full PostgreSQL ALTER TABLE support.
- No PostgreSQL 18 parser support.
- No live catalog validation.
- No runtime FK behavior validation.
- No lock/rewrite duration estimate.
- No support yet for `SET EXPRESSION`, `ADD GENERATED ... AS IDENTITY`, `EXCLUDE USING`, or `ALTER TABLE ALL IN TABLESPACE ... SET TABLESPACE ...`.
- No v1.0/stable API contract claim.
