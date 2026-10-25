# T11 red evidence — replay on pre-T11 product 8b6f3098

Red proof was produced by replaying the T11 manifest on a detached
worktree at `8b6f30984d1c610ff44d133e30178fd3a1c165ce` (the T10 evidence
head = parent of the T11 product commit). This is a replay, not the
original development run.

## Two tiers

### 1. registry-red.stdout.txt

Running the full committed `testdata/ddl-golden/T11.json` on 8b6f3098
fails immediately:

```
[ddl-golden][FAIL] manifest policy profile 't11-fk-exists' enables unknown rule: ddl.alter.drop_foreign_key.exists.require
```

The new rule `ddl.alter.drop_foreign_key.exists.require` does not exist
in the pre-T11 registry — profile instantiation itself is the red for
the eight acceptance rows bound to the `t11-fk-exists` /
`t11-fk-exists-notice` profiles
(`t11-cli-dropfk-exists-offline`, `t11-mysql80-ordered-drop-present`,
`t11-mysql80-dropfk-missing`, `t11-mysql80-dropfk-provider-present`,
`t11-mysql80-dropfk-constraints-unknown`,
`t11-mysql80-dropfk-check-bearing`,
`t11-mysql80-create-bare-fk-drop-derived`,
`t11-mysql80-rename-parent-invalidates`).

### 2. golden-replay.stdout.txt + T11-replay-artifact.json

For a behavioral red the replay manifest removed only those two
profiles and their eight cases (32 of the 40 shipped cases remained;
`required_case_ids` pruned accordingly). Result on 8b6f3098:

- **32 cases, 17 passed / 15 failed, 386 assertions**
- artifact `head_sha=8b6f30984d1c`

Representative real reds (not fabricated — actual CLI output):

- `t11-cli-fk-alter-firstpath` / `t11-mysql80-firstpath` /
  `t11-tidb85-firstpath`: `applicable=0`, `findings=0`,
  `verdict=pass`, exit 0 — the frozen first golden
  (`ALTER TABLE child ADD CONSTRAINT fk_c FOREIGN KEY (c) REFERENCES
  parent(id)`) produced no `ddl.table.foreign_key.forbid` blocker at
  all on the pre-T11 product.
- `t11-mysql80-composite-actions`: `ON DELETE SET NULL ON UPDATE
  CASCADE` surfaced as
  `unsupported=alter_table.add_constraint.constraint.foreign_key.parts`
  (refer actions unmodeled).
- `t11-mysql80-drop-needed-index`: `coverage=unverified`, three
  evidence gaps — no FK/index dependency knowledge pre-T11.
- FK naming and `pk columns.max_count` ALTER paths: `applicable=0`
  (rule never selected `alter.Constraint` subjects).

The 17 replay passes are existing-green controls (CREATE-table FK
forbid, add-constraint notice, drop controls, off-profile) — they were
already satisfied before T11 and stayed green.

## Why replay instead of preserved dev logs

The intermediate development golden runs (32/38 then 35/38 passes on
the working tree) were overwritten by later runs in the same artifact
directory before this archive existed. The replay binds the same
behavioral delta to an exact committed baseline and is independently
reproducible:

```
git worktree add /tmp/t11-red-wt 8b6f30984d1c610ff44d133e30178fd3a1c165ce
cp testdata/ddl-golden/T11.json /tmp/t11-red-wt/testdata/ddl-golden/
cd /tmp/t11-red-wt && T11_MYSQL_PASSWORD=root \
  python3 ./scripts/ddl_golden.py run --task T11 --artifact-dir /tmp/t11-red-golden
```
