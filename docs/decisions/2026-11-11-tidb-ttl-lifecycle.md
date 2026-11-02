# Decision: TiDB 8.5 TTL declarations, prospective state, and the shared checkTTLInfoValid outcome (MySQL/TiDB)

Date: 2026-11-11 — milestone decision-record convention: decision dates
are assigned per task slot and run ahead of commit dates; neighboring
records follow the same convention.
Status: Accepted
Related milestone/version: mysql-tidb-ddl-completion (issue #79, task T19 under #98)
Related commits: this task's commits on the milestone line after `4ef016df`
Related tests: `internal/application/audit` T19 suites
(`batch_state_t19_test.go`, `batch_state_t19_state_test.go`),
`internal/infrastructure/parser/tidb/ttl_test.go` plus
`ttl_ast_probe_test.go`, `internal/infrastructure/metadata/mysql/provider_ttl_test.go`,
`internal/domain/spec` TTL suites, `pkg/deltascope/audit_t19_test.go`,
corpus fixture `testdata/sql-corpus/tidb/ddl/findings/alter_table_ttl.*`,
and `make ddl-golden TASK=T19 ARTIFACT_DIR=...` against
`testdata/ddl-golden/T19.json`
Related docs: module README updates in the same change
(`internal/domain/spec`, `internal/application/audit`,
`internal/domain/rule/ddl`, `internal/infrastructure/parser/tidb`,
`internal/infrastructure/metadata/mysql`, `pkg/deltascope`),
`testdata/ddl-inventory/inventory.yaml`, coverage catalog
`docs/reference/ddl-coverage.*` (TiDB TTL row reports
`normalized_silent` under the no-provider census; the warning path is
metadata-backed)

## Context

Issue #98 owns the TiDB 8.5 TTL surface: `TTL=<col> + INTERVAL <n> <unit>`,
`TTL_ENABLE='ON|OFF'`, `TTL_JOB_INTERVAL='<text>'`, and `REMOVE TTL` on
CREATE and ALTER, plus the dedicated `ddl.tidb.ttl.warn` finding. TiDB
executes these options inside `/*T![ttl] ... */` feature comments in
`SHOW CREATE` output and evaluates a strict prerequisite order
(`checkTTLInfoValid`: temporary table → interval expression →
common-handle primary key → inbound foreign keys → time column) plus the
`checkTableForeignKey` parent restriction (no new edge may reference a
TTL-carrying parent).

Two hard boundaries shaped the design:

- The PingCAP parser restores the TTL interval through `ValueExpr`, so a
  quoted literal keeps its quotes (and a charset introducer) while an
  expression shape is normalized text. Modeling only the stripped value
  would conflate `'abc'`, `'1:3'`, `NULL`, and `1` — forms TiDB
  evaluates differently.
- A TTL-bearing table can be a *foreign-key parent*. The reverse-edge
  set is a catalog fact the provider never loaded before this change,
  and MySQL has no such concept — a single `ReferencedBy` fact had to
  carry "never read" separately from "read and empty".

## Decision

1. **`spec.TTLDeclaration` carries the authored clause separately from
   state.** Each member records its `Member` name, `Source`
   (`native`/`feature_comment`/`unproven` — attributed lexically so
   `ttl`-spelling identifiers can never be misread as clause syntax),
   the normalized `TimeColumn`/`IntervalValue`/`IntervalUnit` display
   facts, the *restored* `IntervalExpr` used by the evaluability gate,
   the `Enabled` pointer (nil = unobserved), `JobInterval`, and bounded
   `Unmodeled` member evidence. Duplicate/conflicting members are
   unproven; unmodeled members are unproven; MySQL native TTL is a
   proven refusal while feature-comment members stay opaque comments.

2. **One shared tri-state adjudicator models `checkTTLInfoValid`.**
   `spec.TTLDefinitionLandingOutcome` applies TiDB's exact check order
   against a `TableSnapshot`; `spec.TTLCreateOutcome` wraps it for the
   CREATE side — a brand-new destination has a proven-empty inbound set,
   LIKE reuses the bound source's member facts, and CTAS's select tail
   keeps the time column unprovable. `spec.TTLForeignKeyParentOutcome`
   applies the parent restriction: non-TiDB applies unconditionally, an
   unvouched parent TTL fact is unproven, a proven TTL parent refuses,
   a proven no-TTL parent applies. Ordered state and `ddl.tidb.ttl.warn`
   consume the *same* outcomes, so a refused or unproven definition can
   never publish a config and narrate "enables deletion" at once.

3. **The interval gate classifies evaluability, not just presence.**
   Numeric literals with a single-component unit apply; quoted literals
   are unquoted through the restored text and then validated per unit
   class (invalid content is a proven refusal); non-literal or
   unmodeled expression shapes stay unproven.

4. **`ReferencedBy` carries three states through two fields.** `nil`
   means the reverse-edge read never ran (resolver then marks
   `ReferencedByUnknown`); a non-nil empty slice is the catalog's proven
   "no inbound edges" answer; a populated slice is a proven refusal
   input. The MySQL provider never issues the read; the TiDB provider
   runs a TiDB-only `KEY_COLUMN_USAGE` reverse-edge query whose failure
   propagates as a provider error and is never cached. Batch-derived
   edges merge into the same fact through `recordInboundEdge`.

5. **State transitions mirror TiDB atomicity.** An applied definition
   publishes a deep-owned `TTLConfig` (independently known
   `Enabled`/`JobInterval` members survive definition-member churn); a
   proven refusal preserves the entire pre-state including proven
   no-TTL; an unproven definition withholds only the members it cannot
   prove and otherwise clears `TTLKnown`. `REMOVE TTL` publishes proven
   no-TTL even on partial projections while leaving unrelated unknown
   collections unvouched. `TTLConfig.Enabled` deep-copies on clone so
   successor snapshots never alias provider memory.

## Rationale

- The shared domain outcome is the only seam where findings and ordered
  state can meet; duplicating prerequisite logic between
  `createDestShape`, `applyTTLDefinition`, and the warning rule was the
  exact divergence the milestone prohibits.
- `nil`-vs-empty on `ReferencedBy` follows the established
  provider-blindness contract: a provider that never ran a query must
  not be reinterpreted as proving an empty set.
- Restored-expression classification (rather than raw AST or stripped
  value) is the narrowest fact that matches TiDB's `EvalExpireTime`
  boundary without claiming arbitrary expression evaluation.

## Public Contract

- `ddl.tidb.ttl.warn` fires only for TiDB statements carrying modeled
  TTL members, and only when the shared adjudicator answers
  *applied with enabled=true*. Proven refusals are silent; unproven
  prerequisites emit a `target_table.ttl` (or member-specific
  `target_table.ttl_enable`) evidence gap instead.
- `TableSnapshot.TTLKnown` distinguishes a successful catalog/ordered
  read from an unobserved configuration; `TTLConfig.Enabled==nil` is
  unobserved, never "off".
- `TableSnapshot.ReferencedBy==nil` is "reverse-edge set never loaded",
  never "proven empty"; `ReferencedByUnknown` marks the unvouched flag
  on bound projections.
- `REMOVE TTL` emits no finding and publishes `TTLKnown=true, TTL=nil`
  where the table state is otherwise known.
- Offline (no provider) CREATE TTL still warns when the declared shape
  is self-sufficient — the pseudo-shape's prerequisites come from the
  declaration itself — while offline ALTER TTL owes the
  `target_table.ttl` gap.

## Deferred / Out Of Scope

- `CREATE TABLE ... LIKE ... TTL=...` is unparseable in TiDB's grammar
  and `CREATE TABLE ... AS SELECT` (CTAS) remains outside the supported
  statement surface — both carry ready-but-unreachable branches in the
  shared outcome so no re-modeling is needed if either graduates.
- TTL job scheduling/cadence introspection (`TTL_JOBS` catalog reads)
  and per-row expiration metrics are not modeled.
- Interval expressions beyond literal-evaluable forms stay unproven by
  design; widening them requires a pinned TiDB expression-evaluation
  evidence basis first.

## Verification Evidence

- Ordered-state matrices (`TestT19CreateTTLRespectsCheckTTLInfoValid`,
  `TestT19ForeignKeyIntoTTLParent`, `TestT19CreateLikeAndSelectTTL`,
  `TestT19TTLDefinition*`) cover applied/refused/unproven per
  prerequisite including self-references, LIKE/CTAS boundaries, partial
  projections, and pointer-aliasing.
- Rule-parity suite `TestT19TTLRuleParity` pins one warn on applied,
  silence on refused, and exactly-one `target_table.ttl` gap on
  unproven for both CREATE and ALTER.
- Parser suites pin lexical source attribution, restored-expression
  preservation, member values, and the PingCAP AST shape the extractor
  depends on.
- Provider suites pin `SHOW CREATE` TTL scalar parsing (unknown
  `TTL_ENABLE` never fabricates OFF), the TiDB-only reverse-edge query,
  error propagation, and no failed-read caching.
- Corpus fixture `alter_table_ttl` proves the metadata-backed warning
  path; the offline census row honestly reports `normalized_silent`.
- `make ddl-golden TASK=T19` replays the whole contract against real
  MySQL 5.7.44/8.0.46/8.4.10 and TiDB v8.5.0 anchors.

## Consequences

- Any future shape that reaches `TableSnapshot` with a nil
  `ReferencedBy` is unvouched for TTL purposes; providers that add
  reverse-edge reads must publish non-nil sets only on success.
- Widening interval/expression support or surfacing CTAS TTL must go
  through the shared outcomes — not local rule shortcuts — or the
  finding/state parity guarantee dissolves.
- The corpus `referenced_by` fixture field now exists for every module
  that converts fixture snapshots.

## Links

- Issue: <https://github.com/Fanduzi/DeltaScope/issues/98>
- Tests: `batch_state_t19*_test.go`, `ttl_test.go`,
  `ttl_ast_probe_test.go`, `provider_ttl_test.go`, `audit_t19_test.go`
- Golden manifest: `testdata/ddl-golden/T19.json`
- Corpus: `testdata/sql-corpus/tidb/ddl/findings/alter_table_ttl.*`
