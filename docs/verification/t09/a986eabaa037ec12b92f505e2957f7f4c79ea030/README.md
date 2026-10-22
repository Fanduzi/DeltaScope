# T09-R1 verification archive — a986eaba

Bound code SHA: `a986eabaa037ec12b92f505e2957f7f4c79ea030`
(`fix(ddl): gate index comparisons and ADD PRIMARY KEY on proven facts`)
Contract/doc SHA: `7ac054e57be756c7f0caf9911c594c19254c9e3c`
(`docs(t09): amend index-governance contract for R1 fact sufficiency`)
Base: `dc54ecacd06445b50e8c193ef2e0e7b8f6dda817` (original T09 closeout)

Scope: T09-R1-INDEX-FACTS-AND-STATE — bounded rework of issue #88.
Two defect groups fixed without re-planning T09 and without entering T10:

- R1-A: duplicate / left-prefix / unique-overlap comparisons previously
  used only `kind + Columns`. A shared predicate `indexPlainComparable`
  now gates all three rules on the recorded key-part facts (prefix parts,
  DESC parts, expression keys, unmodeled parts, predicates, included
  columns, GLOBAL, unmodeled options). Non-plain candidates are skipped,
  not convicted. Provider indexes now carry the same comparability
  vocabulary: `information_schema.statistics` gained `SUB_PART` and
  `COLLATION`; NULL `COLUMN_NAME` marks an expression member; unclassifiable
  collation increments `UnmodeledKeyParts`.
- R1-B: `ADD PRIMARY KEY` previously published the post-state before
  proving member existence. The primary branch now proves the vouched
  column set, plain key parts, and every member's existence first, then
  reconciles member `NotNull=true`, clears NULL-default facts, and
  preserves real defaults — matching the native result on all anchors.

## Layout

- `golden-run.log` — the six golden commands, results, and run history.
- `golden/<T>/artifact.json` — runner artifacts (T04–T09), each
  head-bound to a986eaba and independently re-validated
  (`golden/<T>-validate.log`).
- `golden/<T>-run.log` — raw runner stdout/stderr per task.
- `native-probe/` — the R1 bounded native confirmation:
  `t09r1_probe.py` (script, persisted verbatim),
  `native-matrix-r1.json` (raw cells), `native-matrix-r1.txt`
  (readable matrix), `probe-run.log` (stdout).
- `gates/` — raw logs at the bound SHA:
  - `make-test.log` — `GOFLAGS=-count=1 make test` (39 package ok, 0 FAIL)
  - `pg-unit-test-gates.log`, `sql-corpus-gates.log`,
    `ddl-inventory-gate.log`, `ddl-coverage-catalog-test.log`,
    `docs-example-gates.log`, `decision-record-gate.log`,
    `ddl-golden-validator-test.log` — all rc=0
  - `race.log` — `go test -race -count=1` on audit + ddl rules +
    mysql provider (rc=0)
  - `r1-focused-tests.log` — the R1 permanent tests green at the
    bound SHA (the red-before-fix phase is described in
    `docs/work-plans/t09-index-governance/CONTRACT.md` Amendment R1
    and was observed on the pre-fix tree)
  - `gofmt.log` — two pre-existing unformatted T07 test files
    (`batch_state_t07_test.go`, `batch_state_t07_matrix_test.go`,
    committed unformatted in d2bbbfa6, outside this diff); every file
    in the R1 diff is gofmt-clean.
- `identity/` — binary identities:
  - `work-cli-sha256.txt` / `work-cli-goversion.txt` —
    `CGO_ENABLED=0 make build-cli` at the bound SHA. The `+dirty`
    module version reflects only the preserved external `.mcp.json`
    modification; no task file was uncommitted.
  - `golden-cli-identity.txt` — the runner-built CLI digests recorded
    inside each artifact (identical sha256 across tasks, same source).

## Prior archives

The original T09 archive at `docs/verification/t09/51c8eec3.../` keeps
its own SHA binding and is unchanged. The T08-R3 YEAR archive under
`docs/verification/t08/` is likewise untouched.

## Known limitations

- The red-phase outputs (failing tests on the pre-fix tree) were
  observed in-session; the permanent tests that encode them are in the
  fix commit and re-verified green here. No post-hoc red log was
  fabricated.
- This archive does not commit binaries, credentials, DSNs,
  `.mcp.json`, or unrelated working-tree content.
