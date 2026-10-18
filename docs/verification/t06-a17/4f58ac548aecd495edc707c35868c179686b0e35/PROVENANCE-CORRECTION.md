# T06-A17-R1 correction of historical execution attribution

This correction changes attribution, not the original observations. The
historical logs, artifact, case records, binary hashes, baseline originals,
and pass/fail results in this directory remain unchanged.

## What the historical Golden records actually establish

- `gates/ddl-golden-T06.log` records
  `head=a08190969237 cases=496 passed=496 assertions=11772`.
- `artifact.json.head_sha` and `artifact.json.cli.build.head_sha` both equal
  `a08190969237404b280cfcc8d2cec8c01a082316`.
- The artifact's CLI path is
  `/private/tmp/ds-a17-proof/golden/T06/bin/deltascope`; its SHA-256 is
  `20fa5257f5e8fcd479410774aaa744640683757abb3008a86fb1dd965d568830`.
- R1 read-only inspection of the surviving binary matches that hash.
  Actual `go version -m` reports revision `a08190969237404b280cfcc8d2cec8c01a082316`
  and `vcs.modified=true`.

Thus the Golden run used a binary built while Git HEAD was `a0819096`
with a modified checkout. The conversation records uncommitted A17 work at
that point, but no contemporaneous source snapshot was retained to bind
those exact bytes to the build. Neither today's source nor a newly built
binary can supply that missing proof. This is not a verified run of the
unmodified pre-fix code, and it is not a run after final implementation
commit `4f58ac54`.

The historical `worktree-status.txt` was captured later, during evidence
assembly; it cannot establish the build-time dirty path set. In particular,
`vcs.modified=true` cannot be attributed solely to `.mcp.json`.

## Three separate surviving binaries

The following are observations made during R1, not newly fabricated
historical tool output. Their unabridged command output, argv, inspection
HEAD, timestamps, stderr, and direct return codes are retained in the R1
evidence under `old-identity/`.

| Role | Path | SHA-256 | Actual build revision / modified |
|---|---|---|---|
| Work CLI | `/Users/fan/GolangProjects/DeltaScope/bin/deltascope` | `5d37945162b6d1e254005e354b6b761013b6d982e1b9a72273ce444430e4e3aa` | `4f58ac548aecd495edc707c35868c179686b0e35` / `true` |
| Golden runner CLI | `/tmp/ds-a17-proof/golden/T06/bin/deltascope` | `20fa5257f5e8fcd479410774aaa744640683757abb3008a86fb1dd965d568830` | `a08190969237404b280cfcc8d2cec8c01a082316` / `true` |
| Four-anchor probe | `/tmp/ds-a17-proof/a17probe` | `bdc7bd49cc6e83c01fd58b314971d35e27b6d7f2cb9f6b908896272683a6b322` | `a08190969237404b280cfcc8d2cec8c01a082316` / `true` |

Both CLIs report `CGO_ENABLED=0`; the old probe reports `CGO_ENABLED=1`.
All three report `go1.26.1`, `darwin/arm64`, and actual Git VCS fields.
The work CLI's revision/version must not substitute for the Golden CLI's
or the probe's identity. The probe source was retained as
`anchor-probe/main.go.txt`, but the original evidence did not bind that
source snapshot and all provider dependencies to the probe binary with a
complete build record.

## Scope of the correction

The old 496-case observations and four-anchor normal samples remain useful
historical evidence. Their SHA attribution and collection guarantees were
incomplete. R1 supplies complete-set-sensitive assertions, a committed
strict collector, and a new final-commit-bound execution. It does not
rewrite these originals, re-label their HEAD, or change production type
representation, rules, policies, or Golden denominators.
