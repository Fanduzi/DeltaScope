# T16 gates — evidence records

This directory archives the gate tuples for the T16/#95 evidence bundle.
Each `<name>.command.txt` records the exact command, `<name>.rc.txt` the
return code, and `<name>.stdout.txt` / `<name>.stderr.txt` the captured
streams — see `INDEX.md` for the summary table.

`l2_committed_range_check.sh` is a committed helper script (copied from
the T15 bundle verbatim) that applies the worktree three-level-doc L2
rule to a committed git range; this README exists because the committed
range itself contains this `.sh` file, so the module-directory rule
applies to `gates/` as well.
