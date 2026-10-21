# T08 regression-oracle closure — proof at 3b35fe30

Round: `T08-REGRESSION-ORACLE-CLOSURE`. Scope: precise migration of the
eleven pre-T08 oracle cells affected by the shipped T08 capability
upgrades, plus direct-return-code and real race evidence. **No production
code changed** — the product tree at `3b35fe30` is byte-identical to the
frozen `c983fc9d2e8f887981a5f031b1d55b699d711e84` (proof:
`product-tree-proof.txt`).

## SHA separation

| Role | SHA |
|---|---|
| Frozen product code | `c983fc9d2e8f887981a5f031b1d55b699d711e84` |
| Contract/oracle + docs commit (this round's proof code) | `3b35fe302c7afdb1697fbef0c3cb1f5daeb77128` |
| Prior evidence HEAD (delivery closeout) | `c8bd6da4a2a03638781f06ec3f896f461db59f63` |

All new runs execute in a detached worktree pinned to `3b35fe30`
(`/tmp/ds-t08-oracle-closure`, clean tree); each run log records argv, cwd,
HEAD, dirty state, start/end, and the **direct** process return code —
no pipeline-derived status.

## Results

| Gate | Result | Evidence |
|---|---|---|
| T03 golden at 3b35fe30 | **118/118 PASS**, rc=0, artifact valid | `T03-run/`, `runs/T03-run.log`, `runs/T03-validate.log` |
| T04 golden at 3b35fe30 | **163/163 PASS**, rc=0, artifact valid | `T04-run/`, `runs/T04-run.log`, `runs/T04-validate.log` |
| T05 golden at 3b35fe30 | **247/247 PASS**, rc=0, artifact valid | `T05-run/`, `runs/T05-run.log`, `runs/T05-validate.log` |
| Validator contract suite | **758 cases, 0 failures** | `gates/ddl-golden-validator-test.log` |
| T08 contract oracle | 25/25 PASS | `gates/t08-contract-test.log` |
| T07 contract oracle | failures=0 | `gates/t07-contract-test.log` |
| Race detector (audit pkg, T08/T07R1/R1/isolation set) | PASS, `direct_rc=0` | `race/go-test-race-audit.log` |
| docs-example-gates | PASS | `gates/docs-example-gates.log` |
| decision-record-gate | PASS | `gates/decision-record-gate.log` |
| three-level-doc | OK | `gates/three-level-doc.log` |
| T06/T07/T08 reused originals revalidated | all `artifact valid`, rc=0, binary check ON | `reuse/` |
| T02 | reused accepted original (10/10 @ aba44783) — not re-run | `reuse/README.md` |

528 original T03+T04+T05 cases all pass under the migrated contract; no
cell retired, no denominator reduced.

## Migrated cells

`oracle-migration/` — eleven-cell identity table (`cells.json`), verbatim
expect diffs (`manifest-diff.txt`), contract-code diff (`oracle-diff.txt`),
and the mutation-sensitivity summary.

## Binary identity

Run binaries (one per task, identical bytes): sha256
`b011f5cb4e4f3a664abf28dc5e8d28a2ef6d4aa17d8250b1592b636fba0c7dc3`,
go1.26.1, `deltascope devel (mysql, tidb)` — see `identity/`. Binaries are
not committed (per archive precedent); each artifact's `cli.sha256`
records the binding and the binary file remains on local disk for
binary-check revalidation.

## Honest gaps

- Runner `rc` for the previous closeout's five reruns was pipeline-derived
  (noted in its README); this round's `direct_rc=` values are captured
  without pipes.
- The `<oracle-closure proof sha>` referenced in the corrected closeout
  index is this directory: `3b35fe302c7afdb1697fbef0c3cb1f5daeb77128`.
