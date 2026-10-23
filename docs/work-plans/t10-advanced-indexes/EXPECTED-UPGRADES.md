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
