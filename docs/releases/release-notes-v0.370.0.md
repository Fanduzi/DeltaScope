# DeltaScope v0.370.0 Release Notes

## Summary — TiDB RETURNING Dialect Boundary

v0.370.0 upgrades the TiDB parser and draws a clean dialect boundary for DML `RETURNING`. TiDB dialect now accepts parser-recognized `RETURNING` (`INSERT ... RETURNING`, `UPDATE ... RETURNING`, and single-table `DELETE ... RETURNING`) as valid TiDB syntax. MySQL Server does not support DML `RETURNING`, so on the MySQL dialect a parsed `RETURNING` clause emits a dedicated global notice instead of a successful silent audit.

`RETURNING` is no longer treated as a PostgreSQL-only syntax token. Before this release it reached the PostgreSQL mismatch hint; after the parser bump it parses successfully, so the PostgreSQL notice would have been wrong for valid TiDB SQL. `RETURNING` is removed from the PostgreSQL mismatch token list, and `ON CONFLICT`, `::`, `ALTER COLUMN TYPE USING`, and `GENERATED AS IDENTITY` keep their existing PostgreSQL mismatch behavior.

This is a dialect-contract correction, not a formatting task. There is no `severity` field; DeltaScope uses `level`. The new finding carries only bounded metadata (selected dialect, suggested dialect, token name). It does not carry raw SQL, returned column names, expressions, parser fragments, connection details, or credentials.

## What Changed

- TiDB parser bumped so DML `RETURNING` parses on the MySQL/TiDB parser path for `INSERT`, `UPDATE`, and single-table `DELETE`.
- `spec.DML` gains an additive boolean JSON field `has_returning`, set only when the parsed statement has a real `RETURNING` clause. The field is parser-derived; DeltaScope does not project returned column names, expressions, aliases, or parser subtrees. An identifier or alias named `returning` does not set it.
- `RETURNING` removed from the PostgreSQL mismatch token heuristic. `ON CONFLICT`, `::`, `ALTER COLUMN TYPE USING`, and `GENERATED AS IDENTITY` keep their existing PostgreSQL mismatch notices.
- New non-configurable global finding `dialect.mysql.returning.unsupported.notice` (level `notice`), emitted from the success path only when the selected dialect is `mysql` and the parsed DML has a `RETURNING` clause. It does not fire for TiDB (TiDB supports `RETURNING`) or PostgreSQL. The message is MySQL-Server-scoped and suggests re-running with `--dialect tidb` when the SQL targets TiDB. It is not inferred from raw SQL.
- SDK, CLI, HTTP, and MCP surface the behavior consistently through the shared audit result. No surface adds a bespoke RETURNING interface.

## What Stayed the Same

- `level` remains the public priority field. No `severity` field is introduced.
- The rule catalog is unchanged (371 rules). This release moves parser dialect behavior and one non-configurable global finding; it is not a registered-rule change.
- PostgreSQL mismatch behavior for `ON CONFLICT`, `::`, `ALTER COLUMN TYPE USING`, and `GENERATED AS IDENTITY` is unchanged.
- `REPLACE ... RETURNING` is not supported this release and keeps its current parser-error/unsupported path. The target parser does not attach `RETURNING` to `REPLACE`.
- DeltaScope does not auto-switch dialect.
- Findings and diagnostics carry no raw SQL, parser fragments, returned column names, expressions, or connection details.

## Non-Goals

- Not a MariaDB dialect. DeltaScope does not claim the MySQL dialect supports MariaDB `RETURNING`.
- Not `ReturningColumns`, `ReturningExpressions`, or parser-subtree projection.
- Not support for `REPLACE ... RETURNING` or the parser's unsupported multi-table `DELETE ... RETURNING` variants.
- Not a configurable policy rule for allowing or forbidding `RETURNING`.
- Not a parser fallback or inference from raw parse-error SQL.
- Not new SDK, HTTP, or MCP bespoke interfaces; surface parity comes from the shared audit result.
- No `severity` field is introduced; `level` remains the public priority field.

## Rule Catalog Facts

The rule catalog is unchanged from v0.360.0. This release corrects parser dialect behavior; it is not a registered-rule change.

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

- SQL corpus: **582/582**, **100.0%**, **247** YAML fixture files (up from 245; two new `RETURNING` dialect-boundary corpus cases).
- PostgreSQL ALTER TABLE config entries: **53** (unchanged).
- DDL coverage catalog: **400** entries (unchanged).

## Decision Record

`docs/decisions/2026-06-23-v0.370.0-tidb-returning-dialect-boundary.md`
