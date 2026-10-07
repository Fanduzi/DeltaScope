# T06-A8 expected-output upgrade table

Every intentional old→new expectation change caused by the three comment
fidelity fixes: decoded `Column.Comment` content (no SQL literal quoting),
code-point `ddl.table.comment.max_length`, and `Table.Comment` on the
complete CREATE-derived shape. Statement text, case/fixture IDs, rule IDs,
messages, and metadata keys are unchanged; only the affected assertions
moved.

## Shared Go tests — decoded column comment

| file | case | old (25c4eaa2) | new (767a19c9) | causation |
|---|---|---|---|---|
| `internal/infrastructure/parser/tidb/extractor_test.go` `TestExtractorCreateTableCapturesStructuralFacts` | column `note COMMENT 'memo'` → `Column.Comment` | `'memo'` (quote-wrapped) | `memo` | extractor stores the decoded `ValueExpr` string, not the quoted literal |
| `internal/infrastructure/parser/tidb/extractor_t06a3_test.go` `TestT06A3DefaultLiteralPreservation` | `columns[3] COMMENT 'note'` → `Comment` | `'note'` | `note` | same seam; the DEFAULT assertions in this test are untouched |
| `internal/infrastructure/parser/tidb/extractor_t06a3r1_test.go` `TestT06A3R1DuplicateDefaultIsolation` | column `c COMMENT 'x'` → `Comment` | `'x'` | `x` | same seam; DEFAULT/dedup assertions untouched |
| `internal/application/audit/extract_test.go` `TestExtractMapsAlterTable` | `ADD COLUMN ... COMMENT 'age'` → `Definition.Comment` | `'age'` | `age` | `extractColumn` is shared by CREATE and ALTER column definitions |
| `internal/application/audit/batch_state_a6_matrix_test.go` `TestAuditSQLT05A6SameNameAddUsesTheNewDefinition` ×2 | DROP pre-state of `obsolete COMMENT 'retired'` → `Comment` | `'retired'` | `retired` | derived shape carries the decoded comment through `cloneColumn` |

## Rule-level behavior changes (not expectation edits — new assertions)

| input | old (25c4eaa2) | new (767a19c9) | causation |
|---|---|---|---|
| `CREATE TABLE t (c INT COMMENT '')` under `ddl.column.comment.require` | pass — `' '`/`''` wrappers defeated `TrimSpace` | reject, one blocker `column "c" must include a comment` | decoded content `""` trims to empty |
| `CREATE TABLE t (c INT COMMENT '   ')` same policy | pass | reject, same finding | `"   "` trims to empty |
| `CREATE TABLE t (c INT) COMMENT='中文注'` under `ddl.table.comment.max_length limit=8` | reject, `actual=9` (UTF-8 bytes) | pass, no finding | `actual` is now `utf8.RuneCountInString` = 3 |
| `COMMENT='中文注中文注abc'` same policy | reject, `actual=21` | reject, `actual=9` | code points, not bytes |
| `CREATE ... COMMENT='表注'; ALTER ...` second-statement pre-state | `Table.Comment=""` | `Table.Comment="表注"` | `derivedCreateShape` copies the declared comment |

## Golden manifest & validator

| file | item | old | new | causation |
|---|---|---|---|---|
| `testdata/ddl-golden/T06.json` | denominator | 188 cases | 228 cases (+24 CLI `t06-a8-{mysql,tidb}-*`, +16 metadata `t06-a8-{anchor}-*`) | A8 contract; all 188 old cases preserved with identical specs (verified: zero removed/changed objects) |
| same | `policies.profiles` | 14 profiles | 17 profiles (+`t06-a8-table-comment-required`, `+t06-a8-column-comment-required`, `+t06-a8-table-comment-length`) | three single-rule isolations; `limit=8`/`required=true` are case params, shipped defaults unchanged |
| `scripts/ddl_golden.py` | T06 contract | 188-case contract | 228-case contract + A8 case/profile builders; comment raw/CHAR_LENGTH/OCTET_LENGTH/HEX oracle queries; `SET NAMES utf8mb4` delivery recording | same |
| `scripts/test_ddl_golden.py` | fixture + mutations | 335 contract cases | 357 contract cases; `t06-a8-` excluded from the pre-A7 generic mutation loops; A8 rules added to the fake catalog; `stdout_contains` markers surfaced into synthetic execute records | validator must pin the new contract |

## CLI behavior (same input, both dialects)

Input `CREATE TABLE t (c INT) COMMENT='中文注';` under a
`ddl.table.comment.max_length`-only policy at `limit=8` `--fail-on blocker`:

| | old HEAD 25c4eaa2 | tested HEAD 767a19c9 |
|---|---|---|
| verdict | `reject` | `pass` |
| findings | 1 blocker `ddl.table.comment.max_length`, `actual=9` | none |
| exit code | `1` | `0` |

Input `CREATE TABLE t (c INT COMMENT '');` under a
`ddl.column.comment.require`-only policy `--fail-on blocker`:

| | old HEAD 25c4eaa2 | tested HEAD 767a19c9 |
|---|---|---|
| verdict | `pass` (declared-but-empty read as commented) | `reject` |
| findings | none | 1 blocker `ddl.column.comment.require` |
| exit code | `0` | `1` |

Baseline originals: `baseline-25c4eaa2/runs/` (16-run matrix, old binary).
Red evidence: `red-25c4eaa2/` — the committed T06A8 tests fail at the old
HEAD (column decoding, empty/blank require, rune length, derived
`Table.Comment`).

## Native anchor facts (all four anchors identical)

| case | product | driver | stored oracle |
|---|---|---|---|
| `t06-a8-*-column-empty` (`COMMENT ''`) | reject | rc 0 | `COLUMN_COMMENT` `0:0:`, `TABLE_COMMENT` `0:0:` |
| `t06-a8-*-column-decoded` (`it''s 中文` / `表注`) | pass | rc 0 | column `7:11:6974277320E4B8ADE69687`, table `2:6:E8A1A8E6B3A8` |
| `t06-a8-*-table-rune-at` (`中文注中文注ab`) | pass | rc 0 | table `8:20:E4B8ADE69687E6B3A8E4B8ADE69687E6B3A86162`, column `3:3:636F6C` |
| `t06-a8-*-table-rune-above` (`中文注中文注abc`) | reject `actual=9` | rc 0 | table `9:21:E4B8ADE69687E6B3A8E4B8ADE69687E6B3A8616263` |

Every anchored execute step runs under an explicitly recorded
`SET NAMES utf8mb4` session (`stdout_contains: utf8mb4` assertion), so the
multibyte literal is delivered losslessly and the delivery encoding is
part of the artifact, not an assumption.
