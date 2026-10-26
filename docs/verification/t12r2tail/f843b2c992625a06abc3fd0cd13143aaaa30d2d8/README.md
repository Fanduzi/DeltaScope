# T12-R2-tail evidence bundle — f843b2c9

Acceptance candidate evidence for T12 (#91) R2-tail remediation, bound to product
commit `f843b2c992625a06abc3fd0cd13143aaaa30d2d8` (parent `0c7ee19f`, the R2
evidence commit).

## Scope

Three tail items only; R2 fixes are preserved untouched.

- **A** — `--` comment lexing: MySQL requires whitespace/control after `--`.
  `v--1` is `v - -1`, not a comment. Unified via `dashDashOpensComment` in
  `internal/infrastructure/parser/tidb/extractor.go` across all four scanning
  paths (element splitting, word scanning, `skipCommentOrQuoted`, raw
  CHECK-body extraction).
- **B** — generic DROP undecidable must unvouch PK when the target may bind the
  PK object. `dropCheckUndecidable` now evaluates `pkMayBind` before the
  collection-unvouched early return; rule-layer `EvidenceGaps` attributes the
  actually missing side (`target_table.primary_key` when only PK identity is
  uncertain).
- **D** — same-statement `DROP CHECK` is subtracted from the identity premise
  before RENAME/CHANGE refusal evaluation in `prepareMultiAction`
  (`batch_state_multi.go`). Proven drops unlock the identity action;
  unproven/generic drops withhold the successor instead of fabricating a
  refusal or preserving stale pre-state.

## Red baseline

`red/tail_red.log` — captured on the R2 product (`a4ce0e2d`) before the tail
fixes, via `git stash` of the implementation with the new regression file left
untracked. Five red cases (A ×2 with bare-CHECK control green, B ×2, D ×3
families), two controls green as designed.

## Green

`gates/go_test_internal.run.log` and `gates/go_test_all.run.log` — all R2-tail
regressions green on `f843b2c9`; whole-repo `go test ./...` and
`go test -race ./internal/...` zero failures.

Regression file: `internal/application/audit/batch_state_t12r2tail_test.go`
(public Audit path; successor Metadata/findings/gaps asserted, not just
first-statement findings).

## Native probes (bounded, archived under docs/work-plans/t12-check-enforcement/probes/r2/)

- `zz_r2tail_p4a_mysql57_dashdash.log` — MySQL 5.7.44. `CHECK(v--1>0) NOT
  ENFORCED` → rc=1, 1064 (explicit clause → old-version refusal); bare
  `CHECK(v--1>0)` → rc=0 lands column v; `CHECK(w>0) -- NOT ENFORCED` → rc=0
  (real comment still hides the clause). Confirms `v--1` parses as `v - -1`.
- `zz_r2tail_p4d_mysql80_drop_rename.log` — MySQL 8.0.46. `DROP CHECK chk_a,
  RENAME COLUMN a TO a2` and the reversed action order → rc=0; CHANGE COLUMN
  same-family → rc=0; generic `DROP CONSTRAINT chk_a` + RENAME → rc=0; control
  `RENAME` with surviving referencing CHECK → rc=1, 3959 (atomic). Column and
  CHECK catalog captured before/after each statement.

## Gates (18 groups, serial, bound to f843b2c9)

`gates/` holds per-gate four-tuples (`*.cmd`, `*.out`, `*.err`, `*.rc`).

| gate | rc |
|------|----|
| ddl-coverage-catalog-test | 0 |
| ddl-golden-validator-test | 0 |
| ddl-inventory-gate | 0 |
| decision-record-gate | 0 |
| docs-example-gates | 0 |
| go test -race ./internal/... | 0 |
| go test ./... | 0 |
| go vet ./... | 0 |
| gofmt -l ./internal ./pkg ./cmd | 0 (only the two accepted T07 baseline files) |
| golangci-lint (scoped) | 1 — 13 issues, byte-identical set to the accepted baseline; zero new |
| parser-error-unsupported-contract-test | 0 |
| pg-confidence-gates | 0 |
| query-access-corpus-gates | 0 |
| release-dialect-hygiene-gates | 0 |
| sql-corpus-gates | 0 |
| three-level-doc (script) | 0 |
| three-level-doc committed-diff README sync | 0 |
| unsupported-diagnostics-evidence-test | 0 |

## Golden suites (serial, shared anchors, bound to f843b2c9)

`golden/<TASK>/artifact.json` (+ per-case records and built CLI under `bin/`).

| suite | cases | assertions | status |
|-------|-------|------------|--------|
| T09 | 125 / 125 | 3442 | PASS |
| T10 | 126 / 126 | 3175 | PASS |
| T11 | 40 / 40 | 506 | PASS |
| T12 | 25 / 25 | 282 | PASS |

All four `head_sha` and `cli.build.head_sha` = `f843b2c9…`; all four CLI
binaries sha256-identical (`2b67388e1244cf1f7f3344fec58407e0a97fa57a55addd3e5238e48d2fb7a4ff`,
go1.26.1, CGO_ENABLED=0). Per-run cleanup `compose_down_rc=0`,
`residual_containers=[]`.

Note: golden runs require `T0{9,10,11,12}_MYSQL_PASSWORD` env (here `root`,
matching the compose anchors); a first attempt without them produced rc=1
reachability failures and was discarded and rerun — artifacts in this bundle
are the credentialed runs only.

## Observed vs unobserved

Observed natively this round: 5.7.44 dash-dash lexing (3 forms), 8.0.46
same-statement DROP CHECK + RENAME/CHANGE in both orders plus generic-spelling
and surviving-CHECK controls.
Unobserved (modeled conservatively, no claim of proof): TiDB same-statement
DROP CHECK + identity combinations, DROP CHECK + RENAME on versions other than
8.0.46, full success templates for the unblocked identity action (successor
withheld/ungap'd paths asserted instead).
