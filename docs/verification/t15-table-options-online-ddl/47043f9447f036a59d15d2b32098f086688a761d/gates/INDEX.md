# T15 gates index — addendum @47043f94

Documentation-closeout gates re-run on `47043f9447f036a59d15d2b32098f086688a761d`
(doc commit `ba56922c` + gates-README commit `47043f94` on top of the
`f9febe77` product line). Gates unaffected by documentation-only changes
were not re-collected; their records remain in the `f9febe77…/gates/`
bundle.

| Gate | Command | rc |
|---|---|---|
| decision-record-gate | `make decision-record-gate` | 0 |
| ddl-inventory-gate | `make ddl-inventory-gate` | 0 |
| ddl-golden-validator-test | `make ddl-golden-validator-test` | 0 |
| docs-example-gates | `make docs-example-gates` | 0 |
| three-level-doc (worktree) | `bash /Users/fan/.agents/skills/check-three-level-doc/scripts/check_three_level_doc.sh` | 0 |
| l2-committed-docfix | `bash …/f9febe77…/gates/l2_committed_range_check.sh f9febe77..47043f94` | 0 |

The committed-range L2 check covers the `0e20ff2d` evidence bundle, the
`ba56922c` documentation commit, and the `47043f94` gates-README commit —
693 changed files, `[three-level-doc] OK`.
