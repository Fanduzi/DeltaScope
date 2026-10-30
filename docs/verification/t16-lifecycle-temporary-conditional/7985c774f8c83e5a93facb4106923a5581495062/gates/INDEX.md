# T16 gates index — evidence bundle @7985c774

Gate tuples (`.command.txt` / `.rc.txt` / `.stdout.txt` / `.stderr.txt`)
recorded on the milestone branch at `7985c774f8c83e5a93facb4106923a5581495062`
(product commit `7985c774`; the `eb919d19` probes-README commit adds no
product change). Every tuple is an exact command + return code + captured
stdout/stderr — nothing was retyped.

| Gate | Command | rc |
|---|---|---|
| go-test | `go test ./...` | 0 |
| sql-corpus-gates | `make sql-corpus-gates` | 0 |
| ddl-inventory-gate | `make ddl-inventory-gate` | 0 |
| ddl-golden-validator-test | `make ddl-golden-validator-test` | 0 |
| ddl-coverage-catalog-test | `make ddl-coverage-catalog-test` | 0 |
| parser-error-unsupported-contract-test | `make parser-error-unsupported-contract-test` | 0 |
| unsupported-diagnostics-evidence-test | `make unsupported-diagnostics-evidence-test` | 0 |
| decision-record-gate | `make decision-record-gate` | 0 |
| docs-example-gates | `make docs-example-gates` | 0 |
| three-level-doc (worktree, post-commit) | `bash /Users/fan/.agents/skills/check-three-level-doc/scripts/check_three_level_doc.sh` | 0 |
| l2-committed-range | `bash …/gates/l2_committed_range_check.sh 74a344dc..eb919d19` | 0 |

The worktree three-level-doc check reports "no changed files" after the
product commit — the committed-range L2 check covers the actual milestone
diff (66 files across `74a344dc..eb919d19`, including every touched
module README and the probes-directory README required by the `.sh`
files): `[three-level-doc] OK`.

## Golden runs

| Suite | Command | Result |
|---|---|---|
| T16 | `T16_MYSQL_PASSWORD=root python3 ./scripts/ddl_golden.py run --task T16 --artifact-dir artifacts/t16` | 55/55 cases, 739 assertions, PASS — `golden/T16-run.log`, artifact `golden/T16.artifact.json` (`head_sha == cli.build.head_sha == 7985c774`) |
| T09–T15 regression | `python3 ./scripts/ddl_golden.py run --task <T> --artifact-dir artifacts/regress` serially | see `golden/regress-runs.log` — recorded verbatim, denominators unchanged |
