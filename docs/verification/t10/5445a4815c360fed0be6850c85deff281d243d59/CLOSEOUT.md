# T10 closeout — advanced indexes and index visibility

Base: fbd411308e28332b5e8408b8074dd1c1fa501576
Contract: 57e8b1f9 + 94deda7f (frozen manifest/oracle)
Code: 7bb6658f + 5445a481 (final, all evidence bound)
Oracle amendment: 5c47f434 (T04/T05 ledgered upgrades)

## Scope delivered (issue #89 / #79 T10)

- `ALTER INDEX name VISIBLE|INVISIBLE` is an audited ordered-state
  action: extractor binds the target (`alter.Name`/`Index.OldName`) and
  the requested visibility (`Index.Definition.Visibility`,
  `VisibilityExplicit=true`); ordered batch state transitions the
  member's effective visibility, clearing the authored bit on published
  shapes; multi-action tombstones keep unprovable collections honest.
- Declared visibility attributes on primary keys, table indexes,
  standalone `CREATE INDEX`, and `ALTER ADD` forms are normalized facts;
  `CAST(... AS ... ARRAY)` members count as both expression and
  multi-valued parts.
- Provider probes `information_schema.columns` for
  `STATISTICS.IS_VISIBLE` (probe errors propagate, zero rows mean
  incapable): MySQL 8.0/8.4 and TiDB load effective `YES`/`NO`
  visibility; MySQL 5.7 leaves the fact unknown. `INDEX_TYPE` classifies
  SPATIAL and FULLTEXT natively.
- Rules: `ddl.index.invisible.forbid` (default warning) fires on every
  declared or transitioned invisibility and stays silent on authored
  VISIBLE; `ddl.alter.alter_index.exists.require` reuses shared
  existence semantics — MySQL `PRIMARY` resolves against the primary-key
  collection and reports `target_table.primary_key` gaps when unvouched
  (never a fabricated blocker); `ddl.index.advanced.compatibility.require`
  projects feature-scoped issues and bounded version gaps from
  `spec.AssessAdvancedIndexes` (visibility ≥8.0.0, expression ≥8.0.13,
  multi-valued ≥8.0.17, descending stored-or-ignored per product,
  primary-key invisibility refusal).
- Coverage audits `alter_index`; `multi_valued` member/query semantics
  stay bounded unsupported evidence.
- Inventory rows for governed alter-table index operations moved to
  `semantically_checked`; advanced create-index rows keep
  `generic_notice` where FULLTEXT/SPATIAL/GLOBAL variants remain
  partially unaudited.

## Verified at the bound SHA

- Golden: T10 81/81 cases, 1,910 assertions; regression T04 163,
  T05 247, T06 496, T07 207, T08 172, T09 125 — all pass at
  head=5445a481.
- Unit/spec: `make test` 39 ok / 0 FAIL; focused T10 suites green;
  spec-level decision table 10/10; race clean on the five touched
  packages.
- Contract: 33/33 adversarial validator checks; manifest denominator 81
  with frozen ID SHA `4ec9ba71…`.
- Gates: corpus, coverage catalog, inventory, parser-error,
  unsupported-diagnostics, census, docs-example, decision-record,
  three-level-doc, validator — rc=0 each.
- Native pins: 3522/1176/8200/1064/4028/1252 error classes and
  catalog observations recorded across all four anchors
  (native-probe/round1+round2).

## Legal upgrades recorded

- T04/T05 CLI cells expecting the retired `*.option.invisible` /
  `alter_table.alter_index` unaudited aspects now expect
  pass+complete under all-off policies (EXPECTED-UPGRADES §2; oracle
  amendment 5c47f434). First-pass regression failures are kept in
  golden/regression-all.log.
- T03 unit oracles for alter_index/create-index-invisible promoted from
  unsupported expectations to governed semantics (same commit series).

## Known bounds

- Multi-valued member/query semantics are not governed — bounded
  `multi_valued` unsupported evidence, not a version-only answer.
- Expression-member internal equivalence is not evaluated (gap-bounded,
  same stance as T09).
- FULLTEXT/SPATIAL on TiDB are vendor boundaries (8200); GLOBAL options
  and other advanced forms remain in `generic_notice` inventory rows.
- Provider IS_VISIBLE probe errors surface as provider load errors —
  incapable detection is "zero rows", not silent fallback.

## Review disposition

Two-axis review (Standards + Spec since fbd41130) ran before this
archive; all blocking findings closed — see README "Notes" for the
disposition list. No merge/PR/tag/release/issue action taken; branch
awaits independent T10 acceptance.
