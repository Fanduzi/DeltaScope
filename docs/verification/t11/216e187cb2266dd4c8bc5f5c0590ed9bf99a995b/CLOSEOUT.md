# T11-R1 closeout — issue #90 hardening round

## What shipped

Six root-cause groups closed on top of the T11 baseline. Product HEAD
`216e187cb2266dd4c8bc5f5c0590ed9bf99a995b` (test oracles) on
`20649919` (R1 product, 39 files +3223/−183).

### R1-A — PK structural rules on ALTER paths

`ALTER ... ADD PRIMARY KEY (cols)` members now resolve through the
statement's ordered pre-state column facts (`alterDeclaredPrimaryKeyMembers`
+ `orderedColumnFact`): proven violation → finding, proven compliant →
silent, withheld columns → `target_table.columns` evidence gap, and a
name-only member never predicates on a zero-value `spec.Column`. CREATE
parity kept. Prior PG oracles that pinned the name-only stub misreport
are corrected in `216e187c` with documented old/new premises.

### R1-B — collection contract, SDK clone, per-target gaps

Unified contract: a nil `Constraints`/`Indexes` is the *loaded empty
set*; only the explicit `*Unknown` flags withhold. SDK
`cloneConstraints`/`cloneIndexes` preserve nil-vs-empty. FK existence
`EvidenceGaps` mirrors the per-target Evaluate premise — a proven
member binding answers under an unvouched sibling space (CHECK-bearing
tables), the gap is owed only when an unmatched name may be hidden.
`ConstraintsUnknown` was removed from every PK-side premise (spec name
bindings, visibility rules, metadata rules, batch add/drop paths) — PK
truth lives on `PrimaryKey*` facts and proven member rows.

### R1-C — cross-schema referenced identity

`foreignKeyReferencedPremise` resolves the parent's effective schema
from the *child's* schema — one identity now drives the provider read,
cache key, display, and dependency edges. Two-database fixtures verify
the provider reads the correct object, explicit-schema, unqualified,
and self-reference arms.

### R1-D — FK landing premises

`applyAddForeignKey`/`createTableForeignKeyPremise` check the native
landing premises before publishing: local columns present, refer
actions admissible (SET NULL on NOT NULL → 1830; SET DEFAULT → 1215
pre-8.0 MySQL, accepted on 8.x/TiDB, unproven version undecidable), and
a usable referenced key — leftmost in-order coverer, primary/unique
everywhere, non-unique only pre-8.4 and on TiDB (8.4+ defers to the
observed `restrict_fk_on_non_standard_key`, 6125), specialized kinds
never. Proven refusal is a no-op; unprovable unvouches.

### R1-E — supporting-index lifecycle

CREATE-table FKs derive the implicit supporting index through the same
`publishForeignKeySupportingIndex` as ALTER adds. Undecidable adds
propagate index uncertainty (`propagateForeignKeyIndexUncertainty`)
unless coverage is provably already satisfied. Every removal entry —
member `drop_index`, `primary` spellings bound to the PK object, DROP
PRIMARY KEY, standalone forms, and a loaded child's binding on the
parent — answers the 1553 sole-coverer dependency via
`foreignKeyMemberDependency`/`foreignKeyPKDependency`. Coverage
eligibility honors index kind (specialized never supports; unknown kind
unanswerable).

### R1-F — DROP CHECK|CONSTRAINT target type

Shared-AST action answers by proven target type: TiDB never removes an
FK this way (no-op under `tidb_enable_check_constraint=OFF`, 3940 under
ON — probe-verified; earlier oracle corrected), MySQL 5.7 refuses the
syntax (1064), MySQL 8.x keyword divergence (`DROP CONSTRAINT` removes,
`DROP CHECK` refuses 3821) unvouches the member's fate via
`unvouchForeignKeyMember`. Real CHECK semantics stay with T12.

## Verified vs not verified

**Verified:** all of the above on `216e187c`; four-anchor probes
(`native-probe/`); deterministic CLI build (`4b366a4f…` identical
across the work build and all three golden artifacts); T09/T10/T11
denominators preserved (125/126/40 cases).

**Not verified / bounded:**
- `restrict_fk_on_non_standard_key` unobserved on a live 8.4+ target
  resolves by version fallback — observed-OFF arm not probed (factory
  ON confirmed on 8.4.10).
- Lazy dependency invalidation remains bounded to already-loaded
  entries (no whole-catalog reverse scan), per CONTRACT §6.
- `MATCH` clauses stay unmodeled evidence; real CHECK removal stays
  T12; inline column `REFERENCES` stays parse-and-ignore.
- PG gap vocabulary unchanged — `orderedGapDialect` confines the new
  ALTER-side gaps to the ordered dialects.

## Gate summary

17 gate quads under `gates/`, all bound to `216e187c`: 16 × rc=0 plus
`lint` rc=1 identical to the accepted baseline (15 issues, zero added —
all on pre-R1 lines). `ddl-coverage-catalog-test` and
`pg-confidence-gates` caught the R1-A downstream oracles mid-round;
the catalog was regenerated and the oracle commit re-frozen before the
bound run — both pass.

## Remote / lifecycle status

Local branch `milestone/mysql-tidb-ddl-completion` at `216e187c` plus
this evidence commit. Fast-forward push to `origin/workfree` per the
standing authorization. No PR, merge, issue closure, tag, release, or
deployment performed or claimed; T11 stops at the
independent-acceptance candidate.
