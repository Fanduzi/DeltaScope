# T10-R6 verification archive — bound to 6cec4e6c

Independently checkable evidence for the T10-R6 acceptance candidate
(issue #89, parent contract #79). The product commit is
`6cec4e6c98f9258ddda168494d7bca7f95bfcad3`, appended after R5 evidence
`b24655fa`; this directory was committed as a separate evidence commit on
top. Scope preserved from R5: provider error propagation, the MySQL/TiDB
suffix boundaries, ordered-state publication, and every accepted R4/R5
consumer behavior — R6 closes only the attribution, ambiguous-consumer,
and batch-state gaps the R5-A independent review confirmed.

## Layout

| Path | Contents |
|------|----------|
| identity.txt | Product SHA, parent chain, branch/remote binding, work-CLI and golden-CLI sha256, anchor versions |
| gates/ | Per-gate quad files (`<gate>.{command,stdout,stderr,rc}.txt`) executed against the bound SHA on this host |
| golden/ | `T09-artifact.json` / `T10-artifact.json` emitted by `make ddl-golden` plus the runner logs — T09 125/125 cases / 3442 assertions, T10 126/126 / 3175, both `head_sha=6cec4e6c…` and identical to the frozen R5 baselines |
| green/ | Focused `go test -count=1` logs for the four touched packages on the bound SHA (all ok) |
| red/ | Worktree-replay red evidence: the three R6 test files copied onto product `2f37fbd4` (R5) — 16 top-level behavioral FAILs (9 audit / 5 provider / 2 rule). See `red/README.md` for the replay method and honest labels |
| native-probe/ | `r6-attribution-probe.sh` + verbatim `.log` — the minimal TiDB v8.5.0 supplementary probes for the same-sequence-candidate and reversed-sequence catalog shapes behind the provider change |

## Gate index (all bound to 6cec4e6c)

Every gate under `gates/` records the exact command, stdout, stderr, and
exit code. Summary: `go test ./...` 0, `go vet ./...` 0,
`go test -race -count=1 ./internal/...` 0, `gofmt` clean on
internal/pkg/cmd, `sql-corpus-gates` 0, `query-access-corpus-gates` 0,
`ddl-inventory-gate` 0, `ddl-coverage-catalog-test` 0,
`ddl-golden-validator-test` 0, `parser-error-unsupported-contract-test`
0, `unsupported-diagnostics-evidence-test` 0, `docs-example-gates` 0,
`decision-record-gate` 0, `pg-confidence-gates` 0 (unit gates plus the
three Docker-backed PostgreSQL E2E suites), `three-level-doc` OK, and
`lint` rc=1 with the pre-existing baseline findings only — R6 removed
the five `batch_state_index.go` findings (dead `indexSubjectName`, four
unused `statement` params) and added none.

## Honest caveats

- Red evidence is a **worktree replay**: R6 test files run against a
  detached `2f37fbd4` worktree — real behavioral failures on the
  pre-fix product, not the original capture session.
- `domain-spec.log` under `red/` records 0 FAILs: the spec-layer fix
  (handle-unknown PK-object binding) is covered by an audit-layer test,
  so no spec-package test goes red on the R5 product.
- The R5 archive under `docs/verification/t10/2f37fbd4…/` keeps its
  original logs; R6 commits narrow three wording claims there (red is a
  replay; the probe log's run-block count; the named native-cell scope).
- `pg-confidence-gates` ran on this host with Docker; its three E2E
  suites self-provision their anchors.
