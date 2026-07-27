# DeltaScope v0.450.0 Release Notes

## Summary - Exact Shape Expansion for Online MySQL/TiDB Literal Operands

v0.450.0 extends online MySQL/TiDB Query Access with exact literal-only, reversed, and all-constant operand shapes. On a caller-owned session (SDK), an online CLI connection, or an HTTP registered `connection_id`, common queries such as `SELECT COUNT(1) FROM app.orders` or `SELECT LOWER('x') FROM app.users` now return `admissible` with precise physical requirements instead of `indeterminate`. The supported profiles are MySQL 5.7, 8.0, 8.4, and TiDB 8.5.

This release does not change default offline SDK, CLI, or HTTP behavior, and MCP still has no Query Access tool. Query Access does not execute user SQL.

## What Changed

### New Admitted Shapes

The MySQL/TiDB builtin semantic manifest now admits these additional shapes on the online path (caller-owned `*sql.Conn` via SDK, online CLI connection, or HTTP `connection_id`):

**Unary literal-only (`[const]` operand):**

| Function | Example |
|----------|---------|
| `LOWER` | `SELECT LOWER('x') FROM app.users` |
| `UPPER` | `SELECT UPPER('x') FROM app.users` |
| `LENGTH` | `SELECT LENGTH('x') FROM app.users` |
| `CHAR_LENGTH` | `SELECT CHAR_LENGTH('x') FROM app.users` |
| `ABS` | `SELECT ABS(42) FROM app.users` |
| `CEIL` | `SELECT CEIL(42) FROM app.users` |
| `CEILING` | `SELECT CEILING(42) FROM app.users` |
| `FLOOR` | `SELECT FLOOR(42) FROM app.users` |

**Aggregate literal (`[const]` operand):**

| Function | Example |
|----------|---------|
| `COUNT(1)` | `SELECT COUNT(1) FROM app.orders` |

**Reversed binary (`[const, column]` operands):**

| Functions | Example |
|-----------|---------|
| `COALESCE`, `NULLIF`, `IFNULL` | `SELECT NULLIF('x', name) FROM app.users` |

**All-constant binary (`[const, const]` operands):**

| Functions | Example |
|-----------|---------|
| `COALESCE`, `NULLIF`, `IFNULL` | `SELECT COALESCE('a', 'b') FROM app.users` |

Each shape is admitted across all four profiles (MySQL 5.7, 8.0, 8.4, TiDB 8.5), giving 60 profile-shape combinations total.

### Requirement Model

Every admitted query requires at least one resolved physical base relation:

- A resolved physical base relation contributes `read_table`.
- A direct physical column contributes `read_column` plus its table read.
- A literal contributes no table or column requirement.
- No `admissible` result produces an empty requirements list.

For example, `SELECT NULLIF('x', name) FROM app.users` requires `app.users` and `app.users.name`; `SELECT COUNT(1) FROM app.orders` requires only `app.orders`.

### Manifest Validation

`validateBuiltinSemanticEntry` now rejects malformed fixed-arity entries:
- `len(OperandKinds) != Arity` when `MinArity == 0` and `Arity > 0`
- Arity-0 entries with non-star operand kinds

Regression test: `TestBuiltinSemanticManifest_RejectsInvalidEntries`.

### Architecture

The MySQL/TiDB path bypasses `ValidatePhase1PureEffectCandidates` entirely. In `service.go`, the MySQL/TiDB builtin gateway (`proveBuiltinSemantics`) runs directly on candidates without Phase-1 eligibility filtering. The PostgreSQL path (`resolveAndProveEffects`) calls `ValidatePhase1PureEffectCandidates` which rejects literal-only operands. This preserves the PostgreSQL boundary while enabling literal-only shapes for MySQL/TiDB.

### Previously Supported (unchanged from v0.440.0)

`COALESCE(column, const)`, `NULLIF(column, const)`, and `IFNULL(column, const)` with `[column, const]` operand order were already admitted on the online MySQL/TiDB path. v0.450.0 adds the reversed `[const, column]` and all-constant `[const, const]` variants.

## What Stayed the Same

- Default offline SDK, CLI, and HTTP behavior is unchanged. Without an online session, function-bearing queries remain `indeterminate`.
- CLI `--tls-mode` defaults to `disabled`. Enabling TLS requires explicit `--tls-mode=enabled`.
- Query Access emits structured requirements only. It does not authenticate callers, evaluate grants, enforce RLS, mask columns, auto-grant privileges, rewrite SQL, or guarantee a later execution snapshot.
- MCP tools remain `audit_sql`, `describe_rule`, `list_rules`, and `get_capabilities` only. No Query Access tool is added.
- The audit rule catalog and default audit behavior are unchanged. `level` remains the public audit priority field; no severity field is introduced.
- Query Access results do not include raw SQL, literals, function names, DSNs, credentials, driver errors, session data, endpoint addresses, or secrets.

## Non-Goals

- Not a general pure-function or SELECT admission. Only the exact shapes listed above are admitted; all other literal-only, nested, or multi-operand forms remain `indeterminate`.
- Not relationless literal-only `SELECT` (no FROM clause). Every admitted query must have at least one resolved physical base relation.
- Not 3+ operand `COALESCE`/`NULLIF`/`IFNULL`. Only exactly two operands are admitted.
- Not PostgreSQL literal operands. Literal operand expansion is MySQL/TiDB online only.
- Not nested expressions, casts, parameters, UDFs, quoted/qualified calls, or arbitrary function support.
- Not SQL execution or data-returning APIs.
- Not database authorization, grants, roles, RLS, masking, rewrite, or execution-snapshot guarantees.
- Not an MCP Query Access tool.
- Not a severity field; not a change to the registered audit rule catalog.

## Rule Catalog Facts

The registered audit rule catalog is unchanged from v0.440.0. This release changes Query Access manifest scope only.

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

- `docs/decisions/2026-07-26-query-access-literal-only-and-reversed-operands.md` (this release)
- `docs/decisions/2026-07-22-query-access-cli-tls-and-credentials.md` (v0.440.0/v0.430.0)
- `docs/decisions/2026-07-20-query-access-online-connection-registry.md` (v0.420.0)
- MySQL/TiDB builtin semantic manifests (v0.410.0): `docs/decisions/2026-07-18-query-access-mysql-tidb-builtin-semantic-manifests.md`
- Common pure-effect Query Access (v0.400.0): `docs/decisions/2026-07-16-query-access-common-pure-effects.md`
- Trusted PostgreSQL SDK (v0.390.0): `docs/decisions/2026-07-12-query-access-pure-read-admissibility.md`
- Query Access foundation (v0.380.0): `docs/decisions/2026-07-11-query-access-analysis-foundation.md`
