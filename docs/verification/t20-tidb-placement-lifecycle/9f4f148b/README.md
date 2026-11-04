# T20-R3 — placement catalog-premise rule + topology evidence gap

Bound to product commit `9f4f148b70cde57058791a945d76725e05b70380`
(parent `1e630493b47676ba076010be59c28898c16626d8`, chain
`7bf6e257 → b24f7331 → 43c08677 → 1e630493 → 9f4f148b`).

## Scope

Final-acceptance review of the real-topology round surfaced two contract
gaps against #99 (拓扑未知给gap而非猜测可部署 / 不存在策略给对应错误):

1. A proven applied bind produced `coverage=complete` while the product
   holds no topology facts — deployability was implied, never verified.
2. A proven-absent policy reference produced `pass/0/0` because the only
   enabled placement rule was the binding-risk warning, which by design
   fires on applied outcomes only.

Fix (this commit):

- `ddl.tidb.placement.warn` emits `placement_policy.topology` evidence
  gaps for topology-relevant applied outcomes (binds, policy ALTERs)
  while `spec.Metadata.PlacementTopology` is nil.
  `spec.PlacementTopologyFacts` (label universe + observed
  `SCHEDULING_STATE`, `json:"-"`) is the receiving slot; no provider
  populates it, so the gap fires honestly even against a provably
  satisfiable cluster.
- `ddl.tidb.placement.exists.require` (blocker, default-enabled) owns
  the catalog premise: proven-absent references (8239), proven-bound
  drops (8241), create-on-existing refuse publicly. Disarmed forms
  (`IF EXISTS`/`IF NOT EXISTS`/`OR REPLACE`) and `= DEFAULT` unbinds
  carry no premise; unvouched catalog/bound-target facts gap per rule.

## Gates

`gates/` — receipts captured at run time (rc files per gate):

- 01 `go test ./...` rc=0
- 02 `go vet ./...` rc=0
- 03 `go test -race ./internal/...` rc=0
- 04 sql-corpus gates rc=0 (includes new `placement_policy_missing`
  corpus covering `ddl.tidb.placement.exists.require`)
- 05 ddl-inventory rc=0
- 06 coverage-catalog (postgresql tag) rc=0
- 07 parser-unsupported contract (both tags) rc=0
- 08 unsupported-diagnostics (both tags) rc=0
- 09 docs-examples rc=0
- 10 decision-record `main...HEAD` rc=0
- 11 L2 committed-range `7bf6e257..9f4f148b` rc=0 (110 changed files —
  includes prior evidence/fixture commits; every changed source module
  ships its README)
- 12 golden-validator rc=0 (758 contract cases)
- 13/14 lint scoped/full rc=1 — inherited baseline diagnostics only
- 15 lint identity comparison rc=0 — same normalization as the T19/T20-R2
  baseline: scoped 18/18 added=0 removed=0; full 33/33 added=0 removed=0.
  Scoped command extended with `./internal/domain/policy/...` (this
  round touched it); normalization identical.

## Golden

`green/T20/T20/artifact.json` — real golden run, `head_sha =
cli.build.head_sha = 9f4f148b70cde57058791a945d76725e05b70380`:

- 25/25 cases pass, **286 assertions** (prior artifact: 255; +31 from
  the corrected oracles' per-rule gap/finding entries).
- `t20-placement` profile now enables both rules
  (`warn` + `exists.require@blocker`) — recorded in the artifact.
- Oracle corrections (case IDs preserved, history artifact untouched):
  - `tidb85-bind-missing-refused`, `tidb85-drop-bound-refused`,
    `tidb85-alterdb-partial`: `pass` → `reject`, exit 0→1,
    `fail_on_triggered`, `ddl.tidb.placement.exists.require` blocker.
    Task basis: #99 缺失/未知策略给对应错误 — internal state refusal
    alone is not a public check.
  - `tidb85-golden-bind`, `tidb85-schema-bind`,
    `tidb85-partition-bind`, `tidb85-alter-policy`: `complete` →
    `unverified` + `placement_policy.topology` gap.
  - `tidb-create-policy-gap`, `tidb-alter-bind-gap`,
    `tidb-drop-policy-gap`, `tidb-alter-range-global-gap`: 1 → 2 gaps
    (each enabled rule owns its own `placement_policy.catalog`
    premise; the drop warn gap carries
    `[catalog, bound_targets]` as one combined fact list).

`attempts/01-golden-f9633bde-drop-gap-facts.log.txt` — first run at
pre-amend `f9633bde`: 24/25, the drop-policy warn gap's combined
`[catalog, bound_targets]` fact list was under-projected in the oracle;
corrected and amended into `9f4f148b`.

## Red evidence

`probes/00-t20r3-red-evidence-7bf6e257.txt` — verbatim `go test -run
TestT20R3` on the unmodified product at `7bf6e257` (temp worktree,
removed after capture): 5/8 subtests fail — no topology gap, disabled
warn swallowing the missing-policy refusal, catalog gap owned only by
the warn rule, no blocker on proven refusal. Confirms both defects
before the fix.

`probes/batch_state_t20r3_test.go.txt` — the red-matrix source (`.go.txt`
per evidence-source convention; the committed test lives at
`internal/application/audit/batch_state_t20r3_test.go`).

## PG gates

`pg/pg-unit.*` — `make pg-unit-test-gates` rc=0.
`pg/pg-confidence.*` — `make pg-confidence-gates` rc=0
(docker_default 10.210.0.0/16 accommodation as previously recorded).

## Topology environment

Unchanged this round — the real PD/TiKV fixture evidence lives under
`../7bf6e257-topology/` (cluster_id `7704189008621554081`, 3× TiKV Up,
labels `region=t20rev zone=z{1..3} host=h{1..3}`, STRICT; SCHEDULED /
8239 / 8243 / persistent-PENDING matrix). The new contract keeps the
honest boundary: the product still does not read topology — it now
says so structurally instead of implying verification.
`probes/25-cli-identity.txt` under that directory (committed this
round) pins the CLI build the 22–24 probes used (`vcs.revision=7bf6e257`).

## Not claimed

- No provider wires `PlacementTopologyFacts`; real deployability
  remains unverified by design.
- No push, PR, merge, tag, release, deployment, or T21 work.
