# T23 lightweight-stack native run — evidence index

Execution identity
- run-id: `t23r10-20261108t13311794144718z-a1`
- run tree (executed): `00ac3ada62c7cdcd2039c544c15f2d3e19eb79fa`
- evidence archive commit: `a7904f1de7ab56bd0505eaee3abe46b0e149049a`
- manifest sha256 (as executed):
  `c63ea704ae7198379af29152cfa016b28b4d5378e4b59b790547221720a9e6eb`
- environment: approved colima profile
  `colima-ds-t21-20261107t1158z-e514c8` (docker 29.5.2 arm64),
  tidb85 unistore + mysql 5.7.44/8.0.46/8.4.10; no PD/TiKV/TiFlash.

Archive layout
- `00ac3ada62c7/golden/T23-artifact.json` — canonical final artifact
  (35 cases, 503 assertions, all pass).
- `00ac3ada62c7/golden/T23.attempts/00ac3ada62c7/attempt-{1..6}/` —
  per-attempt evidence, 40 files each, byte-identical to the originals
  under `/tmp/t23r10-artifact/` (sha256-verified at archive time).
  Each attempt preserves its own expects/actuals/status; earlier
  attempts were not rewritten to the final expectations.
- `00ac3ada62c7/receipts.json` — lifecycle ledger of the final
  invocation (generations + teardown actions).
- `latest` — pointer file written by the runner (`attempt-6`).

Attempt history (all on run tree `00ac3ada62c7`)
- attempt-1: 16 pass / 19 fail — fixture credential env anchors wrong
  (all anchors unreachable; `SELECT VERSION()` rc=1).
- attempt-2: 19 pass / 16 fail — mysql57/84/tidb85 anchors still
  unreachable (same credential/exec-shape class).
- attempt-3: 24 pass / 11 fail — `golden` database missing on tidb85
  (`ERROR 1049`), clustered-mode oracle assumed `int_only` (real v8.5.0
  default `ON`), stale vendor-diagnostic count.
- attempt-4: 27 pass / 8 fail — clustered mode values uppercase,
  SET GLOBAL/CREATE same-connection scope defect, stale catalog
  column checks (EXTRA vs TIDB_ROW_ID_SHARDING_INFO, NEXT_ROW_ID),
  cache-table teardown DROP refused by upstream guard.
- attempt-5: 34 pass / 1 fail — B-split-region post-verify table count
  6 vs seeded 5.
- attempt-6: 35/35 pass, 503/503 assertions — the accepted result.

Recorded evidence gaps (kept, not reconstructed)
- Four manually deleted anonymous volumes: full names not preserved in
  saved records; deletion predates this index and was a procedural
  deviation from ledger-consumption (see ADR amendment). Do not
  reconstruct names from time/count/shape.
- `A-alter-layout-matrix` clustered-mode restore: `SET GLOBAL ...='ON'`
  rc=0 proven; no post-restore readback exists for that case (later
  cases are mode-agnostic). The other four clustered cases' restores
  are indirectly confirmed by the next clustered case's baseline read.
- Real TiFlash positive/recovery arms and the `alter-primary-key=true`
  axis remain unexecuted (separate authorization + infrastructure).

Archive-vs-execution dates
- Execution occurred on 2026-11-08 (receipt `created_at`
  20261108T13…Z). Attempts 1–5 were added to this archive directory
  after the initial evidence commit; their internal content retains
  original execution-time timestamps and identities.
