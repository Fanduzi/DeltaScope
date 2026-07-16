# Design: Query Access Common Pure-Effect Admissibility

Date: 2026-07-16
Status: Proposed
Specification: `docs/plans/2026-07-16-query-access-common-pure-effects-spec.md`

## Design Principles

1. Dependency completeness and effect trust are separate gates. A pure
   function cannot compensate for missing table/column requirements.
2. A parser spelling is a candidate, never proof. Proof is dialect and
   version specific.
3. Unknown is safe. Every failure path preserves `indeterminate` and bounded
   reason codes.
4. The public result stays free of raw SQL, literals, identities, credentials,
   session data, trust flags, and `severity`.
5. PostgreSQL reuses the existing single proof gateway. New code must not
   create a second admission path in the parser or SDK.

## Pipeline

```text
AST traversal
  -> complete relation/column/output extraction
  -> internal pure-effect candidates
  -> dependency-completeness gate
  -> dialect proof provider
  -> manifest/policy decision
  -> existing admission recomputation
  -> public result without proof internals
```

The parser must visit every expression-bearing location before it can claim
completeness: projection, WHERE, JOIN condition, GROUP BY, HAVING, ORDER BY,
LIMIT/OFFSET, aggregate arguments, aggregate FILTER/local order when later
supported, window partition/order/frame, VALUES, set operations, CTEs,
subqueries, and dialect-specific expression holders. Any unhandled node is an
unsupported traversal, not an implicit pure expression.

## Internal Candidate Model

Introduce an internal, dialect-neutral candidate representation only after
characterization establishes exact AST coverage. It contains:

- dialect;
- bounded candidate class (`aggregate_count`, `aggregate_sum`,
  `aggregate_avg`, `aggregate_min`, `aggregate_max`, `window_row_number`,
  `window_rank`, `window_dense_rank`);
- stable ordinal in AST order;
- arity, aggregate/window flags, distinct/filter/local-order/frame flags;
- direct operand provenance and operand type facts where the dialect can prove
  them; and
- references to already-extracted dependency roles.

It must not enter `domain.Result`, SDK/CLI/HTTP JSON, corpus public output, or
error text. Raw function spelling may exist transiently inside a parser or
resolver but cannot become a reason code or public field.

The candidate collector is not an allowlist. It only says a query may be
eligible for a dialect proof provider. An unknown function node retains the
existing unproven/unknown reason.

## Dependency-Completeness Gate

Before a provider is invoked, the application verifies:

- all candidate operands and relevant `OVER`/grouping dependencies have a
  physical, uniquely resolved source;
- strict requirements contain every source with its semantic usage;
- no wildcard, ambiguity, unbound relation, view, derived/CTE input, or
  unresolved metadata exists in the candidate's effect scope; and
- the candidate has no Phase 1 excluded modifier.

For `COUNT(*)`, a fully resolved physical base relation supplies the table
dependency. Direct-column aggregate arguments are projection dependencies. For
windows, partition columns use `window` and order columns use `window` plus
existing ordering semantics. Existing deterministic usage order is preserved.

`projection_only` never bypasses this gate. It may omit non-output
requirements according to its published contract, but retains the
inference-risk warning and cannot use omitted strict dependencies as proof that
a query is authorization-ready.

## Dialect Proof Providers

### PostgreSQL 17

The existing `ControlledEffectIdentityResolver`, atomic proof operation, fact
pinning, and `TrustPolicy` remain authoritative. The implementation expands
only the PG17 manifest after a catalog ledger records each aggregate/window
function's exact OID, namespace, argument types, result type, implementation
details, and semantic audit. The operation remains bound to the caller-owned
connection and its database/role/version/session/search-path context.

No default PostgreSQL service receives a resolver or manifest. Only
`AnalyzePostgreSQLQueryAccessWithSession` invokes the trusted path.

### MySQL and TiDB

Do not reuse PostgreSQL OIDs or its session contract by analogy. First define
a `DialectPureEffectProofProvider` boundary whose output is facts, not trust.
The provider design is conditional on research:

- If the supported server exposes a bounded, version-scoped way to establish
  builtin identity and reject shadowing, use that provider plus an
  application-owned manifest/policy.
- If no such identity proof exists, do not add a name-based default allowlist.
  Retain `unknown_function_effect` and record the dialect as deferred.
- A live provider, if required, uses a caller-owned/session-bound connection
  and must not silently make CLI/HTTP open a database connection.

The policy can share candidate classes and test matrices with PostgreSQL, but
identity facts, versions, resolver implementation, and public availability
remain dialect-specific.

## Admission and Reasons

The service collects proof decisions per candidate. It removes a function
reason only when every candidate is proven and dependency complete. It never
uses a global "one candidate succeeded" condition. Any failure retains the
dialect's existing bounded reason:

- PostgreSQL: `unproven_function_effect` plus relevant identity/coercion code.
- MySQL/TiDB: `unknown_function_effect` until a documented dialect-specific
  reason contract is accepted.

The final existing admission recomputation remains the sole place that can
produce `admissible`.

## Threat Model and Kill Criteria

Stop promotion for a dialect if any condition is true:

- proof reduces to a spelling/name/schema/volatility allowlist;
- candidate identity is not unique under supported overload, UDF, plugin, or
  shadowing rules;
- a dependency role cannot be extracted from every supported AST position;
- metadata, type, and identity facts cannot be tied to the required context;
- a public surface would expose proof internals or silently acquire a
  connection; or
- no version-scoped test environment can reproduce the claim.

The fallback is a bounded indeterminate result, not partial promotion.

## Cross-Surface Contract

PostgreSQL trusted promotion stays SDK-only. MySQL/TiDB promotion may affect
the default shared application service only if feasibility explicitly permits a
connection-free model; otherwise it requires a separately designed session
path. CLI/HTTP/MCP must not claim a capability their request/connection model
cannot prove.

All surfaces consume the same domain result. No surface gets a custom
classification branch, raw proof data, or a `severity` field.
