# T09-R3 closeout — truncated-projection pairing + discriminated default evidence

Base: b074aee0187938363789569e4a07f2483746d83a
Docs: 8e68b44c4eddb4cb654486a42c144dccf81dc270
Code: 1a3dd01f2e13fab8780b35f93ff5d0b055e44dea (final, all evidence bound)

## Reproduction

- R3-A: provider-loaded `i_pre` secondary with `Columns=["a"]`,
  `HasExpressionKeys=true`, `ExpressionCount=1` (the recorded shape of
  `CREATE INDEX i_pre ON t(a, (LOWER(b)))`). Auditing
  `CREATE INDEX i_new ON t(a);` under an isolated
  `ddl.index.redundant_left_prefix.forbid` profile produced 0 findings
  and 0 EvidenceGaps — `isLeftPrefix(["a"], ["a"])` returned false on the
  truncated projection, so coverage stayed complete and the verdict was
  pass. The `ALTER TABLE t ADD INDEX i_new (a)` wrapper missed the same
  way through the shared pairing.
- R3-B: not a product defect — the R2 probe's `--batch --raw` display
  rendered string `'NULL'` and SQL NULL identically, so the "uppercase
  string preserved" claim lacked discriminating native evidence.

## Fix

- `common.go` `leftPrefixPairing`: when the existing member carries
  expression key parts (`ExpressionCount>0`/`HasExpressionKeys`), the
  named projection is incomplete — pairing uses `hasKeyPartPrefix`
  (non-strict prefix over the named columns) instead of strict
  `isLeftPrefix`. Members with a complete projection keep the strict
  proof. No expression evaluation added; `IndexesUnknown` untouched;
  duplicate/unique-overlap pairings unchanged (they already pair on the
  truncated-equal view).
- Entries: `lifecycleIndexGap` (standalone create_index) and
  `alterAddedIndexRule.EvidenceGaps` (ALTER add_index/add_constraint)
  both route through the corrected pairing.
- B: product code unchanged; new probe cells only.

## Verified

- 7 red leaf subtests captured pre-fix (gates/red-before-fix.log):
  rule-level create_index/alter_add_index/equal-length cells plus the
  2-entry × 2-dialect audit cells; all scope controls passed pre-fix and
  post-fix.
- Post-fix green: r3-focused-tests.log (both packages ok, 0 FAIL).
- Controls: declared column absent / over-long prefix / named-order
  disproof / complete-projection non-plain / all-expression member → no
  gap; proper prefix and equal-length truncated → gap; unrelated
  declared index → 0 findings, 0 gaps, complete, pass.
- make test 39 ok / 5 no-test / 0 FAIL; pg / corpus / inventory /
  catalog / docs / ADR / validator / t09-contract / three-level-doc /
  race — all rc=0 at the bound SHA.
- Goldens T04–T09 all PASS at 1a3dd01f with frozen denominators and
  identical assertion totals (163/1889, 247/5109, 496/11772, 207/6190,
  172/4240, 125/3442); see golden-run.log for the credential-env rerun
  note.
- Native (all four anchors): `'null'`→IS NULL=0 / 6E756C6C,
  `'NULL'`→IS NULL=0 / 4E554C4C, SQL NULL→IS NULL=1 after
  `ADD PRIMARY KEY` — see native-probe/.

## Delivery

Fast-forward push to origin/workfree only. No merge/PR/tag/release/issue
action. Ready for human review.
