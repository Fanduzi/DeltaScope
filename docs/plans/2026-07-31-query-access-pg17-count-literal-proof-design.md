# Design: PG17 Query Access `COUNT(1)` Proof

## Status

Proposed. This design is deliberately narrower than the MySQL/TiDB literal
manifest work. PostgreSQL must use its existing session-bound catalog proof.

## Existing Boundary

The PostgreSQL trusted path is entered only through a caller-owned `*sql.Conn`.
It resolves and proves effect candidates with PostgreSQL metadata before a
result can become admissible. The default SDK, CLI, HTTP, and MCP paths do not
obtain that trusted session and therefore remain fail-closed.

MySQL/TiDB profile manifests and their parser behavior are not reusable proof
for PostgreSQL. PostgreSQL aggregate admission must continue to derive from
the connected PG17 server's identity and catalog facts.

## Proposed Flow

```text
caller-owned PG17 *sql.Conn
  -> PostgreSQL session identity and catalog resolver
  -> strict Query Access extraction
  -> internal-only exact COUNT(integer_one) characterization
  -> narrow PG17 COUNT(1) Phase-1 eligibility exception
  -> unchanged physical relation completeness proof
  -> PG17 catalog-bound pg_catalog.count(any) identity proof
  -> read_only + admissible + [read_table]
```

Every stage is conjunctive. Any missing relation metadata, unsupported
modifier, candidate mismatch, catalog mismatch, unsupported server identity,
or failed session proof produces the current indeterminate result.

## Proof Inputs

The parser must inspect the AST constant without retaining its source text and
emit a non-serialized `integer_one` shape only for an uncast integer constant
whose parsed value is `1`. Every other constant remains non-admitted. This
shape is a bounded syntax fact, not a type OID and not a trust root.

The implementation must determine, from the connected PG17 catalog rather
than inferred names, that the resolved function is the `pg_catalog` aggregate
`count(any)`. The catalog facts must establish namespace, aggregate class, and
polymorphic signature under the captured session context. The implementation
must not synthesize an argument OID by treating the literal as a column. If the
existing metadata queries cannot establish this identity without executing user
SQL or adding a broad resolver, this proposal stops at indeterminate and the
ADR remains Proposed or is recorded as Deferred.

The literal text and value are never emitted. Only the bounded `integer_one`
shape and the server-resolved aggregate identity may participate in the proof.

## Narrow Phase-1 Exception

The present PostgreSQL path evaluates Phase 1 before catalog identity lookup,
and its general arity-one `const` rule remains rejecting. The implementation
must add a separate internal eligibility predicate for exactly one candidate:
an unqualified `COUNT` function with one `integer_one` operand and no modifier
or nested effect. This predicate supplies eligibility only; it cannot promote a
result, bypass strict physical requirements, or prove the aggregate identity.
All other literal-bearing candidates continue through the existing rejecting
Phase-1 path.

## Requirement Semantics

`COUNT(1)` reads its sole base relation. The literal does not name a physical
object. The admitted statement envelope therefore requires exactly one resolved
physical table and no referenced columns; a successful strict result has one
`read_table` requirement and no synthetic column requirement. Multiple base
relations, including joins and comma joins, are outside this milestone.

The implementation must not weaken `strictPhysicalRequirementsComplete` or
the ordinary relation/column completeness path. It adds a narrow candidate
proof after those conditions are already true.

## Rejection Matrix

| Shape or condition | Required outcome |
| --- | --- |
| `COUNT(1) FROM app.orders` with PG17 session and complete metadata | Candidate for proof |
| `COUNT(*)` or `COUNT(column)` | Existing behavior unchanged |
| `SELECT COUNT(1)` | Indeterminate |
| `COUNT(NULL)`, `COUNT(2)`, `COUNT('1')`, or another non-`integer_one` constant | Indeterminate |
| Join, comma join, or more than one physical base relation | Indeterminate |
| Unqualified, view, CTE, derived, wildcard, ambiguous, or unresolved source | Indeterminate |
| Parameter, cast, expression, nested call, DISTINCT, FILTER, ORDER, OVER, arity mismatch | Indeterminate |
| PostgreSQL version/catalog/identity mismatch | Indeterminate |
| Default SDK, CLI, HTTP, MCP, MySQL, TiDB | Existing behavior unchanged |

## Safety Properties

- The only database operations remain session identity, catalog metadata, and
  existing safe probes. User SQL is never sent to the driver.
- Public output remains the existing result schema. No proof details, literal
  values, OIDs, connection values, or raw errors become public fields.
- Reason codes remain deterministic machine identifiers and cannot be supplied
  by callers.

## Alternatives Rejected

### Copy the MySQL/TiDB manifest

Rejected because profile names and literal parser classifications do not prove
the selected PostgreSQL catalog identity.

### Allow constants in PostgreSQL Phase 1 globally

Rejected because it would admit non-proven scalar, operator, cast, parameter,
and relationless shapes.

### Treat all parser `const` operands as `COUNT(1)`

Rejected because the existing candidate shape intentionally does not preserve
literal text or type. A dedicated bounded `integer_one` fact is required to
avoid promoting `COUNT(NULL)`, strings, or other constants.

### Support all PostgreSQL literal functions together

Rejected because each function family has distinct coercion, identity, and
session-context evidence requirements.
