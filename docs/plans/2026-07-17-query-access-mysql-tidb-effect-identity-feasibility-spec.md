# Specification: Query Access MySQL/TiDB Builtin Effect Identity Feasibility

Date: 2026-07-17
Status: Proposed
Baseline: v0.400.0 (`e01d7e8`)
Decision: `docs/decisions/2026-07-17-query-access-mysql-tidb-effect-identity-feasibility.md`

## Product Problem

MySQL and TiDB users can already receive structured requirements for simple
function-free `SELECT` queries. Once a function, aggregate, or window function
appears, the shared parser records `function_call` and Query Access preserves
the safe `unknown_function_effect` result. This blocks ordinary reporting
queries even when their relation and column requirements are otherwise known.

The desired result is not broad function support. It is a defensible path to
admit a small set of builtin reporting effects only when DeltaScope can prove
the executed identity and the complete static data requirements.

## Supported Versions and Surfaces

The feasibility environment is pinned before any promotion claim:

| Dialect | Initial probe environment | Promotion scope before evidence |
| --- | --- | --- |
| MySQL | Docker `mysql:8.4` from `docker/cli-e2e-compose.yaml` | none |
| TiDB | Docker `pingcap/tidb:v8.5.0` from `docker/cli-e2e-compose.yaml` | none |

The executor must record the actual server version returned by each probe. A
future supported range must be evidence-backed and version-scoped; an image
tag is not sufficient proof by itself.

Current behavior remains the baseline on all public surfaces:

- MySQL/TiDB function-bearing queries: `indeterminate` with
  `unknown_function_effect`.
- MySQL/TiDB function-free, metadata-complete read-only queries: existing
  admissible behavior is unchanged.
- PostgreSQL trusted SDK behavior: unchanged.
- Default SDK, CLI, HTTP, and MCP: no new live connection or promotion path.

## Candidate Scope

The following are research candidates, not a syntax allowlist and not a
shipping promise:

| Candidate | Eligible shape if a dialect reaches GO |
| --- | --- |
| `COUNT(*)` | one or more resolved physical base relations; no excluded modifiers |
| `COUNT(column)` | direct, uniquely resolved physical base column |
| `SUM`, `AVG`, `MIN`, `MAX` | one direct, uniquely resolved physical base column |
| `ROW_NUMBER`, `RANK`, `DENSE_RANK` | zero direct function args and direct physical partition/order columns only |

Each candidate must be represented internally with stable ordinal, bounded
kind, arity, modifier flags, direct operand provenance, and links to extracted
dependency roles. It may not carry raw SQL/literals into public results.

## Strict Requirement Contract

An admitted query must include all of these dependencies in strict mode:

- every physical base relation read by the query;
- direct aggregate argument columns as projection sources;
- `GROUP BY`, `HAVING`, and ordering sources when present;
- window partition sources with `window` usage and window order sources with
  both window and ordering semantics where the existing contract expresses
  them; and
- any expression source that influences the admitted result.

`projection_only` keeps its inference-risk semantics. It may not use omitted
strict dependencies to establish a proof or an authorization-ready claim.

## Mandatory Fail-Closed Matrix

All of the following remain `indeterminate` unless a later accepted decision
introduces a narrower proof:

- an unknown, stored, UDF, plugin, or schema-qualified non-builtin function;
- nested or arbitrary scalar function operands;
- parameters, literals, `NULL`, casts, coercion, or overloaded/ambiguous type
  resolution;
- `DISTINCT`, aggregate-local `ORDER BY`, `FILTER`, ordered-set aggregates,
  named windows, explicit frames, or windowed aggregates;
- unqualified relations where the dialect cannot bind the physical source in
  the proof context; views, CTEs, derived tables, wildcard input, ambiguity,
  missing metadata, resolver errors, or unsupported AST;
- multi-statements, writes, locking reads, `SELECT INTO OUTFILE`, or any
  existing not-read-only/rejected shape.

## Identity Proof Requirements

A dialect can promote only if the implementation can answer all of these for
the candidate being analyzed:

1. What exact server-resolved builtin will execute, independent of the raw
   spelling used in the SQL?
2. Can stored functions, UDFs/plugins, qualification, compatibility modes, or
   relevant session state select a different implementation?
3. Which version/build and connection context make those facts valid?
4. Can relation metadata, column types, identity facts, and initial/final
   context be read on one caller-owned `*sql.Conn` and a dialect-appropriate
   consistent-read boundary?
5. Can an application-owned, version-scoped manifest audit the returned facts
   without trusting a resolver-provided `Trusted` flag?

Forbidden roots are function spelling, parser node kind, `DETERMINISTIC`, a
schema label, a generic vendor claim, or a copied PostgreSQL OID rule.

## Result and Privacy Invariants

- A proof failure cannot turn a rejected result into `indeterminate` or an
  indeterminate result into `admissible`.
- One proven candidate cannot remove another candidate's reason.
- Reasons are bounded deterministic identifiers. They contain no function
  name, source name, OID, signature, SQL, literal, credential, DSN, driver
  error, resolver state, or `severity`.
- Public JSON has no candidate, identity, manifest, connection, transaction,
  or session fields.
- A session API, if feasibility requires one, does not close the caller-owned
  connection and rejects an externally supplied resolver that could mix
  connections.

## Success Criteria

The milestone succeeds in one of two ways:

1. One or both dialects reach GO with a tested, opt-in, version-scoped proof
   path and only the exact candidate subset proven by Docker E2E, corpus, and
   independent review.
2. The evidence establishes DEFER/KILL. The code remains fail-closed,
   `unknown_function_effect` is retained, and the decision records why a
   name-based implementation would be unsafe.

Neither outcome selects a release version, adds an MCP tool, or broadens
authorization semantics.
