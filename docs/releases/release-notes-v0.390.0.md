# DeltaScope v0.390.0 Release Notes

## Summary — Trusted PostgreSQL Query Access SDK

v0.390.0 ships an **opt-in** Trusted PostgreSQL Query Access SDK path for callers that already own a live `*sql.Conn`. On a **PG17** connection, and only for shapes covered by a closed, audited effect-identity manifest, the path may return `read_only` + `admissible` after same-connection metadata, type, and identity proof.

This is **not** broad PostgreSQL common SELECT support. Default SDK, CLI, and HTTP remain fail-closed for effect-bearing PostgreSQL queries. Query Access still does not authenticate callers, evaluate grants, enforce RLS, mask columns, rewrite SQL, or guarantee a later execution snapshot.

There is no `severity` field. Results omit raw SQL, literals, credentials, connection strings, driver/catalog internals, and parser fragments. MCP tools remain `audit_sql`, `describe_rule`, `list_rules`, and `get_capabilities` only — still no query-access MCP tool.

## What Changed

- Public SDK (postgresql build tag; non-postgresql builds expose the same symbols and return `ErrPostgreSQLSessionNotAvailable`):
  - `NewPostgreSQLQueryAccessSessionFromConn`
  - `AnalyzePostgreSQLQueryAccessWithSession`
- Caller-owned `*sql.Conn` contract: the session does not close the connection; metadata and identity proof bind the same connection (same-connection session contract).
- PG17 **manifest-gated** pure-read admissibility for a narrow proven subset.
- Public E2E-verified shapes on the trusted path include:
  - `count(*)` over a schema-qualified base relation
  - schema-qualified base-column comparison / JOIN
- Default `AnalyzeQueryAccess`, CLI `query-access analyze`, and HTTP `POST /v1/query-access/analyze` remain fail-closed for these effect-bearing PostgreSQL cases when no trusted session is used.
- HTTP Query Access error responses are bounded and do not echo raw wrapped internal errors.
- Decision record: `docs/decisions/2026-07-12-query-access-pure-read-admissibility.md` (Accepted; Related milestone/version: v0.390.0).

## What Stayed the Same

- Existing audit behavior, default policy, and registered rule catalog are unchanged.
- MySQL and TiDB Query Access behavior is unchanged.
- MCP tools are unchanged (still four tools; no query-access tool).
- `level` remains the public priority field for audit findings. No `severity` field is introduced.
- Query access results have no `severity` field and are not audit findings.
- Privacy / no-leak: no raw SQL, literals, credentials, connection strings, or parser fragments in the structured result contract.
- Foundation surfaces from v0.380.0 remain available: SDK `AnalyzeQueryAccess`, CLI `query-access analyze`, HTTP `POST /v1/query-access/analyze`.

## Non-Goals

- Not full PostgreSQL common SELECT admissibility on the default path.
- Not runtime grant evaluation, caller authentication, or database session authorization.
- Not row-level security evaluation, column masking, automatic grant, or SQL rewrite.
- Not a guarantee that a later execution uses the same snapshot as the analysis connection.
- Not CLI/HTTP trusted-session promotion.
- Not an MCP query-access tool.
- Not a MySQL/TiDB trusted identity promotion change.
- Not a change to existing audit behavior or the registered rule catalog.
- No `severity` field is introduced.

## Rule Catalog Facts

The registered audit rule catalog is unchanged from v0.380.0. This release adds an opt-in trusted PostgreSQL SDK path; it is not a registered-rule change.

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

- SQL corpus: **582/582**, **100.0%**, **247** YAML fixture files (unchanged).
- PostgreSQL ALTER TABLE config entries: **53** (unchanged).
- DDL coverage catalog: **400** entries (unchanged; mysql 61, tidb 54, postgresql 285, parser_upgrade_candidate 18).

## Decision Records

- `docs/decisions/2026-07-12-query-access-pure-read-admissibility.md` (this release)
- Prior foundation (v0.380.0): `docs/decisions/2026-07-11-query-access-analysis-foundation.md`
- Prior lineage (v0.380.0): `docs/decisions/2026-07-11-cte-derived-table-lineage-resolution.md`
