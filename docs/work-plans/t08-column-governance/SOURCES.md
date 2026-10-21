# T08 column governance — sources

Versioned product documentation and issue records consulted while freezing
`CONTRACT.md`. Native anchor records in `docs/verification/t08/<sha>/native/`
are the binding oracle; these sources supply the documented premises and the
explicitly open questions the native cells answer.

## MySQL (8.0/8.4 grammar; 5.7 differs only where noted)

- https://dev.mysql.com/doc/refman/8.0/en/alter-table.html —
  `ALTER [COLUMN] c {SET DEFAULT {literal|(expr)} | SET {VISIBLE|INVISIBLE} |
  DROP DEFAULT}`; `MODIFY [COLUMN] c column_definition [FIRST|AFTER col]`;
  `CHANGE [COLUMN] old new column_definition [FIRST|AFTER col]`;
  `ADD [COLUMN] c column_definition [FIRST|AFTER col]`; parenthesized
  `ADD (col def, ...)` carries no position clause; `RENAME COLUMN` (8.0+ only).
- https://dev.mysql.com/doc/refman/8.0/en/data-type-defaults.html —
  explicit vs implicit defaults; expression defaults since 8.0.13.
- MySQL full-redefinition semantics: MODIFY/CHANGE restate the complete
  column definition; omitted attributes reset (well-established MySQL
  behavior; e.g. liquibase/liquibase#7574 maintainer summary of the manual's
  contract). Confirmed per anchor by native cells N-omit/N-omit-ai.
- https://github.com/pingcap/tidb/issues/10195 — RENAME COLUMN supported in
  TiDB (closed 2020; present in 8.5).

## TiDB (stable docs, v8.5 anchor)

- https://docs.pingcap.com/tidb/stable/sql-statement-alter-table/ —
  multi-action semantics: "TiDB validates statements according to the table
  schema before execution" (documented example: `ADD c1, ADD c2 AFTER c1`
  errors because c1 is not in the pre-state); "the order of execution in
  TiDB is one change after another from left to right"; modifying the same
  object in multiple changes is unsupported.
- https://docs.pingcap.com/tidb/stable/sql-statement-modify-column/ —
  `MODIFY ... ColumnType ColumnOption* ('FIRST'|'AFTER' ColumnName)?`
  grammar; meta-only vs Reorg-Data classes; no explicit statement on
  omitted-DEFAULT preservation → open question answered by N-omit/N-tidb
  native cells before §5.4/§7.3 code lands.
- https://github.com/pingcap/tidb/issues/51324 — DROP DEFAULT catalog
  boundary context (default presence/absence in catalog).

## Internal

- `docs/work-plans/t07-advanced-columns/CONTRACT.md` §3: MySQL/TiDB
  `set_column_default`/`drop_column_default` deliberately do not reuse the
  PostgreSQL `set_default`/`drop_default` action space; T08 binds the
  existing explicit-default-change rules to the MySQL/TiDB actions rather
  than renaming actions into the PG space.
- `docs/decisions/2026-10-02-ddl-ordered-schema-state-first-path.md`,
  `2026-10-03-ddl-drop-recreate-state.md`, T05/T06 ADRs — ordered-state
  templates, tombstone/publication seam, A4 authored-vs-catalog source
  boundary.
- `internal/domain/rule/ddl/register.go`, `common.go`,
  `alter_semantic_rules.go`, `alter_compatibility_rules.go`,
  `metadata_rules.go`; `internal/application/audit/batch_state*.go`,
  `coverage.go`; `internal/infrastructure/parser/tidb/extractor.go` —
  the reuse-map source of truth (§2).
