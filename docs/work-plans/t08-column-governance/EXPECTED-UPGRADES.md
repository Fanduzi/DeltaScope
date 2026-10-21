# T08 expected upgrades

Authored before applying the corresponding old-oracle edits. These are bounded
promotions from the frozen #87 contract, not expectations copied from product
actuals. Existing SQL, role identities, profiles and unrelated negatives stay.
T06 remains 496, T07 remains 207 — no change to their frozen expectations
beyond the rows below, each bound to an exact case/statement/rule_id.

## Bound upgrade rows

- **U-pos** (`internal/application/audit/t03_coverage_…test.go`, four cases):
  `ADD COLUMN … FIRST`, `ADD COLUMN … AFTER col`, `MODIFY COLUMN … AFTER col`,
  `CHANGE COLUMN … FIRST` were in the unsupported/conservative table. Under
  T08 the `column_position` aspect is audited and these cases moved out of the
  unsupported expectations; the new T08 test file owns their audited verdicts.

- **U-pos** (`internal/application/audit/…A5 unsafe/non-precise`): the
  positioned single `CHANGE COLUMN` expectation flipped from tombstone to the
  bounded successor; the multi-action `CHANGE + RENAME TO` statement in the
  same test still tombstones (rename is not a position-capable identity
  transition).

- **U-multi** (`internal/application/audit/…A4`): MODIFY/CHANGE position
  cases formerly classified as conservative-boundary are now provable
  templates and moved to the dedicated T08 test file.

- **U-compat-default** (`internal/application/audit/…A4 omitted attributes`):
  `MODIFY COLUMN c INT NULL` on a column with stored `DEFAULT 0` now emits
  the effective-default transition warning (`literal` → `none`) under the
  compatibility rule — native evidence (all four anchors) confirmed an
  omitted `DEFAULT` clause clears the stored default.

- **U-pos** (`T07` generated-column golden case): `ADD COLUMN … FIRST` on a
  generated-column-dependent fixture now produces the bounded
  `incomplete_alter_dependency_effects` evidence gap and the
  `alter.column_position` gap instead of the generic unsupported marker.
  Same statement, same verdict class — the gap entries were rewritten.

- **U-coverage** (T05 zero-quota / aspect tests): expected unsupported /
  gap counts shrink where the aspect set `alter.column_position` and
  multi-action alter handling are now audited surfaces.

- **U-corpus** (`testdata/sql-corpus` mysql + tidb): three new statements and
  their private fixtures plus the two new rule IDs
  (`ddl.alter.set_default.exists.require`,
  `ddl.alter.drop_default.exists.require`,
  `ddl.alter.column_position.exists.require`) added to the coverage corpus.

## Admission reconciliation (Stage 6)

The frozen `roles.json` denominator is 162. During manifest binding the ids
were mechanically renamed to golden-runner namespaces:

- `T08.db.<anchor>.<role>` → `T08.meta.t08-<anchor>-<role>` (121 cases)
- `T08.err.syntax` → `T08.cli.err-syntax` — parse failures emit a result JSON
  to stdout, so the role is a cli case, not a pre-result clierr case
- `T08.err.unsupported-aspect` → `T08.cli.unsupported-aspect`
- `T08.err.bad-dialect` / `T08.err.provider-failure` → `T08.clierr.*`
- `T08.err.policy-invalid` → `T08.clierr.err-policy-invalid` (missing policy
  config exits before any audit result)
- `T08.err.cancel` stays admitted by the named unit tests

The manifest additionally binds 8 baseline db roles (`T08.db.<anchor>.ddl` /
`.syntax_negative`) and 2 baseline cli roles (`T08.cli.mysql` / `.tidb`) for
anchor health. The executed manifest denominator is therefore 171 =
8 db + 39 cli + 3 clierr + 121 meta (the 162-role denominator minus the
unit-admitted `err.cancel`, plus the 10 baseline health cases).

## Post-review amendments

- **A-tidb-ai-drop** (`t08-{A}-modify-ai-drop`, anchors `all4`): the role
  declared TiDB coverage but the manifest only materialized mysql57/mysql84.
  `T08.meta.t08-tidb85-modify-ai-drop` now binds the native split confirmed in
  `native-matrix.json` — audit emits the same declared-transition warning,
  the statement is rejected with `ERROR 8200`, and the catalog stays the
  fixture shape. Denominator 162 → 163 role ids, manifest 171 → 172,
  ids-sha256 `fb4721077cb94d0a6dab9cc826fea8f16d3bb13615da20d71d9749e22d31b18c`.
  The same review pass tombstoned the TiDB ordered-state successor for
  AUTO_INCREMENT removal (`modifyTiDBAutoIncrementRemoval`) and completed the
  position rule's MySQL accumulated premise set (rename destinations and
  dropped names now resolve/flag sibling AFTER references).

## Second review round (spec axis on 9c4fe175)

- **S-multi-action-duplicate-apply**: `multiActionApply` `add_columns` never
  rechecked the working shape, so TiDB's `RENAME a TO x, ADD x` /
  `CHANGE a TO x, ADD x` published a duplicate-name successor — the apply now
  refuses a name already materialized by a sibling (the server refuses it).
- **S-produced-set-laundering**: `alterProducedColumnNames` counted setter /
  visibility / modify target definitions as produced names, masking a later
  `AFTER missing` premise on the MySQL accumulated set. Only `add_columns`
  and change/rename destinations now count as produced; regression tests
  pin setter-target and modify-target AFTER failures.
- **S-eval-gap-coexist binding**: role `t08-cli-eval-gap-coexist-tidb`
  over-claimed — `lower('X')` is a structurally proven negative, not the
  eval-unproven cell, and the offline case carries no gap. The role's
  declared expectation now binds the blocker case plus
  `TestAuditSQLT08EvalGapCoexistsWithBlocker` for provider-path
  blocker+gap coexistence (the live-anchor cell is unpinnable because the
  boundary is value-dependent).
- **S-extraction-anomaly-wording**: CONTRACT §4 described the parenthesized
  multi-column `ADD (...) + position` anomaly as an "extraction error";
  the shipped mechanism records `UnextractedOptions:["column_position"]`
  on the first emitted alter → incomplete coverage + tombstone. Contract
  wording amended to match.

## Old-golden oracle migration (regression-oracle closure)

The T08 delivery-closeout rerun of the frozen T03/T04/T05 oracles at product
SHA `c983fc9d` surfaced eleven cells whose old expectations still bound the
pre-T08 contract. All eleven belong to the two upgrade classes authorized
above (U-pos and U-compat-default); the migrated expectations below were
derived from the T08 contract and cross-checked against the recorded
product actuals — they were not copied wholesale from actual output. Case
SQL, ids, dialects, anchors, profiles and args are byte-identical to the old
cells; denominators are unchanged (T03 118, T04 163, T05 247).

### Position capability cells (9)

All nine cells ran under each task's default `policy_profile` with no
per-case `policy` override.

| case_id | dialect / sql | old expect | new expect |
| --- | --- | --- | --- |
| `T03.cli.mysql-alter-add-column-position` | mysql `ALTER TABLE t ADD COLUMN c INT FIRST;` | exit 1, review/incomplete, d=1, u=1 | exit 0, pass/complete, d=0, u=0 |
| `T03.cli.tidb-alter-add-column-position` | tidb `ALTER TABLE t ADD COLUMN c INT AFTER id;` | exit 1, review/incomplete, d=1, u=1 | exit 0, pass/complete, d=0, u=0 |
| `T03.cli.mysql-alter-modify-column-position` | mysql `ALTER TABLE t MODIFY COLUMN c INT AFTER id;` | exit 1, review/incomplete, d=1, u=1 | exit 0, pass/complete, d=0, u=0 |
| `T04.cli.mysql-alter-add-column-position` | mysql `ALTER TABLE t ADD COLUMN c INT FIRST;` | exit 1, review/incomplete, d=1, u=1 | exit 0, pass/complete, d=0, u=0 |
| `T04.cli.tidb-alter-add-column-position` | tidb `ALTER TABLE t ADD COLUMN c INT AFTER id;` | exit 1, review/incomplete, d=1, u=1 | exit 0, pass/complete, d=0, u=0 |
| `T04.cli.mysql-alter-modify-column-position` | mysql `ALTER TABLE t MODIFY COLUMN c INT AFTER id;` | exit 1, review/incomplete, d=1, u=1, gaps=1 | exit 0, review/unverified, d=0, u=0, gaps=1 (unchanged) |
| `T05.cli.mysql-alter-add-column-position` | mysql `ALTER TABLE t ADD COLUMN c INT FIRST;` | exit 1, review/incomplete, d=1, u=1 | exit 0, pass/complete, d=0, u=0 |
| `T05.cli.tidb-alter-add-column-position` | tidb `ALTER TABLE t ADD COLUMN c INT AFTER id;` | exit 1, review/incomplete, d=1, u=1 | exit 0, pass/complete, d=0, u=0 |
| `T05.cli.mysql-alter-modify-column-position` | mysql `ALTER TABLE t MODIFY COLUMN c INT AFTER id;` | exit 1, review/incomplete, d=1, u=1, gaps=1 | exit 0, review/unverified, d=0, u=0, gaps=1 (unchanged) |

Old `unsupported_entries` carried `alter_table.add_columns.column_position`
or `alter_table.modify_column.column_position` with reason
"parsed by the shared parser but not covered by audited semantics". T08
makes `column_position` an audited aspect on plain single-column `ADD` and
`MODIFY`, so the unsupported entry and its diagnostic are no longer true.
Two sub-shapes remain distinct, and the manifest keeps them honest:

- **T03** runs `all-rules-disabled`, so nothing evaluates: the audited
  position statement completes with `pass/complete`. This is the product
  result under a policy that enables no rules — it does not claim the
  `AFTER` target exists in any real database.
- **T04/T05** run the modify-column compatibility isolation profile. Their
  `MODIFY … AFTER id` cells keep `verdict=review`, `coverage=unverified`,
  and the preserved `missing_source_column` evidence gap for
  `ddl.alter.modify_column.compatibility.require` (required fact
  `source_column.definition`) — the offline audit still cannot observe the
  provider-side source column, so the protection is not removed, only the
  stale unsupported marker is.

The contract pins these exactly via `position_migration_cases` /
`position_migration_manifest_failures` in `scripts/ddl_golden.py`, wired
into `validate_artifact`; mutation coverage lives in
`scripts/test_ddl_golden.py::t08_oracle_migration_tests`.

### Effective-default transition cells (2)

| case_id | anchor | sql | change |
| --- | --- | --- | --- |
| `T05.meta.t05-a4-mysql84-omit-integer` | mysql84 | `CREATE TABLE t (id INT PRIMARY KEY, c INT UNSIGNED NOT NULL DEFAULT 1 COMMENT 'old'); ALTER TABLE t MODIFY COLUMN c INT; ALTER TABLE t MODIFY COLUMN c INT NOT NULL;` | findings 2 → 3 |
| `T05.meta.t05-a4-tidb85-omit-integer` | tidb85 | same shape | findings 2 → 3 |

Both cells stay `exit=1`, `verdict=reject`, `coverage=complete`,
`fail_on_triggered=true`. The original two findings are kept verbatim:

- index 1, unsigned `source_unsigned=true → target_unsigned=false`;
- index 2, nullability `source_not_null=false → target_not_null=true`.

The added third finding is index 1, rule
`ddl.alter.modify_column.compatibility.require`, level `blocker`, with
metadata `{action: "modify_column", table: "t", name: "c", column_name:
"c", source_default: "literal", target_default: "none"}` — the
U-compat-default upgrade: an omitted `DEFAULT` on `MODIFY` clears the
stored default on all four anchors (native-verified), so the audit now
reports the effective-default transition instead of silently passing it.
The runner and validator treat `finding_entries`/`finding_metadata` at the
same `(index, rule_id)` as an ordered one-to-one list, so the three
findings cannot be collapsed or cross-paired.

Unchanged by the migration: the manifest's native/setup/execute/structure/
post_verify assertions, `expect_rc`s, teardown checks, SQL text, anchors,
policy binding, and every other expectation field; the audit still does not
execute SQL and the driver still replays the three statements in order.

### Rationale

Each migrated cell is capability fulfilment, not protection removal: the
old expectations encoded "unsupported / not covered" or an absence of the
default finding, while the new expectations encode exactly the audited
result the T08 contract specifies — including the gaps T08 still keeps
open (source-column definition under the compat profile). Cells outside
the two authorized classes were not touched.
