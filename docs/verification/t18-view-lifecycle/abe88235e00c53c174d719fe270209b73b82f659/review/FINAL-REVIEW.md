# T18-R1-tail review handoff

Product: `abe88235e00c53c174d719fe270209b73b82f659`

Red baseline: `19286f7ff73b79f6fca1cc76d11074f3aee751f2`

## Fixed issues

- P1: `Updatable` is now published only from positive proof. TEMPTABLE, a statically non-updatable definition, a non-mergeable effective result, or a proven non-updatable view source yields `false`; unknown algorithm/source updatability stays unpublished. CHECK OPTION and view-state publication share `spec.ViewSnapshotNonUpdatable`.
- P2: an unresolved ALTER merges option attributes branch-by-branch. Prior MERGE plus possible UNDEFINED success publishes unknown; equal branch values survive; proven refusal preserves prior state and proven success publishes final.
- Archive isolation: the replay source snapshot is `.go.txt` with identical bytes, preventing Go package discovery from compiling evidence.

## Golden

- T16: 60/60 cases, 832 assertions.
- T17: 52/52 cases, 834 assertions.
- T18: 150/150 cases, 3,778 assertions (+1 case, +23 assertions from `mysql80-check-temptable-state`).

## Boundaries

- No T19 work, PR, tag, release, deployment, or reviewer-acceptance claim.
- No credential values are archived; only environment variable names and sanitized command shapes.
- Companion/internal projections are separate from Golden public counts.
