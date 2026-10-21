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
