# DeltaScope v0.150.0 Release Notes

## Summary — PostgreSQL ALTER TABLE Table Relationship Rules

v0.150.0 adds 4 new PostgreSQL-only ALTER TABLE rules covering table inheritance and typed table association operations. The PostgreSQL ALTER TABLE residual census now shows 54 of 66 forms `finding_covered` (up from 50), with `unsupported_boundary` reduced from 11 to 7. Total PostgreSQL ALTER TABLE rule count rises to 26.

## New Rules

### Table Relationships (4 rules)

| Rule ID | Default Level | Trigger |
|---------|--------------|---------|
| `ddl.pg.alter.add_inherit.notice` | notice | `ALTER TABLE ... INHERIT` adds a parent table inheritance relationship |
| `ddl.pg.alter.drop_inherit.notice` | notice | `ALTER TABLE ... NO INHERIT` removes a parent table inheritance relationship |
| `ddl.pg.alter.add_of_type.notice` | notice | `ALTER TABLE ... OF` associates the table with a typed table composite type |
| `ddl.pg.alter.drop_of_type.notice` | notice | `ALTER TABLE ... NOT OF` removes the typed table association |

## Census Movement

| Metric | Before v0.150.0 | After v0.150.0 |
|--------|----------------|----------------|
| total | 66 | 66 |
| finding_covered | 50 | 54 |
| normalized_silent | 1 | 1 |
| unsupported_boundary | 11 | 7 |
| parser_error | 4 | 4 |
| unclassified | 0 | 0 |

## Table Properties Coverage

| Category | Covered | Total |
|----------|---------|-------|
| table properties | 10 | 12 |

## SQL Corpus

| Metric | Value |
|--------|-------|
| supported_rule_dialect_targets | 529 |
| covered_rule_dialect_targets | 529 |
| coverage_percent | 100.0% |
| expected_yaml_files_total | 237 |

## No-Leak Contract

- **Table relationships**: findings do not emit parent table names, type names, catalog state, dependency graph, type shape, column shape, or validation result.

## Non-Goals

- Not full PostgreSQL ALTER TABLE support.
- No live catalog validation.
- No parent/type compatibility validation.
- No PostgreSQL 18 parser support.
- Remaining `unsupported_boundary` forms (7) deferred to later milestones.
- 4 `parser_error` forms waiting on pg_query_go v7.
- No v1.0/stable API contract claim.
