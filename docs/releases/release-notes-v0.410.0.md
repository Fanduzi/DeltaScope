# DeltaScope v0.410.0 Release Notes

## Summary - MySQL/TiDB Builtin Semantic Manifests

v0.410.0 introduces the **opt-in** MySQL/TiDB builtin semantic manifest proof model for Query Access. On a caller-owned MySQL or TiDB `*sql.Conn`, a profiled query may return `read_only` + `admissible` only after same-connection metadata resolution, parser-native-form proof, complete candidate closure, and immutable manifest lookup.

The proven subset covers `COUNT(*)`, direct-column `COUNT` / `SUM` / `AVG` / `MIN` / `MAX` for all four profiles, and `ROW_NUMBER` / `RANK` / `DENSE_RANK` with direct-column partition and order dependencies for MySQL 8.0, MySQL 8.4, and TiDB 8.5. MySQL 5.7 has no native ranking-window support; its ranking-window entries remain deferred. This is still not broad MySQL/TiDB common SELECT support or authorization.

Default SDK, CLI, and HTTP remain fail-closed for function-bearing MySQL/TiDB queries. The explicit same-connection SDK session (`AnalyzeMySQLTiDBQueryAccessWithSession`) is the only promotion path. MCP still has no Query Access tool. PostgreSQL behavior is unchanged.

## What Changed

- New public SDK session boundary: `NewMySQLTiDBQueryAccessSessionFromConn` and `AnalyzeMySQLTiDBQueryAccessWithSession` accept a caller-owned `*sql.Conn`, reject external schema resolvers, and construct the private semantic capability internally.
- New closed public profile enum: `QueryAccessAnalysisProfileMySQL57` / `MySQL80` / `MySQL84` / `TiDB85`. Unknown values and dialect mismatches return bounded validation errors.
- Immutable production builtin semantic registry populated for four profiles, each backed by primary documentation and live Docker probes against the exact server image (`mysql:5.7.44`, `mysql:8.0.46`, `mysql:8.4.10`, `pingcap/tidb:v8.5.7`).
- Parser effect-candidate collector traverses projection, WHERE, HAVING, GROUP BY, ORDER BY, LIMIT/OFFSET, join conditions, derived tables, CTEs, scalar subqueries, set operations, aggregate modifiers, and window partition/order/frame expressions. Unsupported nodes emit explicit fail-closed markers.
- Gateway enforces canonical native form (rejects quoted, schema-qualified, noncanonical spacing, `IGNORE_SPACE`-dependent ambiguity), complete candidate closure (no duplicate/gap/foreign ordinals), strict physical requirements (ModeStrict, no unresolved, no views/CTEs/derived/wildcards, complete `read_table`/`read_column` requirements), and `RequireWindowPartition`/`RequireWindowOrder` for ranking windows.
- CLI `query-access analyze --profile` and HTTP `POST /v1/query-access/analyze` accept the profile input but remain offline and indeterminate without the explicit session API.
- Decision record: `docs/decisions/2026-07-18-query-access-mysql-tidb-builtin-semantic-manifests.md` (Accepted; Related milestone/version: v0.410.0).
- Evidence ledger: `docs/decisions/2026-07-18-query-access-mysql-tidb-builtin-semantic-manifests-evidence-ledger.md`.

## What Stayed the Same

- Default `AnalyzeQueryAccess`, CLI `query-access analyze`, and HTTP `POST /v1/query-access/analyze` do not open database connections and remain fail-closed for function-bearing MySQL/TiDB queries.
- Query Access emits static requirements only. It does not authenticate callers, evaluate grants, enforce RLS, mask columns, auto-grant privileges, rewrite SQL, or guarantee a later execution snapshot.
- MCP tools remain `audit_sql`, `describe_rule`, `list_rules`, and `get_capabilities` only.
- The audit rule catalog and default audit behavior are unchanged. `level` remains the public audit priority field; no severity field is introduced.
- Query Access results do not include raw SQL, literals, function names, manifest internals, profile internals, candidates, parser facts, DSNs, credentials, driver errors, or session data. No `severity` field is present.
- PostgreSQL's controlled-session catalog/OID proof path is unchanged. No MySQL/TiDB manifest can affect PostgreSQL and vice versa.

## Non-Goals

- Not a generic function-name allowlist, a volatility-only allowlist, or a caller-supplied manifest.
- Not support for arbitrary or user-defined functions, stored functions, plugin/loadable UDFs, or schema-qualified/quoted calls.
- Not support for MySQL/TiDB casts, operators, literals, parameters, nested expressions, `FILTER`, `DISTINCT`, aggregate-local `ORDER BY`, named windows, explicit frames, ordered-set behavior, or broad common `SELECT`.
- Not database connections or runtime identity lookup in default SDK/CLI/HTTP.
- Not an MCP Query Access tool.
- Not runtime authorization, grant evaluation, RLS, masking, automatic grant, SQL rewrite, or execution-snapshot proof.
- No severity field is added, and the registered audit rule catalog is unchanged.

## Support Matrix

| Profile | Dialect | Aggregates | Ranking Windows | Live Docker Image |
|---|---|---|---|---|
| `mysql-5.7` | mysql | `COUNT(*)`, `COUNT`/`SUM`/`AVG`/`MIN`/`MAX` (direct column) | deferred (5.7 has no native support) | `mysql:5.7.44` |
| `mysql-8.0` | mysql | same as 5.7 | `ROW_NUMBER`/`RANK`/`DENSE_RANK` (direct partition+order columns) | `mysql:8.0.46` |
| `mysql-8.4` | mysql | same as 5.7 | same as 8.0 | `mysql:8.4.10` |
| `tidb-8.5` | tidb | independently evidenced | independently evidenced | `pingcap/tidb:v8.5.7` |

Ranking-window entries require both `PARTITION BY` and `ORDER BY` clauses with direct physical base-table columns. This is deliberately stricter than MySQL's syntax contract (which accepts ranking windows without `ORDER BY`) to preserve the strict partition/order dependencies boundary.

## Rule Catalog Facts

The registered audit rule catalog is unchanged from v0.400.0. This release changes only the Query Access MySQL/TiDB semantic proof path.

| Metric | Count |
|--------|------:|
| Total rules | **371** |
| blocker | 72 |
| warning | 142 |
| notice | 157 |

| Dialect scope | Rules |
|---------------|------:|
| common | 177 |
| postgresql | 191 |
| mysql | 1 |
| tidb | 2 |

| Statement kind | Rules |
|----------------|------:|
| ddl | 361 |
| dml | 10 |

## Unchanged Metrics

- SQL corpus: **582/582**, **100.0%**, **247** YAML fixture files.
- PostgreSQL ALTER TABLE config entries: **53**.
- DDL coverage catalog: **400** entries (mysql 61, tidb 54, postgresql 285, parser_upgrade_candidate 18).

## Decision Records

- `docs/decisions/2026-07-18-query-access-mysql-tidb-builtin-semantic-manifests.md` (this release)
- Common pure-effect Query Access (v0.400.0): `docs/decisions/2026-07-16-query-access-common-pure-effects.md`
- Trusted PostgreSQL SDK (v0.390.0): `docs/decisions/2026-07-12-query-access-pure-read-admissibility.md`
- Query Access foundation (v0.380.0): `docs/decisions/2026-07-11-query-access-analysis-foundation.md`
