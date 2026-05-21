# DeltaScope v0.140.0 Release Notes

## Summary — PostgreSQL ALTER TABLE Bounded Residual Pack

v0.140.0 adds 8 new PostgreSQL-only ALTER TABLE rules covering column attribute mutations (5) and cluster/detach-finalize operations (3). The PostgreSQL ALTER TABLE residual census now shows 50 of 66 forms `finding_covered` (up from 40), with `unsupported_boundary` reduced from 21 to 11.

## New Rules

### Column Attributes (5 rules)

| Rule ID | Default Level | Trigger |
|---------|--------------|---------|
| `ddl.pg.alter.set_column_statistics.notice` | notice | `ALTER TABLE ... ALTER COLUMN ... SET STATISTICS` |
| `ddl.pg.alter.set_column_options.notice` | notice | `ALTER TABLE ... ALTER COLUMN ... SET (...)` with attribute options |
| `ddl.pg.alter.reset_column_options.notice` | notice | `ALTER TABLE ... ALTER COLUMN ... RESET (...)` with attribute options |
| `ddl.pg.alter.set_column_storage.notice` | notice | `ALTER TABLE ... ALTER COLUMN ... SET STORAGE` |
| `ddl.pg.alter.set_column_compression.notice` | notice | `ALTER TABLE ... ALTER COLUMN ... SET COMPRESSION` |

### Cluster / Detach-Finalize (3 rules)

| Rule ID | Default Level | Trigger |
|---------|--------------|---------|
| `ddl.pg.alter.cluster_on.notice` | notice | `ALTER TABLE ... CLUSTER ON` |
| `ddl.pg.alter.set_without_cluster.notice` | notice | `ALTER TABLE ... SET WITHOUT CLUSTER` |
| `ddl.pg.alter.detach_partition_finalize.notice` | notice | `ALTER TABLE ... DETACH PARTITION ... FINALIZE` |

## Census Movement

| Metric | Before v0.140.0 | After v0.140.0 |
|--------|----------------|----------------|
| total | 66 | 66 |
| finding_covered | 40 | 50 |
| normalized_silent | 1 | 1 |
| unsupported_boundary | 21 | 11 |
| parser_error | 4 | 4 |
| unclassified | 0 | 0 |

## SQL Corpus

| Metric | Value |
|--------|-------|
| policy_rule_ids | 334 |
| supported_rule_dialect_targets | 525 |
| covered_rule_dialect_targets | 525 |
| coverage_percent | 100.0% |
| expected_yaml_files_total | 233 |

## No-Leak Contract

- **Column attributes**: findings do not emit raw SQL, option names or values (`n_distinct`, `-1`, `100`), storage parameter names (`plain`, `external`, `extended`, `main`), compression method names (`lz4`, `pglz`), or `compression_kind`.
- **Cluster / detach-finalize**: findings do not emit partition bounds, index names from catalog, or live validation claims.

## Non-Goals

- Not full PostgreSQL ALTER TABLE support.
- No live catalog validation.
- No rewrite duration estimate.
- No runtime behavior validation.
- No DCL expansion.
- Remaining `unsupported_boundary` forms (11) deferred to later milestones.
- No v1.0/stable API contract claim.
