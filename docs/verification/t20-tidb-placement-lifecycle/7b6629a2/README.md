# T20-R3 third-review corrections — unvouched-catalog laundering closed

Bound to product commit `7b6629a258ab0d1d981f0f05d98e738821cb422b`
(parent `72971d19`, chain `… → cb310591 → 72971d19 → 7b6629a2`).

## Scope

Fourth product commit. The focused verification of `cb310591` confirmed
claims 1/2/3/5 correct but proved claim 4 unsound — the tombstone-create
seed had been applied too broadly:

1. **OR REPLACE under an unvouched catalog fabricated a proven-empty
   bound set.** `CREATE OR REPLACE` lands unconditionally, so the
   applied branch ran with `exists == nil`; the entry then carried
   `refs={}` + `refsKnown` even though the replaced policy may hold
   live bindings (probed: bindings persist across OR REPLACE). A later
   `DROP PLACEMENT POLICY` would have applied silently — missing the
   8241 blocker AND the gap. `refs`/`refsKnown` are now seeded only
   when existence was *proven* absent; `exists == nil` keeps the bound
   set unvouched.
2. **Pre-existing amplification, made reachable:** `placementPoliciesProjection`
   returned a partial map whenever ≥1 entry was present. Consumers read
   absent keys as proven-false — fabricating missing-policy blockers on
   unproven names. The projection now returns nil unless
   `placementsKnownComplete`, matching the ordered state's tri-state
   answers (`placementPolicyExists` already returned nil for absent
   keys under incomplete knowledge).
3. `dropFinding`'s bound-target dereference is nil-guarded.

New public-path cell: `or_replace under unvouched catalog fabricates
nothing` — OR REPLACE → DROP → bind-to-other under a nil catalog yields
`[catalog, bound_targets]` gaps on both rules, never a silent apply and
never a fabricated blocker.

## Golden

- `green/T20/T20/artifact.json` — 25/25, 286 assertions, rc=0,
  `head_sha=7b6629a258ab`. No oracle change — golden cases never reach
  the unvouched-create-replace path.

## Gates (all rc=0, receipts under `gates/`)

vet, race, full `go test ./...`, corpus + inventory contract, coverage
catalog (postgresql), docs-examples, decision-record, golden validator
all green. Lint identities unchanged: scoped 18→18, full 33→33
(+0/-0 vs the 2405d09a baseline, same commands + normalization).
