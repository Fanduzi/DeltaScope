# DeltaScope v0.380.0 Release Notes

## Summary — Query Access Analysis Foundation

v0.380.0 ships Query Access Analysis as a separate public capability next to the existing audit path. It inspects SQL and reports read classification, admission eligibility, permission-bearing relations and columns, output lineage, and structured requirements. It does not authenticate callers, evaluate grants, connect to a database for authorization, rewrite SQL, or act as a policy engine.

The public admission values are `admissible`, `rejected`, and `indeterminate`. Authorization layers should treat `indeterminate` as fail-closed (deny by default). There is no `severity` field; query access is not an audit finding. Results omit raw SQL, literals, credentials, connection strings, and parser fragments.

SDK, CLI, and HTTP expose the capability. The MCP tool set remains `audit_sql`, `describe_rule`, `list_rules`, and `get_capabilities` only — query-access is not an MCP tool in this release.

## What Changed

- New Query Access Analysis use case with read classification `read_only`, `not_read_only`, and `indeterminate`.
- Admission derived from classification: `read_only` → `admissible`, `not_read_only` → `rejected`, otherwise `indeterminate`.
- Modes:
  - `strict` (default): every resolved source column used in projection, filter, join, grouping, having, ordering, or window becomes a `read_column` requirement.
  - `projection_only`: only output (projection) source columns require `read_column`. Non-projected columns can still support inference; the result emits a `projection_only_inference_risk` warning when that trade-off applies.
- Permission objects are base tables and views only. CTEs and derived tables do not require permission directly; lineage must resolve to physical sources.
- Fail-closed when metadata is incomplete, references are ambiguous, wildcards cannot fully expand, or function/operator effects are unknown.
- PostgreSQL keeps a conservative boundary: uncertain expressions (for example operators, function calls, and casts with unknown side effects) stay non-admissible (`indeterminate` classification / admission).
- Public surfaces:
  - SDK: `deltascope.AnalyzeQueryAccess`
  - CLI: `deltascope query-access analyze`
  - HTTP: `POST /v1/query-access/analyze`
  - MCP: no query-access tool (explicitly deferred)
- Query-access corpus: **44** cases (**22** MySQL/TiDB path + **22** PostgreSQL), **88** fixture files; `make query-access-corpus-gates` is green.
- Decision records: `docs/decisions/2026-07-11-query-access-analysis-foundation.md`, `docs/decisions/2026-07-11-cte-derived-table-lineage-resolution.md`.

## What Stayed the Same

- Existing audit behavior, default policy, and registered rule catalog are unchanged.
- MCP tools are unchanged (still four tools; no query-access tool).
- `level` remains the public priority field for audit findings. No `severity` field is introduced.
- Query access results do not use `severity` (no `severity` field) and are not audit findings.
- Privacy / no-leak: no raw SQL, literals, credentials, connection strings, or parser fragments in the structured result contract.

## Non-Goals

- Not runtime grant evaluation, caller authentication, or database session authorization.
- Not a connection to the database for the purpose of authorizing the caller.
- Not a policy engine, automatic grant, or SQL rewrite service.
- Not row-level security evaluation or column masking.
- Not an MCP query-access tool.
- Not a claim of full SQL grammar coverage or dialect parity for every expression shape.
- No `severity` field is introduced.

## Rule Catalog Facts

The registered audit rule catalog is unchanged from v0.370.0. This release adds a separate query-access capability; it is not a registered-rule change.

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

- `docs/decisions/2026-07-11-query-access-analysis-foundation.md`
- `docs/decisions/2026-07-11-cte-derived-table-lineage-resolution.md`
