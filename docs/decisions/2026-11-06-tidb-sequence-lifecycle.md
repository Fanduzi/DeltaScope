# Decision: TiDB 8.5 sequence lifecycle — typed definitions, ordered state, and the runtime-issuance boundary (A1/A2 split)

Date: 2026-11-06
Status: Accepted
Related milestone/version: mysql-tidb-ddl-completion (issue #79, task T21 under #100)
Related commits: uncommitted A1 patch on the milestone branch after `b33a83ec`
Related tests: `internal/application/audit` T21 suite
(`batch_state_t21_test.go`), corpus fixture
`testdata/sql-corpus/tidb/ddl/findings/sequence_lifecycle_rules.*`,
manifest `testdata/ddl-golden/T21.json` (authored, not executed)
Related docs: module README updates in the same change
(`internal/domain/spec`, `internal/application/audit`,
`internal/domain/rule/ddl`), `testdata/ddl-inventory` sequence rows,
`configs/deltascope.example.yaml`

## Context

Issue #100 owns the TiDB sequence surface: `CREATE | ALTER | DROP
SEQUENCE`, existence guards, the parameter family
(START/INCREMENT/MINVALUE/MAXVALUE/CACHE/CYCLE/COMMENT), `RESTART
[WITH]`, shared table-namespace identity, multi-target DROP, bounded
inbound references, and — explicitly — the runtime-issuance boundary
(current value, next value, cache consumption, persisted progress,
database restart recovery).

The frozen Planner decision splits delivery into A1 (static lifecycle +
ordered state) and A2 (runtime and persistence semantics). Prior to A1,
sequence statements parsed but options were unaudited aspects
(`create_sequence.options`, `alter_sequence.options`,
`create_sequence.option.comment`, `drop_sequence.unaudited_targets`) and
no ordered state existed for the object kind.

## Decision

- Sequence options are typed authored declarations
  (`spec.SequenceOption` in declaration order) that project into a
  nullable catalog `SequenceDefinition` — absent members mean
  unproven, never implicitly zero. `RESTART`/`RESTART WITH` are marked
  as issuance requests and are excluded from definition merges.
- A dedicated ordered registry (`batch_state_sequence.go`) binds
  sequence identities through the shared once-per-request object cache,
  under the same schema-scope gates as tables: a proven-dead schema
  proves members absent; unvouched/invalidated scopes tombstone. A
  proven non-sequence object at the same name is a shared-namespace
  conflict (TiDB sequences share the table namespace).
- Transitions are refusal-preserving and per-target: guards absorb the
  proven contradiction (`IF [NOT] EXISTS`), a proven duplicate/absent
  target blocks without publishing post-state, and a multi-target DROP
  adjudicates member by member — no all-or-nothing rollback is assumed.
- Ordered state runs ahead of provider snapshots inside a batch:
  batch-created and batch-dropped identities bind on later statements.
- Four TiDB-only rules are wired: `ddl.tidb.sequence.valid` (blocker,
  existence contracts + state/runtime premise gaps),
  `ddl.tidb.sequence.restart.warn` (warning + owns the
  `sequence.runtime_state` gap on live restart),
  `ddl.tidb.sequence.cycle.warn` (declared or catalog-proven
  enablement), `ddl.tidb.sequence.references.warn` (vouched
  inbound-reference set only).
- Unvouched identity yields `unknown_sequence_state` gaps — an empty or
  permission-filtered catalog read is never laundered into proven
  absent, and resolver errors propagate fail-loud and uncached.

## Rationale

- Modeling typed options retires the `has_options` evidence-marker
  family for sequences while keeping the boundary honest: COMMENT is a
  bounded presence marker (text never retained), and unrecognized
  shared table-option tail members still tombstone the touched
  identities as unbounded aspects.
- Keeping runtime facts out of the definition model is what makes the
  RESTART path auditable at all: the rule can warn about reuse risk and
  charge exactly one bounded evidence gap without implying a
  current/next value exists, which is the frozen first-path contract
  (1 warning + 1 gap, review/unverified, unsupported 0).
- Per-target DROP adjudication mirrors the TiDB source semantics
  documented in the P-AB research — partial member effects are
  provable, so the model must not assume statement-level atomicity.

## Public Contract

- `spec.DDL.Sequence`, `spec.ObjectSnapshot.Sequence` /
  `SequenceReferences`, and `spec.Metadata.Sequences` are internal
  evidence (JSON-hidden); statement metadata now carries a
  `Sequences` pre-state projection keyed `schema.name`.
- Findings carry bounded identities (schema, sequence, operation,
  reference identity strings) — option values and COMMENT text never
  appear.
- Coverage: modeled sequence options no longer emit
  `create_sequence.options` / `alter_sequence.options` /
  `create_sequence.option.comment` / `drop_sequence.unaudited_targets`
  unsupported aspects; evidence gaps lower complete coverage to
  unverified per the existing contract.

## Deferred / Out Of Scope

- A2: runtime allocator, cache consumption, current/next value,
  persisted progress, database process restart recovery. `CACHE` is a
  static definition parameter here — its consumption is A2.
- NATIVE-PENDING branches (require the isolated TiDB 8.5 environment
  and separate resource authorization): `ALTER SEQUENCE ... IF EXISTS`
  acceptance, out-of-range/extreme `RESTART WITH` verdicts, native
  multi-target partial-effect oracle, `RENAME TABLE` sequence moves,
  drop-with-references behavior, `information_schema.SEQUENCES`
  permission filtering. `testdata/ddl-golden/T21.json` encodes these as
  `pending_native` probes — authored only, never executed, never a
  native PASS claim.
- Permission-filtered catalogs: an unvouched read stays unknown; proven
  absent requires the formally approved sufficient-proof condition.

## Verification Evidence

- Frozen first path red→green through the public `AuditSQL` path:
  `ALTER SEQUENCE seq RESTART WITH 1` under an isolated policy +
  provider fixture yields exactly 1 `ddl.tidb.sequence.restart.warn`
  finding and 1 `sequence.runtime_state` gap.
- Permanent matrix in `batch_state_t21_test.go`: guards, cycle,
  references, resolver-error propagation, ordered-ahead-of-provider,
  multi-target partial effects, unknown-state gap, dropped-schema
  member death, disabled-rule silence, namespace conflict.
- Corpus coverage inventory: 734/734 supported rule-dialect targets
  with the four new rules TiDB-only; legacy option-gap fixtures
  reconciled to the modeled contract.

## T21-R1 Amendment (2026-11-07)

Independent offline review of the A1 patch identified three defects
against the frozen contract. This amendment records the directed repair
without reopening the A1 scope.

### What changed

- **B1 — outcome-consistent legacy notices.** The three stable notice
  IDs (`ddl.create_sequence.notice`, `ddl.alter_sequence.notice`,
  `ddl.drop_sequence.notice`) previously claimed successful
  creation/modification/deletion unconditionally. They now consume the
  same ordered `SequenceState` the valid rule adjudicates: proven
  refusals and guard no-ops suppress the success claim per bound target,
  applicable and unproven targets keep bounded declarative-intent
  wording, multi-target statements adjudicate per member, and dialects
  with no sequence bindings keep the legacy generic path. Policy
  blockers are never consulted — the adjudicator reads ordered outcome,
  not verdict.
- **B2 — structured inbound references.** `spec.SequenceRefEdge`
  (owner-kind, owner schema/name, slot → target) replaces the opaque
  string form. `references.warn` emits one warning per proven edge
  (provider-vouched or derived from live column defaults through
  `ExpressionFacts.SequenceRefs`, which the expression-facts visitor
  collects from `ast.TableNameExpr`), and owes a
  `sequence.references` / `unknown_sequence_references` evidence gap
  whenever set completeness is unvouched or batch-diverged — positive
  edges and the gap coexist. Proven-empty sets stay silent. Owner
  invalidation rides the ordered-state machinery: proven-absent owners
  retire their edges cleanly, undecidable owners remove the edge and
  diverge completeness, and a deferred reconcile covers every
  create/alter-table exit including unsupported early returns.
- **B3 — official sequence reader.** `Provider.ResolveObject` dispatches
  TiDB `sequence` requests to `loadSequenceObject`, reading
  `information_schema.SEQUENCES` definition columns. Nullable members
  stay unproven, zero rows stay unknown (privilege-limited visibility is
  never proven absence), query/scan errors propagate uncached, and no
  runtime issuance position is queried; inbound-reference completeness
  remains explicitly unvouched (`SequenceReferencesUnknown`).

### Verification evidence

- Byte-exact A1 reconstruction (baseline `b33a83ec` + frozen tracked
  diff SHA-256 `5345c80d…2930`, untracked files recovered from session
  tool records hash-verified against the manifest): the new regression
  suite fails on the rejected implementation — 13 red assertions across
  B1 (notice success claims on proven refusals, per-member adjudication)
  B2 (missing completeness gap, stale positive edges) and B3
  (unavailable reader at unit and public-AuditSQL levels).
- On the repaired tree: 18 `TestT21*` functions / 31 subtests PASS in
  `internal/application/audit`; 6 `TestT21R1*` PASS in
  `internal/infrastructure/metadata/mysql`; 4 `TestT21R1*` PASS in
  `internal/application/auditmeta` (real provider over a stub driver
  through public `AuditSQL`). Zero FAIL/SKIP; affected-package suite
  green.
- Native branches remain NATIVE-PENDING exactly as listed under
  Deferred / Out Of Scope; `pending_native` manifest probes stay
  unexecuted and unclaimed.
