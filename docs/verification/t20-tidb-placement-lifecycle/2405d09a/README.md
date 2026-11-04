# T20-R3 post-review precision corrections

Bound to product commit `2405d09a6d0e5d85b83ef831097d81e81b910526`
(parent `f457df422933152cefe938e4201b1e5cdc1e9613`, chain
`7bf6e257 → b24f7331 → 43c08677 → 1e630493 → 9f4f148b → f457df42 →
2405d09a`).

## Scope

Second-round product commit applying the precision defects the
independent review of `1e630493..9f4f148b` surfaced (spec axis) plus a
laundering bug found while testing the corrections:

1. **DROP premise attribution is now exact** under both placement rules:
   an unvouched catalog owes `[placement_policy.catalog,
   placement_policy.bound_targets]` (existence decides whether the bound
   set is consultable at all), while a proven-existing policy whose
   bound-target read was not performed owes `bound_targets` alone.
   Previously the warn rule always reported the merged pair and
   `exists.require` reported catalog alone.
2. **`CREATE OR REPLACE` on a proven-existing policy carries the
   topology gap** (`placementRuleOutcome.topology` set when
   `or_replace=true` and existence proven) — it rewrites the definition
   every bound object deploys, the same live reassignment as
   `ALTER PLACEMENT POLICY`. A fresh OR REPLACE create stays gap-free.
3. **Ordered-state seeding no longer launders unvouched bound sets**:
   `batch_state_placement.go` sets `refsKnown=true` only when
   `PlacementPolicyFact.BoundTargets != nil`. Previously a nil read was
   allocated into an empty proven set, which let a DROP apply where the
   premise was unknowable — the exact "unknown → proven" laundering the
   milestone invariant forbids.
4. Cleanups from the same review: shared `firstNamedPlacementBinding`
   extraction across warn/exists paths, `ctx.Err()` parity in
   `exists.require`, bounded `target`/`target_kind` metadata on
   missing-policy findings, and an unreachable-statement removal caught
   by `go vet` before freeze.
5. `testdata/ddl-inventory/inventory.yaml`: the five placement rows are
   promoted to `semantically_checked` with concrete `file:` refs and
   T20-R3 evidence appended; the golden oracle for
   `tidb-drop-policy-gap` reflects the corrected fact pair.

## Golden

- `green/T20/T20/artifact.json` — 25/25 cases, 286 assertions, rc=0,
  `head_sha=2405d09a6d0e`, generated against the live four-anchor
  fixture (mysql 5.7.44/8.0.46/8.4.10 + tidb v8.5.0 unistore).
- `green-f1a39e99-stale/` — green run bound to the pre-amend SHA
  `f1a39e99` (identical behavior; amended only for the vet-flagged
  unreachable statement). Retained as a receipt, not the official
  artifact.
- `../f1a39e99/attempt1-db-flake/` — three consecutive runs that failed
  all `db.mysql*` cases. Root cause identified: the manifest's mysql
  anchors require the `T20_MYSQL_PASSWORD` fixture-credential env var;
  the runs lacked it (`required fixture credential environment is
  unavailable`). Not a product defect — green reruns with the variable
  exported prove it. The artifacts are kept because the receipt rule
  forbids rewriting history.

## Gates (all rc=0, receipts under `gates/`)

| gate | command | result |
|---|---|---|
| 01 | `go vet ./...` | clean after unreachable-code removal |
| 02 | `go test -race` audit+rule+spec+policy | pass |
| 03 | `go test ./... -count=1` | pass |
| 04 | sql-corpus contract tests | pass |
| 05 | `TestDDLInventoryContract` | pass |
| 06 | `TestDDLCoverageCatalog -tags postgresql` | pass |
| 07 | `TestDDLParserErrorUnsupportedContract` | pass |
| 08 | `TestParserErrorDiagnosticEvidence` | pass |
| 09 | `make docs-example-gates` | PASS |
| 10 | `check_decision_record.sh main...HEAD` | PASS |
| 11 | L2 parity over `f457df42..2405d09a` | audit README in-diff; ddl README covers `exists.require` from `9f4f148b` |
| 12 | `python3 ./scripts/test_ddl_golden.py` | PASS |
| 13/14 | golangci-lint scoped + full | rc=1 (diagnostics present), see 15 |
| 15 | lint identity comparison | scoped 18→18 (+0/-0), full 33→33 (+0/-0) vs `9f4f148b` baseline |

## Identity notes

- Product SHA: `2405d09a6d0e5d85b83ef831097d81e81b910526`
- The compose fixture used by the golden run is the runner-managed
  `deltascope-ddl-golden` project (down -v cleanup verified in the
  artifact); no foreign containers were touched.
- This round changed no real-cluster assertions; the
  `7bf6e257-topology/` native receipts remain the topology evidence.
