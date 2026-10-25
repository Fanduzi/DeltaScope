# T10-R6-tail closeout — fd849db5

Bounded tail closure on top of R6 (`6cec4e6c` product, `248f8c05`
evidence). Three requested corrections plus two same-family defects
found by the focused independent review; no T10 re-expansion, no T11.

## Corrections

- **A. Proven PK absence outranks stale storage unknowns.**
  `spec.ResolveIndexNameBinding` now consults
  `PrimaryKeyHandleUnknown`/`PrimaryKeyClusteredUnknown` only while PK
  presence is not proven absent — a dropped object's leftover flags can
  no longer resurrect an ambiguous `primary` owner, so the following
  `DROP PRIMARY KEY` fires the proven-missing 1091 finding with complete
  coverage and reject verdict. `primaryNameMayOwnTarget`
  (batch_state_index.go) got the same precedence — a review finding: an
  unrelated ambiguous name-keyed drop could otherwise re-open the
  proven absence. Storage unknowns keep their meaning while a PK exists;
  nothing outside the dropped object's facts is cleared.
- **B. A member-owned `primary` owes no PK gap.** The
  drop-primary-key exists rule returns no `target_table.primary_key`
  gap for any decided name-keyed binding (member-owned, PK-bound, or
  freed); only a genuinely ambiguous binding owes exactly the unproven
  side, and the `drop_constraint`-derived path keeps its
  constraint-identity premise. Second review finding closed in the same
  seam: `Evaluate` previously applied the
  `PrimaryKeyUnknown`/`ConstraintsUnknown` flag gate before binding, so
  a freed `primary` under an unvouched constraint set passed silently
  (no finding, no gap) — the flag gate now applies only to the
  constraint-identity path, keeping Evaluate and EvidenceGaps on one
  premise.
- **C. Folded CLUSTERED merge is YES > unproven > NO.**
  `loadTiDBClusteredRows` folds case-variant `TIDB_INDEXES` names to one
  key and merges read-order-independently: an unproven (NULL or
  unrecognized) row is never covered by a same-key `NO`; `NO` answers
  only when every row under the key is a recognized `NO`; `YES` still
  identifies the clustered candidate. Pinned by a partial injected
  fixture (`TestT10R6TiDBFoldedClusteredRowKeepsUnproven` both orders,
  `…AllNOProvesNonclustered` control) — no native NULL-coexistence
  claim.

## Review outcome

One focused independent review covered only: absence-vs-stale-unknown
precedence, member-bound gap ownership, order-independent tri-state
merge, and regression of the main R6 fixes. It confirmed the fixes and
surfaced the two defects above (flag-gate ordering; stale-unknown read
in `primaryNameMayOwnTarget`), both closed with their own red tests
before the product SHA froze. All prior R6 behavior stands: ordered-KCU
unique attribution, `KindUnknown` consumers, `drop_constraint`
identity routing, binding-before-member-gates, provider error
propagation, MySQL reserved-name semantics.

## Gates bound to fd849db5

`gates/` quads (17 gate sets as archived): go test ./... 0 · go vet 0 ·
race ./internal/... 0 · gofmt — no new formatting issues this round; the
two accepted T07 baseline files remain · sql-corpus 0 ·
query-access-corpus 0 · ddl-inventory 0 · ddl-coverage-catalog 0 ·
ddl-golden-validator 0 · parser-error-unsupported-contract 0 ·
unsupported-diagnostics-evidence 0 · docs-example 0 ·
release-dialect-hygiene-gates 0 · decision-record 0 ·
pg-confidence-gates 0 · three-level-doc OK (archived stdout is the
script's post-commit "no changed files" output; the actual static-diff
check ran in-session on the changed files before commit and passed) ·
lint rc=1 baseline-only
(15 issues / 52 output lines, identical to the R6 baseline modulo line
numbers, 0 added).

Golden: T09 125/125 / 3442 assertions, T10 126/126 / 3175 — frozen
oracle denominators, `head_sha=fd849db5`, CLI sha256 `e22931ada0…`
(deterministic build).

## Verified / unverified boundary

Verified: the five red→green behaviors, all offline gates, PG gates, and
both golden runs on the bound SHA; native premises reused from the R6
TiDB v8.5.0 probes. Not verified: native TiDB NULL coexistence for
folded CLUSTERED rows (fixture-level only), MySQL PK-less
member-named-`primary` reservation (unchanged R5 boundary). Not
claimed: release, tag, PR, CI status, T11.
