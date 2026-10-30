# T16 evidence bundle — lifecycle, temporary namespaces, rename ordering @7985c774

This bundle closes T16 / issue #95 under the mysql-tidb-ddl-completion
milestone (#79). It records the gates re-collected on the product commit
plus the full golden line at this head. Product implementation is
`7985c774`; `eb919d19` adds the probes-directory README required by the
committed-range L2 check and contains no product change.

## Commits covered

| Commit | Content |
|---|---|
| `7985c774` | `feat(ddl): T16 table lifecycle, temporary namespaces, and rename ordering` — 65 files: spec/extractor lifecycle facts (`ReferTable`, `SelectSources`, `TemporaryScope`, `OnCommitDelete`, `RenameStatementForm`), disjoint persistent/temp identity slots + role-aware binding (`batch_state_namespace.go`), ordered transitions (create/like/ctas/drop/truncate/rename) with probed refusal semantics replacing T05-era tombstones, bound metadata projections (`TargetTables`/`SourceTable`/`SourceTables`), inbound-FK edges (`ReferencedBy`/`ReferencedByUnknown`), eight new blocker rules, coverage aspect reclassification, 55-case manifest, CONTRACT + native probe logs, 12 corpus fixtures, inventory row updates, ADR `2026-11-09`, module README sync. |
| `eb919d19` | `docs(t16): probes README` — archive index for `probe16_lifecycle.sh` / `probe16_supplement.sh` and their logs (required by the L2 committed-range check once `.sh` files enter the range). Documentation only. |

## Golden line — eight suites at this head

Re-executed serially with `python3 scripts/ddl_golden.py run --task <T>`.
T16 records `head_sha == cli.build.head_sha == 7985c774`; the regression
suites record `eb919d19` (post-commit worktree, identical product code).
Every artifact: `required_case_ids == executed`, zero duplicate case IDs,
`cleanup.compose_down_rc == 0`, zero residual containers. Denominators
identical to prior bundles — no weakening.

| Task | Cases | Assertions | Artifact |
|---|---|---|---|
| T16 | 55/55 | 739 | `golden/T16.artifact.json` (head 7985c774) |
| T09 | 125/125 | 3442 | `golden/T09/artifact.json` (head eb919d19) |
| T10 | 126/126 | 3175 | `golden/T10/artifact.json` |
| T11 | 40/40 | 506 | `golden/T11/artifact.json` |
| T12 | 25/25 | 282 | `golden/T12/artifact.json` |
| T13 | 47/47 | 697 | `golden/T13/artifact.json` |
| T14 | 69/69 | 1087 | `golden/T14/artifact.json` |
| T15 | 42/42 | 470 | `golden/T15/artifact.json` |

Raw runner output: `golden/T16-run.log`, `golden/regress-runs.log`.

## Gates collected

See `gates/INDEX.md`: `go test ./...`, sql-corpus-gates,
ddl-inventory-gate, ddl-golden-validator-test, ddl-coverage-catalog-test,
parser-error-unsupported-contract-test,
unsupported-diagnostics-evidence-test, decision-record-gate,
docs-example-gates, worktree three-level-doc, and the committed-range L2
check `74a344dc..eb919d19` — all rc=0.

## Native evidence

`docs/work-plans/t16-lifecycle-temporary-conditional/probes/` —
368-label matrix + tail supplement on all four anchors (mysql 5.7.44 /
8.0.46 / 8.4.10, tidb v8.5.0), indexed by `probes/README.md`. The matrix
established the semantics the CONTRACT freezes: sequential rename
work-set + atomic publication, version-gated multi-DROP fates
(MySQL ≥8.0 atomic / 5.7 sequential / TiDB sequential-persistent +
veto-shaped temp), temp-shadow binding directions, TiDB local-temp
refusals (8200 ALTER/RENAME, 8006 LIKE), TiDB CTAS refusal (1105),
inbound-FK TRUNCATE refusal (1701), TiDB GLOBAL TEMPORARY as a catalog
object, and the deferred FLASHBACK/RECOVER boundary.
