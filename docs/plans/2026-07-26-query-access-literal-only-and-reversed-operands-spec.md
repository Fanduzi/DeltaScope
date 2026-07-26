# Specification: Query Access Literal-Only and Reversed Operand Shapes

Date: 2026-07-26
Status: Proposed
Baseline: `main@d2c4d91`
Decision: `docs/decisions/2026-07-26-query-access-literal-only-and-reversed-operands.md`

## Product Requirement

v0.440.0 proves one mixed operand shape for MySQL/TiDB online Query Access:
`COALESCE`, `NULLIF`, and `IFNULL` with a direct physical column followed by a
literal. This proposal considers three still-deferred shapes without turning
the manifest into a generic function-name allowlist:

1. A proven scalar with literal-only operands, such as `LOWER('x')` or
   `ABS(42)`.
2. `COUNT(1)` over a resolved physical base relation.
3. A proven binary scalar whose direct physical column is in the second
   position, such as `NULLIF('x', name)`.

The implementation must continue to identify exactly what a query reads, must
never execute the submitted SQL, and must preserve the existing offline
fail-closed boundary.

## Scope

This proposal applies only after all of these conditions hold:

- The caller uses the explicit online MySQL/TiDB session path: caller-owned
  SDK session, CLI online connection flags, or HTTP `connection_id`.
- The connected server is identified as MySQL 5.7.x, 8.0.x, 8.4.x, or TiDB
  8.5.x and selects the corresponding existing profile.
- Every relation that the query reads is a schema-qualified, resolved physical
  base table.
- Every function candidate has an exact entry in the immutable manifest and
  satisfies all existing syntax and modifier restrictions.

PostgreSQL, default SDK, offline CLI, offline HTTP, and MCP remain outside this
proposal. They keep their current behavior.

## Required Behavior

### Exact Operand Shapes

The manifest must match operand kinds by position. The proposed initial shapes
are deliberately finite:

| Call | Permitted operand kinds | Arity | Profiles |
| --- | --- | --- | --- |
| `LOWER`, `UPPER`, `LENGTH`, `CHAR_LENGTH`, `ABS`, `CEIL`, `CEILING`, `FLOOR` | `[const]` | exactly 1 | MySQL 5.7/8.0/8.4, TiDB 8.5 |
| `COUNT` | `[const]` | exactly 1 | MySQL 5.7/8.0/8.4, TiDB 8.5 |
| `NULLIF`, `IFNULL` | `[const, column]` | exactly 2 | MySQL 5.7/8.0/8.4, TiDB 8.5; `IFNULL` is not PostgreSQL |
| `COALESCE` | `[const, column]` | exactly 2 | MySQL 5.7/8.0/8.4, TiDB 8.5 |
| `COALESCE`, `NULLIF`, `IFNULL` | `[const, const]` | exactly 2 | MySQL 5.7/8.0/8.4, TiDB 8.5 |

`const` means the parser's existing literal kind only. It does not mean a
parameter, cast, expression, subquery, identifier, wildcard, or arbitrary AST
node. The current v0.440.0 `[column, const]` shapes remain unchanged.

The proposal does not permit a matcher to repeat the final operand kind for
variable-arity calls. Three-or-more-argument `COALESCE` remains
`indeterminate` until separately specified and proven.

### Requirements and Classification

Literals create no table or column requirement. Direct physical column
operands create the same table and column requirements regardless of operand
position. A resolved `FROM app.table` relation creates its ordinary table-read
requirement even when no function operand is a column.

| SQL | Required static reads | Expected online result |
| --- | --- | --- |
| `SELECT COUNT(1) FROM app.orders` | `app.orders` / `read_table` | `read_only` + `admissible` |
| `SELECT LOWER('x') FROM app.orders` | `app.orders` / `read_table` | `read_only` + `admissible` |
| `SELECT NULLIF('x', name) FROM app.users` | `app.users` / `read_table`; `app.users.name` / `read_column` | `read_only` + `admissible` |
| `SELECT COALESCE('x', 'y') FROM app.users` | `app.users` / `read_table` | `read_only` + `admissible` |

Relationless function queries such as `SELECT LOWER('x')` are deferred. This
milestone must not invent an empty-requirement admission contract.

### Fail-Closed Rules

The following remain `indeterminate`:

- Any default/offline surface, PostgreSQL surface, or unsupported server
  series.
- Unqualified, unresolved, view, wildcard, CTE, derived-table, or otherwise
  non-physical relation input.
- Parameters, casts, arithmetic, nested functions, subqueries, expressions,
  quoted or qualified function calls, noncanonical syntax, unknown functions,
  UDFs, stored functions, plugin functions, and unsupported SQL modifiers.
- `COALESCE` with arity other than two, including forms that a repeated-tail
  variable-arity matcher could accept.
- Literal-only `COUNT(1)` with no physical base relation.

`admissible` remains a static statement about identified reads. It is not
database authorization, grant/RLS/masking/rewrite evaluation, a query result,
or an execution-snapshot guarantee.

## Privacy and Error Contract

No public result, CLI output, HTTP response, or HTTP access log may contain a
literal value, raw submitted SQL, manifest entry, parser detail, profile,
connection credential, endpoint, or driver error because of this change.
Existing bounded connection-error behavior is unchanged.

## Non-Goals

- General literal support or a generic pure-function allowlist.
- Empty-requirement admission for relationless `SELECT`.
- Any PostgreSQL literal promotion.
- Parameters, casts, nested expressions, literal values embedded in output,
  function evaluation, SQL execution, or data return.
- A change to HTTP authentication, TLS, connection registry, CLI credentials,
  MCP, public JSON shape, or release version.

## Acceptance Criteria

The ADR may move from Proposed only when all conditions are met:

1. Parser characterization tests prove each exact operand vector and reject
   nearby shapes.
2. Manifest validation rejects malformed arity/kind combinations and matching
   never broadens a declared vector.
3. Unit and corpus tests prove requirements for column-first, literal-first,
   literal-only scalar, and `COUNT(1)` cases.
4. Docker-backed public SDK, CLI, and HTTP tests prove every admitted shape on
   MySQL 5.7/8.0/8.4 and TiDB 8.5; the default offline paths remain
   `indeterminate`.
5. No-leak tests inject unique literal markers and prove they are absent from
   public responses, CLI output, HTTP output, and HTTP logs.
6. PostgreSQL and every deferred shape have explicit negative regression tests.
7. Oracle reports no P0/P1/P2 findings and Momus returns `[OKAY]` for the
   implementation plan.
