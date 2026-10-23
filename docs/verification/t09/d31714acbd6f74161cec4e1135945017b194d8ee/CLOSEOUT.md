# T09-R2 closeout — member-comparability gap + PK string-default boundary

Base: 4c62b47aaf0470f6fd77f5e32b484c39e1a42b3a
Docs: b8215e600afd754f2d25b2de253a61e013bbb160
Code: d31714acbd6f74161cec4e1135945017b194d8ee (final, all evidence bound)

## Reproduction

- R2-A: provider returns an index with SUB_PART set (PrefixParts=1) on a
  vouched index set; `CREATE INDEX i_new ON t(a)` or `ALTER TABLE t ADD
  INDEX i_new (a)` under the duplicate/left-prefix/unique-overlap rules
  evaluated 0 findings and 0 EvidenceGaps — coverage=complete,
  verdict=pass. The skipped pair was invisible.
- R2-B: provider-loaded column `VARCHAR(32) DEFAULT 'null'` arrives
  HasDefault=true, DefaultValue="null", DefaultIsNull=false, string
  DefaultLiteral — and the PK branch still cleared it via
  EqualFold(TrimSpace(DefaultValue), "null").

## Fix

- `common.go`: `indexKeyPartsGapFact` + `memberComparisonGap` + the three
  pairing shapes (`duplicatePairing`, `leftPrefixPairing`,
  `uniqueOverlapPairing`).
- `index_rules.go`: `lifecycleIndexGap` takes the rule's pairing; known
  sets now route to `memberComparisonGap` instead of returning nil.
- `alter_semantic_rules.go`: `alterAddedIndexRule.EvidenceGaps` does the
  same via `alterAddedIndexPairing(ruleID)`; non-lifecycle wrappers get no
  pairing.
- `batch_state_index.go`: the PK conversion clears member defaults only on
  `DefaultIsNull`.

## Verified

- 24 red subtests captured pre-fix (gates/red-before-fix.log); all green
  post-fix (r2-focused-tests.log).
- Gap scoping controls: non-pairing non-plain member → 0 findings, 0 gaps,
  complete coverage, pass verdict; plain pair → finding, no gap; unknown
  set → original `target_table.indexes` gap; known-empty → nothing.
- B matrix: 'null'/'NULL'/'NuLl'/' null '/'abc'/numeric provider defaults
  survive; declared DEFAULT 'null' survives; declared DEFAULT NULL clears;
  provider catalog-NULL stays default-free.
- make test / pg / corpus / inventory / catalog / docs / ADR / validator /
  three-level-doc / race — all rc=0 at the bound SHA.
- Goldens T04–T09 all PASS at d31714ac with frozen denominators and
  identical assertion totals.
- Native: string defaults preserved through PK conversion on all four
  anchors (native-probe/).

## Delivery

Fast-forward push to origin/workfree only. No merge/PR/tag/release/issue
action. Ready for human review.
