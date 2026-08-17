# Decision: Consolidate Query Access Proof Orchestration

- Date: 2026-08-16
- Status: Accepted
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
permits the common promotion checks to continue; proof-specific reason
removal happens inside the function on the kept reason set.

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

## Acceptance Evidence

Accepted after two rejection rounds. Round 1 found four P2 findings (dual
reason normalization, test-only `promotionProof.reasonCodes`, stale module
README description, acceptance evidence not covering the final non-ADR
candidate), fixed in `d0341838875ceaeeffac5ad6c15874c6b6c64d7d`. Round 2
found one P2 finding (contract documentation stale after the
`promotionProof.reasonCodes` deletion), fixed in
`98056130563f2eca4e421d1f30b87262fa43fecb` with the ADR reverted to Proposed
in `54641b3`. Fresh Standards and Spec/security reviews of the fixed
candidate `98056130563f2eca4e421d1f30b87262fa43fecb` against the immutable
base `099ebbf0b79a229afbd7489344a218e5d404b43c` (origin/main) reported no
unresolved P0, P1, or P2. The final candidate `f2a10b846466d21ce9371a1932f16cd6844fdf08`
adds only a markdown code-fence rendering fix in the design doc; no behavior
changed after review.

- Commits `e893220` (propose), `569baa1` (characterize), `e1f244f`
  (consolidate), `9a0ab0f` (L3 header), `f7ba597` (review comment),
  `8340384` (revert acceptance), `d034183` (fix round-1 findings),
  `54641b3` (revert acceptance), `9805613` (sync contract descriptions), and
  `f2a10b8` (fence fix) form the milestone on branch
  `feat/query-access-proof-orchestration-issue-15-20260816`.
- Characterization evidence: the contract tests demonstrate RED when proof
  runs before requirements (exact COUNT(1) case), when a barrier is allowed
  through, when the wrong reason is removed (kept-set witness), or when a
  dialect's proof applicability changes; every mutation was restored
  byte-for-byte.
- Equivalence evidence: default (958) and `postgresql`-tagged (1268)
  queryaccess suites, default and tagged full tests, affected race tests,
  Query Access corpus, `pg-unit-test-gates`, `pg-confidence-gates`
  (Docker-backed CLI/HTTP/MCP E2E), recording/live fixtures, builds, dual-tag
  vet, lint, decision-record, gofmt, three-level-doc, and tidy gates pass.
- No public SDK/CLI/HTTP/MCP, SQL support, manifest, fixture, workflow,
  version, or release surface changed; issue #15 remains open pending
  separate merge/push authorization.