# Red evidence — T10-R6-tail

Two provenance layers, honestly labeled:

- **First capture (in-session).** The five R6-tail tests were compiled
  and run on the working tree while it still carried the pre-fix product
  (HEAD `6cec4e6c` plus the uncommitted new test files). All five failed
  with the behavioral assertions described below. Those runs were
  interactive session output and are not separately archived.
- **Worktree replay (archived here).** `audit-replay.stdout.txt` and
  `provider-replay.stdout.txt` are verbatim `-v` runs of the new test
  files copied onto a detached `6cec4e6c` worktree — the same pre-fix
  product, captured to disk. Identical failure set:

  | Test | Concern |
  |------|---------|
  | `TestT10R6DroppedPKStaleStorageUnknownKeepsAbsenceDecided` | (A) dropped-PK stale `PrimaryKeyHandleUnknown`/`PrimaryKeyClusteredUnknown` re-ambiguated the freed `primary` name |
  | `TestT10R6DropPKMemberBoundOwesNoPrimaryKeyGap` | (B) member-owned `primary` owed an unrelated `target_table.primary_key` gap |
  | `TestT10R6DroppedPKUnrelatedNameKeepsAbsenceDecided` | (review finding) `primaryNameMayOwnTarget` re-opened a proven absence on an unrelated ambiguous name-keyed drop |
  | `TestT10R6DropPKFreedPrimaryFiresUnderConstraintsUnknown` | (review finding) the `PrimaryKeyUnknown`/`ConstraintsUnknown` flag gate suppressed the freed-`primary` 1091 finding before binding |
  | `TestT10R6TiDBFoldedClusteredRowKeepsUnproven` | (C) a folded same-name `NO` row overwrote a `NULL` sibling, fabricating a proven-NO clustered answer; the replay failed on the first of the two row orders, and the fixed green run covers both orders |

  `TestT10R6DropPKObjectUnderUnvouchedMembers` and
  `TestT10R6TiDBFoldedClusteredAllNOProvesNonclustered` ran as controls
  and passed on the pre-fix product.

Green counterparts (the same suite on `fd849db5`) live under `../green/`
as `focused-*.log`; the four affected packages' full `go test -count=1`
logs are the remaining `green/*.log` files.
