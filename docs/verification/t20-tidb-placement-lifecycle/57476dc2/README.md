# T20-R3 verification — 57476dc2

Fourth product commit of the T20-R3 (local acceptance) round: closes
the two conservative follow-ups the final verification pass on
`cb310591..7b6629a2` reported after both laundering claims verified
CORRECT.

## Product under test

- HEAD: `57476dc2ea0078e7b07efc988c3505d93702aba3`
- Chain: `7b6629a2 -> 57476dc2`
- Task-commit scope: `internal/application/audit/batch_state_placement.go`,
  `internal/application/audit/batch_state_t20r3_test.go`

## What changed

1. `placementBoundSet` — a tombstoned (proven-absent) policy's bound
   set is provably empty (a nonexistent object binds nothing), so a
   bare re-drop now adjudicates `refused` via the missing-policy
   blocker (8239 parity) instead of `unproven`. Matches the helper's
   own documented contract and the nil-entry arm above it.
2. `applyAlterPlacementPolicy` — removed the unreachable `entry == nil`
   arm: an applied ALTER requires `exists == proven true`, which
   requires a present registry entry.

New test cell P (t20r3 matrix): `DROP p; DROP p` under a complete
catalog — the second drop emits the exists-require blocker, no gap.

## Golden

```
[ddl-golden] task=T20 head=57476dc2ea00 cases=25 passed=25 assertions=286
[ddl-golden] artifact=docs/verification/t20-tidb-placement-lifecycle/57476dc2/green/T20/T20/artifact.json
[ddl-golden] PASS
```

## Gates (all rc=0, receipts under `gates/`)

vet, race, full `go test ./...`, sql-corpus, inventory contract,
coverage catalog (postgresql tag), parser unsupported + diagnostics,
docs-examples, decision-record, golden validator (758 contract cases,
0 failures). Lint identity comparison vs the `2405d09a`/`cb310591`/
`7b6629a2` baseline: scoped 18→18 (+0/-0), full 33→33 (+0/-0), same
commands and normalization (`file:linter:message`, locations stripped).

## Review status

`cb310591..7b6629a2` verified independently: both laundering claims
CORRECT; the two minor follow-ups above were the only remaining items
and are fixed here. No new product surface.
