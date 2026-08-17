# Decision: Consolidate Query Access Proof Orchestration

- Date: 2026-08-16
- Status: Proposed
- Related: [Pure-read admissibility](2026-07-12-query-access-pure-read-admissibility.md), [PG17 COUNT literal proof](2026-07-31-query-access-pg17-count-literal-proof.md), [Unified online analysis entry](2026-08-12-query-access-online-analysis-entry.md)
- Spec: `docs/plans/2026-08-16-query-access-proof-orchestration-spec.md`
- Design: `docs/plans/2026-08-16-query-access-proof-orchestration-design.md`
- Implementation: `docs/plans/2026-08-16-query-access-proof-orchestration-implementation.md`

## Context

Query Access has one public online analysis entry but its application service
still coordinates promotion through three internal clocks. Ordinary PostgreSQL
manifest proof runs before requirements, while exact PostgreSQL `COUNT(1)` and
MySQL/TiDB builtin proof run afterward. Each branch performs some combination
of reason normalization, read reclassification, and admission recomputation.

This is behaviorally correct today, but the shape makes promotion safety depend
on updating every branch consistently. The exact `COUNT(1)` and builtin paths
also establish the stronger invariant that physical requirements are complete
before proof may permit promotion.

## Proposed Decision

Build requirements before every Effect Proof. Route ordinary PostgreSQL, exact
PostgreSQL `COUNT(1)`, MySQL/TiDB builtin, and no-effect applicability through
one private orchestration function. The function returns only whether proof
permits common promotion checks and the resulting bounded reason set.

After orchestration, normalize reasons, reclassify reads, and recompute
admission once. Promotion Barriers, unresolved facts, remaining reasons,
missing resolvers, incomplete requirements, and write classification continue
to fail closed.

Keep the PostgreSQL and MySQL/TiDB proof models separate behind this function.
Do not add a proof interface, registry, factory, plugin, or public API.

## Rationale

One sequencing point makes the safety invariant visible: requirements exist
before proof, and final admission is derived once after proof. A private
function is sufficient because the product set is closed and there is one
orchestration caller. An interface would add speculative flexibility without
removing another responsibility.

Keeping proof-specific decisions inside their current implementations preserves
PostgreSQL atomic catalog evidence and MySQL/TiDB manifest semantics. The common
layer needs only the narrower fact that proof allows the existing promotion
checks to continue; it does not need a synthetic shared proof domain.

## Contract

- Public results, supported SQL, errors, transports, and releases do not change.
- Physical Requirement Completeness precedes every proof-based promotion.
- Promotion Barriers cannot be removed or overridden by proof.
- PostgreSQL indeterminate results require `all_proven`; absent proof is not
  vacuous success.
- MySQL/TiDB without effect candidates retain ordinary reclassification;
  candidates require successful builtin proof.
- Successful proof removes only its existing owned reason codes.
- Catalog probes, atomic same-session behavior, cancellation, caller ownership,
  no-execution, and no-leak behavior remain unchanged.
- Final reason normalization, reclassification, and admission computation each
  have one orchestration point.

## Consequences

Positive:

- Future proof changes have one application sequencing point.
- `Service.Analyze` no longer exposes proof-specific or variadic arguments in
  its common final-state logic.
- Requirements-before-proof becomes a pipeline invariant rather than a
  special-case property.

Costs:

- Ordinary PostgreSQL proof moves past a pure in-memory requirement build.
- A security-sensitive internal refactor requires broad equivalence and
  recording evidence despite having no public feature change.

## Alternatives Rejected

- Add a `proofEngine` interface: rejected as a one-caller, closed-set
  abstraction with no current polymorphic need.
- Extract helpers but retain three clocks: rejected because sequencing drift
  remains possible.
- Merge dialect proof models: rejected because their evidence and failure
  semantics are intentionally different.
- Refactor identity-resolution protocols simultaneously: rejected because it
  would combine two trust-boundary changes.
- Optimize away existing proof calls: rejected because it could change probe,
  cancellation, or reason behavior.

## Deferred Scope

- Identity resolver interface cleanup.
- New proof types, SQL shapes, products, versions, or manifests.
- Catalog-probe optimization and rejected-query short-circuiting.
- Test ownership changes or transport/API redesign.

## Acceptance Criteria

This decision remains Proposed until a fixed candidate demonstrates exact
result and probe equivalence, all required gates pass, and fresh Standards and
Spec/security reviews report no unresolved P0, P1, or P2. Acceptance evidence
must cite immutable base and candidate SHAs.