# T02 reuse record (delivery closeout)

Per the closeout work order §六B: T02 may reuse an already-accepted original
when its entry, runner, anchor and consumption paths did not change.
No T02 rerun was performed at c983fc9d.

## Reused original

- Artifact: `docs/verification/t07/aba4478328875790e39ed6aeaae0ad67c6d5a996/regressions/T02/artifact.json`
- Run era: T07 task window (tested SHA `aba44783`), 10/10 cases passed
- Manifest bound: `testdata/ddl-golden/T02.json`, required ids
  `T02.db.<anchor>.ddl|syntax_negative` x4 anchors + `T02.cli.mysql|tidb`
- Manifest last content change: `91ccb103` ("feat: establish four-version
  official DDL inventory and executable baseline") — unchanged since the
  reused run and unchanged on the T08 branch.

## Why paths are unchanged in the T08 diff (65b88f98..c983fc9d)

- `scripts/ddl_golden.py` delta is T08-scoped only: it imports
  `t08_contract.t08_artifact_failures` and invokes it inside
  `validate_artifact`; `t08_artifact_failures` returns `[]` immediately for
  any artifact/manifest whose `task_id != 'T08'` (scripts/t08_contract.py:869).
- `docker/ddl-golden-compose.yaml`: zero diff in the range — same four
  anchors, images and digests.
- T02 cases exercise baseline create/alter/drop + per-anchor syntax negative
  + cli dialect audit. T08 touched `ALTER TABLE` column semantics
  (position, defaults, multi-action state) — none of which T02's bound
  statements contain.
- Same-shape baseline cells were re-verified live at c983fc9d inside the T08
  golden run: `T08.db.<anchor>.ddl` + `T08.db.<anchor>.syntax_negative` (8/8)
  and `T08.cli.mysql` / `T08.cli.tidb` — see `../../golden/T08/artifact.json`.

## Classification

`有效可复用 (valid reuse)` — original exists, manifest unchanged, runner delta
proven T08-scoped, anchor set unchanged, consumption path unchanged.
