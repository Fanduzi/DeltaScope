# Decision: SERVER ordered channel — sourced OrderedAbsent and the provable-distinct invalidation domain

Date: 2026-11-13
Status: Accepted
Related milestone/version: issue #105 / milestone T26 (T26-SERVER-ORDERED-ABSENCE-01)
Related commits: this commit (product + tests)
Related tests: internal/application/audit/server_ordered_absence_exec_test.go; updated T26 SERVER premise/vertical witnesses
Related docs: T26-SERVER-ORDERED-ABSENCE-01 Phase 0 contract S1–S12 (frozen)

## Context

The SERVER ordered channel (introduced by T26-SERVER-DROP-EXECUTION-01 and extended by the CREATE/ALTER executions) published `OrderedAbsent` without source provenance and treated the identity relation `R(A,B)` conservatively: provably-same or unproven, with the provably-distinct domain empty. The frozen T26-SERVER-ORDERED-ABSENCE-01 contract changes two things: S1 requires `OrderedAbsent` to carry the publishing statement's batch index and conditional provenance, and S6/S7 require writes on provably-distinct identities to preserve unrelated ordered answers while writes of unproven relation must still retire them.

## Decision

- `spec.ServerState` gains `OrderedAbsentSourceSet`/`OrderedAbsentSourceIndex` (nonserialized): the batch statement index of the statement whose publish arm established the absence. `batchServerEntry` mirrors it as `publishedIndex`/`publishedIndexSet`; the drop publish arm records `s.applyStmtIndex`, and `bindMySQLServer` projects it onto `Metadata.Server` so terminal-refusal and guarded arms answer with their source.
- `serverRel` gains a provable-distinct domain: fold-differing space-clean ASCII pairs are provably distinct (PAD SPACE collations govern trailing pads only, so a leading-space difference is also distinct). Trailing-space pairs and any non-ASCII involvement stay unproven; byte-equal and fold-equal stay same.
- All write effects route through one helper pair: `serverInvalidateUnknown` (barrier the writer's own bucket, retire every non-distinct entry) and `serverInvalidateOthers` (retire all but the publishing entry). Publish arms (proven drop, proven create, proven alter) invalidate non-distinct *other* entries; every non-publish drop/create arm takes the unknown-effect invalidation. ALTER's non-publish arms remain a strict no-op — ALTER is presence-preserving by contract.
- The unaudited-write path keeps literal-name writes as whole-channel taint and records named writes for the relation scan.

## Rationale

With provably-distinct empty, `CREATE s2` after `DROP s1` erased s1's proven answer even though the two names provably denote different catalog rows — forcing needless `batch_effects` barriers and resolver reads the frozen contract says must not happen. Extending only the ASCII space-clean fold-differing domain is the smallest enlargement that satisfies S7 without assuming any catalog collation beyond PAD-SPACE semantics; every identity the product cannot prove distinct still shadows and invalidates, preserving the evidence-first invariant that unknown relations never yield stale ordered answers.

## Public Contract

- A drop consuming `OrderedAbsent` (terminal refusal or `IF EXISTS` guarded arm) behaves identically in findings/gaps/coverage — the source index is internal provenance, never serialized.
- Statements on provably-distinct ASCII identities now issue their own catalog reads where they previously read `blocked_batch_effects`; `server.batch_effects` still appears exactly when a relation stays unproven.
- Names remain verbatim-keyed internally; the objectCache never merges spellings.

## Deferred / Out Of Scope

- Generic SERVER lifecycle beyond the audited drop/create/alter arms, parser grammar changes, SRS ordered-publication changes, and any generic batch-framework change — all excluded by the work order.
- Trailing-space and non-ASCII canonicalisation beyond the unproven boundary — no catalog collation is consulted at apply time.

## Verification Evidence

- `server_ordered_absence_exec_test.go`: S1 source-index consumers (refusal + guarded arms cite index 0), S6 invalidation matrix (same-key invalidate, literal taint, distinct-key preserve, non-ASCII channel retire), S7/S8 `serverRel` domain table.
- Updated existing witnesses: `batch_state_t26_test.go` (provably-distinct lookups), `server_alter_premise_test.go`/`server_alter_r2_test.go` (unproven-relation write retires ordered presence), `server_create_premise_test.go` (distinct write preserves ordered present), `pkg/deltascope` vertical tests (AM-7.4 recounted; relation-boundary chain moved to the genuinely unprovable s1/`sé` pair).
- `go test ./...`, `go vet ./...`, `gofmt` clean on touched files.

## Consequences

- Any future SERVER arm that publishes ordered state must keep `publishedIndex`/`publishedIndexSet` honest, and any future write effect must route through the relation-based invalidation helpers rather than minting barriers ad hoc.
- Tests that froze counts under the empty-distinct contract may legitimately need recounting only where the relation is provably distinct; unproven-relation expectations must never silently relax.

## Links

- Commits: this commit
- Contract: T26-SERVER-ORDERED-ABSENCE-01 Phase 0 S1–S12 (frozen), evidence archive `t26-server-ordered-absence-phase0-r2` lineage
