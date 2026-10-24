# T10 expected upgrades — legal result changes vs the T09-R3 baseline

Every entry below is a *recorded* upgrade: a T10 capability legitimately
changes an old result because a new fact or new rule exists. Each lists
the new fact, the applicability condition, and the oracle that now holds.

## 1. `alter_index` action promotion

- Old: `ALTER TABLE … ALTER INDEX name VISIBLE|INVISIBLE` produced the
  `alter_table.alter_index` unaudited aspect, `coverage=incomplete`, and
  only the `ddl.table.exists.alter.require` existence gap.
- New: the action is audited — `alter.Name`/`Index.OldName` carry the
  target, `Index.Definition.Visibility` carries the requested target;
  `ddl.alter.alter_index.exists.require`,
  `ddl.index.invisible.forbid`, and the `visibility` feature of
  `ddl.index.advanced.compatibility.require` apply; `coverage=complete`
  when every enabled check has its facts.
- Oracle: §1.1 first path; missing-name blocker; unknown-set gap;
  5.7/inapplicable-version blocker.

## 2. Declared index visibility attributes modeled

- Old: `KEY|INDEX … VISIBLE|INVISIBLE` recorded the bounded unmodeled
  option names `visible`/`invisible` and the
  `<op>.index.option.(in)visible` unaudited aspects.
- New: `Index.Visibility` + `Index.VisibilityExplicit` carry the fact on
  every declared path; the two aspect names retire. Explicit visibility
  declarations also engage the `visibility` version feature (MySQL ≥8.0;
  `VISIBLE` on 5.7 is a syntax error natively, so both directions are
  gated).
- Oracle: `CREATE INDEX i ON t(a) INVISIBLE`, `CREATE TABLE … KEY i(a)
  INVISIBLE`, `ALTER TABLE t ADD INDEX i(a) INVISIBLE` — invisibility
  finding + complete coverage; `VISIBLE` attributes produce no
  invisibility finding.
- R1 catch-up: the T03 `mysql-alter-index-invisible` /
  `mysql-create-index-invisible` cells still pinned the retired
  `alter_table.alter_index` / `create_index.create_index.index.option.invisible`
  aspects (the T04/T05 cells were amended in the first round; T03 was not
  in that regression set). Same retirement, same all-off expectation:
  pass + complete coverage, no unsupported entries.

## 3. Provider index visibility

- New: `STATISTICS.IS_VISIBLE` read on anchors that expose it
  (8.0/8.4/TiDB 8.5) → provider members carry
  `Visibility="visible"|"invisible"`, `VisibilityExplicit=false`.
  MySQL 5.7 lacks the column — `Visibility=""` stays unresolved there.
- Oracle: metadata cases reading a natively-INVISIBLE member expose
  `visibility="invisible"` inside `Metadata.TargetTable` on later
  statements; the batch transition preserves it.

## 4. Provider SPATIAL kind fidelity

- Old: `classifyIndex` had no SPATIAL branch — a provider SPATIAL index
  loaded as `secondary`/`unique`, which could also satisfy a declared
  secondary duplicate signature.
- New: `INDEX_TYPE='SPATIAL'` → `IndexKindSpatial`; signature prefix and
  kind metadata now distinguish it.
- Oracle: metadata case loading `INDEX_TYPE='SPATIAL'` exposes
  `kind="spatial"`; a declared secondary duplicate over the same columns
  no longer matches the spatial signature.

## 5. Multi-valued member mark

- New: `CAST(… AS … ARRAY)` key parts count in `Index.MultiValuedParts`
  (still counted as expression members). Emits the new unaudited aspect
  `<op>.index.multi_valued` and the `multi_valued` compatibility feature
  (MySQL ≥8.0.17; TiDB 8.5 accepted and cataloged as a functional index —
  multi-valued query semantics remain unaudited).

## 6. Version applicability for advanced index forms

- New: `ddl.index.advanced.compatibility.require` consumes
  `AssessAdvancedIndexes`: `visibility`≥8.0 / `expression_keys`≥8.0.13 /
  `multi_valued`≥8.0.17 on MySQL, TiDB validated-8.5 series, descending
  silently ignored on MySQL <8.0 and on TiDB (`descending_ignored`),
  `primary_index_visibility` for MySQL PRIMARY targets. Missing or
  out-of-range version facts emit `missing_target_version` /
  `target_version_out_of_validated_range` gaps instead of silence.
- Legal upgrades: statements that previously kept only bare part-count
  aspects now also carry findings/gaps when the target version is
  provably insufficient.

## 7. `InvalidTransition` blocks post-state publication

- New: determined-negative index issues (`version_unsupported`,
  `primary_index_visibility`) prevent `applyAlterIndexVisibility` and
  `applyAddIndex` from publishing a post-state for the failing statement.
  `descending_ignored` does not invalidate — the index is created,
  merely without the declared ordering.

## 8. R1 scope restoration — primary-key identity and member legality

- `Index.PrimaryKeyType` now models the TiDB `CLUSTERED|NONCLUSTERED`
  primary-key attribute (column inline and index-option sites). The T03–T05
  `*-inline-pk-nonclustered` oracle cells change: on TiDB the
  `create_table.column.primary_key_type` unaudited aspect retires (a
  modeled fact, `coverage=complete`); on MySQL the same syntax now reports
  the index-path vendor boundary
  `create_table.index.option.primary_key_type` instead of the column-path
  unextracted marker — the statement stays `incomplete`, only the aspect
  identity and metadata shape change. Semantic basis: TiDB-only option
  parsed by the shared parser, never a MySQL capability.
- Implicit primary-key role: a qualifying `UNIQUE NOT NULL` index (first
  declared candidate, all-column members, all members proven NOT NULL)
  refuses invisibility (3522 on both products — natively pinned). New
  findings/gaps only where invisibility was declared on the holder;
  multi-candidate provider snapshots emit the
  `target_table.indexes.ordinal` gap instead of guessing.
- TiDB `ALTER INDEX PRIMARY`: PKIsHandle tables keep the missing-index
  blocker (1176); real index objects (NONCLUSTERED / non-integer
  clustered) accept `VISIBLE` as a no-op and refuse `INVISIBLE` at 3522.
- Member-legality features `prefix_length`/`fulltext`/`spatial`/`global`
  emit natively-pinned findings (1170/1283/1687/1252/1054/3152/8200)
  only when enabled in the compatibility rule's `features` param; the
  default four-feature profiles are unchanged.
