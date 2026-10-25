# T10-R6-tail verification — fd849db5

Product commit `fd849db52ef2f2291fedde4680dcea2f7bbf16be`, appended after
the R6 evidence commit `248f8c05`. Scope: three bounded fact-handling
corrections (plus two review-found same-family defects) on top of R6 —
no T10 expansion, no T11.

| Directory | Contents |
|-----------|----------|
| gates/ | Per-gate quad files (`<gate>.{command,stdout,stderr,rc}.txt`) executed against the bound SHA on this host |
| green/ | Focused green logs for the new tests plus full `go test -count=1` logs of the four affected packages on `fd849db5` |
| red/ | Worktree-replay red evidence: the R6-tail test files run against a detached `6cec4e6c` worktree — 5 behavioral FAILs (4 audit / 1 provider). See `red/README.md` for the first-capture vs replay distinction |
| golden/ | `T09-artifact.json`, `T10-artifact.json`, verbatim run logs — frozen oracle denominators, `head_sha=fd849db5` |
| native-probe/ | README only — no new native probe this round (partial injected fixture for C; R6 anchors stand) |
| identity.txt | Product SHA, parent chain, CLI sha256, golden totals, anchor versions |

## Gate index (all bound to fd849db5)

Every gate under `gates/` records the exact command, stdout, stderr, and
exit code — **17 gate quads as archived** (the R6 set plus the
previously unarchived `release-dialect-hygiene-gates`). Summary: `go
test ./...` 0, `go vet ./...` 0, `go test -race -count=1
./internal/...` 0, `gofmt` reports no new formatting issues this round —
the two accepted T07 baseline files (`batch_state_t07_matrix_test.go`,
`batch_state_t07_test.go`) remain — `sql-corpus-gates` 0,
`query-access-corpus-gates` 0, `ddl-inventory-gate` 0,
`ddl-coverage-catalog-test` 0, `ddl-golden-validator-test` 0,
`parser-error-unsupported-contract-test` 0,
`unsupported-diagnostics-evidence-test` 0, `docs-example-gates` 0,
`release-dialect-hygiene-gates` 0 (dialect-isolation policy check across
PostgreSQL/MySQL/TiDB default rules — not a release authorization),
`decision-record-gate` 0, `pg-confidence-gates` 0 (unit gates plus the
three Docker-backed PostgreSQL E2E suites), `three-level-doc` OK
(archived stdout is the script's post-commit "no changed files" output;
the actual static-diff check ran in-session on the changed files before
the product commit and passed), and
`lint` rc=1 with the pre-existing baseline findings only — identical to
the R6 baseline modulo line-number shifts (15 issues / 52 output lines,
0 added vs the R6 baseline).

## Golden

T09 **125/125 cases, 3442 assertions**; T10 **126/126 cases, 3175
assertions** — frozen oracle denominators, unchanged. Both artifacts
record `head_sha=fd849db52ef2…` and CLI sha256 `e22931ada0…` (identical
to the locally rebuilt `CGO_ENABLED=0` binary — deterministic build).
Anchors: shared ddl-golden compose (MySQL 5.7.44 / 8.0.46 / 8.4.10,
TiDB v8.5.0), serial use.

## Honest caveats

- Red evidence is a **worktree replay**: the R6-tail test files ran on a
  detached `6cec4e6c` worktree. The in-session first capture produced
  the same five behavioral failures but was not archived to disk; see
  `red/README.md`.
- The C fixture is a partial injected catalog fixture — no native TiDB
  NULL-coexistence observation is claimed (`native-probe/README.md`).
- `pg-confidence-gates` ran on this host with Docker; its three E2E
  suites self-provision their anchors.
- `release-dialect-hygiene-gates` is a policy hygiene check, not a
  release authorization; no tag/release/deploy was performed.
