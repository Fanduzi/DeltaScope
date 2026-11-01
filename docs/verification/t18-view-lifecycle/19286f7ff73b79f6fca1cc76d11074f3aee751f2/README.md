# T18-R1 review evidence

This archive is bound to product commit
`19286f7ff73b79f6fca1cc76d11074f3aee751f2`. It contains the independent-review
follow-up evidence for the four findings in GitHub issue #97: target-schema
premise consumption, CHECK OPTION static updatability, inherited-algorithm
fallback, and layered correlated-subquery scope resolution.

The previous `2ab3f181f0bfefbeb5566a6b9358bdc030ab58eb` archive is preserved
unchanged and is not evidence for this product SHA.

## Layout

- `baseline/`: branch, remote, parent, worktree, untracked foreign paths, and
  Docker resource observations taken before and after the task.
- `red/`: replay of the R1 tests on unfixed product `2ab3f181...` in an isolated
  worktree. The direct internal replay records a compile failure because the
  fixed SHA introduces the `TargetSchemaAbsent` metadata field; the compatible
  replay source records the real semantic failures for all four defects.
- `green/`: focused internal companion and public Audit tests on product
  `19286f7f...`.
- `native/`: bounded MySQL 8.0.46 disposable-container observations and cleanup
  receipts. Native facts inform the product assertions but are not a substitute
  for internal projection tests.
- `golden/`: final passing T16/T17/T18 artifacts and policies on `19286f7f...`,
  plus `summary.json` with artifact and CLI identities, case-set checks, and
  cleanup records.
- `gates/`: final gate command/stdout/stderr/rc/identity receipts, lint
  diagnostic-identity comparison, and the committed product path list.
- `attempts/`: failed T18 Golden attempts caused by missing environment
  injections. These are diagnostic records only and are not passing semantic
  evidence.
- `review/`: independent-reviewer handoff material.
- `COPYMANIFEST.json`: byte-level source/destination provenance for copied
  receipts.
- `evidence.sha256.txt`: SHA-256 inventory for every archived file.

## Result summary

- Red replay on `2ab3f181...`: internal compatible run rc=1; public Audit run
  rc=1. The failures cover F1–F4 and the positive controls remain passing.
- Fixed product `19286f7f...`: focused internal and public suites pass.
- Golden on `19286f7f...`: T16 60/60 and 832 assertions; T17 52/52 and 834
  assertions; T18 149/149 and 3,755 assertions. Required and executed case sets
  match exactly; no duplicate case IDs; cleanup rc=0; no residual containers.
- Final gates: all non-lint gates rc=0. The two inherited lint commands return
  rc=1, but diagnostic identity comparison against the accepted T17-tail
  baseline is rc=0 with 22 scoped and 39 full-repository diagnostics unchanged.
