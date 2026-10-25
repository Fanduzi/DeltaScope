# T12 (#91) closeout

Product: `1f8e5a0b4e89d8cc4e6f8dd34708476808e2ba91`
Evidence: this bundle + T11-tail README corrections (same commit).

## Verified

- Golden Path `ALTER TABLE t ALTER CHECK chk_id NOT ENFORCED` on
  MySQL 8.0.46/8.4.10 and TiDB flag-ON: member retained with
  `enforced=false`, `ddl.constraint.check.not_enforced.notice` fires,
  live DB state verified by catalog/`SHOW CREATE` probes.
- Counter-cases: `ENFORCED` reverse (no notice), ordinary column change
  (no notice), MySQL 5.7 declaration (ignored.notice, not effective
  enforcement), 5.7 ALTER/DROP CHECK (version blocker, 1064 class),
  TiDB flag OFF (ignored.notice), missing/non-CHECK flip target
  (exists blocker, not a false "disabled" claim), unobserved
  version/flag/member facts (attributable gaps).
- Ordered state: ADD→ALTER, ADD→DROP, in-column members, CREATE arms,
  `ALTER CONSTRAINT` alias, 3959-refusal publication, TiDB CHANGE
  cascade vs RENAME refusal, duplicate-name refusal preserving state,
  unvouched/unsafe transitions tombstoning instead of fabricating.
- Provider: MySQL `TABLE_CONSTRAINTS.ENFORCED` + `CHECK_CONSTRAINTS`
  catalog/schema/name join; TiDB `TIDB_CHECK_CONSTRAINTS` with
  enforcement unvouched; `tidb_enable_check_constraint` instance fact.
- Full Go suite, race, vet, corpus, inventory, coverage-catalog,
  parser-error contracts, docs examples, decision-record gate, doc-diff
  README rule: all rc=0. Lint = accepted 15-issue baseline (identical
  output to T11 bundle).

## Not verified / boundaries

- MySQL patch releases other than 5.7.44/8.0.46/8.4.10.
- TiDB enforcement via catalog (no surface exists); SHOW CREATE only.
- ENFORCED-flip outcome against violating rows (data-conditional).
- Non-probed refusal/drop arms on flag-unobserved TiDB.
- CHECK expression semantics (solving, MATCH clauses) — out of scope.

## Delivered commits

- `1f8e5a0b` product (73 files)
- evidence commit (this bundle + T11-tail corrections) — pushed ff to
  origin/workfree; origin/milestone/mysql-tidb-ddl-completion untouched.
