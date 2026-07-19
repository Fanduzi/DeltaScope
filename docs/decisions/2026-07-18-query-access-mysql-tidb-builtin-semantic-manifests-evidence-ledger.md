# Evidence Ledger: MySQL/TiDB Builtin Semantic Manifests

- Date: 2026-07-18
- Status: Accepted
- Decision: [2026-07-18-query-access-mysql-tidb-builtin-semantic-manifests.md](2026-07-18-query-access-mysql-tidb-builtin-semantic-manifests.md)
- Baseline: `main@9491c5f`
- HEAD: `9c26a15`

## Purpose

This ledger records the per-entry evidence chain for every production
builtin semantic manifest entry. Each row is backed by primary
documentation, live Docker probes against the exact server image,
parser-native-form facts, complete candidate closure, strict physical
dependency requirements, no-leak coverage, and cross-surface parity.

## Probe ID Scheme

Probe IDs follow `P-<profile>-<function>-<shape>` for positive probes and
`N-<profile>-<boundary>` for negative boundary probes. The live probe
implementation is in `internal/infrastructure/metadata/mysql/builtin_semantic_live_probes_test.go`
and `internal/infrastructure/metadata/mysql/builtin_semantic_boundary_live_probes_test.go`
(build tag `integration`). The live SDK session E2E is in
`pkg/deltascope/query_access_session_mysql_tidb_live_e2e_test.go`.

## Docker Evidence Matrix

| Profile | Docker image | Observed VERSION() | Compose service | Fixture |
|---|---|---|---|---|
| `mysql-5.7` | `mysql:5.7.44` | `5.7.44` | `mysql57` | `docker/query-access-builtin-mysql-init.sql` |
| `mysql-8.0` | `mysql:8.0.46` | `8.0.46` | `mysql80` | `docker/query-access-builtin-mysql-init.sql` |
| `mysql-8.4` | `mysql:8.4.10` | `8.4.10` | `mysql84` | `docker/query-access-builtin-mysql-init.sql` |
| `tidb-8.5` | `pingcap/tidb:v8.5.7` | `8.0.11-TiDB-v8.5.7` | `tidb85` + `tidb85-fixture` | `docker/query-access-builtin-tidb-init.sql` |

Compose lifecycle:

```bash
docker compose -f docker/query-access-builtin-compose.yaml down -v --remove-orphans
docker compose -f docker/query-access-builtin-compose.yaml up -d --wait mysql57 mysql80 mysql84 tidb85 tidb85-fixture
docker compose -f docker/query-access-builtin-compose.yaml down -v --remove-orphans
```

Live probe command:

```bash
go test -tags integration ./internal/infrastructure/metadata/mysql \
  -run 'TestBuiltinSemantic(57|80|84|TiDB85)_Live' -count=1 -v
```

Live SDK E2E command:

```bash
go test -tags integration ./pkg/deltascope \
  -run TestLiveProfile -count=1 -v
```

## Shared Boundary Probes

The following boundary probes apply to every aggregate entry in every
profile. They are executed per-profile against the exact Docker image.

| Probe ID | Boundary | Expected outcome |
|---|---|---|
| `N-any-stored-function-collision` | `CREATE FUNCTION count (value INT) RETURNS INT DETERMINISTIC RETURN value` | accepted for MySQL (builtin name shadowing exists); rejected for TiDB (TiDB rejects stored functions) |
| `N-any-udf-creation` | `CREATE FUNCTION semantic_probe_count RETURNS STRING SONAME 'deltascope_missing_udf.so'` | rejected |
| `N-any-qualified-call` | `SELECT app.COUNT(*) FROM app.builtin_semantic_facts` | rejected |
| `N-any-quoted-call` | `` SELECT `COUNT`(*) FROM app.builtin_semantic_facts `` | rejected |
| `N-any-noncanonical-spacing` | `SELECT COUNT (id) FROM app.builtin_semantic_facts` (no `IGNORE_SPACE`) | rejected |
| `N-any-ignore-space-spacing` | `SELECT COUNT (id) FROM app.builtin_semantic_facts` (with `IGNORE_SPACE`) | accepted |
| `N-any-ignore-space-comment` | `SELECT COUNT/**/(*) FROM app.builtin_semantic_facts` (with `IGNORE_SPACE`) | rejected |

The following boundary probes apply to every window entry in every 8.x
profile:

| Probe ID | Boundary | Expected outcome |
|---|---|---|
| `N-any-window-explicit-frame` | `ROW_NUMBER() OVER (... ROWS BETWEEN 1 PRECEDING AND CURRENT ROW)` | indeterminate (not promoted) |
| `N-any-window-named-window` | `ROW_NUMBER() OVER w ... WINDOW w AS (...)` | indeterminate (not promoted) |
| `N-any-window-nested-partition` | `ROW_NUMBER() OVER (PARTITION BY ABS(dept) ...)` | indeterminate (not promoted) |
| `N-any-window-nested-order` | `ROW_NUMBER() OVER (... ORDER BY ABS(id))` | indeterminate (not promoted) |
| `N-any-window-literal-partition` | `ROW_NUMBER() OVER (PARTITION BY 1 ...)` | indeterminate (not promoted) |
| `N-any-window-missing-order` | `ROW_NUMBER() OVER (PARTITION BY dept)` | indeterminate (not promoted) |
| `N-any-window-missing-partition` | `ROW_NUMBER() OVER (ORDER BY id)` | indeterminate (not promoted) |

The following boundary probes apply to every aggregate entry with a
column operand:

| Probe ID | Boundary | Expected outcome |
|---|---|---|
| `N-any-distinct` | `SELECT COUNT(DISTINCT amount) FROM app.builtin_semantic_facts` | indeterminate (not promoted) |
| `N-any-filter` | `SELECT COUNT(*) FILTER (WHERE id > 0) FROM app.builtin_semantic_facts` | indeterminate (not promoted) |
| `N-any-agg-local-order` | `SELECT COUNT(id ORDER BY id) FROM app.builtin_semantic_facts` | indeterminate (not promoted) |
| `N-any-nested-operand` | `SELECT COUNT(ABS(amount)) FROM app.builtin_semantic_facts` | indeterminate (not promoted) |
| `N-any-literal-operand` | `SELECT COUNT(1) FROM app.builtin_semantic_facts` | indeterminate (not promoted) |
| `N-any-unknown-function` | `SELECT app_specific_rollup(amount) FROM app.builtin_semantic_facts` | indeterminate (not promoted) |
| `N-any-mixed-proven-unknown` | `SELECT COUNT(*), app_specific_rollup(id) FROM app.builtin_semantic_facts` | indeterminate (not promoted) |

## Per-Entry Evidence

### mysql-5.7 / count (aggregate, arity=0, star)

- **Entry key**: `mysql|mysql-5.7|count|aggregate|0|[star]|false false false false false false false false false false`
- **Primary documentation**: [MySQL 5.7 COUNT() reference](https://dev.mysql.com/doc/refman/5.7/en/aggregate-functions.html#function_count) (retrieved 2026-07-18)
- **Docker image**: `mysql:5.7.44` (observed `5.7.44`)
- **Positive probe ID**: `P-mysql57-count-star` — `SELECT COUNT(*) FROM app.builtin_semantic_facts` returns `4`
- **Negative probe IDs**: `N-any-stored-function-collision`, `N-any-udf-creation`, `N-any-qualified-call`, `N-any-quoted-call`, `N-any-noncanonical-spacing`, `N-any-ignore-space-spacing`, `N-any-ignore-space-comment`, `N-any-distinct`, `N-any-filter`, `N-any-agg-local-order`, `N-any-nested-operand`, `N-any-literal-operand`, `N-any-unknown-function`, `N-any-mixed-proven-unknown`
- **Strict dependency shape**: `COUNT(*)` has arity=0, no column operands; strict mode requires `read_table` for the base table only
- **Excluded modifiers**: `DISTINCT`, `FILTER`, aggregate-local `ORDER BY`, nested operands, literals, parameters, casts
- **Disposition**: supported

### mysql-5.7 / count (aggregate, arity=1, column)

- **Entry key**: `mysql|mysql-5.7|count|aggregate|1|[column]|false false false false false false false false false false`
- **Primary documentation**: [MySQL 5.7 COUNT(col) reference](https://dev.mysql.com/doc/refman/5.7/en/aggregate-functions.html#function_count) (retrieved 2026-07-18)
- **Docker image**: `mysql:5.7.44` (observed `5.7.44`)
- **Positive probe ID**: `P-mysql57-count-column` — `SELECT COUNT(amount) FROM app.builtin_semantic_facts` returns `4`
- **Negative probe IDs**: `N-any-stored-function-collision`, `N-any-udf-creation`, `N-any-qualified-call`, `N-any-quoted-call`, `N-any-noncanonical-spacing`, `N-any-ignore-space-spacing`, `N-any-ignore-space-comment`, `N-any-distinct`, `N-any-filter`, `N-any-agg-local-order`, `N-any-nested-operand`, `N-any-literal-operand`, `N-any-unknown-function`, `N-any-mixed-proven-unknown`
- **Strict dependency shape**: `COUNT(col)` has arity=1, one column operand; strict mode requires `read_table` for the base table and `read_column` for the column
- **Excluded modifiers**: `DISTINCT`, `FILTER`, aggregate-local `ORDER BY`, nested operands, literals, parameters, casts
- **Disposition**: supported

### mysql-5.7 / sum (aggregate, arity=1, column)

- **Entry key**: `mysql|mysql-5.7|sum|aggregate|1|[column]|false false false false false false false false false false`
- **Primary documentation**: [MySQL 5.7 SUM() reference](https://dev.mysql.com/doc/refman/5.7/en/aggregate-functions.html#function_sum) (retrieved 2026-07-18)
- **Docker image**: `mysql:5.7.44` (observed `5.7.44`)
- **Positive probe ID**: `P-mysql57-sum-column` — `SELECT SUM(amount) FROM app.builtin_semantic_facts` returns `250`
- **Negative probe IDs**: same set as `mysql-5.7 / count (column)`
- **Strict dependency shape**: same as `mysql-5.7 / count (column)`
- **Excluded modifiers**: same as `mysql-5.7 / count (column)`
- **Disposition**: supported

### mysql-5.7 / avg (aggregate, arity=1, column)

- **Entry key**: `mysql|mysql-5.7|avg|aggregate|1|[column]|false false false false false false false false false false`
- **Primary documentation**: [MySQL 5.7 AVG() reference](https://dev.mysql.com/doc/refman/5.7/en/aggregate-functions.html#function_avg) (retrieved 2026-07-18)
- **Docker image**: `mysql:5.7.44` (observed `5.7.44`)
- **Positive probe ID**: `P-mysql57-avg-column` — `SELECT AVG(amount) FROM app.builtin_semantic_facts` returns `62.5`
- **Negative probe IDs**: same set as `mysql-5.7 / count (column)`
- **Strict dependency shape**: same as `mysql-5.7 / count (column)`
- **Excluded modifiers**: same as `mysql-5.7 / count (column)`
- **Disposition**: supported

### mysql-5.7 / min (aggregate, arity=1, column)

- **Entry key**: `mysql|mysql-5.7|min|aggregate|1|[column]|false false false false false false false false false false`
- **Primary documentation**: [MySQL 5.7 MIN() reference](https://dev.mysql.com/doc/refman/5.7/en/aggregate-functions.html#function_min) (retrieved 2026-07-18)
- **Docker image**: `mysql:5.7.44` (observed `5.7.44`)
- **Positive probe ID**: `P-mysql57-min-column` — `SELECT MIN(amount) FROM app.builtin_semantic_facts` returns `20`
- **Negative probe IDs**: same set as `mysql-5.7 / count (column)`
- **Strict dependency shape**: same as `mysql-5.7 / count (column)`
- **Excluded modifiers**: same as `mysql-5.7 / count (column)`
- **Disposition**: supported

### mysql-5.7 / max (aggregate, arity=1, column)

- **Entry key**: `mysql|mysql-5.7|max|aggregate|1|[column]|false false false false false false false false false false`
- **Primary documentation**: [MySQL 5.7 MAX() reference](https://dev.mysql.com/doc/refman/5.7/en/aggregate-functions.html#function_max) (retrieved 2026-07-18)
- **Docker image**: `mysql:5.7.44` (observed `5.7.44`)
- **Positive probe ID**: `P-mysql57-max-column` — `SELECT MAX(amount) FROM app.builtin_semantic_facts` returns `100`
- **Negative probe IDs**: same set as `mysql-5.7 / count (column)`
- **Strict dependency shape**: same as `mysql-5.7 / count (column)`
- **Excluded modifiers**: same as `mysql-5.7 / count (column)`
- **Disposition**: supported

### mysql-5.7 / ranking windows — DEFERRED

- **Reason**: MySQL 5.7 has no native ranking-window support. The
  `ROW_NUMBER()`, `RANK()`, and `DENSE_RANK()` functions are not
  available in MySQL 5.7. Live probes confirm the ranking-window
  evidence is deferred for this profile.
- **Disposition**: deferred

### mysql-8.0 / count (aggregate, arity=0, star)

- **Entry key**: `mysql|mysql-8.0|count|aggregate|0|[star]|false false false false false false false false false false`
- **Primary documentation**: [MySQL 8.0 COUNT() reference](https://dev.mysql.com/doc/refman/8.0/en/aggregate-functions.html#function_count) (retrieved 2026-07-18)
- **Docker image**: `mysql:8.0.46` (observed `8.0.46`)
- **Positive probe ID**: `P-mysql80-count-star` — `SELECT COUNT(*) FROM app.builtin_semantic_facts` returns `4`
- **Negative probe IDs**: same set as `mysql-5.7 / count (star)`
- **Strict dependency shape**: same as `mysql-5.7 / count (star)`
- **Excluded modifiers**: same as `mysql-5.7 / count (star)`
- **Disposition**: supported (independently evidenced from mysql-5.7)

### mysql-8.0 / count (aggregate, arity=1, column)

- **Entry key**: `mysql|mysql-8.0|count|aggregate|1|[column]|false false false false false false false false false false`
- **Primary documentation**: [MySQL 8.0 COUNT(col) reference](https://dev.mysql.com/doc/refman/8.0/en/aggregate-functions.html#function_count) (retrieved 2026-07-18)
- **Docker image**: `mysql:8.0.46` (observed `8.0.46`)
- **Positive probe ID**: `P-mysql80-count-column` — `SELECT COUNT(amount) FROM app.builtin_semantic_facts` returns `4`
- **Negative probe IDs**: same set as `mysql-5.7 / count (column)`
- **Strict dependency shape**: same as `mysql-5.7 / count (column)`
- **Excluded modifiers**: same as `mysql-5.7 / count (column)`
- **Disposition**: supported (independently evidenced from mysql-5.7)

### mysql-8.0 / sum (aggregate, arity=1, column)

- **Entry key**: `mysql|mysql-8.0|sum|aggregate|1|[column]|false false false false false false false false false false`
- **Primary documentation**: [MySQL 8.0 SUM() reference](https://dev.mysql.com/doc/refman/8.0/en/aggregate-functions.html#function_sum) (retrieved 2026-07-18)
- **Docker image**: `mysql:8.0.46` (observed `8.0.46`)
- **Positive probe ID**: `P-mysql80-sum-column` — `SELECT SUM(amount) FROM app.builtin_semantic_facts` returns `250`
- **Negative probe IDs**: same set as `mysql-5.7 / count (column)`
- **Strict dependency shape**: same as `mysql-5.7 / count (column)`
- **Excluded modifiers**: same as `mysql-5.7 / count (column)`
- **Disposition**: supported (independently evidenced from mysql-5.7)

### mysql-8.0 / avg (aggregate, arity=1, column)

- **Entry key**: `mysql|mysql-8.0|avg|aggregate|1|[column]|false false false false false false false false false false`
- **Primary documentation**: [MySQL 8.0 AVG() reference](https://dev.mysql.com/doc/refman/8.0/en/aggregate-functions.html#function_avg) (retrieved 2026-07-18)
- **Docker image**: `mysql:8.0.46` (observed `8.0.46`)
- **Positive probe ID**: `P-mysql80-avg-column` — `SELECT AVG(amount) FROM app.builtin_semantic_facts` returns `62.5`
- **Negative probe IDs**: same set as `mysql-5.7 / count (column)`
- **Strict dependency shape**: same as `mysql-5.7 / count (column)`
- **Excluded modifiers**: same as `mysql-5.7 / count (column)`
- **Disposition**: supported (independently evidenced from mysql-5.7)

### mysql-8.0 / min (aggregate, arity=1, column)

- **Entry key**: `mysql|mysql-8.0|min|aggregate|1|[column]|false false false false false false false false false false`
- **Primary documentation**: [MySQL 8.0 MIN() reference](https://dev.mysql.com/doc/refman/8.0/en/aggregate-functions.html#function_min) (retrieved 2026-07-18)
- **Docker image**: `mysql:8.0.46` (observed `8.0.46`)
- **Positive probe ID**: `P-mysql80-min-column` — `SELECT MIN(amount) FROM app.builtin_semantic_facts` returns `20`
- **Negative probe IDs**: same set as `mysql-5.7 / count (column)`
- **Strict dependency shape**: same as `mysql-5.7 / count (column)`
- **Excluded modifiers**: same as `mysql-5.7 / count (column)`
- **Disposition**: supported (independently evidenced from mysql-5.7)

### mysql-8.0 / max (aggregate, arity=1, column)

- **Entry key**: `mysql|mysql-8.0|max|aggregate|1|[column]|false false false false false false false false false false`
- **Primary documentation**: [MySQL 8.0 MAX() reference](https://dev.mysql.com/doc/refman/8.0/en/aggregate-functions.html#function_max) (retrieved 2026-07-18)
- **Docker image**: `mysql:8.0.46` (observed `8.0.46`)
- **Positive probe ID**: `P-mysql80-max-column` — `SELECT MAX(amount) FROM app.builtin_semantic_facts` returns `100`
- **Negative probe IDs**: same set as `mysql-5.7 / count (column)`
- **Strict dependency shape**: same as `mysql-5.7 / count (column)`
- **Excluded modifiers**: same as `mysql-5.7 / count (column)`
- **Disposition**: supported (independently evidenced from mysql-5.7)

### mysql-8.0 / row_number (window, arity=0)

- **Entry key**: `mysql|mysql-8.0|row_number|window|0|[]|false false false false false false true true true true`
- **Primary documentation**: [MySQL 8.0 ROW_NUMBER() reference](https://dev.mysql.com/doc/refman/8.0/en/window-functions.html#function_row-number) (retrieved 2026-07-18)
- **Docker image**: `mysql:8.0.46` (observed `8.0.46`)
- **Positive probe ID**: `P-mysql80-row-number` — `SELECT id, ROW_NUMBER() OVER (PARTITION BY dept ORDER BY amount DESC, id) FROM app.builtin_semantic_facts ORDER BY id` returns `[(1,1),(2,2),(3,1),(4,2)]`
- **Negative probe IDs**: `N-any-window-explicit-frame`, `N-any-window-named-window`, `N-any-window-nested-partition`, `N-any-window-nested-order`, `N-any-window-literal-partition`, `N-any-window-missing-order`, `N-any-window-missing-partition`
- **Strict dependency shape**: `ROW_NUMBER()` has arity=0; window requires `PARTITION BY` and `ORDER BY` with direct column operands; strict mode requires `read_table` for the base table and `read_column` for every partition and order column
- **Excluded modifiers**: frames, named windows, `FILTER`, `DISTINCT`, aggregate-local `ORDER BY`, nested operands, literals, parameters, casts
- **Disposition**: supported

### mysql-8.0 / rank (window, arity=0)

- **Entry key**: `mysql|mysql-8.0|rank|window|0|[]|false false false false false false true true true true`
- **Primary documentation**: [MySQL 8.0 RANK() reference](https://dev.mysql.com/doc/refman/8.0/en/window-functions.html#function_rank) (retrieved 2026-07-18)
- **Docker image**: `mysql:8.0.46` (observed `8.0.46`)
- **Positive probe ID**: `P-mysql80-rank` — `SELECT id, RANK() OVER (PARTITION BY dept ORDER BY amount DESC, id) FROM app.builtin_semantic_facts ORDER BY id` returns `[(1,1),(2,2),(3,1),(4,2)]`
- **Negative probe IDs**: same set as `mysql-8.0 / row_number`
- **Strict dependency shape**: same as `mysql-8.0 / row_number`
- **Excluded modifiers**: same as `mysql-8.0 / row_number`
- **Disposition**: supported

### mysql-8.0 / dense_rank (window, arity=0)

- **Entry key**: `mysql|mysql-8.0|dense_rank|window|0|[]|false false false false false false true true true true`
- **Primary documentation**: [MySQL 8.0 DENSE_RANK() reference](https://dev.mysql.com/doc/refman/8.0/en/window-functions.html#function_dense-rank) (retrieved 2026-07-18)
- **Docker image**: `mysql:8.0.46` (observed `8.0.46`)
- **Positive probe ID**: `P-mysql80-dense-rank` — `SELECT id, DENSE_RANK() OVER (PARTITION BY dept ORDER BY amount DESC, id) FROM app.builtin_semantic_facts ORDER BY id` returns `[(1,1),(2,2),(3,1),(4,2)]`
- **Negative probe IDs**: same set as `mysql-8.0 / row_number`
- **Strict dependency shape**: same as `mysql-8.0 / row_number`
- **Excluded modifiers**: same as `mysql-8.0 / row_number`
- **Disposition**: supported

### mysql-8.4 / count (aggregate, arity=0, star)

- **Entry key**: `mysql|mysql-8.4|count|aggregate|0|[star]|false false false false false false false false false false`
- **Primary documentation**: [MySQL 8.4 COUNT() reference](https://dev.mysql.com/doc/refman/8.4/en/aggregate-functions.html#function_count) (retrieved 2026-07-18)
- **Docker image**: `mysql:8.4.10` (observed `8.4.10`)
- **Positive probe ID**: `P-mysql84-count-star` — `SELECT COUNT(*) FROM app.builtin_semantic_facts` returns `4`
- **Negative probe IDs**: same set as `mysql-5.7 / count (star)`
- **Strict dependency shape**: same as `mysql-5.7 / count (star)`
- **Excluded modifiers**: same as `mysql-5.7 / count (star)`
- **Disposition**: supported (independently evidenced from mysql-5.7 and mysql-8.0)

### mysql-8.4 / count (aggregate, arity=1, column)

- **Entry key**: `mysql|mysql-8.4|count|aggregate|1|[column]|false false false false false false false false false false`
- **Primary documentation**: [MySQL 8.4 COUNT(col) reference](https://dev.mysql.com/doc/refman/8.4/en/aggregate-functions.html#function_count) (retrieved 2026-07-18)
- **Docker image**: `mysql:8.4.10` (observed `8.4.10`)
- **Positive probe ID**: `P-mysql84-count-column` — `SELECT COUNT(amount) FROM app.builtin_semantic_facts` returns `4`
- **Negative probe IDs**: same set as `mysql-5.7 / count (column)`
- **Strict dependency shape**: same as `mysql-5.7 / count (column)`
- **Excluded modifiers**: same as `mysql-5.7 / count (column)`
- **Disposition**: supported (independently evidenced from mysql-5.7 and mysql-8.0)

### mysql-8.4 / sum (aggregate, arity=1, column)

- **Entry key**: `mysql|mysql-8.4|sum|aggregate|1|[column]|false false false false false false false false false false`
- **Primary documentation**: [MySQL 8.4 SUM() reference](https://dev.mysql.com/doc/refman/8.4/en/aggregate-functions.html#function_sum) (retrieved 2026-07-18)
- **Docker image**: `mysql:8.4.10` (observed `8.4.10`)
- **Positive probe ID**: `P-mysql84-sum-column` — `SELECT SUM(amount) FROM app.builtin_semantic_facts` returns `250`
- **Negative probe IDs**: same set as `mysql-5.7 / count (column)`
- **Strict dependency shape**: same as `mysql-5.7 / count (column)`
- **Excluded modifiers**: same as `mysql-5.7 / count (column)`
- **Disposition**: supported (independently evidenced from mysql-5.7 and mysql-8.0)

### mysql-8.4 / avg (aggregate, arity=1, column)

- **Entry key**: `mysql|mysql-8.4|avg|aggregate|1|[column]|false false false false false false false false false false`
- **Primary documentation**: [MySQL 8.4 AVG() reference](https://dev.mysql.com/doc/refman/8.4/en/aggregate-functions.html#function_avg) (retrieved 2026-07-18)
- **Docker image**: `mysql:8.4.10` (observed `8.4.10`)
- **Positive probe ID**: `P-mysql84-avg-column` — `SELECT AVG(amount) FROM app.builtin_semantic_facts` returns `62.5`
- **Negative probe IDs**: same set as `mysql-5.7 / count (column)`
- **Strict dependency shape**: same as `mysql-5.7 / count (column)`
- **Excluded modifiers**: same as `mysql-5.7 / count (column)`
- **Disposition**: supported (independently evidenced from mysql-5.7 and mysql-8.0)

### mysql-8.4 / min (aggregate, arity=1, column)

- **Entry key**: `mysql|mysql-8.4|min|aggregate|1|[column]|false false false false false false false false false false`
- **Primary documentation**: [MySQL 8.4 MIN() reference](https://dev.mysql.com/doc/refman/8.4/en/aggregate-functions.html#function_min) (retrieved 2026-07-18)
- **Docker image**: `mysql:8.4.10` (observed `8.4.10`)
- **Positive probe ID**: `P-mysql84-min-column` — `SELECT MIN(amount) FROM app.builtin_semantic_facts` returns `20`
- **Negative probe IDs**: same set as `mysql-5.7 / count (column)`
- **Strict dependency shape**: same as `mysql-5.7 / count (column)`
- **Excluded modifiers**: same as `mysql-5.7 / count (column)`
- **Disposition**: supported (independently evidenced from mysql-5.7 and mysql-8.0)

### mysql-8.4 / max (aggregate, arity=1, column)

- **Entry key**: `mysql|mysql-8.4|max|aggregate|1|[column]|false false false false false false false false false false`
- **Primary documentation**: [MySQL 8.4 MAX() reference](https://dev.mysql.com/doc/refman/8.4/en/aggregate-functions.html#function_max) (retrieved 2026-07-18)
- **Docker image**: `mysql:8.4.10` (observed `8.4.10`)
- **Positive probe ID**: `P-mysql84-max-column` — `SELECT MAX(amount) FROM app.builtin_semantic_facts` returns `100`
- **Negative probe IDs**: same set as `mysql-5.7 / count (column)`
- **Strict dependency shape**: same as `mysql-5.7 / count (column)`
- **Excluded modifiers**: same as `mysql-5.7 / count (column)`
- **Disposition**: supported (independently evidenced from mysql-5.7 and mysql-8.0)

### mysql-8.4 / row_number (window, arity=0)

- **Entry key**: `mysql|mysql-8.4|row_number|window|0|[]|false false false false false false true true true true`
- **Primary documentation**: [MySQL 8.4 ROW_NUMBER() reference](https://dev.mysql.com/doc/refman/8.4/en/window-functions.html#function_row-number) (retrieved 2026-07-18)
- **Docker image**: `mysql:8.4.10` (observed `8.4.10`)
- **Positive probe ID**: `P-mysql84-row-number` — `SELECT id, ROW_NUMBER() OVER (PARTITION BY dept ORDER BY amount DESC, id) FROM app.builtin_semantic_facts ORDER BY id` returns `[(1,1),(2,2),(3,1),(4,2)]`
- **Negative probe IDs**: same set as `mysql-8.0 / row_number`
- **Strict dependency shape**: same as `mysql-8.0 / row_number`
- **Excluded modifiers**: same as `mysql-8.0 / row_number`
- **Disposition**: supported (independently evidenced from mysql-8.0)

### mysql-8.4 / rank (window, arity=0)

- **Entry key**: `mysql|mysql-8.4|rank|window|0|[]|false false false false false false true true true true`
- **Primary documentation**: [MySQL 8.4 RANK() reference](https://dev.mysql.com/doc/refman/8.4/en/window-functions.html#function_rank) (retrieved 2026-07-18)
- **Docker image**: `mysql:8.4.10` (observed `8.4.10`)
- **Positive probe ID**: `P-mysql84-rank` — `SELECT id, RANK() OVER (PARTITION BY dept ORDER BY amount DESC, id) FROM app.builtin_semantic_facts ORDER BY id` returns `[(1,1),(2,2),(3,1),(4,2)]`
- **Negative probe IDs**: same set as `mysql-8.0 / row_number`
- **Strict dependency shape**: same as `mysql-8.0 / row_number`
- **Excluded modifiers**: same as `mysql-8.0 / row_number`
- **Disposition**: supported (independently evidenced from mysql-8.0)

### mysql-8.4 / dense_rank (window, arity=0)

- **Entry key**: `mysql|mysql-8.4|dense_rank|window|0|[]|false false false false false false true true true true`
- **Primary documentation**: [MySQL 8.4 DENSE_RANK() reference](https://dev.mysql.com/doc/refman/8.4/en/window-functions.html#function_dense-rank) (retrieved 2026-07-18)
- **Docker image**: `mysql:8.4.10` (observed `8.4.10`)
- **Positive probe ID**: `P-mysql84-dense-rank` — `SELECT id, DENSE_RANK() OVER (PARTITION BY dept ORDER BY amount DESC, id) FROM app.builtin_semantic_facts ORDER BY id` returns `[(1,1),(2,2),(3,1),(4,2)]`
- **Negative probe IDs**: same set as `mysql-8.0 / row_number`
- **Strict dependency shape**: same as `mysql-8.0 / row_number`
- **Excluded modifiers**: same as `mysql-8.0 / row_number`
- **Disposition**: supported (independently evidenced from mysql-8.0)

### tidb-8.5 / count (aggregate, arity=0, star)

- **Entry key**: `tidb|tidb-8.5|count|aggregate|0|[star]|false false false false false false false false false false`
- **Primary documentation**: [TiDB COUNT() reference](https://docs.pingcap.com/tidb/stable/aggregate-functions/) (retrieved 2026-07-18)
- **Docker image**: `pingcap/tidb:v8.5.7` (observed `8.0.11-TiDB-v8.5.7`)
- **Positive probe ID**: `P-tidb85-count-star` — `SELECT COUNT(*) FROM app.builtin_semantic_facts` returns `4`
- **Negative probe IDs**: `N-any-stored-function-collision` (rejected for TiDB), `N-any-udf-creation`, `N-any-qualified-call`, `N-any-quoted-call`, `N-any-noncanonical-spacing`, `N-any-ignore-space-spacing`, `N-any-ignore-space-comment`, `N-any-distinct`, `N-any-filter`, `N-any-agg-local-order`, `N-any-nested-operand`, `N-any-literal-operand`, `N-any-unknown-function`, `N-any-mixed-proven-unknown`
- **Strict dependency shape**: same as `mysql-5.7 / count (star)`
- **Excluded modifiers**: same as `mysql-5.7 / count (star)`
- **Disposition**: supported (independently evidenced; not copied from MySQL)

### tidb-8.5 / count (aggregate, arity=1, column)

- **Entry key**: `tidb|tidb-8.5|count|aggregate|1|[column]|false false false false false false false false false false`
- **Primary documentation**: [TiDB COUNT(col) reference](https://docs.pingcap.com/tidb/stable/aggregate-functions/) (retrieved 2026-07-18)
- **Docker image**: `pingcap/tidb:v8.5.7` (observed `8.0.11-TiDB-v8.5.7`)
- **Positive probe ID**: `P-tidb85-count-column` — `SELECT COUNT(amount) FROM app.builtin_semantic_facts` returns `4`
- **Negative probe IDs**: same set as `tidb-8.5 / count (star)`
- **Strict dependency shape**: same as `mysql-5.7 / count (column)`
- **Excluded modifiers**: same as `mysql-5.7 / count (column)`
- **Disposition**: supported (independently evidenced; not copied from MySQL)

### tidb-8.5 / sum (aggregate, arity=1, column)

- **Entry key**: `tidb|tidb-8.5|sum|aggregate|1|[column]|false false false false false false false false false false`
- **Primary documentation**: [TiDB SUM() reference](https://docs.pingcap.com/tidb/stable/aggregate-functions/) (retrieved 2026-07-18)
- **Docker image**: `pingcap/tidb:v8.5.7` (observed `8.0.11-TiDB-v8.5.7`)
- **Positive probe ID**: `P-tidb85-sum-column` — `SELECT SUM(amount) FROM app.builtin_semantic_facts` returns `250`
- **Negative probe IDs**: same set as `tidb-8.5 / count (star)`
- **Strict dependency shape**: same as `mysql-5.7 / count (column)`
- **Excluded modifiers**: same as `mysql-5.7 / count (column)`
- **Disposition**: supported (independently evidenced; not copied from MySQL)

### tidb-8.5 / avg (aggregate, arity=1, column)

- **Entry key**: `tidb|tidb-8.5|avg|aggregate|1|[column]|false false false false false false false false false false`
- **Primary documentation**: [TiDB AVG() reference](https://docs.pingcap.com/tidb/stable/aggregate-functions/) (retrieved 2026-07-18)
- **Docker image**: `pingcap/tidb:v8.5.7` (observed `8.0.11-TiDB-v8.5.7`)
- **Positive probe ID**: `P-tidb85-avg-column` — `SELECT AVG(amount) FROM app.builtin_semantic_facts` returns `62.5`
- **Negative probe IDs**: same set as `tidb-8.5 / count (star)`
- **Strict dependency shape**: same as `mysql-5.7 / count (column)`
- **Excluded modifiers**: same as `mysql-5.7 / count (column)`
- **Disposition**: supported (independently evidenced; not copied from MySQL)

### tidb-8.5 / min (aggregate, arity=1, column)

- **Entry key**: `tidb|tidb-8.5|min|aggregate|1|[column]|false false false false false false false false false false`
- **Primary documentation**: [TiDB MIN() reference](https://docs.pingcap.com/tidb/stable/aggregate-functions/) (retrieved 2026-07-18)
- **Docker image**: `pingcap/tidb:v8.5.7` (observed `8.0.11-TiDB-v8.5.7`)
- **Positive probe ID**: `P-tidb85-min-column` — `SELECT MIN(amount) FROM app.builtin_semantic_facts` returns `20`
- **Negative probe IDs**: same set as `tidb-8.5 / count (star)`
- **Strict dependency shape**: same as `mysql-5.7 / count (column)`
- **Excluded modifiers**: same as `mysql-5.7 / count (column)`
- **Disposition**: supported (independently evidenced; not copied from MySQL)

### tidb-8.5 / max (aggregate, arity=1, column)

- **Entry key**: `tidb|tidb-8.5|max|aggregate|1|[column]|false false false false false false false false false false`
- **Primary documentation**: [TiDB MAX() reference](https://docs.pingcap.com/tidb/stable/aggregate-functions/) (retrieved 2026-07-18)
- **Docker image**: `pingcap/tidb:v8.5.7` (observed `8.0.11-TiDB-v8.5.7`)
- **Positive probe ID**: `P-tidb85-max-column` — `SELECT MAX(amount) FROM app.builtin_semantic_facts` returns `100`
- **Negative probe IDs**: same set as `tidb-8.5 / count (star)`
- **Strict dependency shape**: same as `mysql-5.7 / count (column)`
- **Excluded modifiers**: same as `mysql-5.7 / count (column)`
- **Disposition**: supported (independently evidenced; not copied from MySQL)

### tidb-8.5 / row_number (window, arity=0)

- **Entry key**: `tidb|tidb-8.5|row_number|window|0|[]|false false false false false false true true true true`
- **Primary documentation**: [TiDB ROW_NUMBER() reference](https://docs.pingcap.com/tidb/stable/window-functions/) (retrieved 2026-07-18)
- **Docker image**: `pingcap/tidb:v8.5.7` (observed `8.0.11-TiDB-v8.5.7`)
- **Positive probe ID**: `P-tidb85-row-number` — `SELECT id, ROW_NUMBER() OVER (PARTITION BY dept ORDER BY amount DESC, id) FROM app.builtin_semantic_facts ORDER BY id` returns `[(1,1),(2,2),(3,1),(4,2)]`
- **Negative probe IDs**: same set as `mysql-8.0 / row_number`
- **Strict dependency shape**: same as `mysql-8.0 / row_number`
- **Excluded modifiers**: same as `mysql-8.0 / row_number`
- **Disposition**: supported (independently evidenced; not copied from MySQL)

### tidb-8.5 / rank (window, arity=0)

- **Entry key**: `tidb|tidb-8.5|rank|window|0|[]|false false false false false false true true true true`
- **Primary documentation**: [TiDB RANK() reference](https://docs.pingcap.com/tidb/stable/window-functions/) (retrieved 2026-07-18)
- **Docker image**: `pingcap/tidb:v8.5.7` (observed `8.0.11-TiDB-v8.5.7`)
- **Positive probe ID**: `P-tidb85-rank` — `SELECT id, RANK() OVER (PARTITION BY dept ORDER BY amount DESC, id) FROM app.builtin_semantic_facts ORDER BY id` returns `[(1,1),(2,2),(3,1),(4,2)]`
- **Negative probe IDs**: same set as `mysql-8.0 / row_number`
- **Strict dependency shape**: same as `mysql-8.0 / row_number`
- **Excluded modifiers**: same as `mysql-8.0 / row_number`
- **Disposition**: supported (independently evidenced; not copied from MySQL)

### tidb-8.5 / dense_rank (window, arity=0)

- **Entry key**: `tidb|tidb-8.5|dense_rank|window|0|[]|false false false false false false true true true true`
- **Primary documentation**: [TiDB DENSE_RANK() reference](https://docs.pingcap.com/tidb/stable/window-functions/) (retrieved 2026-07-18)
- **Docker image**: `pingcap/tidb:v8.5.7` (observed `8.0.11-TiDB-v8.5.7`)
- **Positive probe ID**: `P-tidb85-dense-rank` — `SELECT id, DENSE_RANK() OVER (PARTITION BY dept ORDER BY amount DESC, id) FROM app.builtin_semantic_facts ORDER BY id` returns `[(1,1),(2,2),(3,1),(4,2)]`
- **Negative probe IDs**: same set as `mysql-8.0 / row_number`
- **Strict dependency shape**: same as `mysql-8.0 / row_number`
- **Excluded modifiers**: same as `mysql-8.0 / row_number`
- **Disposition**: supported (independently evidenced; not copied from MySQL)

## Cross-Profile Non-Aliasing Verification

Each profile's manifest entries are constructed independently. MySQL 5.7
has 6 aggregate entries and 0 window entries. MySQL 8.0 and 8.4 each
have 6 aggregate entries and 3 window entries. TiDB 8.5 has 6 aggregate
entries and 3 window entries. No profile inherits entries from another
dialect or version. The `TestProfileBoundaryRejectsCrossDialectPromotion`
test verifies a MySQL profile cannot affect TiDB and vice versa.

## Test Coverage Matrix

| Test class | Test file | Build tag |
|---|---|---|
| Manifest deep-copy and mutation | `builtin_semantic_manifest_test.go` | default |
| Gateway proof and adversarial rejection | `builtin_semantic_gateway_test.go` | default |
| Profile-specific regression (MySQL 5.7/8.0/8.4, TiDB 8.5) | `builtin_semantic_profile_regression_test.go` | default |
| SDK profile validation and no-leak | `query_access_profile_test.go` | default |
| SDK session boundary (synthetic driver) | `query_access_session_mysql_tidb_test.go` | default |
| Live infra probes (all 4 profiles) | `builtin_semantic_live_probes_test.go` | `integration` |
| Live boundary probes (all 4 profiles) | `builtin_semantic_boundary_live_probes_test.go` | `integration` |
| Live SDK session E2E (all 4 profiles) | `query_access_session_mysql_tidb_live_e2e_test.go` | `integration` |
| CLI offline profile pass-through and no-leak | `internal/interfaces/cli/query_access_test.go` | default |
| HTTP offline profile pass-through and no-leak | `internal/interfaces/http/query_access_test.go` | default |
| MCP no Query Access tool contract | `internal/interfaces/mcp/query_access_surface_contract_test.go` | default |
