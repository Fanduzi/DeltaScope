# Decision: Assign Query Access Tests to One Evidence Owner

- Date: 2026-08-15
- Status: Accepted
- Related: [Unified online analysis entry](2026-08-12-query-access-online-analysis-entry.md), [Dialect session API deprecation](2026-08-14-query-access-dialect-session-api-deprecation.md), [PG17 online surface contract](2026-08-03-query-access-pg17-count-online-surface-contract.md), [test consolidation issue #4](https://github.com/Fanduzi/DeltaScope/issues/4)
- Spec: `docs/plans/2026-08-15-query-access-test-ownership-consolidation-spec.md`
- Design: `docs/plans/2026-08-15-query-access-test-ownership-consolidation-design.md`
- Implementation: `docs/plans/2026-08-15-query-access-test-ownership-consolidation-implementation.md`

## Context

The unified online Query Access SDK entry now owns product-to-proof routing, and
CLI and HTTP delegate to it. Migration milestones intentionally retained the
old SDK, CLI, and HTTP behavior matrices to prove equivalence. The resulting
suite repeats many products, profiles, SQL shapes, admissions, requirements,
and reasons across surfaces.

Some repetition is essential. CLI and HTTP expose different lifecycle, error,
authorization, logging, and privacy boundaries. PostgreSQL foreign-table
rejection is a trust boundary that Accepted decisions require on every online
surface. Deprecated APIs also retain a source and runtime compatibility
contract across all supported targets.

The decision is therefore not simply to delete similar SQL tests. It is to
assign every behavior to the narrowest owner that can observe it and require
replacement evidence before deletion.

## Proposed Decision

Make the unified SDK entry the sole owner of exhaustive product, version,
profile, SQL-shape, admission, requirement, reason, proof-failure, and detailed
recording-driver matrices.

Keep transport tests only for their external contracts plus minimal real
routing smoke. CLI owns flags, session/TLS lifecycle, exit codes, stdout/stderr,
bounded failures, and CLI no-leak. HTTP owns request/status/body, registry,
`connection_id`, authorization/purpose, zero-dial guards, lifecycle, access
logs, and HTTP no-leak. Both retain MySQL 8.4, TiDB 8.5, and PG17 live smoke;
PG17 also retains syntax-envelope, foreign-table, and offline/default negatives.

Keep one focused CLI and one focused HTTP recording test for adapter-level
delegation, no-execution, close, and error mapping. The SDK retains complete
probe sequencing.

Reduce deprecated API tests to the minimum compatibility contract, including
one unified-versus-old equivalence case for MySQL 5.7, 8.0, 8.4, TiDB 8.5, and
PostgreSQL 17. Keep old errors, validation priority, ownership, deprecation, and
tagged/untagged stub evidence.

Require a deletion ledger mapping every removed test to retained semantic and,
when applicable, transport/compatibility evidence. Use five temporary mutation
probes to demonstrate that retained owners detect representative regressions.
Add no shared test framework or permanent mutation/checker machinery.

## Rationale

The unified entry is a real behavioral seam. Exhaustive semantic coverage there
gives new capabilities one test owner. Transport suites remain valuable where
they observe behavior the SDK cannot see, not where they repeat parser and proof
decisions.

A ledger prevents superficial deduplication from orphaning evidence. Temporary
mutation probes demonstrate sensitivity more directly than line coverage or a
deletion percentage. Keeping those probes uncommitted avoids introducing a new
framework for a one-time migration.

Preserving ADR-cited file paths where possible keeps historical evidence
navigable. Follow-up notes are required when paths must move, but prior
acceptance history is not rewritten.

## Contract

- Unified SDK owns exhaustive semantics for every supported target.
- CLI and HTTP retain their external lifecycle, error, privacy, and real-route
  evidence.
- PostgreSQL foreign-table fail-closed remains proven on SDK, CLI, and HTTP.
- Deprecated APIs retain minimum compatibility evidence for all five supported
  identity targets.
- MCP continues to expose no Query Access tool.
- Every deletion has a ledger mapping and no observable contract is orphaned.
- Required gates, fixtures, production behavior, public APIs, versions, and
  release workflows remain unchanged.
- No line-count or runtime target may justify weaker evidence.

## Consequences

Positive consequences:

- New semantic capability work updates one exhaustive matrix.
- CLI and HTTP suites describe their actual responsibilities more clearly.
- Repeated code and Docker work decrease without reducing supported behavior.
- Deprecated API compatibility remains explicit rather than relying on broad
  accidental coverage.

Costs and limitations:

- Transport privacy and lifecycle tests remain intentionally repetitive.
- The cleanup requires a detailed ledger, full gates, and temporary mutation
  proof before any deletion is accepted.
- Old APIs still require one case per supported target until a separate
  breaking-change decision removes them.

## Alternatives Rejected

- Keep all matrices: rejected because migration equivalence is established and
  ongoing duplication multiplies maintenance.
- Generate one cross-package matrix: rejected because it adds coupling and a
  test framework without creating clearer ownership.
- Keep SDK tests only: rejected because SDK tests cannot observe transport
  authorization, sinks, logs, lifecycle, or external errors.
- Keep one generic transport negative: rejected because SQL syntax and
  relation-kind trust are distinct boundaries.
- Keep one old-API target: rejected because target/profile mapping remains part
  of the deprecated runtime contract.
- Gate on coverage or deleted lines: rejected because those metrics do not
  prove fail-closed, privacy, or ownership behavior.
- Add a permanent duplicate-test checker: rejected because test names and SQL
  text are brittle policy inputs.

## Deferred Scope

- Production refactors, public API changes, new Query Access capabilities, and
  removal of deprecated APIs.
- New test framework, matrix generator, static checker, or mutation dependency.
- Makefile/workflow/fixture/gate changes.
- Version bump, release notes, tag, release, or publication.

## Acceptance Evidence

### Issue #13 HTTP evidence note

Issue #13 removes only the ledger-authorized HTTP MySQL/TiDB online
product/profile/shape matrix rows and the HTTP adapter's fixed PG17 probe
assertions. It retains real MySQL 8.4/TiDB 8.5 admitted and fail-closed HTTP
smoke, PG17 syntax-envelope and foreign-table evidence, offline/default,
registry/authorization-before-dial, lifecycle, bounded errors, synchronized
request-ID access logs, and response/log no-leak tests. The unified SDK remains
the detailed semantic and probe-sequence owner. A temporary authorization
bypass made `TestHandlerQueryAccessOnlineGuardPathsOpenNothing` fail before the
original guard was restored; the mutation is not committed.

### Final reconciliation evidence

The complete row-by-row deletion ledger, post-consolidation inventory, and
measured gate evidence are recorded in the implementation plan's Issue #14
final reconciliation. Each required temporary mutation made its retained owner
RED, then was restored byte-clean without committing mutation code. The full
required default/tagged/race/corpus/Docker/TLS/build/vet/lint/npm/docs/decision/
gofmt/tidy/diff matrix passed.

The independent Standards and Spec/security reviews examined the fixed review
candidate `51e548ecf234e8ecf84cbb81dfcddd19650c9dd8` against the fixed milestone
range `db4e73a19233d0475a480f2f333784d85f2d616a...51e548ecf234e8ecf84cbb81dfcddd19650c9dd8`.
They found no unresolved P0, P1, or P2 and confirmed retained semantic,
compatibility, authorization, privacy, lifecycle, foreign-table,
offline/default, and MCP-absence evidence; production behavior and restricted
delivery surfaces remain unchanged.
