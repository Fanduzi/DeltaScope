# DeltaScope v0.460.0 Release Notes

## Summary - Relationless Literal-Only Query Access for Online MySQL/TiDB

v0.460.0 removes the FROM clause requirement for exact literal-only manifest entries on online MySQL/TiDB sessions. On a caller-owned session (SDK), an online CLI connection, or an HTTP registered `connection_id`, queries such as `SELECT LOWER('x')`, `SELECT COUNT(1)`, or `SELECT COALESCE('a','b')` now return `read_only` + `admissible` with empty requirements — no tables, no relations, no referenced columns. The supported profiles are MySQL 5.7, 8.0, 8.4, and TiDB 8.5.

This release does not change default offline SDK, CLI, or HTTP behavior, and MCP still has no Query Access tool. Query Access does not execute user SQL.

## What Changed

### Relationless Literal-Only Shapes

The same exact literal-only shapes admitted by v0.450.0 (with a FROM clause) now also succeed without one. On the online path (caller-owned `*sql.Conn` via SDK, online CLI connection, or HTTP `connection_id`), these queries return `read_only` + `admissible` with empty requirements:

**Unary literal-only (`[const]` operand):**

| Function | Example |
|----------|---------|
| `LOWER` | `SELECT LOWER('x')` |
| `UPPER` | `SELECT UPPER('x')` |
| `LENGTH` | `SELECT LENGTH('x')` |
| `CHAR_LENGTH` | `SELECT CHAR_LENGTH('x')` |
| `ABS` | `SELECT ABS(42)` |
| `CEIL` | `SELECT CEIL(42)` |
| `CEILING` | `SELECT CEILING(42)` |
| `FLOOR` | `SELECT FLOOR(42)` |

**Aggregate literal (`[const]` operand):**

| Function | Example |
|----------|---------|
| `COUNT(1)` | `SELECT COUNT(1)` |

**All-constant binary (`[const, const]` operands):**

| Functions | Example |
|-----------|---------|
| `COALESCE`, `NULLIF`, `IFNULL` | `SELECT COALESCE('a', 'b')` |

Each shape is admitted across all four profiles (MySQL 5.7, 8.0, 8.4, TiDB 8.5). The relationless path adds no table or column requirements to the result.

### Requirement Model

Relationless literal-only queries produce an empty requirements list:

- No resolved physical base relation is present, so no `read_table` entry is emitted.
- No direct physical column is referenced, so no `read_column` entry is emitted.
- Each literal operand contributes no requirement.
- The result is `read_only` + `admissible` with empty requirements, empty relations, and empty referenced columns.

Empty/omitted requirements means static analysis found no database object reads. This is not authorization, grants, RLS, masking, SQL mode, or execution permission.

### Previously Supported (unchanged from v0.450.0)

v0.450.0 admitted literal-only, reversed, and all-constant operand shapes on the online MySQL/TiDB path when a FROM clause is present. That behavior is unchanged. v0.460.0 extends the same literal-only shapes to work without a FROM clause.

## What Stayed the Same

- Default offline SDK, CLI, and HTTP behavior is unchanged. Without an online session, function-bearing queries remain `indeterminate`.
- CLI `--tls-mode` defaults to `disabled`. Enabling TLS requires explicit `--tls-mode=enabled`.
- Query Access emits structured requirements only. It does not authenticate callers, evaluate grants, enforce RLS, mask columns, auto-grant privileges, rewrite SQL, or guarantee a later execution snapshot.
- MCP tools remain `audit_sql`, `describe_rule`, `list_rules`, and `get_capabilities` only. No Query Access tool is added.
- The audit rule catalog and default audit behavior are unchanged. `level` remains the public audit priority field; no severity field is introduced.
- Query Access results do not include raw SQL, literals, function names, DSNs, credentials, driver errors, session data, endpoint addresses, or secrets.
- Query Access does not execute user SQL, does not return data.

## Non-Goals

- Not a general pure-function or SELECT admission. Only the exact shapes listed above are admitted without a FROM clause; all other literal-only, nested, or multi-operand forms remain `indeterminate`.
- Not `SELECT 1` (candidate-free). Candidate-free queries stay at their existing behavior and are not part of this release.
- Not relation-bearing or column-bearing queries. Those still require physical requirements proof (unchanged).
- Not 3+ operand `COALESCE`/`NULLIF`/`IFNULL`. Only exactly two literal operands are admitted.
- Not default/offline SDK, CLI, HTTP, PostgreSQL, or MCP. Those paths remain unchanged and fail-closed.
- Not parameters, casts, operators, nested functions, subqueries, UDFs, quoted/qualified/noncanonical calls, unknown functions, or unsupported modifiers.
- Not a JSON field, authorization flag, or permission-free flag.
- Not SQL execution or data-returning APIs.
- Not database authorization, grants, roles, RLS, masking, rewrite, or execution-snapshot guarantees.
- Not an MCP Query Access tool.
- Not a severity field; not a change to the registered audit rule catalog.
- `tls_mode` default remains disabled.

## Rule Catalog Facts

The registered audit rule catalog is unchanged from v0.450.0. This release changes Query Access manifest scope only.

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

- `docs/decisions/2026-07-28-query-access-relationless-literal-selects.md` (this release)
- `docs/decisions/2026-07-26-query-access-literal-only-and-reversed-operands.md` (v0.450.0)
- `docs/decisions/2026-07-22-query-access-cli-tls-and-credentials.md` (v0.440.0/v0.430.0)
- `docs/decisions/2026-07-20-query-access-online-connection-registry.md` (v0.420.0)
- MySQL/TiDB builtin semantic manifests (v0.410.0): `docs/decisions/2026-07-18-query-access-mysql-tidb-builtin-semantic-manifests.md`
- Common pure-effect Query Access (v0.400.0): `docs/decisions/2026-07-16-query-access-common-pure-effects.md`
- Trusted PostgreSQL SDK (v0.390.0): `docs/decisions/2026-07-12-query-access-pure-read-admissibility.md`
- Query Access foundation (v0.380.0): `docs/decisions/2026-07-11-query-access-analysis-foundation.md`
