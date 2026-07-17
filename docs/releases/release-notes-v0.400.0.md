# DeltaScope v0.400.0 Release Notes

## Summary - Common Pure-Effect Query Access

v0.400.0 expands the existing **opt-in** Trusted PostgreSQL Query Access SDK. On a caller-owned **PG17** `*sql.Conn`, a query may return `read_only` + `admissible` only after same-connection metadata, type, catalog-identity, and audited manifest proof.

The new proven subset covers `COUNT(*)`, `COUNT(base_column)`, typed `SUM` / `AVG` / `MIN` / `MAX` over a direct base column, and `ROW_NUMBER` / `RANK` / `DENSE_RANK` with direct-column window partition and ordering dependencies. This is still not broad PostgreSQL common SELECT support or authorization.

Default SDK, CLI, and HTTP remain fail-closed for effect-bearing PostgreSQL queries. MySQL and TiDB remain fail-closed for aggregate/window effects because no dialect-specific identity proof model is shipped. MCP still has no Query Access tool.

## What Changed

- PG17 trusted SDK manifest now proves the bounded aggregate and ranking-window subset after catalog identity validation on the caller-owned connection.
- Strict requirement extraction now includes aggregate arguments, window partition/order dependencies, `DISTINCT ON`, aggregate `FILTER`, and LIMIT/OFFSET subqueries before admission is evaluated.
- Explicit window frames are detected and remain excluded. `FILTER`, DISTINCT, named windows, ordered-set aggregates, nested expressions, casts, views, wildcards, parameters, unqualified relations, unresolved metadata, and non-manifest identities remain `indeterminate`.
- MySQL/TiDB dependency extraction closes derived-subquery and ordering gaps without making function-bearing queries admissible. Their aggregate/window results remain `unknown_function_effect` / `indeterminate`.
- Decision record: `docs/decisions/2026-07-16-query-access-common-pure-effects.md` (Accepted; Related milestone/version: v0.400.0).

## What Stayed the Same

- The public trusted entry points remain `NewPostgreSQLQueryAccessSessionFromConn` and `AnalyzePostgreSQLQueryAccessWithSession`; DeltaScope does not close the caller-owned connection.
- Default `AnalyzeQueryAccess`, CLI `query-access analyze`, and HTTP `POST /v1/query-access/analyze` do not open trusted metadata connections and remain fail-closed for these PostgreSQL effect cases.
- Query Access emits static requirements only. It does not authenticate callers, evaluate grants, enforce RLS, mask columns, auto-grant privileges, rewrite SQL, or guarantee a later execution snapshot.
- MCP tools remain `audit_sql`, `describe_rule`, `list_rules`, and `get_capabilities` only.
- The audit rule catalog and default audit behavior are unchanged. `level` remains the public audit priority field; no severity field is introduced.
- Query Access results do not include raw SQL, literals, credentials, connection strings, OIDs, catalog internals, session data, or parser fragments.

## Non-Goals

- Not full PostgreSQL common SELECT or arbitrary function support.
- Not a MySQL/TiDB identity-manifest or trusted promotion change.
- Not CLI/HTTP trusted-session promotion or an MCP Query Access tool.
- Not runtime authorization, grant evaluation, RLS, masking, automatic grant, SQL rewrite, or execution-affinity enforcement.
- No severity field is added, and the registered audit rule catalog is unchanged.

## Rule Catalog Facts

The registered audit rule catalog is unchanged from v0.390.0. This release changes only the bounded Query Access proof path.

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

- `docs/decisions/2026-07-16-query-access-common-pure-effects.md` (this release)
- Trusted PostgreSQL SDK (v0.390.0): `docs/decisions/2026-07-12-query-access-pure-read-admissibility.md`
- Query Access foundation (v0.380.0): `docs/decisions/2026-07-11-query-access-analysis-foundation.md`
