# DeltaScope v0.420.0 Release Notes

## Summary - Online Query Access Connection Registry

v0.420.0 replaces the inline HTTP `connection` object with operator-managed named connections referenced by `connection_id`. HTTP online audit and Query Access now resolve database targets from a server-side connection registry rather than accepting host/port/user/password fields on each request. TLS mode, API key allowlists, and capability derivation all attach to the named connection entry.

A bounded scalar function subset (LOWER, UPPER, LENGTH, CHAR_LENGTH, ABS, CEIL, FLOOR, COALESCE, NULLIF, IFNULL) is promoted for online Query Access when the connected server identity matches a supported profile (MySQL 5.7, MySQL 8.0, MySQL 8.4, TiDB 8.5, PostgreSQL 17). The subset is intentionally narrow. COALESCE and NULLIF accept direct-column operands only.

Default offline SDK, CLI, and HTTP remain unchanged. CLI retains direct connection flags and adds `--database` for PostgreSQL target selection. MCP still has no Query Access tool.

## What Changed

- HTTP online audit and Query Access resolve targets from operator-managed named connections via `connection_id`. The inline HTTP `connection` object (host/port/user/password_env/schema) is removed with no compatibility switch.
- TLS mode is declared per connection: `disabled` (default) or `enabled` with CA chain path and hostname verification. No TLS configuration is accepted on the HTTP request body.
- Bounded scalar function subset promoted for online Query Access: LOWER, UPPER, LENGTH, CHAR_LENGTH, ABS, CEIL, FLOOR, COALESCE, NULLIF, IFNULL. COALESCE and NULLIF accept direct-column operands only. IFNULL is MySQL/TiDB-only (N/A on PostgreSQL).
- Online sessions derive capability from connected-server identity: MySQL 5.7, MySQL 8.0, MySQL 8.4, TiDB 8.5, PostgreSQL 17. No caller-supplied manifest or profile override is accepted.
- HTTP authentication uses API key allowlists per connection entry. No per-request credentials are accepted.
- CLI retains direct connection flags (`--host`, `--port`, `--user`, `--ask-password`, `--schema`). CLI `--database` flag added for PostgreSQL target selection.
- Decision record: `docs/decisions/2026-07-20-query-access-online-connection-registry.md` (Accepted; Related milestone/version: v0.420.0).

## What Stayed the Same

- Default `AnalyzeQueryAccess`, CLI `query-access analyze`, and HTTP `POST /v1/query-access/analyze` without a `connection_id` remain fail-closed for function-bearing queries. No default SDK/CLI/HTTP path automatically promotes all function queries.
- Query Access emits static requirements only. It does not authenticate callers, evaluate grants, enforce RLS, mask columns, auto-grant privileges, rewrite SQL, or guarantee a later execution snapshot.
- MCP tools remain `audit_sql`, `describe_rule`, `list_rules`, and `get_capabilities` only. No Query Access tool is added.
- The audit rule catalog and default audit behavior are unchanged. `level` remains the public audit priority field; no severity field is introduced.
- Query Access results do not include raw SQL, literals, function names, DSNs, credentials, driver errors, session data, endpoint addresses, or secrets. No `severity` field is present.
- PostgreSQL's controlled-session catalog/OID proof path is unchanged. No MySQL/TiDB connection registry entry can affect PostgreSQL and vice versa.

## Non-Goals

- Not SQL execution or data-returning APIs.
- Not arbitrary host/password HTTP requests. Request bodies cannot submit database credentials, host, DSN, or CA path.
- Not UDF/stored functions, casts, literals, nested expressions, or broad function-name allowlists.
- Not grants, RLS, masking, rewrite, or execution-snapshot guarantees.
- Not MySQL/TiDB profile as SQL mode attestation.
- Not an MCP Query Access tool.
- No severity field is added, and the registered audit rule catalog is unchanged.

## Support Matrix

| Function | MySQL 5.7 | MySQL 8.0 | MySQL 8.4 | TiDB 8.5 | PostgreSQL 17 |
|---|---|---|---|---|---|
| LOWER, UPPER, LENGTH, CHAR_LENGTH | ✓ | ✓ | ✓ | ✓ | ✓ |
| ABS, CEIL, FLOOR | ✓ | ✓ | ✓ | ✓ | ✓ |
| COALESCE, NULLIF | ✓ (direct col only) | ✓ (direct col only) | ✓ (direct col only) | ✓ (direct col only) | ✓ (direct col only) |
| IFNULL | ✓ | ✓ | ✓ | ✓ | N/A |

COALESCE and NULLIF accept direct physical base-table column operands only. Nested expressions, literals, and function calls inside these operators are not supported. IFNULL is a MySQL/TiDB builtin with no PostgreSQL equivalent.

## Rule Catalog Facts

The registered audit rule catalog is unchanged from v0.410.0. This release changes only the Query Access connection model and online scalar function subset.

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

- `docs/decisions/2026-07-20-query-access-online-connection-registry.md` (this release)
- MySQL/TiDB builtin semantic manifests (v0.410.0): `docs/decisions/2026-07-18-query-access-mysql-tidb-builtin-semantic-manifests.md`
- Common pure-effect Query Access (v0.400.0): `docs/decisions/2026-07-16-query-access-common-pure-effects.md`
- Trusted PostgreSQL SDK (v0.390.0): `docs/decisions/2026-07-12-query-access-pure-read-admissibility.md`
- Query Access foundation (v0.380.0): `docs/decisions/2026-07-11-query-access-analysis-foundation.md`
