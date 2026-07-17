# Design: Query Access MySQL/TiDB Builtin Effect Identity Feasibility

Date: 2026-07-17
Status: Proposed
Specification: `docs/plans/2026-07-17-query-access-mysql-tidb-effect-identity-feasibility-spec.md`
Decision: `docs/decisions/2026-07-17-query-access-mysql-tidb-effect-identity-feasibility.md`

## Design Position

This design is deliberately conditional. It defines the evidence and boundary
required for a safe MySQL or TiDB implementation; it does not assume either
dialect has a PostgreSQL-style catalog identity. The feasibility phase may end
with no production promotion code.

The existing shared TiDB parser path is useful for candidate extraction and
dependency collection. It is not an identity authority, and it cannot make
MySQL and TiDB one proof domain.

## Existing Pipeline and Required Extension

```text
SQL
  -> TiDB parser/extractor
  -> relations, columns, bounded effect candidates, reason flags
  -> metadata resolution and strict requirement construction
  -> dialect-specific proof gate (only after feasibility GO)
  -> existing admission recomputation
  -> public result with no proof internals
```

The new proof gate must be reached only after existing fail-closed checks for
statement classification, unsupported traversal, unqualified/ambiguous source,
view, wildcard, unresolved metadata, and strict dependency completeness. It
must use the existing final admission recomputation rather than adding a parser
or transport-specific `admissible` shortcut.

## Candidate and Dependency Model

Reuse the internal candidate model where it can represent MySQL/TiDB facts;
extend it only after a characterization test proves a real gap. Required fields
are bounded kind, AST ordinal, arity, aggregate/window modifiers, direct
operand provenance, and references to dependency roles already extracted by
Query Access.

Phase 1 eligibility is intentionally narrower than grammar acceptance:

| Candidate | Permitted input | Required strict dependencies |
| --- | --- | --- |
| `COUNT(*)` | no modifier, physical base relations only | every `read_table` |
| Direct-column aggregate | exactly one physical base column, no cast/expression | table + argument column |
| Ranking window | no arguments; direct physical partition/order columns; default frame only | table + partition/window + order/window/ordering columns |

The collector must recursively visit projection, FROM/subqueries, join
conditions, WHERE, GROUP BY, HAVING, ORDER BY, LIMIT/OFFSET, VALUES, set
operations, CTEs, aggregate arguments/modifiers, and window definitions. Any
unhandled expression node or role causes a bounded fail-closed outcome. The
proof gate may not compensate for incomplete traversal.

## Feasibility Evidence Protocol

For each dialect and exact tested server version, run the same protocol against
a real Docker server on a dedicated caller-owned `*sql.Conn`:

1. Capture an initial context containing vendor/version/build facts, current
   database, SQL mode, and any session/compatibility setting found by research
   to affect function or identifier resolution. Do not expose these facts.
2. Probe normal, qualified, and malformed forms of each candidate. Attempt
   stored-function, UDF/plugin, and compatibility/shadowing counterexamples
   that the server permits. Record creation failure as evidence only; it does
   not itself prove a builtin identity.
3. Determine whether the server can return a unique, non-name identity for the
   resolved builtin, including the effective argument/result type facts and
   implementation class. `SHOW`, `INFORMATION_SCHEMA`, `EXPLAIN`, prepare, or
   another server facility is acceptable only if it proves this binding rather
   than echoing the input spelling.
4. Read relation metadata and column types on the same connection. Start the
   dialect-appropriate consistent-read transaction only if the server supports
   the required metadata/identity snapshot semantics; prove the scope with
   initial and final context checks.
5. Capture final context, compare it to the initial context, and reject all
   facts on a relevant mismatch, incomplete read, transport failure, or
   unresolved ambiguity.

The research ledger must identify which captured context fields are relevant
and why. It must never convert unknown relevance into an ignored field.

## Conditional Trusted Session Architecture

Only if a dialect reaches GO, expose a separate opt-in public session API for
that dialect. The exact names are a design task after evidence, but it must
mirror these ownership properties:

```text
caller opens *sql.Conn
  -> opaque dialect Query Access session
  -> internal connection-backed relation/type resolver
  -> internal identity adapter and context capture
  -> application proof gateway and manifest policy
  -> public QueryAccessResult
caller continues to use or close *sql.Conn
```

Rules:

- The public constructor takes `context.Context` and a caller-owned `*sql.Conn`.
- DeltaScope does not own or close that connection.
- Every resolver used by the trusted analysis path is constructed from that
  exact connection. No adapter may retain or open a `*sql.DB` fallback.
- The public analysis wrapper rejects a non-nil external `SchemaResolver` and
  a mismatched dialect rather than mixing proof sources.
- The default `AnalyzeQueryAccess`, CLI, HTTP, and MCP paths do not call this
  constructor and do not open live metadata connections.
- Non-supported build tags expose matching bounded stubs, not absent symbols.

If the feasibility result is a server-independent static model, it must still
prove version, shadowing, and type binding. It may use the normal default
service only after a separate decision explicitly accepts that no live context
is necessary. This milestone must not silently choose that path.

## Fact, Manifest, and Trust Boundaries

The dialect resolver returns facts only. Its internal fact batch must include
enough structured data to establish candidate binding without public leakage:

- candidate ordinal and bounded candidate kind;
- server-resolved object identity, implementation class, and argument/result
  type identities, if the server exposes them;
- dialect/version and connection-context pins; and
- canonical structured signature, if and only if it is derived from the
  identity facts rather than copied from the SQL spelling.

The application validates ordinal closure, candidate/fact binding, type
binding, initial/final context compatibility, and fact pinning before the
application-owned policy compares the entire tuple to an audited manifest.
The policy is the sole trust authority. Neither the resolver nor metadata
adapter returns a boolean `Trusted` field.

There is no fallback from a missing fact to a function name or determinism
class. An unknown, duplicate, stale, cross-connection, or non-manifest fact
leaves `unknown_function_effect` in place.

## Privacy and Error Boundary

Resolver errors may be logged only through existing server-side safe logging
policy. Public SDK errors, CLI/HTTP JSON, corpus output, reason codes, and
public structs must not include raw driver errors, SQL, literals, server
identity/version/context, candidate names, signatures, manifest data, DSNs,
or credentials. No `severity` field is introduced.

Tests must inject marker strings into resolver/driver-like errors and assert
they cannot reach every public surface that participates in the chosen path.

## Kill Criteria

Stop a dialect before API or admission work if any of these is true:

1. The best available identity is the function name, parser token,
   `DETERMINISTIC`, schema, or a generic vendor promise.
2. A stored function, UDF/plugin, qualification, type/coercion, or relevant
   session mode can select a different implementation without a unique fact
   detecting it.
3. Metadata/type/identity/context cannot be tied to one connection and a
   dialect-appropriate consistent-read scope.
4. Strict dependency completeness cannot be proved for a candidate shape.
5. A required public path would open a connection implicitly or leak proof
   internals.

The implementation response is to retain `unknown_function_effect`, document
the evidence, and mark that dialect DEFER or KILL. It is never to add a wider
syntax allowlist.

## Cross-Surface Matrix

| Surface | MySQL/TiDB before GO | After a dialect-specific GO |
| --- | --- | --- |
| Default SDK | existing behavior; functions indeterminate | unchanged unless a later decision proves a connection-free model |
| Opt-in SDK | absent | caller-owned session path only, if live proof is required |
| CLI | existing default behavior | unchanged |
| HTTP | existing default behavior and bounded errors | unchanged |
| MCP | no Query Access tool | unchanged |

The documentation must state the exact dialect/version/surface matrix. It may
not describe PostgreSQL proof as MySQL/TiDB support.
