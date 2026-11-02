# T18-R1-tail2 evidence — 95696257941db9dc4a69c9eef031a08b5b19f796

This archive is the independently reviewable evidence set for issue #97 T18-R1-tail2.
It does not claim reviewer acceptance.

## Identities

- Product commit: `95696257941db9dc4a69c9eef031a08b5b19f796`
- Behavior-red baseline: `abe88235e00c53c174d719fe270209b73b82f659`
- Prior evidence head / push base: `d9eeb34aa5bd057aaaf22406413a92071463ba44`

## Layers

- `baseline/`: candidate HEAD, remote refs, parent chain, worktree inventory, and protected Docker inventory.
- `red/`: public/internal behavior-red receipts on `abe88235e00c53c174d719fe270209b73b82f659` and `.go.txt` source snapshots.
- `native/`: successful one-shot MySQL 8.0.46 updatable-view/projection fixture and cleanup proof.
- `native-extra/`: successful one-shot MySQL 8.0.46 derived/CTE/output-alias fixture and cleanup proof.
- `native-reuse/`: one-shot MySQL 8.0.46 global-table-reuse boundary probes and cleanup proof.
- `green/`: focused internal/public/spec green receipts on `95696257941db9dc4a69c9eef031a08b5b19f796`.
- `golden/`: final T16/T17/T18 artifacts, policies, and summary.
- `gates/`: 23 final gate receipts bound to `95696257941db9dc4a69c9eef031a08b5b19f796`.
- `attempts/`: non-semantic credential/setup/source-compatibility attempt records only.
- `delivery-tree/`: post-archive `go list`/`go test` receipts.
- `review/`: archive integrity and handoff notes.

Replay/test source snapshots use `.go.txt`; no archived evidence file participates in Go package discovery.
