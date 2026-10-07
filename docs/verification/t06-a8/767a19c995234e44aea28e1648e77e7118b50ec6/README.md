# T06-A8 evidence: comment content fidelity, code-point length, and CREATE-derived Table.Comment

Tested code commit: `767a19c995234e44aea28e1648e77e7118b50ec6`
(`feat(ddl): decode column comments, count table comment code points,
carry Table.Comment (#85)`). This directory is the evidence-only commit on
top of it — the evidence HEAD is not the tested HEAD.

## Scope

Three bounded production changes plus their proofs:

1. **Decoded column comments.** `extractColumn`'s `ColumnOptionComment`
   branch reads `ast.ValueExpr.GetValue()` through the new private
   `decodedStringValue` — `Column.Comment` is the parser-decoded content
   (`COMMENT 'it''s 中文'` → `it's 中文`, `COMMENT ''` → `""`), never a
   quote-wrapped SQL literal. `normalizedExprText` and every DEFAULT
   consumer are byte-identical; the shared extraction reaches ALTER
   ADD/MODIFY/CHANGE column definitions uniformly.
2. **Code-point comment length.** `ddl.table.comment.max_length` computes
   `actual` via `utf8.RuneCountInString` — `COMMENT='中文注'` counts 3, not
   9 UTF-8 bytes. `limit`/`actual` share one unit; message, suggestion,
   metadata keys, default `limit=128`, default `warning`, and the
   `limit >= 1` constructor bound are unchanged.
3. **Derived-shape table comment.** `derivedCreateShape` copies
   `ddl.Table.Comment` into the new `snapshot.Table`. Conditional
   contracts unchanged: `IF NOT EXISTS` on a known-present table keeps
   the provider comment, unknown targets still publish present-incomplete
   (no declared comment), DROP+CREATE never resurrects the old comment.

ADR: `docs/decisions/2026-10-07-ddl-comment-value-and-length-fidelity.md`.

## Contents

- `artifact.json` — formal `make ddl-golden TASK=T06` artifact:
  **228/228 cases, 5403 assertions, head_sha=767a19c99523**, cleanup rc=0.
- `cases/` — all 228 per-case records (sql/argv/stdout/stderr/rc, parsed
  JSON, structure/query/step records, version evidence).
- `policies/` — the 17 generated policy files the run used, including the
  three A8 isolated profiles (`t06-a8-table-comment-required`,
  `t06-a8-column-comment-required`, `t06-a8-table-comment-length`) and the
  all-rules-disabled control.
- `T06-manifest.json` — frozen manifest at 767a19c9 (188→228 additive only).
- `anchors.json` — per-anchor identity (product/image/digest/container/
  version), artifact cli sha256, policy hashes, head_sha, cleanup verdict.
  Binary not stored; `build-identity.txt` carries `go version -m` and the
  recomputed sha256 (matches artifact).
- `baseline-25c4eaa2/` — the authorized offline A8 baseline collected at
  old HEAD `25c4eaa2`: four probe policies, 16 CLI runs (8 roles ×
  mysql/tidb) with argv/stdout/stderr/rc, the real parser/extractor probe
  (module + `replace`, source + `probe-output.jsonl`), `catalog.json`,
  build identity. This is the planning-stage original — it measured the
  byte-count defect (`actual=9` for `中文注`), the quote-wrapped column
  comment, and the missing derived `Table.Comment`.
- `red-25c4eaa2/` — the committed T06A8 tests executed against a worktree
  of the old HEAD: `TestT06A8ColumnEmptyCommentRequired` (empty/blank
  column comments), `TestT06A8TableCommentRuneLength`, and
  `TestT06A8DerivedStateCarriesTableComment` fail there, plus the column
  decoding matrix at the parser seam — the real red phase at field, audit,
  and enrichment levels.
- `EXPECTED-UPGRADES.md` — itemized old→new table for every expectation
  touched by the three changes.
- `gates/` — argv/stdout/stderr/rc captures at the tested SHA:
  `gofmt-check`, `focused-t06a8` (parser+rule+audit T06A8),
  `focused-entries` (SDK/HTTP/CLI/MCP), `make-test` (`go test ./...`),
  `pg-unit-test-gates`, `sql-corpus-gates`, `ddl-inventory-gate`,
  `ddl-coverage-catalog-test`, `docs-example-gates`,
  `decision-record-gate`, `ddl-golden-validator-test` (357 contract cases,
  0 failures), `ddl-golden-T06` (228/228), `golden-validate`, `task-diff`.

## Notes

- The `+dirty` build marker on the baseline binary reflects the repo's
  pre-existing untracked environment directories, not code changes; the
  tested-SHA golden binary hash matches the artifact.
- No database-side comment limits were substituted for the team policy
  parameter; native `CREATE` always succeeded in the anchored roles,
  including the product-side rejects.
- T02–T05 golden regression reuse was authorized for this slice (no
  coverage/catalog change); the shared Go, corpus, and validator suites
  above all ran at 767a19c9.
