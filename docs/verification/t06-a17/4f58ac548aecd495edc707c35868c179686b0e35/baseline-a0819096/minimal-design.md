# T06-A17 minimal repair design (design only — no production change this round)

## Corrected layered conclusion (supersedes A16's over-broad claim)

A16 recorded that "AST 和 provider 都静默丢弃" type parameters. Measured reality:

- **AST layer: fully faithful.** `GetFlen`/`GetDecimal` sentinels (-1 = unspecified) distinguish
  `DECIMAL(12,0)` / `DECIMAL(12)` / `DECIMAL`; `GetElems` preserves member case, order, and
  edge literals byte-exactly. `Restore` reproduces declared spelling.
- **Upstream formatted text (`String`/`CompactStr`): params rendered, but two merges** —
  (a) unspecified scale/flen rendered as defaults (`DECIMAL(12)`→`decimal(12,0)`,
  `DECIMAL`→`decimal(10,0)`); (b) explicit `(M,0)` on FLOAT/DOUBLE elided (`FLOAT(10,0)`→`float`).
- **spec.Column (extractor): param text carried, plus one own defect** — the blanket
  `strings.ToLower` case-folds ENUM/SET member literals (`enum('Ready','Closed')` ≡
  `enum('ready','closed')`), and it inherits both upstream merges.
- **Provider: true truncation.** `parseColumnType` (provider.go:493) cuts at `(` and parses
  only the first integer → all scale and all members lost; `decimal(12,2)≡(12,3)≡(12,0)`,
  all enum/set variants merge.

So: the declaration path retains most facts in `Type` text; the provider path loses them
structurally; the extractor's own loss is member-case folding only.

## Is existing `Type` sufficient? — Yes, no new fields needed

Evidence: Type already carries parameter text on the declaration path today
(`varchar(8)`, `decimal(12,2)`, `enum('ready','closed')` flow through golden-green consumers).
`baseType`/`columnTypeFamily` (common.go:496/872) strip at `(`/space, so parameterized text is
the consumed norm, not an exception. `spec.Column` is cloned by `cloneColumn` as a value type —
a faithful `Type` string needs no ownership/deep-copy work. `Length` stays first-param flen,
unchanged on both paths (parser: 12 for `decimal(12,2)`; provider: 12 for `decimal(12,x)`).

Unknown vs explicit-zero vs empty distinctions needed by this fixture group are resolvable
in Type text when Type is rendered from AST sentinels rather than formatted defaults:
`decimal` vs `decimal(12)` vs `decimal(12,0)` are three distinguishable strings;
`enum('','a,b')` keeps the empty member as `''`; `ENUM()` (empty list) is not expressible in
MySQL grammar so "empty list" needs no representation. Catalog-source cannot recover
declaration spelling beyond what `column_type` encodes — verbatim parameter text is the
provider's honest limit (documented boundary, not a gap to hide).

## Recommended change (two surgical edits + tests)

1. **Extractor** (`internal/infrastructure/parser/tidb/extractor.go`, `extractColumn`):
   replace `strings.ToLower(col.Tp.String())` with a small sentinel-aware renderer:
   - `ENUM`/`SET` (AST `GetElems() != nil`): `lower(keyword)` + `(` + verbatim re-quoted
     members (`'`→`''`) + `)` — preserves declared member case/order/literals exactly.
   - `DECIMAL`/`FLOAT`/`DOUBLE`/`REAL` family: render params from sentinels —
     `flen==-1` → bare keyword (kills phantom `decimal(10,0)`);
     `dec==-1` → `keyword(flen)` (preserves `DECIMAL(12)` spelling);
     else → `keyword(flen,dec)` (keeps explicit `(10,0)` upstream elides).
   - All other types: keep current `ToLower(String())` — `int(11)`, `bigint(20) unsigned`,
     charset/collation suffixes unchanged (C-controls recorded unchanged per contract).
   ~30 lines, extractor-local; `Length`/`Unsigned` untouched.

2. **Provider** (`internal/infrastructure/metadata/mysql/provider.go`): stop truncating —
   set `Column.Type` to lowercased keyword + **verbatim** parameter substring of the raw
   `column_type` (do not lowercase inside `(...)`; member literals from catalog keep their
   case, matching extractor output for the same spelling). Keep first-int `Length` and
   `unsigned` flag behavior. `parseColumnType` either gains a `typeText` return or
   `loadColumns` derives Type from the raw row — either way ~10 lines.

## Consumer compatibility (source-code deduction; enrich path not re-run)

- `baseType`/`columnTypeFamily`: strip at `(` — unaffected by faithful param text.
- A15 `estimatedColumnBytes`: dispatches on `baseType`, uses `Length` — unchanged.
- `cloneColumn`/`derivedCreateShape`: string field copies by value — no change.
- ALTER compatibility findings carry `source_type`/`target_type` metadata verbatim —
  values become more faithful (`enum('Ready','Closed')`, `decimal(12,2)` from provider);
  matching logic (baseType comparisons) unaffected.
- No rule, finding id, coverage, unsupported, or gap contract changes; no new rules.

## Expected file surface & regression scope

- `internal/infrastructure/parser/tidb/extractor.go` + extractor tests (fixture assertions
  for N1/N2/L1/L2 distinctions; existing enum/decimal Type assertions updated).
- `internal/infrastructure/metadata/mysql/provider.go` + `provider_test.go`
  (`TestParseColumnType` table at :400 — expected values change from truncated `decimal`
  to faithful `decimal(10,2)`; add member-case/verbatim-param cases).
- Application-level: one AuditSQL seam test asserting `spec.Column.Type` faithfulness
  through the real pipeline (no public output change).
- Direct regression: `go test ./internal/infrastructure/parser/tidb/...`,
  `.../metadata/mysql/...`, `.../application/audit/...`, `.../domain/rule/ddl/...`;
  golden metadata spot-check on cases whose findings embed column Type.
- Explicitly out of scope: A15 formulas, `Length` semantics, unsigned flag, ALTER policy,
  PG model, denominators, validator catalog.

## Rejected alternative

Adding `Precision`/`Scale`/`Elements` fields: unnecessary — the observed facts fit in Type
text once the two defect sites stop losing them; would impose clone/copy burden on
`cloneColumn`, widen spec contract, and duplicate what catalog `column_type` already
approximates — for zero added consumer (no rule reads scale/members today).
