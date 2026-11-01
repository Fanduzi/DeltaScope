# T18-R1-tail evidence — abe88235e00c53c174d719fe270209b73b82f659

This archive is the independently reviewable evidence set for issue #97 T18-R1-tail.
It does not claim reviewer acceptance.

## Identities

- Product commit: `abe88235e00c53c174d719fe270209b73b82f659`
- Behavior-red baseline: `19286f7ff73b79f6fca1cc76d11074f3aee751f2`
- Evidence-isolation commit: `38836a07da7ff0952939d325485b6341ede96717`
- Prior evidence head / push base: `601d4fad84b519d6629b5f1903eee9ba6a88619b`

## Layers

- `baseline/`: HEAD, remote refs, worktree, and protected Docker/worktree inventory.
- `archive-build/`: proof that the `.go` evidence snapshot polluted package discovery, its byte-preserving `.go.txt` isolation, and post-isolation `go list`/`go test`.
- `red/`: public/internal behavior-red receipts on `19286f7ff73b79f6fca1cc76d11074f3aee751f2`, source snapshots as `.go.txt`, and the final-test replay on the same baseline.
- `native/`: one-shot MySQL 8.0.46 TEMPTABLE/CHECK OPTION receipts and cleanup proof.
- `green/`: focused internal/public and affected-package green receipts on `abe88235e00c53c174d719fe270209b73b82f659`.
- `golden/`: final T16/T17/T18 artifacts and policies.
- `gates/`: 23 final gate receipts bound to `abe88235e00c53c174d719fe270209b73b82f659`.
- `attempts/`: non-semantic harness/lint-stem attempt records only.
- `delivery-tree/`: `go list`/`go test` receipts after the evidence directory was added.
- `review/`: archive integrity and handoff notes.

Replay/test source snapshots use `.go.txt`; no archived evidence file participates in Go package discovery.
