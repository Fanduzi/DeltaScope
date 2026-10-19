# T07 expected upgrades

Authored before applying the corresponding old-oracle edits. These are bounded
promotions from the frozen #86 contract, not expectations copied from product
actuals. Existing SQL, role identities, profiles and unrelated negatives stay.
T06 remains 496, with no change to its ordinary type/default/null/comment/row
size or A17 contracts. Further same-root test rows are appended explicitly.

## Frozen old Golden roles

### U01 — ordinary generated CREATE is no longer unmodeled

Files: `testdata/ddl-golden/T03.json`, `T04.json`, `T05.json`.
Exact case: `mysql-create-table-generated-column` (one statement, index0).
SQL remains:

```sql
CREATE TABLE t (a INT, b INT GENERATED ALWAYS AS (a+1) STORED);
```

Policy remains all-rules-disabled, args remain `--fail-on none`.

| Field | Old | New |
|---|---|---|
| exit | 1 | 0 |
| verdict | review | pass |
| statements/findings | 1 / 0 | unchanged |
| diagnostics/unsupported | 1 / 1 | 0 / 0 |
| aggregate / statement0 coverage | incomplete | complete |
| unsupported feature/entry | create_table.column.generated / unaudited reason | empty |
| statement_sql | exact original | unchanged |

Reason: typed generated facts and local AST dependency extraction now cover the
form. Missing target version does not create a gap for a disabled compatibility
rule. Do not alter index visibility, inline references/CHECK/UNIQUE, procedure,
AUTO_RANDOM or other unsupported controls in these manifests.

### U02 — MySQL generated ADD no longer poisons a known table

File: `testdata/ddl-golden/T05.json`.
Case: `t05-mysql84-unaudited-invalidation`.
SQL/profile remain exact; statement2 adds STORED generated g referencing id,
statement3 adds ordinary c to t and statement4 adds c to independent u.

| Field | Old | New |
|---|---|---|
| exit/verdict | 1 / review | 0 / pass |
| diagnostics/unsupported | 1 / 1 | 0 / 0 |
| statement_coverage | complete,complete,incomplete,unverified,complete | five complete |
| aggregate coverage | incomplete | complete |
| statement3 gaps | table-existence and add-column-existence unknown_table_state | none |
| findings / fail_on_triggered | 0 / false | unchanged |
| unsupported entry | statement2 add_columns.column.generated | removed |

Reason: MySQL8.4.10 independently accepts ALTER ADD STORED; complete declaration
facts can conditionally publish g and preserve existing id/identity/index facts.
This does not authorize recovery from an unrelated poisoned state or a stale
provider reread. Do not change the SQL to manufacture a different unsupported
clause. Other unknown-state controls remain active, and T07 C02/C08 keep explicit
unknown-effect/dependency negatives.

### U03 — TiDB generated ADD is modeled but its known-invalid shape stays unknown

File: `testdata/ddl-golden/T05.json`.
Case: `t05-tidb85-unaudited-invalidation`.
Same SQL/profile as U02; observed target is TiDB8.5.0.

| Field | Old | New |
|---|---|---|
| exit | 1 | 0 (gap remains below blocker fail threshold) |
| verdict | review | unchanged |
| diagnostics/unsupported | 1 / 1 | 0 / 0 |
| statement_coverage | complete,complete,incomplete,unverified,complete | complete,complete,complete,unverified,complete |
| aggregate coverage | incomplete | unverified |
| statement3 gaps | exact two existing gap identities/facts | unchanged |
| findings / fail_on_triggered | 0 / false | unchanged |

The isolated old profile disables the new compatibility rule, so there is no
new policy finding. Independent known TiDB ALTER ADD STORED rejection still
prevents a fabricated successful post-shape. The t gap and u isolation remain;
this negative is not silently converted to MySQL's positive.

## U04 — internal source/model/test representation

- MySQL generated declarations: UnextractedOptions `generated` becomes a typed
  Generated fact with storage and known references. Other option markers stay.
- Parenthesized valid DEFAULT expressions and column visibility enter the real
  adapter grammar rather than parser-failure/unsupported placeholders.
- MySQL/TiDB default actions change `alter_column` to `set_column_default` or
  `drop_column_default`. Existing PostgreSQL `set_default`/`drop_default` and
  their default forbid rules do not acquire MySQL/TiDB applicability.
- Controlled MySQL provider COLUMNS fixtures add the ninth selected value,
  GENERATION_EXPRESSION, empty for ordinary columns. This is a query row-shape
  upgrade, not a production fallback accepting truncated results.
- Existing default-presence policy excludes genuine generated columns rather
  than requiring their illegal ordinary DEFAULT. Ordinary/SQL NULL/current-time
  checks stay unchanged.
- Inventory row `mysql.alter-table.alter-column-default` keeps T08's general
  default-change policy-equivalence/unknown-prior-state responsibility; its
  source narrative must mention T07's advanced-expression fact/state seam.

Exact affected software test/corpus paths are recorded after source inspection
and before any further oracle edit; a blanket expected-output regeneration is
not authorized.

## U05 — existing JSON-generated corpus facts

File: `testdata/sql-corpus/mysql/ddl/boundary/create_table_json_generated.expected.yaml`.
The SQL remains unchanged. Its two generated columns use the existing
JSON_UNQUOTE/JSON_EXTRACT and UNHEX/SHA2/CAST forms; these are recognized
static deterministic structure, with payload references and no reference from
the `$.type` string literal. No expression or hash is evaluated.

`expect.unsupported.count` changes 2 -> 0 and the generated unsupported include
set becomes empty. The existing ordinary-column/comment/not-null/audit-role
finding requirements and JSON/PK exclusions remain. The new compatibility
rule must not appear as a finding when target version is absent; missing
version is an Evidence Gap, with shared T07 tests covering its exact rule/fact/
coverage shape. The current corpus DSL does not expose gap/coverage assertions;
it is not extended merely to duplicate the shared-semantic matrix.

## U06 — preserve old software-test SQL during promotion

`coverage_t03_test.go` row `mysql create table generated column` retains the
exact `(a+1) STORED` input. Move/mark that unchanged input as the supported
contrast with zero generated unsupported items and complete coverage under its
all-off profile. Add a separately named unknown-function generated expression
negative if needed; never replace the old input with T07_UNKNOWN_FN to keep its
old unsupported expectation.

`batch_state_test.go` case `TestBatchStateUnauditedMutationInvalidatesOnlyItsTable`
retains its exact `ADD COLUMN g ... (id+1) STORED` SQL and isolated old policy.
Its MySQL source facts are now modeled: nil unsupported error, no generated
aspect, no fabricated unknown-state gaps for following adds to t/u. Preserve a
separate newly named unknown-function mutation/isolation negative. This is the
software counterpart of U02, not deletion or weakening of that old negative.

U06 application note: the `coverage_t03_test.go` unsupported-aspects table
requires `ErrUnsupportedStatement` plus a named unaudited feature in every row,
so a supported contrast cannot live in that table. The row's SQL was replaced
with the contract's unknown-function vehicle
(`T07_UNKNOWN_FN(a)` -> `create_table.column.expression_dependencies`), which
keeps a valid unsupported-boundary row without laundering the obsolete
`create_table.column.generated` expectation. The unchanged `(a+1) STORED`
input remains covered as the supported contrast in the clean-audit table of
the same file (asserts complete coverage and zero unsupported details) and
through the frozen U01-U03 golden roles.

## U07 — four new corpus rule/dialect representatives

Add `advanced_column_dependency` and `advanced_column_compatibility` under both
mysql/tidb `ddl/findings` (four cases, not new Golden identities). Dependency
uses the unchanged two-statement #86 seed with fixed observed version metadata;
its include is the stable dependency rule and compatibility finding excluded.
Compatibility uses a static generated RAND() declaration: recognized forbidden
nondeterministic structure, no function execution, unsupported0, stable
compatibility rule finding and no dependency finding. These exact role/oracles
are lead-authored; no actual is used to generate expected findings.

## H01 — same-contract manifest/harness integration corrections

The frozen 207 role IDs and ordered digest do not change.

1. The existing runner chooses `cli_cases` instead of expanding `cli_audit` when
   the former exists. Materialize the two already-required mysql/tidb baseline
   rows in cli_cases, so all 207 IDs actually execute. The list has 36 entries:
   2 baseline + 34 offline semantic roles, not a new denominator.
2. The disabled rule_summary contract is an absent JSON key, not a present
   object containing zero counters. Remove incompatible optional generic
   counter pins for disabled roles; the independent full-entry validator still
   rejects null/false/empty/present keys, while required=false remains loaded
   with applicable0. This fixes the positive control, not a weakened rule.
3. TiDB V04's retained ordinary table permits both SET INVISIBLE and SET VISIBLE
   native rejections to execute. Keep both frozen audited statements and add
   the previously omitted second rejection driver record/intermediate schema.
   Never skip that required native outcome or reduce the case count.
4. T07 setup requires successful prior COUNT0 before creating; only a successful
   creates_fixture step claims ownership. COUNT/version/setup errors and races
   preserve unowned resources. Primary and fallback cleanup errors are both
   recorded; credentials enter Docker/mysql only through environment.

The first full-entry constructed-control failure and corrected passing run are
retained under `/tmp/ds-t07-proof/contract-records/`; sensitivity results are
not product/database acceptance evidence.

## U08 — diagnostics count mirrors the unsupported-entry count

The frozen T07 oracle pins `diagnostics` equal to the number of unsupported
entries (e.g. `t07-tidb85-v04-alter-visibility`: two visibility rejections ->
`unsupported=2`, `diagnostics=2`). The production path now emits one bounded
unsupported-statement diagnostic per entry, so older manifests whose cases
carry more than one unsupported entry move from the legacy aggregate `1` to
the per-entry count. Only the `diagnostics` counter changes; SQL, IDs,
profiles, verdicts, exits, coverage, findings and unsupported entries stay.

Files: `testdata/ddl-golden/T03.json`, `T04.json`, `T05.json` — the same 19
`cli_cases` in each manifest (57 case expectations):

| Case | Old | New |
|---|---|---|
| mysql-alter-algorithm-lock | 1 | 2 |
| mysql-create-table-tidb-only-options | 1 | 2 |
| mysql-create-table-mixed-uniqueness | 1 | 2 |
| mysql-create-index-expr-prefix-desc | 1 | 3 |
| tidb-create-index-expr-prefix-desc | 1 | 3 |
| mysql-alter-user-current-auth | 1 | 2 |
| tidb-alter-user-current-discard | 1 | 2 |
| mysql-alter-user-current-function | 1 | 2 |
| tidb-create-user-current-function | 1 | 2 |
| mysql-create-index-comment-keyblock | 1 | 2 |
| mysql-two-expression-indexes | 1 | 2 |
| mysql-create-view-combined | 1 | 5 |
| mysql-grant-with-grant | 1 | 2 |
| mysql-grant-require-ssl | 1 | 2 |
| tidb-grant-on-procedure | 1 | 2 |
| mysql-revoke-column-privileges | 1 | 2 |
| mysql-alter-user-dual-password | 1 | 2 |
| tidb-create-placement-policy-or-replace | 1 | 2 |
| mysql-alter-user-current-retain | 1 | 3 |

Reason: the frozen per-entry diagnostics contract. Cases with a single
unsupported entry, parse-failure diagnostics, or zero unsupported keep their
existing counts.
## U09 — pinned exec client charset for expression HEX whitelists

MySQL records string literals inside generated/default expressions in
`information_schema` with the DDL *session's* connection charset introducer
(`_utf8mb4\'a\'` under a utf8mb4 session, `_latin1\'a\'` under the image's
latin1 client default). The collected native evidence used utf8mb4 sessions,
so the frozen HEX whitelists name `_utf8mb4` forms. The first serial run
showed the unpinned `mysql` exec client negotiating latin1 and producing
`_latin1` introducers on 8.0.46/8.4.10 — an environment nondeterminism, not a
semantic disagreement.

The generator now appends `--default-character-set=utf8mb4` to every anchor's
`exec_client` so DDL sessions reproduce the recorded evidence deterministically.
No expectation, HEX whitelist, case identity, profile, or verdict changes; the
207-identity digest is unchanged. Verified live: mysql:8.0.46 stores
`HEX='5F757466386D62345C27615C27'` for `AS ('a') VIRTUAL` under a pinned
utf8mb4 client, exactly the collected evidence form.

Files: `scripts/t07_contract.py` (anchor construction),
`testdata/ddl-golden/T07.json` (regenerated, anchor `exec_client` only).
## U10 — R1 semantic closure contract amendment (§11)

Two missing premises frozen in the original contract are closed by same-task
rework, driven by new bounded native evidence on the pinned anchors. No frozen
identity, SQL, profile, or verdict changes; the 207-identity digest is
unchanged.

R1-1 generated reference premises: the freeze validated version/function/
alter-transition cells but never declared how declaration-time reference
premises classify. New bounded reasons: `generated_self_reference`,
`generated_missing_column`, `generated_column_order` (all four anchors:
3107/1054/3107). Ordinary-column references at any ordinal and owner-qualified
same-table binds are legal; unprovable candidates stay bounded gaps.

R1-2 whole-shape boundary: the frozen `uuid()` positive required only the
function-name set; native probing shows TiDB chooses between verbatim and
evaluate-and-store paths on the *authored* root node. New facts
`ExpressionFacts.TopForm` + `RootParenthesized` and
`Column.DefaultParenthesized`; rescoped `tidb_alter_json_default` from
"JSON-function-on-ALTER" to BLOB/TEXT/JSON target type (1101 evidence);
verbatim-expression positives are target-agnostic — a single bare allowed
call stores the expression verbatim on every non-BLOB/TEXT/JSON target
(uuid() observed verbatim on sixteen type bases across every family;
json_object/json_array observed on varchar/int/decimal/date/timestamp);
`(current_timestamp)` stores the verbatim temporal default on
TIMESTAMP/DATETIME/DATE, folds to a literal on varchar/char/bigint/double/
time, and rejects 1067 on int/decimal/year; root-wrapped shapes evaluate —
`((uuid()))` folds on varchar/char/binary/varbinary and rejects on
numeric/enum/set/time (1067/1292), `((lower))` evaluates on varchar despite
the bare-path 3770 gate, `((cts))`/`((now()))`/`((json_object))` accept but
store a NULL catalog (bounded gap); ref-free `CASE` of uuid shares the
uuid-fold table (1067 numeric); constant arithmetic folds on string,
numeric, time, enum, set, bit, and year targets; `uuid()` joins the CREATE
allowed set (observed positive); folded positives publish literal presence,
temporal cells publish the catalog temporal default, verbatim cells keep
typed facts; every compat-rule determined negative now sets
`InvalidTransition`; MySQL bare non-literal defaults get the new
`expression_default_shape` negative (1064 on 8.0/8.4); new bounded gap
required-facts `source_column.default_function`,
`source_column.default_composite`, `alter.column_position`.

Files: `docs/work-plans/t07-advanced-columns/CONTRACT.md` (§11),
`internal/domain/spec/advanced_columns.go`, `internal/domain/spec/ddl.go`,
`internal/infrastructure/parser/tidb/expression_facts.go`,
`internal/infrastructure/parser/tidb/extractor.go`,
`internal/application/audit/batch_state_modify.go`, R1 tests and proof.
