# T05-A2-R1 verification evidence — ee49d86

Evidence-only commit for the T05-A2-R1 bounded rework of the single-pair
RENAME identity-migration slice. Code head under review:
`ee49d86e757fa6b5ea4175b29f2695ee82402b22` on
`milestone/mysql-tidb-ddl-completion` (review base `6e06062`, task diff
`c38536e..ee49d86` — 4 files, +727/−33).

The rework closes exactly the three review findings, nothing else:

1. **Major — stale loaded FK references.** `applyRenamePair` now computes
   `renameDependentKeys` over request-local loaded entries only: an entry
   whose supplied constraint references either endpoint (explicit
   `referenced_schema` wins; an unqualified reference resolves under the
   owning entry's schema) tombstones to whole-entry unknown on either
   branch — migration or endpoint rejection. No provider discovery, no
   dependency graph, no constraint rewriting; provider objects and earlier
   projections stay immutable; unrelated tables keep their facts.
2. **Minor — PK expressed as a constraint.** `renameShapeMovable` accepts a
   supplied `primary_key` constraint member (same fact as the `PrimaryKey`
   field) and still rejects every other recorded constraint type
   (FK/CHECK/unrecognized). The A2 ADR's boundary wording was corrected to
   the frozen non-PK rebinding boundary.
3. **Minor — cancellation publishing post-state.** `applyRenamePair` checks
   `ctx.Err()` at the rename boundary and again after the required endpoint
   reads, before dependent invalidation and endpoint publication. An
   observed cancellation surfaces `context.Canceled` through the existing
   error channel and publishes nothing; a provider error keeps its wrapped
   identity. There is no guarantee against cancellation arriving after the
   final check — no transaction lock or rollback is claimed.

## Layout

| Path | Content |
|---|---|
| `artifact.json` | T05 golden artifact produced by the post-commit run (`head_sha=ee49d86`) |
| `cases/` | All 167 per-case raw results (command, rc, stdout, stderr, metadata phases) |
| `policies/` | Effective golden policy YAMLs used by the run (sha-bound inside artifact) |
| `gates/` | Gate logs run at `ee49d86` unless noted below |
| `red/` | A2-R1 red→green evidence (see "Red evidence" below) |
| `identity/source-identity.txt` | Code/review base SHAs, worktree status, diff scope, manifest+script digests, toolchain |

## Gate results at ee49d86

| Gate | Command | Result | Log |
|---|---|---|---|
| T05 golden | `make ddl-golden TASK=T05` (runner `scripts/ddl_golden.py`) | 167/167 cases, 2396 assertions, PASS | `gates/golden-run.log` |
| Artifact validator | `ddl_golden.py validate --artifact …` | artifact valid | `gates/validate.log` |
| Validator contract | `make ddl-golden-validator-test` | 98 cases, 0 failures | `gates/ddl-golden-validator-test.log` |
| Focused audit tests | `go test -count=1 ./internal/application/audit` | ok | `gates/go-test-audit.log` |
| PG-tagged audit tests | `go test -count=1 -tags postgresql ./internal/application/audit` | ok | `gates/go-test-audit-postgresql.log` |
| SDK/transport focus | `go test -count=1 ./pkg/deltascope ./internal/interfaces/...` | ok | `gates/go-test-sdk-transport.log` |
| Unit tests | `go test -count=1 ./...` | all 40 packages pass | `gates/go-test-all.log` |
| PG gates | `make pg-unit-test-gates` | 7 packages pass | `gates/pg-unit-test-gates.log` |
| Corpus gates | `make sql-corpus-gates` | pass | `gates/sql-corpus-gates.log` |
| Decision record | `make decision-record-gate` | pass | `gates/decision-record-gate.log` |
| Three-level doc | skill `check-three-level-doc` script, `ee49d86` diff replayed in a detached worktree at `c38536e` (the checker inspects worktree diffs; on a clean tree it is a no-op) | OK (L1 reminder only; root README unchanged — module README edits are internal-contract wording) | `gates/three-level-doc.log` |
| gofmt + whitespace | `gofmt -l` on changed Go files; `git diff --check` | clean | `gates/gofmt-diffcheck.log` |

## T02/T03/T04 evidence reuse

The R1 diff touches `applyRenamePair`, `renameShapeMovable`, and adds
`renameDependentKeys` — all reachable only through the RENAME transition.
No shared read path, invalidation primitive, provider seam, or non-RENAME
transition was modified (verified: `git diff c38536e ee49d86` touches four
files; production changes are inside the rename branch and its helpers).
The full `go test ./...` suite (which includes every ordered-state matrix:
same-key, multi-pair, mixed ALTER, EXECUTE/parse contamination, provider
errors, read-once ledger, two independent `Audit` runs, policy-blocker
orthogonality, and the PG legacy path) was re-run at `ee49d86` and is
green. Per the R1 work order, T02/T03/T04 golden evidence is therefore
reused; no non-RENAME golden path is affected.

## Rename-path oracle evidence (unchanged denominator)

The twelve A2 `metadata_cases` — four anchors × three forms — are present
unchanged in `cases/`:

- `T05.meta.t05-<anchor>-rename-qualified` — `RENAME TABLE src.t TO dst.t2`
- `T05.meta.t05-<anchor>-rename-alter` — `ALTER TABLE src.t RENAME TO dst.t2`
- `T05.meta.t05-<anchor>-rename-unqualified` — `RENAME TABLE src.t TO t2`

The run re-verified the complete six-phase oracle on all four anchors
(MySQL 5.7/8.0/8.4, TiDB 8.5): setup → endpoint-absence verify → product
audit → post_verify → driver-applied statements → structure oracle +
teardown. Denominator is still 167 cases / 2396 assertions; no case was
added, removed, or weakened.

## Red evidence

`red/a2r1-tests-red.log` was produced by checking out the pre-R1 evidence
head `c38536e` (which carries the `6e06062` implementation) in a detached
worktree and running `go test -run 'TestBatchStateA2R1' -count=1 -v` with
the new test file copied in — genuine failures (`go-test-rc=1`):

- `TestBatchStateA2R1LoadedDependentInvalidates` — loaded children keep
  their stale `ReferencedSchema=src, ReferencedTable=t` certainty after
  migration and after CHECK-source endpoint rejection; destination
  references likewise stay deterministic on a non-move cell. Unrelated and
  owner-schema-local control cells correctly PASS on the old code.
- `TestBatchStateA2R1LoadedDependentKeepsPublicGapContract` — the
  follow-up `ALTER TABLE aux.child ADD COLUMN` reports 0 gaps instead of
  the two frozen `unknown_table_state` existence gaps.
- `TestBatchStateA2R1PrimaryKeyPayloadMigrates` — `primary_key` constraint
  payload, partial-knowledge, and unknown-flag variants fail
  (`len(Constraints)==0` rejects the move); the `PrimaryKey`-field-only and
  FK-source controls correctly PASS.
- `TestBatchStateA2R1RenameObservesCancellation` — pre-cancelled context,
  cancel inside the destination read, and enrichment propagation return
  `nil` and publish post-state; the provider-error-identity control
  correctly PASSes.

`red/a2r1-tests-green.log` is the same run at `ee49d86` — all subtests
PASS.

## Honesty notes

- The binary sha256 inside `artifact.json` (`cli.sha256`) is the runner's
  own record of the binary it built and executed; it was not independently
  recomputed by the reviewer of this evidence.
- The three-level-doc check ran against a detached-worktree replay of the
  `ee49d86` diff, because the checker only inspects changed files and is a
  no-op on a clean tree.
- No credential values appear in this evidence; `DS_T05_GOLDEN_PW` is an
  environment variable name, and the recorded command argv uses it as
  such.
