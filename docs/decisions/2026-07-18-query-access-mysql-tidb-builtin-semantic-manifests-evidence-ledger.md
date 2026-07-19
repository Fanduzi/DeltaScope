# Evidence Ledger: MySQL/TiDB Builtin Semantic Manifests

- Date: 2026-07-18
- Status: Accepted
- Decision: [2026-07-18-query-access-mysql-tidb-builtin-semantic-manifests.md](2026-07-18-query-access-mysql-tidb-builtin-semantic-manifests.md)
- Baseline: `main@9491c5f`
- HEAD: `dc2b425`

## Purpose

This ledger records the per-entry evidence chain for every production
builtin semantic manifest entry. Each row is backed by primary
documentation, live Docker probes against the exact server image,
parser-native-form facts, complete candidate closure, strict physical
dependency requirements, no-leak coverage, and cross-surface parity.

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

## Per-Entry Evidence

### mysql-5.7 / count (aggregate, arity=0, star)

- **Primary documentation**: [MySQL 5.7 COUNT() reference](https://dev.mysql.com/doc/refman/5.7/en/aggregate-functions.html#function_count) (retrieved 2026-07-18)
- **Docker image**: `mysql:5.7.44` (observed `5.7.44`)
- **Positive probe**: `SELECT COUNT(*) FROM app.builtin_semantic_facts` returns `4`
- **Boundary probes**: stored-function collision (accepted for `count`); UDF creation rejected; `app.COUNT(*)` rejected; `` `COUNT`(*) `` rejected; `COUNT (id)` rejected without `IGNORE_SPACE`; `COUNT (id)` accepted with `IGNORE_SPACE`; `COUNT/**/(*)` rejected with `IGNORE_SPACE`
- **Strict dependency shape**: `COUNT(*)` has arity=0, no column operands; strict mode requires `read_table` for the base table only
- **Excluded modifiers**: `DISTINCT`, `FILTER`, aggregate-local `ORDER BY`, nested operands, literals, parameters, casts
- **Disposition**: supported

### mysql-5.7 / count (aggregate, arity=1, column)

- **Primary documentation**: [MySQL 5.7 COUNT(col) reference](https://dev.mysql.com/doc/refman/5.7/en/aggregate-functions.html#function_count) (retrieved 2026-07-18)
- **Docker image**: `mysql:5.7.44` (observed `5.7.44`)
- **Positive probe**: `SELECT COUNT(amount) FROM app.builtin_semantic_facts` returns `4`
- **Boundary probes**: same as `COUNT(*)` plus literal operand (`COUNT(1)`) rejected, nested operand (`COUNT(ABS(amount))`) rejected
- **Strict dependency shape**: `COUNT(col)` has arity=1, one column operand; strict mode requires `read_table` for the base table and `read_column` for the column
- **Excluded modifiers**: `DISTINCT`, `FILTER`, aggregate-local `ORDER BY`, nested operands, literals, parameters, casts
- **Disposition**: supported

### mysql-5.7 / sum (aggregate, arity=1, column)

- **Primary documentation**: [MySQL 5.7 SUM() reference](https://dev.mysql.com/doc/refman/5.7/en/aggregate-functions.html#function_sum) (retrieved 2026-07-18)
- **Docker image**: `mysql:5.7.44` (observed `5.7.44`)
- **Positive probe**: `SELECT SUM(amount) FROM app.builtin_semantic_facts` returns `250`
- **Boundary probes**: same as `COUNT(col)`
- **Strict dependency shape**: same as `COUNT(col)`
- **Excluded modifiers**: same as `COUNT(col)`
- **Disposition**: supported

### mysql-5.7 / avg (aggregate, arity=1, column)

- **Primary documentation**: [MySQL 5.7 AVG() reference](https://dev.mysql.com/doc/refman/5.7/en/aggregate-functions.html#function_avg) (retrieved 2026-07-18)
- **Docker image**: `mysql:5.7.44` (observed `5.7.44`)
- **Positive probe**: `SELECT AVG(amount) FROM app.builtin_semantic_facts` returns `62.5`
- **Boundary probes**: same as `COUNT(col)`
- **Strict dependency shape**: same as `COUNT(col)`
- **Excluded modifiers**: same as `COUNT(col)`
- **Disposition**: supported

### mysql-5.7 / min (aggregate, arity=1, column)

- **Primary documentation**: [MySQL 5.7 MIN() reference](https://dev.mysql.com/doc/refman/5.7/en/aggregate-functions.html#function_min) (retrieved 2026-07-18)
- **Docker image**: `mysql:5.7.44` (observed `5.7.44`)
- **Positive probe**: `SELECT MIN(amount) FROM app.builtin_semantic_facts` returns `20`
- **Boundary probes**: same as `COUNT(col)`
- **Strict dependency shape**: same as `COUNT(col)`
- **Excluded modifiers**: same as `COUNT(col)`
- **Disposition**: supported

### mysql-5.7 / max (aggregate, arity=1, column)

- **Primary documentation**: [MySQL 5.7 MAX() reference](https://dev.mysql.com/doc/refman/5.7/en/aggregate-functions.html#function_max) (retrieved 2026-07-18)
- **Docker image**: `mysql:5.7.44` (observed `5.7.44`)
- **Positive probe**: `SELECT MAX(amount) FROM app.builtin_semantic_facts` returns `100`
- **Boundary probes**: same as `COUNT(col)`
- **Strict dependency shape**: same as `COUNT(col)`
- **Excluded modifiers**: same as `COUNT(col)`
- **Disposition**: supported

### mysql-5.7 / ranking windows — DEFERRED

- **Reason**: MySQL 5.7 has no native ranking-window support. The
  `ROW_NUMBER()`, `RANK()`, and `DENSE_RANK()` functions are not
  available in MySQL 5.7. Live probes confirm the ranking-window
  evidence is deferred for this profile.
- **Disposition**: deferred

### mysql-8.0 / aggregates (count, sum, avg, min, max)

- **Primary documentation**: [MySQL 8.0 aggregate functions](https://dev.mysql.com/doc/refman/8.0/en/aggregate-functions.html) (retrieved 2026-07-18)
- **Docker image**: `mysql:8.0.46` (observed `8.0.46`)
- **Positive probes**: identical to mysql-5.7 aggregates, executed independently against the 8.0 image
- **Boundary probes**: identical to mysql-5.7, executed independently
- **Strict dependency shape**: identical to mysql-5.7
- **Excluded modifiers**: identical to mysql-5.7
- **Disposition**: supported (independently evidenced from mysql-5.7)

### mysql-8.0 / row_number (window, arity=0)

- **Primary documentation**: [MySQL 8.0 ROW_NUMBER() reference](https://dev.mysql.com/doc/refman/8.0/en/window-functions.html#function_row-number) (retrieved 2026-07-18)
- **Docker image**: `mysql:8.0.46` (observed `8.0.46`)
- **Positive probe**: `SELECT id, ROW_NUMBER() OVER (PARTITION BY dept ORDER BY amount DESC, id) FROM app.builtin_semantic_facts ORDER BY id` returns `[(1,1),(2,2),(3,1),(4,2)]`
- **Boundary probes**: explicit frame rejected; named window rejected; nested partition operand rejected; nested order operand rejected; literal partition rejected; missing order rejected; missing partition rejected
- **Strict dependency shape**: `ROW_NUMBER()` has arity=0; window requires `PARTITION BY` and `ORDER BY` with direct column operands; strict mode requires `read_table` for the base table and `read_column` for every partition and order column
- **Excluded modifiers**: frames, named windows, `FILTER`, `DISTINCT`, aggregate-local `ORDER BY`, nested operands, literals, parameters, casts
- **Disposition**: supported

### mysql-8.0 / rank (window, arity=0)

- **Primary documentation**: [MySQL 8.0 RANK() reference](https://dev.mysql.com/doc/refman/8.0/en/window-functions.html#function_rank) (retrieved 2026-07-18)
- **Docker image**: `mysql:8.0.46` (observed `8.0.46`)
- **Positive probe**: `SELECT id, RANK() OVER (PARTITION BY dept ORDER BY amount DESC, id) FROM app.builtin_semantic_facts ORDER BY id` returns `[(1,1),(2,2),(3,1),(4,2)]`
- **Boundary probes**: same as `ROW_NUMBER()`
- **Strict dependency shape**: same as `ROW_NUMBER()`
- **Excluded modifiers**: same as `ROW_NUMBER()`
- **Disposition**: supported

### mysql-8.0 / dense_rank (window, arity=0)

- **Primary documentation**: [MySQL 8.0 DENSE_RANK() reference](https://dev.mysql.com/doc/refman/8.0/en/window-functions.html#function_dense-rank) (retrieved 2026-07-18)
- **Docker image**: `mysql:8.0.46` (observed `8.0.46`)
- **Positive probe**: `SELECT id, DENSE_RANK() OVER (PARTITION BY dept ORDER BY amount DESC, id) FROM app.builtin_semantic_facts ORDER BY id` returns `[(1,1),(2,2),(3,1),(4,2)]`
- **Boundary probes**: same as `ROW_NUMBER()`
- **Strict dependency shape**: same as `ROW_NUMBER()`
- **Excluded modifiers**: same as `ROW_NUMBER()`
- **Disposition**: supported

### mysql-8.4 / aggregates (count, sum, avg, min, max)

- **Primary documentation**: [MySQL 8.4 aggregate functions](https://dev.mysql.com/doc/refman/8.4/en/aggregate-functions.html) (retrieved 2026-07-18)
- **Docker image**: `mysql:8.4.10` (observed `8.4.10`)
- **Positive probes**: identical to mysql-5.7 aggregates, executed independently against the 8.4 image
- **Boundary probes**: identical to mysql-5.7, executed independently
- **Strict dependency shape**: identical to mysql-5.7
- **Excluded modifiers**: identical to mysql-5.7
- **Disposition**: supported (independently evidenced from mysql-5.7 and mysql-8.0)

### mysql-8.4 / row_number, rank, dense_rank (window, arity=0)

- **Primary documentation**: [MySQL 8.4 window functions](https://dev.mysql.com/doc/refman/8.4/en/window-functions.html) (retrieved 2026-07-18)
- **Docker image**: `mysql:8.4.10` (observed `8.4.10`)
- **Positive probes**: identical to mysql-8.0 ranking windows, executed independently against the 8.4 image
- **Boundary probes**: identical to mysql-8.0, executed independently
- **Strict dependency shape**: identical to mysql-8.0
- **Excluded modifiers**: identical to mysql-8.0
- **Disposition**: supported (independently evidenced from mysql-8.0)

### tidb-8.5 / aggregates (count, sum, avg, min, max)

- **Primary documentation**: [TiDB aggregate functions](https://docs.pingcap.com/tidb/stable/aggregate-functions/) (retrieved 2026-07-18)
- **Docker image**: `pingcap/tidb:v8.5.7` (observed `8.0.11-TiDB-v8.5.7`)
- **Positive probes**: identical to mysql-5.7 aggregates, executed independently against the TiDB image
- **Boundary probes**: stored-function creation rejected (TiDB rejects stored functions); UDF creation rejected; `app.COUNT(*)` rejected; `` `COUNT`(*) `` rejected; `COUNT (id)` rejected without `IGNORE_SPACE`; `COUNT (id)` accepted with `IGNORE_SPACE`; `COUNT/**/(*)` rejected with `IGNORE_SPACE`
- **Strict dependency shape**: identical to mysql-5.7
- **Excluded modifiers**: identical to mysql-5.7
- **Disposition**: supported (independently evidenced; not copied from MySQL)

### tidb-8.5 / row_number, rank, dense_rank (window, arity=0)

- **Primary documentation**: [TiDB window functions](https://docs.pingcap.com/tidb/stable/window-functions/) (retrieved 2026-07-18)
- **Docker image**: `pingcap/tidb:v8.5.7` (observed `8.0.11-TiDB-v8.5.7`)
- **Positive probes**: identical to mysql-8.0 ranking windows, executed independently against the TiDB image
- **Boundary probes**: identical to mysql-8.0, executed independently
- **Strict dependency shape**: identical to mysql-8.0
- **Excluded modifiers**: identical to mysql-8.0
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
