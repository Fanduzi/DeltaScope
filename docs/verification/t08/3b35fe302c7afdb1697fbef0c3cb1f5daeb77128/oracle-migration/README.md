# Old-golden oracle migration — semantic diff and oracle synchronization stabilisation

Eleven frozen cells migrated in place, all inside the two authorized T08
upgrade classes. `cells.json` is the authoritative per-cell record
(task, full case_id, dialect/anchor/policy/sql/args, complete old expect,
complete new expect). `manifest-diff.txt` is the verbatim git diff of the
three manifests; `oracle-diff.txt` is the git diff of the contract code.

## Changed expect fields only

For every cell, fields outside `expect` (id, sql, dialect, anchor, profile,
args, connect, setup/execute/structure/post_verify, statement_sql,
required_case_ids, denominators) are byte-identical — verified by a
field-by-field comparison that asserts nothing outside `expect` moved.

### Position cells (9) — U-pos

| cell | old | new |
|---|---|---|
| T03 three `*-column-position` | exit 1, review, incomplete, d=1, u=1 (+1 unsupported entry) | exit 0, pass, complete, d=0, u=0 |
| T04/T05 `*-add-column-position` ×2 each | exit 1, review, incomplete, d=1, u=1 | exit 0, pass, complete, d=0, u=0 |
| T04/T05 `mysql-alter-modify-column-position` | exit 1, review, incomplete, d=1, u=1, gaps=1 | exit 0, review, unverified, d=0, u=0, gaps=1 preserved verbatim |

The T04/T05 modify cells keep `verdict=review`/`coverage=unverified` and
the `missing_source_column` gap — the compat profile's unproven boundary
is kept, not laundered to complete. T03's all-rules-disabled profile has
no compat rule enabled, so its modify cell completes.

### Effective-default cells (2) — U-compat-default

`T05.meta.t05-a4-{mysql84,tidb85}-omit-integer`: findings 2→3; the two
original findings (unsigned at index 1, nullability at index 2) are kept
verbatim and the new index-1 blocker pins
`source_default:"literal" → target_default:"none"` on the full metadata
map (`action`, `table`, `name`, `column_name` included). exit 1 /
reject / complete / fail_on_triggered unchanged.

## Independent oracle synchronization

- `t05_a4_contract()` regenerates both omit-integer expects including the
  new default-transition finding and its ordered `(index, rule_id)` →
  metadata pairing; `t05_a4_manifest_failures` diffs the manifest against
  it exactly.
- `position_migration_cases(task)` +
  `position_migration_manifest_failures(manifest)` pin all nine position
  cells (both shapes) inside `validate_artifact` — manifest drift,
  deletion, or expectation weakening fails before artifact content is even
  considered.
- Synchronization proof: `python3 -c` contract-vs-manifest equality check
  reported zero failures for T03/T04/T05 position and all T05 a3–a6
  contracts after the migration; `gates/ddl-golden-validator-test.log`
  runs the full suite (758 cases, 0 failures) including the 24 new
  `t08_oracle_migration_tests` mutations through the complete
  `validate_artifact` entry.

## What the mutations prove

- Pre-T08 `unsupported`/`incomplete` revert on any position cell →
  rejected (`no unsupported entries` / coverage checks).
- Deleting the preserved `missing_source_column` gap while laundering
  coverage to `complete` → rejected.
- Dropping any of the three omit-integer findings, duplicating one,
  cross-pairing metadata, wrong statement/column/transition class → all
  rejected.
- Rebinding policy profile, case id, or statement SQL → rejected.
- Deleting a migrated cell from manifest AND artifact together → rejected
  by the independent contract (denominator protection), as is weakening
  the manifest expectation alone.
