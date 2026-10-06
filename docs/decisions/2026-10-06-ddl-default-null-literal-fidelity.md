# Decision: T06-A3 DEFAULT NULL typed literal fidelity

Date: 2026-10-06
Status: Accepted
Related milestone/version: mysql-tidb-ddl-completion (#79), task T06 / issue #85
Related commits: T06-A3 implementation commit on milestone/mysql-tidb-ddl-completion
Related tests: `internal/infrastructure/parser/tidb/extractor_t06a3_test.go`, `internal/application/audit/batch_state_t06a3_test.go`, `pkg/deltascope/audit_default_null_state_t06a3_test.go`, `internal/interfaces/http/audit_default_null_state_t06a3_test.go`, `scripts/test_ddl_golden.py` T06-A3 contract group
Related docs: `testdata/ddl-golden/T06.json`, `testdata/ddl-inventory/T06-base-traceability.md`, `docs/decisions/2026-10-06-ddl-create-primary-key-nullability.md` (supersedes its deferred `"<nil>"` item)

## Context

T06-A3-PLAN traced the `DEFAULT NULL` fidelity loss to a single site:
`normalizedExprText` renders a parsed default expression via
`ast.ValueExpr.GetValue()`. For the typed SQL NULL datum the pinned TiDB
parser driver returns Go `nil`, so `fmt.Sprint(nil)` produced the bounded
literal `"<nil>"`, and `DefaultIsNull` — derived by comparing that text to
`"null"` — could never become true on the MySQL/TiDB path. A2 recorded this
as a deferred boundary.

The proven downstream consumer was the T05-A6 drop-column seam:
`dropOtherColumnMayReference` treats `HasDefault && !DefaultIsNull &&
!DefaultIsCurrentTimestamp` as an unprovable column reference, so a sibling
column declared `DEFAULT NULL` poisoned the prospective state after
`DROP COLUMN` and forced the follower `CREATE INDEX` into the conservative
`unknown_table_state` evidence-gap path — while the provider-loaded view of
the same schema (`COLUMN_DEFAULT = 'NULL'` text → `DefaultIsNull=true`) did
not. Same schema, different verdict by extraction source.

## Decision

The `ColumnOptionDefaultValue` branch in `extractCreateTable`/`extractColumn`
recognizes SQL NULL from the typed AST, not from rendered text:

- a new private `exprIsNullLiteral(expr)` returns true only when the
  expression is a non-nil `ast.ValueExpr` whose `GetValue()` is nil — the
  null datum's unique observable shape in the pinned parser driver;
- `ast.ParamMarkerExpr` is excluded first (it embeds a `ValueExpr` whose
  datum also reports nil, but `?` is not a NULL literal), and non-literal
  expressions (function calls, variables, arbitrary expressions) are never
  evaluated or guessed;
- a recognized NULL literal records `DefaultValue="NULL"` (matching the
  datum's `Format()` rendering and the `information_schema` text convention)
  and `DefaultIsNull=true` — the boolean now binds the typed fact, not a
  string compare;
- everything else keeps `normalizedExprText` output verbatim:
  `DEFAULT 'NULL'` → `'NULL'`, `DEFAULT '<nil>'` → `'<nil>'`,
  `DEFAULT 0` → `0`, `DEFAULT ''` → `''`, absent clause → `HasDefault=false`
  with empty fields. The string literals' quoting is part of the stored
  text, so they can never collide with the NULL marker.

`normalizedExprText` itself is unchanged — its remaining consumers (COMMENT
extraction, `exprIsCurrentTimestamp` fallback) never see a NULL datum. ALTER
column `Definition` extraction shares `extractColumn`, so `MODIFY`/`CHANGE`/
`ADD COLUMN ... DEFAULT NULL` inherits the same facts; ALTER
`SET/DROP DEFAULT` and expression defaults stay unmodeled. `ColumnOptionNull`
(declared nullability) and `ColumnOptionDefaultValue` (default identity)
remain separate channels — `DEFAULT NULL` does not imply `NotNull=false`,
and `DEFAULT NULL` on a PK member keeps `NotNull=true` from the A2
normalization.

No rule, policy, spec field, provider, state machine, or transport changed.
`dropOtherColumnMayReference` keeps the A6 conservative boundary verbatim —
it simply now receives a true `DefaultIsNull` for real NULL defaults.

`testdata/ddl-golden/T06.json` extends the frozen set to 72 cases: the 56
accepted cases verbatim plus 8 CLI matrix cases (four spellings × two
dialects under the A2 default-presence profile) and 8 anchored metadata
cases — `t06-a3-{anchor}-default-representation` audits a four-column matrix
and the structure oracle distinguishes SQL NULL (`COLUMN_DEFAULT IS NULL`
for `a`/`b`) from literal bytes (`HEX` = `4E554C4C`/`3C6E696C3E` for
`c`/`d`), while `t06-a3-{anchor}-null-drop-state` replays the three-statement
CREATE→DROP→INDEX path on every anchor with mid-flight structure checks.

## Public Contract

Consumers can rely on: `DefaultIsNull=true` means the declared default was
literally `DEFAULT NULL` — never a string literal, a parameter marker, or an
expression. `DefaultValue` text conventions are unchanged for non-NULL
defaults (literals keep their quoting); only the SQL NULL literal now
records the canonical `"NULL"` text instead of the bounded artifact
`"<nil>"`. The `ddl.column.default.require` presence contract is unchanged:
any explicit `DEFAULT` clause satisfies it regardless of spelling.

## Deferred / Out Of Scope

- Metadata default-source semantics stay pending: the MySQL provider reads
  `COLUMN_DEFAULT`, where SQL NULL conflates an absent clause with an
  explicit `DEFAULT NULL` (and a stored `"NULL"` text is indistinguishable
  from a literal by text alone). Parse-derived and provider-derived
  `HasDefault` are two evidence layers and are not required to agree
  field-for-field. See the deferred note in
  `testdata/ddl-inventory/T06-base-traceability.md`.
- Expression defaults, `DEFAULT` execution semantics, generated columns,
  temporal implicit defaults, `ColumnOptionNull` on non-PK columns, and the
  default-presence policy's meaning stay with their owning tasks
  (T07/T12/T27–T30).
- No generalization is claimed for "literal defaults are dependency-free" —
  only the SQL NULL literal is provably column-free today.

## Verification Evidence

- `go test -run T06A3` on parser/audit/SDK/HTTP — field matrix on both
  dialects (absent/NULL/'NULL'/'<nil>'/'null'), helper boundary controls
  (nil node, param marker, function call), literal/timestamp/comment
  preservation, ALTER `Definition` sharing, three-statement drop-state path,
  string-literal conservative control, providerless gap preservation, and
  unchanged default-require semantics.
- `python3 scripts/test_ddl_golden.py` — contract suite extended to 72
  required cases plus T06-A3 mutations (slot interchange, NULL-flag flips
  in both directions, HEX swap/corruption, paired-side query deletion,
  statement collapse, revived gap under complete aggregate, cross-anchor
  rebind, offline demotion, profile swap, paired-side case deletion).
- `make ddl-golden TASK=T06` — 72 cases across the four locked anchors
  (evidence recorded with the task evidence commit).

## Consequences

- The `DefaultValue` text `"NULL"` now joins the canonical literals; consumers
  comparing rendered text must use `DefaultIsNull` for the NULL question, not
  string matching — which is also why `'NULL'`/`'<nil>'` keep their quoted
  forms.
- A2-era tests that pinned the bounded `"<nil>"` artifact were corrected to
  the new contract as authorized; the deferral in the A2 decision record is
  superseded by this record.
- The extraction-vs-provider asymmetry at the drop-column seam is resolved:
  a `DEFAULT NULL` sibling no longer forces the conservative path, while
  every other default spelling keeps the A6 boundary.

## Links

- Tests: `internal/infrastructure/parser/tidb/extractor_t06a3_test.go`,
  `internal/application/audit/batch_state_t06a3_test.go`,
  `pkg/deltascope/audit_default_null_state_t06a3_test.go`,
  `internal/interfaces/http/audit_default_null_state_t06a3_test.go`
- Docs: `testdata/ddl-golden/T06.json`,
  `testdata/ddl-inventory/T06-base-traceability.md`
