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
  -> exact COUNT(const) candidate characterization
  -> unchanged physical relation completeness proof
  -> PG17 catalog-bound COUNT(1) identity proof
  -> read_only + admissible + [read_table]
```

Every stage is conjunctive. Any missing relation metadata, unsupported
modifier, candidate mismatch, catalog mismatch, unsupported server identity,
or failed session proof produces the current indeterminate result.

## Proof Inputs

The implementation must determine, from the connected PG17 catalog rather
than inferred names, the aggregate/function identity selected for the exact
`COUNT(1)` expression and its argument type. The implementation plan must
first identify which existing metadata queries and proof structures already
expose those facts. If they cannot establish the required identity without
executing user SQL or adding a broad resolver, this proposal stops at
indeterminate and the ADR remains Proposed or is recorded as Deferred.

The literal value itself is not a proof input and is never emitted. Only its
parser classification (`const`) and the server-resolved aggregate identity may
participate in the proof.

## Requirement Semantics

`COUNT(1)` reads the query's base relation. The literal does not name a
physical object. Consequently, a successful strict result has the resolved
base table `read_table` requirement and no synthetic column requirement.

The implementation must not weaken `strictPhysicalRequirementsComplete` or
the ordinary relation/column completeness path. It adds a narrow candidate
proof after those conditions are already true.

## Rejection Matrix

| Shape or condition | Required outcome |
| --- | --- |
| `COUNT(1) FROM app.orders` with PG17 session and complete metadata | Candidate for proof |
| `COUNT(*)` or `COUNT(column)` | Existing behavior unchanged |
| `SELECT COUNT(1)` | Indeterminate |
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

### Support all PostgreSQL literal functions together

Rejected because each function family has distinct coercion, identity, and
session-context evidence requirements.
