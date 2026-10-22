# T09-R1 closeout — INDEX-FACTS-AND-STATE

Base: dc54ecacd06445b50e8c193ef2e0e7b8f6dda817
Docs: 7ac054e57be756c7f0caf9911c594c19254c9e3c
Code: a986eabaa037ec12b92f505e2957f7f4c79ea030 (final, all evidence bound)

## Sequence executed

真实红色 → 有界原生确认 → 合同补订 → 最小修复 → 回归 → 最终门禁 → 证据 → ff交付

- Red first: rule-level + audit-level matrices were authored against the
  unfixed tree and failed on exactly the defect paths (duplicate on
  differing prefix lengths, left-prefix conviction through a non-plain
  covering index, unique-overlap on a prefix unique, ADD PRIMARY KEY on
  absent member publishing a false post-state, member nullability not
  reconciled). Opposite controls stayed green.
- Native confirmation: 4 roles x 4 anchors (see native-probe/), plus a
  real-default variant inside role 4. SUB_PART/COLLATION available on all
  anchors; ERROR 1072 + unchanged structure on missing member;
  NOT NULL conversion with NULL default cleared and DEFAULT 5 kept.
- Contract amendment appended to CONTRACT.md, ADR, EXPECTED-UPGRADES
  (commit 7ac054e5). Known-empty vs withheld-set semantics preserved;
  dc54ecac's real composition (Makefile gate wiring + ADR + docs-side
  catalog copy, not evidence-only) recorded.
- Minimal fix at a986eaba: spec.Index.UnmodeledKeyParts;
  indexPlainComparable shared by the three pair rules; provider loads
  SUB_PART/COLLATION and classifies members (NULL COLUMN_NAME ->
  expression; 'A' plain; 'D' desc; NULL/other -> unmodeled);
  applyAddIndex primary branch reordered to prove the member premise,
  publish, then reconcile member facts. add_constraint primary path
  shares the same logic.

## Frozen contract preserved

125-case T09 manifest, role identities, SQL, profiles, deferred
denominator: unchanged. Normal-path plain-index findings, naming,
added-only attribution, DROP/RENAME chains: unchanged (controls +
goldens confirm). T08 YEAR repair and default-value research not
reopened. T10 not entered.

## Gate results at the bound SHA

- make test (GOFLAGS=-count=1): 39 packages ok, 0 FAIL
- pg-unit-test-gates, sql-corpus-gates, ddl-inventory-gate,
  ddl-coverage-catalog-test, docs-example-gates, decision-record-gate,
  ddl-golden-validator-test: all rc=0
- race on touched packages: rc=0
- goldens: T04 163/163, T05 247/247, T06 496/496, T07 207/207,
  T08 172/172, T09 125/125 — all at head a986eaba
- three-level doc check: run green during the change window; all
  changed files carry updated L3 headers and module READMEs were synced
  in the same commits
- gofmt: R1 diff clean; two pre-existing unformatted T07 test files
  noted in gates/gofmt.log (not part of this diff)

## Sensitivity demonstrated

- Stripping provider member classification (SUB_PART/COLLATION ->
  unmodeled marks) fails the new provider matrix cell — restored.
- The permanent tests fail if Columns-only comparison returns, if
  unqueried attributes are treated as known-plain on the provider path,
  if the PK branch precedes the member check, or if member NOT NULL /
  null-default reconciliation is dropped — by construction of the
  matrix assertions.

## Delivery

Fast-forward push to origin/workfree only. No merge, no PR, no tag,
no release, no issue lifecycle action. Ready for human review.
