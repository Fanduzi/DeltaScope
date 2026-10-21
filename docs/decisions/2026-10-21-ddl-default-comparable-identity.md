# Decision: Comparable literal identity for default-value transitions (T08-R1)

Date: 2026-10-21
Status: Accepted
Related milestone/version: mysql-tidb-ddl-completion (issue #79, task T08-R1 under #87; amends `docs/work-plans/t08-column-governance/CONTRACT.md` §5.4)
Related commits: this task's commits on milestone/mysql-tidb-ddl-completion
Related tests: `internal/application/auditmeta/audit_t08r1_default_comparison_test.go`, `internal/infrastructure/metadata/mysql/provider_t08r1_test.go`, `internal/infrastructure/parser/tidb/extractor_t08r1_test.go`, `internal/domain/rule/ddl/alter_compatibility_rules_test.go`
Related docs: `docs/work-plans/t08-column-governance/CONTRACT.md` §5.4-R1, `docs/decisions/2026-10-28-ddl-column-position-and-multi-action-state.md`, module READMEs of the touched packages

## Context

T08's frozen contract gave `modify_column`/`change_column` compatibility
rules an effective-default transition, but the shipped comparison read
`source.DefaultValue` against `target.DefaultValue` — the *display* field.
The two sides shape that text differently: the declaration side renders
the literal with its SQL quote layer (`'x'`), while the provider side
carries verbatim `COLUMN_DEFAULT` bytes (`x`). Two defects followed:

- **False transitions.** A provider-sourced column and a declaration
  target holding the *same* stored literal (`x` vs `'x'`) produced a
  determined `changes its default value` finding.
- **False negatives by collision.** Conversely, a stored `'x'` (literal
  quote content) could compare equal to a declared `'x'` display text.
- **Unknown convicted as changed.** An expression default whose
  fingerprint was missing or empty fell into the same `!=` text compare
  and surfaced as a determined transition.

A second asymmetry hid underneath: a `CREATE`-derived pre-state and a
provider-sourced pre-state holding the same logical default judged
differently, because the declaration side had no representation-neutral
identity to compare.

## Decision

1. `spec.Column` gains `DefaultLiteral *LiteralDefaultFacts` (internal,
   `json:"-"`): `Kind` ∈ `{"string","numeric"}` plus `Value`, the decoded
   literal value bytes. `nil` means the identity is unproven or the
   default is not literal-class; an empty declared string is a proven
   `""`, never the unknown sentinel.
2. Sources publish the same identity contract: declaration side decodes
   the typed AST (`'x'`→`x`, `'''x'''`→`'x'`, `'O''Reilly'`→`O'Reilly`,
   integers including a unary `±` to canonical digits); provider side
   takes catalog bytes verbatim and derives `Kind` from the column's own
   base type family (parameters/attributes stripped), never from content
   guesses.
3. `CURRENT_TIMESTAMP`-looking bytes are the temporal marker only on
   temporal base types (`timestamp`/`datetime`, reduced through
   `datetime(6)`); on any other column they stay the string literal.
4. One shared three-state comparison (`defaultComparison`) drives both
   `Evaluate` and `EvidenceGaps`: presence flips and class switches stay
   determined; same-class literals compare kind+value; same-class
   expressions compare `Known` non-empty fingerprints; everything else is
   unproven — never a transition finding.
5. Unproven splits honestly in the gap vocabulary: a side that cannot
   produce its own identity reports `incomplete_source_column` /
   `source_column.default` (source) or `unproven_default_identity` /
   `target_column.default` (target); a proven-but-cross-domain pair
   reports `unproven_default_identity` on both facts — the comparison,
   not the facts, is unproven.
6. Ordered state carries the fact exactly like `DefaultExpression`: SET
   DEFAULT publishes the declared identity, DROP DEFAULT clears it,
   temporal publication clears it, MODIFY/CHANGE publish the full new
   definition, and every `cloneColumn`/`cloneSpecColumn` path deep-copies
   the pointer.

## Rationale

- `DefaultValue` cannot be the identity: it is source-shaped display text
  whose two representations (quoted declaration render, decoded catalog
  bytes) are legitimately different for the same value — and, worse, can
  *collide* for different values. Any quote-stripping repair on that text
  would reintroduce collisions (`'x'` content vs `'x'` quoting).
- The typed AST and the catalog bytes are the two places the literal's
  decoded value is already provable; the fix belongs at the fact sources,
  not at a post-hoc text repair.
- Kinds never coerce: a `numeric` stored default and a `string` declared
  default are unproven, not equal — modeling coercion would mean
  simulating server conversion semantics, which is exactly the sweep this
  round refuses.
- An empty or unknown fingerprint is an unknown fact, not a difference —
  the same knowledge boundary the T07 evaluation contract already keeps.

## Public Contract

Consumers see no new fields — `DefaultLiteral` is `json:"-"` and findings
keep class-only metadata. Behavior changes in exactly one direction:
proven-same defaults across source representations no longer report a
transition, proven-different ones keep reporting it, and unproven
identities now emit a bounded `unproven_default_identity` /
`target_column.default` evidence gap where a determined finding or a
silent pass would have stood before.

## Deferred / Out Of Scope

- Decimal/float/bit/hex/byte literal forms carry nil identity — unproven,
  not converted. Building a database-literal coercion simulator is
  explicitly out of scope.
- No expression evaluation, no SQL-equivalence proof, no collation or
  `sql_mode` simulation, no provenance framework: `DefaultLiteral` is the
  only new fact and it answers one comparison.
- The catalog cannot recover whether `DEFAULT NULL` was authored — the
  SQL NULL vs omitted-clause ambiguity stays exactly as T06-A4 froze it.

## Verification Evidence

- Real-entry red then green: `audit_t08r1_default_comparison_test.go`
  drives the real parser, real `mysqlmeta.Provider`, and `AuditSQL`
  through provider-sourced and CREATE-derived pre-states, both dialects,
  both MODIFY and CHANGE — same-value cells emit zero findings, different
  and unproven cells keep their determined and bounded outcomes.
- Provider unit proof: `provider_t08r1_test.go` pins decoded-byte
  identity per base-type family including parameterized faithful types
  and the temporal gate.
- Parser unit proof: `extractor_t08r1_test.go` pins AST-decoded values
  and nil for unprovable shapes.
- Rule unit proof: `alter_compatibility_rules_test.go` pins the
  three-state transition, the cross-domain pair gap, and the
  empty-fingerprint unproven cell (the pre-R1 wrong expectation,
  upgraded with its reason recorded).
- State chains: SET→MODIFY, DROP→MODIFY, CHANGE-rename→MODIFY prove the
  published/cleared/carried identity end to end.

## Consequences

- Any future default-comparison surface must compare `DefaultLiteral`
  (and `DefaultExpression` fingerprints), never `DefaultValue`.
- A new literal form earns `DefaultLiteral` only when a cross-source
  identity is provable; otherwise it keeps the nil/unproven contract.
- `pkg/deltascope` cloning, `batch_state` cloning, and any new
  publication path must deep-copy `DefaultLiteral` alongside
  `DefaultExpression`.

## Links

- Contract: `docs/work-plans/t08-column-governance/CONTRACT.md` §5.4-R1
- Issue: #87 (T08-R1) under #79
