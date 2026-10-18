# Decision: T06-A17 declared column type-parameter fidelity

Date: 2026-10-18
Status: Accepted
Related milestone/version: mysql-tidb-ddl-completion (#79), task T06 / issue #85
Related commits: T06-A17 fix commit on milestone/mysql-tidb-ddl-completion
Related tests: `internal/infrastructure/parser/tidb/extractor_t06a17_test.go`, `internal/infrastructure/metadata/mysql/provider_t06a17_test.go`, `internal/application/audit/batch_state_t06a17_test.go`, `internal/application/auditmeta/audit_t06a17_test.go`, `pkg/deltascope/audit_type_facts_t06a17_test.go`
Related docs: T06-A17 external baseline `/tmp/ds-t06-type-facts-baseline/`, EXPECTED-UPGRADES in this slice's evidence

## Context

The T06-A16 closeout left one residual: declared type parameters on the
ordinary-CREATE fact path. The T06-A17 baseline (repository-external probe
on `a0819096`, locked parser `v0.0.0-20260622124716-26db2d85d6cd`) measured
the actual loss layers:

- The **AST is fully faithful**: `GetFlen`/`GetDecimal` sentinels
  (`-1` = unspecified) distinguish `DECIMAL(12,0)` / `DECIMAL(12)` /
  `DECIMAL`, and `GetElems` preserves member case, order, and edge
  literals byte-exactly.
- **Upstream formatted text** (`FieldType.String()`/`CompactStr()`)
  renders parameters but substitutes defaults (`DECIMAL`→`decimal(10,0)`,
  `DECIMAL(12)`→`decimal(12,0)`) and elides explicit `(M,0)` on
  FLOAT/DOUBLE (`FLOAT(10,0)`→`float`).
- **The extractor** carried that text but added its own defect:
  `strings.ToLower` case-folded ENUM/SET member literals, merging
  `enum('Ready','Closed')` with `enum('ready','closed')`.
- **The provider** truncated `column_type` at `(` and parsed only the
  first integer: all scale and all member literals were lost, and an
  `unsigned` substring inside a member literal would falsely set
  `Unsigned`.

`spec.Column` keeps a single `Type` string; no `Precision`/`Scale`/
`Elements` fields exist. The baseline showed the field already carries
parameter text (`varchar(8)`, `decimal(12,2)`), and every consumer
(`baseType`, `columnTypeFamily`, ALTER compatibility echo, A15 length
estimation, `cloneColumn` value copy) reads it compatibly — so the fix
belongs in the two normalization sites, not the schema.

## Decision

- **Extractor: sentinel-faithful rendering for exactly five types.**
  `DECIMAL`, `FLOAT`, `DOUBLE`, `ENUM`, `SET` go through
  `FieldType.Restore` with single-quote + backslash-escape +
  lowercase-keyword flags. Restore writes `(flen[,decimal])` only from
  specified sentinels — `decimal`, `decimal(12)`, `decimal(12,0)` stay
  three distinct strings — keeps the explicit `(10,0)` scale the
  formatter elides, and re-quotes members byte-exact so member case,
  order, empty strings, commas, parens, and quotes survive. Every other
  type keeps the existing `ToLower(String())` text (`int(11)`,
  `bigint(20) unsigned`, charset/collation suffixes).
- **Provider: verbatim parameter span for the same five bases.** A
  quote-aware scan locates the parenthesized span (commas/`)`/`''`
  inside members never end it); `Type` = lowercase base + verbatim span
  + lowercased attribute remainder (`decimal(12,2) unsigned`). `Unsigned`
  is detected on quote-stripped text, so member literals containing
  ` unsigned`/`zerofill` can never leak into attributes. `Length` keeps
  its first-integer meaning; non-target bases keep the bare-keyword
  contract (`varchar(8)`→`varchar`, `int`→`int`).
- **Declaration vs catalog stay distinct layers.** `DECIMAL(12)` declared
  yields `decimal(12)`; a catalog `decimal(12,0)` row yields
  `decimal(12,0)` — the provider preserves what it receives and never
  reconstructs the original spelling, so catalog normalization is not
  laundered into declaration provenance.
- **FLOAT(p) single-argument is the parser's normalization, not ours.**
  `FLOAT(10)` normalizes to `float` and `FLOAT(30)` to `double` with the
  parameter discarded upstream; the extractor reports the normalized
  fact and does not rebuild `p` from SQL text.
- **No new fields, rules, gaps, or unsupported states.** `Type` text is
  the whole contract. Findings that echo `source_type`/`target_type`
  now carry faithful text; rule applicability and family logic are
  unchanged because `baseType` strips at `(`.

## Consequences

- Member case/order and explicit scale are now durable facts in both
  declaration and catalog source columns, and derived batch pre-state
  carries them unchanged (`cloneColumn` needs no new slice fields).
- One same-root expected upgrade: `provider_test.go`'s
  `parseColumnType` table now expects `decimal(10,2)` where it asserted
  the truncated `decimal`.
- Out of scope per contract: precision/scale/member governance rules,
  `DECIMAL` physical-byte formulas, ENUM ordering semantics,
  FLOAT(M,D) deprecation or version policy, PG provider, `Length`
  semantics, and any display-width changes on non-target types.
