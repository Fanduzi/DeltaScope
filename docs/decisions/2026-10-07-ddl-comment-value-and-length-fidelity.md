# Decision: T06-A8 comment content fidelity, code-point comment length, and CREATE-derived Table.Comment

Date: 2026-10-07
Status: Accepted
Related milestone/version: mysql-tidb-ddl-completion (#79), task T06 / issue #85
Related commits: T06-A8 implementation commit on milestone/mysql-tidb-ddl-completion
Related tests: `internal/infrastructure/parser/tidb/extractor_t06a8_test.go`, `internal/domain/rule/ddl/table_comment_t06a8_test.go`, `internal/application/audit/coverage_t06a8_test.go`, `pkg/deltascope/audit_comment_t06a8_test.go`, `internal/interfaces/http/audit_comment_t06a8_test.go`, `scripts/test_ddl_golden.py` T06-A8 contract group
Related docs: `testdata/ddl-golden/T06.json`, `docs/reference/rules.md`, `docs/reference/rules.zh-CN.md`, `docs/reference/audit-capability-matrix.md`, `docs/reference/audit-capability-matrix.zh-CN.md`

## Context

The baseline evidence (`/tmp/ds-t06-a8/`, collected on `25c4eaa2`) proved
three independent comment fidelity defects around ordinary
`CREATE TABLE`:

1. **Column comments carried SQL literal quoting.** `extractColumn`
   routed `ColumnOptionComment` through `normalizedExprText`, which wraps
   a string `ValueExpr` in literal single quotes: `COMMENT '中文注'`
   extracted as `'中文注'` (5 runes with quotes vs 3 without), `COMMENT ''`
   as `''`. Because the two presence rules only `TrimSpace`, a declared
   but empty column comment satisfied `ddl.column.comment.require` — an
   `''` string is non-empty. The same helper feeds `DEFAULT` extraction,
   where the quoted-literal text is the accepted A3 contract, so the
   defect could not be fixed inside the shared function.
2. **The length rule counted bytes while its contract says characters.**
   `tableCommentMaxLengthRule` used `len()`, so `COMMENT='中文注'` reported
   `actual=9` against `limit=8` — three characters measured as nine UTF-8
   bytes, while the message and documentation promise "characters".
3. **Derived CREATE shape dropped the declared table comment.**
   `derivedCreateShape` rebuilt `snapshot.Table` with only schema/name.
   `Options["comment"]` and `Column.Comment` survived, but the direct
   `Table.Comment` field was always empty on a derived shape even though
   provider snapshots populate it — a missing field on the complete-shape
   path, not a T05 state-machine issue.

## Decision

- **`Column.Comment` stores decoded content, not SQL literal text.** The
  `ColumnOptionComment` branch reads `ast.ValueExpr.GetValue()` through a
  new `decodedStringValue` helper: string literal → decoded content, no
  quote wrapping, no re-escaping, no trimming, no case or Unicode
  normalization. Non-string or non-literal expressions (including nil)
  yield `""` — the grammar only admits string COMMENT literals, so the
  defensive branch adds no syntax surface and no public field. Every
  `COMMENT` option assignment overwrites the previous value, including
  an empty string. `normalizedExprText`, `DEFAULT`,
  `exprIsCurrentTimestamp`, and every other column option keep byte-for-
  byte behavior; the shared `extractColumn` change also reaches
  ALTER ADD/MODIFY/CHANGE `Definition.Comment` uniformly.
- **`ddl.table.comment.max_length` counts Unicode code points.**
  `actual` is `utf8.RuneCountInString(statement.DDL.Table.Comment)` — not
  UTF-8 bytes, not grapheme clusters, not trimmed. The rule keeps
  `limit=128` default, `warning` default level, the `limit >= 1`
  constructor bound, `AppliesTo`, message/suggestion wording, and
  metadata keys; `actual` and `limit` are the same unit. Combining
  sequences count per code point, so eight visible glyphs of `e`+U+0301
  are sixteen runes and reject at `limit=8` where a grapheme counter
  would pass.
- **`Table.Comment` is copied into the complete derived shape only.**
  `derivedCreateShape` now copies `ddl.Table.Comment` into the new
  `snapshot.Table` at the existing construction site. Callers still own
  the resolved schema/name; the `ddl.Table` pointer is never aliased;
  `Options["comment"]` keeps its existing copy semantics; nothing is
  guessed from provider state or old options. The conditional contracts
  are unchanged: `IF NOT EXISTS` on a known-present table keeps the
  provider comment, an unknown target still publishes present-incomplete
  with no declared comment, and DROP+CREATE cannot resurrect the old
  comment — this is a missing-field completion on the complete shape,
  not a conditional-state redesign.

## Consequences

- Presence semantics sharpen: `COMMENT ''` and whitespace-only column
  comments now fail `ddl.column.comment.require`, matching the table
  rule's existing empty/whitespace rejection. The require rules remain
  per-field — a table comment can never satisfy the column rule, and a
  column comment can never satisfy the table rule.
- Multibyte table comments pass or fail on their character count:
  `COMMENT='中文注'` reports `actual=3` against `limit=8` and passes,
  `COMMENT='中文注中文注abc'` reports `actual=9` and rejects with the
  unchanged message `table comment must not exceed 8 characters`.
- `T06.json` grows from 188 to 228 required cases: 24 offline CLI roles
  (12 variants × mysql/tidb) and 16 anchored metadata cases (4 roles ×
  MySQL 5.7.44 / 8.0.46 / 8.4.10 / TiDB 8.5.0). The anchored roles pin
  stored comment content as raw text plus `CHAR_LENGTH`/`OCTET_LENGTH`/
  `HEX` under an explicitly recorded `utf8mb4` session, and every
  product-side reject still drives the native `CREATE`.
- The validator gains A8 mutation coverage: empty/blank comments
  recorded as the old pass, byte-count `actual` values, at-limit
  rejected or above-limit passed, table↔column finding identity and
  metadata swaps, quote-wrapped or apostrophe-dropped or padded comment
  content, CHAR/OCTET swaps and mangled HEX, rejected products skipping
  the driver replay, absence/structure/cleanup legs deleted on both
  sides, cross-anchor rebind, offline demotion, profile swaps, and
  both-sides case deletion.
- Prior expected behavior changed in exactly one family: six assertions
  across five test files expected the SQL-literal-quoted column comment
  (`'memo'`, `'note'`, `'x'`, `'age'`, `'retired'`); they now expect the
  decoded content. No corpus fixture, golden expectation, or rule
  default changed.

## Deferred scope

- No column-comment length rule, no new gap kind, no `HasComment`
  provenance flag, and no duplicate-`COMMENT`-option policy rule were
  added — an explicit empty comment is policy-equal to a missing one,
  and later `COMMENT` options win by overwrite without a separate rule.
- `DEFAULT` literal text keeps the A3 quoted contract; provider-native
  comments, the PostgreSQL comment rules, SQL-mode escape matrices, and
  native per-version comment size limits are untouched — the product
  `limit` is a team policy parameter, not a database maximum.
- ALTER comment governance beyond the shared `extractColumn` encoding
  (policy rules on comment changes, comment state transitions) is not
  claimed by this slice.
