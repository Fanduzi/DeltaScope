# Decision: Query Access Literal-Only and Reversed Operand Shapes

- Date: 2026-07-26
- Status: Proposed
- Baseline: `main@d2c4d91`
- Related: [literal operand support](2026-07-25-query-access-literal-operand-support.md), [builtin semantic manifests](2026-07-18-query-access-mysql-tidb-builtin-semantic-manifests.md)
- Spec: `docs/plans/2026-07-26-query-access-literal-only-and-reversed-operands-spec.md`
- Design: `docs/plans/2026-07-26-query-access-literal-only-and-reversed-operands-design.md`
- Implementation: `docs/plans/2026-07-26-query-access-literal-only-and-reversed-operands-implementation.md`

## Context

v0.440.0 added online MySQL/TiDB proof for three exact mixed shapes:
`COALESCE(column, const)`, `NULLIF(column, const)`, and
`IFNULL(column, const)`. It intentionally leaves literal-only calls,
`COUNT(1)`, and reversed mixed positions indeterminate.

Those deferred shapes need a separate decision because they stress the core
meaning of a Query Access requirement. A literal does not identify a physical
column, while a base relation still represents a table read. A broad change
that merely treats constants like columns, or accepts every constant anywhere,
would make the manifest a name allowlist instead of a bounded proof.

## Proposed Decision

If the implementation evidence is completed, extend only the MySQL/TiDB online
manifest with finite, exact operand vectors for:

- Unary pure scalars with `[const]`.
- `COUNT(const)` over a schema-qualified resolved physical base relation.
- `COALESCE`, `NULLIF`, and `IFNULL` with `[const,column]` and
  `[const,const]`, each at exactly two operands.

The existing `[column,const]` forms remain unchanged. Every newly admitted
shape must be explicit per dialect/profile and must match arity and operand
position exactly. No new variable-arity tail expansion is part of this
decision.

## Requirement Model

- A resolved physical base relation contributes `read_table`.
- A direct physical column contributes `read_column` plus its table read.
- A literal contributes no table or column requirement.
- This proposal requires at least one resolved physical base relation. It does
  not introduce `admissible` results with an empty requirements list.

For example, `SELECT NULLIF('x', name) FROM app.users` would require
`app.users` and `app.users.name`; `SELECT COUNT(1) FROM app.orders` would
require only `app.orders`.

## Consequences

The change can make a small set of common, non-writing online queries usable
when their table and column dependencies are fully known. It does not execute
the query or inspect query results. It does not infer authorization, grants,
RLS, masking, SQL mode, or runtime behavior.

The following remain deferred: relationless literal-only `SELECT`, PostgreSQL
literal operands, `COALESCE` with more than two operands, parameters, casts,
nested functions, expressions, UDFs, quoted/qualified calls, and all default
offline surfaces.

## Alternatives Considered

### Keep All Deferred

This is the lowest-risk option but leaves straightforward online reads such as
`COUNT(1) FROM app.orders` unnecessarily unusable.

### Accept Constants Wherever Columns Are Accepted

Rejected. It loses position-specific proof and would silently admit shapes
that have no parser, manifest, requirement, or live-evidence contract.

### Admit Relationless Literal-Only Queries with Empty Requirements

Deferred. It changes the current strict physical-relation proof model and
requires a separate product decision about the meaning and use of an empty
requirement set.

## Acceptance Evidence Required

This ADR remains Proposed until parser, manifest, requirement, corpus,
Docker-backed SDK/CLI/HTTP, default-offline, PostgreSQL-negative, and no-leak
evidence pass for every declared shape. It also requires an Oracle audit with
no P0/P1/P2 findings and a Momus `[OKAY]` implementation-plan review.
