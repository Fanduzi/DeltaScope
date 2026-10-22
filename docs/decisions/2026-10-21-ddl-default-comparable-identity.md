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

## Amendment — T08-R2 comparable domain for numeric literals

R1's shared comparison overclaimed one step further than its fact model
justified: `Kind="numeric"` marked the *value domain* and `Value` equality
decided same/different for every numeric pair. Catalog scale/type
normalization (`decimal(10,2)` stores `1.00`) and AST integer text
(`DEFAULT 1` decodes `1`) are different encodings of possibly-equal
values, so a representation-only difference was convicted as a determined
change.

The corrected boundary: a recorded literal identity is not automatically
a comparable pair. `string` values are decoded bytes — canonical by
definition — and `numeric` values compare directly only when **both**
sides carry canonical-integer text (`0` or `-?[1-9][0-9]*`). Any other
numeric pairing is unproven and lands as `unproven_default_identity` over
both `*_column.default` facts: known-but-incomparable pairs are a
comparison gap, never a fabricated fact deficit or a determined finding.
Both fact sources inherit the same boundary — a derived `DEFAULT 1.00`
yields no comparable identity just as a catalog `1.00` cannot pair
against `1`.

Deferred unchanged: DECIMAL/FLOAT and remaining YEAR-adjacent
normalization across encodings (scale, scientific notation, year
arithmetic, `sql_mode` effects) — no numeric normalizer or evaluator was
added. R1's string and integer positives, presence flips, class
switches, expression fingerprints, and the explicit-op/effective-change
separation are unchanged.

## Amendment — T08-R3 type-normalized literals leave the comparable domain

R2 still overclaimed: it treated canonical-integer text as a proven
encoding on *every* column type. MySQL applies a value normalization for
`YEAR` — a declared `DEFAULT 1` stores `2001` in `COLUMN_DEFAULT` — so
the catalog's canonical `2001` and the declaration's canonical `1` are
the *same* effective default, and comparing them convicted an identical
stored default as changed. Natively confirmed on the pinned MySQL 8.4.10
anchor (stored bytes `2001`→`2001` across the same-definition MODIFY;
the audit emitted `changes its default value`). TiDB 8.5.0 stores the
declared integer verbatim (`1`), so the defect is MySQL-family-specific.

The corrected boundary: canonical-integer text is a shared encoding only
where neither column's type rewrites the literal on storage. A
different-text `numeric` pair touching a `year` base type on either
column is `unproven_default_identity` over both facts — the comparator
now reads the `spec.Column` types it already receives, never statement
text. Equal recorded bytes still short-circuit to same unconditionally
(`2001` vs `2001` on year stays same — proven identical on the anchor).
No year conversion is implemented: pairs that need one are honest gaps,
not verdicts. Both fact sources judge identically — a CREATE-derived
year column carries the same gate.

Known boundary retained: a different-text numeric pair between a
year column and a non-year column is also unproven — the comparator
cannot distinguish a provider-final `2001` from a declaration-raw `1`
without a year conversion, so the safe direction is the bounded gap.

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
- R2 additions: N1–N3 decimal-scale cells (`1.00`/`0.00`/`-1.00` vs
  declared `1`/`0`/`-1`) prove unproven outcomes on provider-sourced and
  CREATE-derived pre-states across MODIFY+CHANGE and both dialects;
  canonical-integer controls (`7→7`, `7→8`) and the string matrix keep
  their R1 verdicts.
- R3 additions: native probe on the pinned MySQL 8.4.10 anchor —
  `YEAR DEFAULT 1` stores `2001`, the same-definition MODIFY leaves the
  stored bytes `2001`→`2001`, and the post-fix audit emits the bounded
  `unproven_default_identity` gap instead of the convicted finding;
  `DEFAULT 2001` and `INT DEFAULT 1` controls stay silent. TiDB 8.5.0
  stores `1` verbatim and stays silent. Real-entry cells
  `y1_year_normalized`/`y2_year_full_same`/`y3_year_diff_unproven`
  (MODIFY+CHANGE, both dialects), derived-parity cells
  (`derived_year_same`/`derived_year_normalized_unproven`/
  `derived_year_full_same`), and rule-level year cells pin the boundary.

## Consequences

- Any future default-comparison surface must compare `DefaultLiteral`
  (and `DefaultExpression` fingerprints), never `DefaultValue`.
- A new literal form earns `DefaultLiteral` only when a cross-source
  identity is provable; otherwise it keeps the nil/unproven contract.
- A numeric pair earns a determined verdict only inside the
  canonical-integer domain; widening the domain requires a proven
  cross-source encoding, not byte equality on the numeric kind alone.
- `pkg/deltascope` cloning, `batch_state` cloning, and any new
  publication path must deep-copy `DefaultLiteral` alongside
  `DefaultExpression`.

## Links

- Contract: `docs/work-plans/t08-column-governance/CONTRACT.md` §5.4-R1/R2
- Issue: #87 (T08-R1/R2) under #79
