# T14-R2 gate bundle

Four-tuple convention per gate: `<name>.command.txt` (full argv),
`<name>.stdout.txt`, `<name>.stderr.txt`, `<name>.rc.txt`.

- `l2_committed_range_check.sh` — committed-range variant of the current
  L2 module-README rule (same logic as
  `check-three-level-doc/scripts/check_three_level_doc.sh`, diff source
  switched to `git diff --name-only <range>`). Archived because the
  T14-R1 committed-diff checker script was not recoverable; its rule is
  unchanged.
- `l2-committed-t14r2` — range `17539dbb..<this-SHA>` (T14-R2 product diff).
- `l2-committed-t14-cumulative` — range `b54a0544..<this-SHA>` (cumulative
  T14 product diff, exercises the probes/README convention restored in R1).
- `l2-final-tree` — the skill's own worktree checker on the final tree.
