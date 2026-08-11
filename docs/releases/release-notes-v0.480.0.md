# DeltaScope v0.480.0 Release Notes

## Summary - PG17 `COUNT(1)` Online Query Access

v0.480.0 proves and surfaces one exact PostgreSQL 17 Query Access envelope: `SELECT COUNT(1) FROM <one schema-qualified physical base table>` on a caller-owned online session. The same catalog-bound proof is available through the trusted SDK, online CLI PostgreSQL connection options, and HTTP with an operator-authorized PostgreSQL 17 `connection_id` for the `query_access` purpose. A successful analysis returns `read_only` + `admissible` with exactly one `read_table` requirement for that base relation.

This is static requirement analysis only. DeltaScope does not execute the submitted SQL, retrieve query results, or decide authorization, grants, RLS, or masking. Default offline SDK/CLI/HTTP behavior, every other PostgreSQL literal or aggregate shape, MySQL/TiDB Query Access, the audit rule catalog, and MCP remain unchanged. MCP still has no Query Access tool.

## What Changed

### Exact PG17 `COUNT(1)` Envelope

The only newly admitted PostgreSQL statement shape is:

```sql
SELECT COUNT(1) FROM app.orders
```

where `app.orders` is one schema-qualified resolved physical base table. Requirements:

- Dialect and server identity: PostgreSQL 17 on a caller-owned online session.
- Aggregate identity: session-bound catalog proof of `pg_catalog.count(any)`.
- Argument: the uncast integer constant `1` only. The parser may record an internal non-serialized `integer_one` syntax fact; it does not retain or expose the literal text.
- Relation: exactly one schema-qualified physical base table. No joins, comma joins, CTEs, views, foreign tables, derived tables, or unqualified names. Foreign tables (`relkind = 'f'`) are deferred and fail closed on both DB-backed and caller-owned resolvers.
- Clauses: no `WHERE`, `GROUP BY`, `ORDER BY`, `LIMIT`, `DISTINCT`, `FILTER`, window, nested call, extra select-list items, set operations, or parameters/casts/expressions.

Success result: `read_only` + `admissible`, with the sole base relation's `read_table` requirement and no referenced columns.

### Online Surfaces

| Surface | Supported path | Unchanged / fail-closed |
|---------|----------------|-------------------------|
| Trusted SDK | Caller-owned online `*sql.Conn` PostgreSQL 17 session | Default offline SDK stays indeterminate for this query |
| CLI | Existing online PostgreSQL connection options | Default/offline CLI stays indeterminate |
| HTTP | Operator-configured, authorized `connection_id` for `query_access` | HTTP without `connection_id` stays indeterminate |
| MCP | — | No Query Access tool; unchanged |

CLI and HTTP reuse the same session-bound catalog proof. They do not add a transport-specific feature flag, parser path, or trust root. HTTP clients never supply endpoints, credentials, secret sources, TLS settings, profile, or server-version claims; `connection_id` selection stays operator controlled and authorized.

### No-Execution and No-Leak Guarantees

- Submitted SQL is never executed, prepared, or explained.
- Public results and logs do not expose SQL/literal markers, connection data, credentials, catalog data, or raw driver errors.
- Recording-driver and live transport evidence cover the positive path, excluded shapes (including foreign tables), connection failure, catalog lookup failure, and HTTP unauthorized/unknown `connection_id` cases.

### Fail-Closed Exclusions

These remain `indeterminate` (not admitted):

- `COUNT(NULL)`, `COUNT(2)`, `COUNT('1')`, parameters, casts, expressions, nested calls, and arbitrary arity
- Relationless `SELECT COUNT(1)`
- Unqualified `FROM orders`, views, foreign tables, derived tables, CTEs, joins, and multi-relation forms
- `FILTER`, windows, `DISTINCT`, ordering, grouping, limits, extra select-list items, and set operations
- Default/offline SDK, CLI, and HTTP
- MCP on every path
- Every other PostgreSQL literal, scalar, binary, or aggregate shape beyond the exact envelope above

MySQL/TiDB online literal-only and relationless shapes from earlier releases are unchanged and are not part of this PostgreSQL proof.

## What Stayed the Same

- SQL audit behavior, the registered audit rule catalog, and default audit output are unchanged. `level` remains the public audit priority field; no severity field is introduced.
- Default offline SDK, CLI, and HTTP stay fail-closed for function-bearing Query Access until an operator or local user intentionally establishes a supported online session.
- MCP tools remain `audit_sql`, `describe_rule`, `list_rules`, and `get_capabilities` only.
- Query Access still does not authenticate callers, evaluate grants, enforce RLS, mask columns, rewrite SQL, auto-grant privileges, or guarantee a later execution snapshot.
- Existing MySQL/TiDB Query Access contracts are unchanged.
- Existing release tags, GitHub Releases, npm packages, and Homebrew casks are untouched.

## Non-Goals

- Not general PostgreSQL literal support, pure-function admission, or aggregate admission.
- Not authorization, grants, roles, RLS, masking, rewrite, or execution-snapshot guarantees.
- Not SQL execution, prepare/explain of user SQL, or data-returning APIs.
- Not default/offline SDK, CLI, or HTTP expansion for this query.
- Not an MCP Query Access tool.
- Not relationless PostgreSQL `COUNT(1)`, other literals, casts, parameters, modifiers, joins, CTEs, views, foreign tables, derived tables, or unqualified sources.
- Not a reuse of the MySQL/TiDB profile/manifest model as PostgreSQL proof.
- Not a severity field; not a change to the registered audit rule catalog.
- Not a change to any previously published artifact or existing tag.

## Rule Catalog Facts

The registered audit rule catalog is unchanged from v0.470.0. This release changes PostgreSQL Query Access proof and online surface contract only.

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

- `docs/decisions/2026-08-03-query-access-pg17-count-online-surface-contract.md` (this release)
- `docs/decisions/2026-07-31-query-access-pg17-count-literal-proof.md` (this release)
- `docs/decisions/2026-07-30-release-recovery-provenance-enforcement.md` (v0.470.0)
- `docs/decisions/2026-07-28-query-access-relationless-literal-selects.md` (v0.460.0)
