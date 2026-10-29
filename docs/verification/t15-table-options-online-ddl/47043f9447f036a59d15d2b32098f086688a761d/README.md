# T15 evidence addendum — documentation closeout @47043f94

This bundle is the documentation-closeout addendum for T15/#94. It records
the post-documentation evidence refresh; it does **not** represent new
product capability. The product implementation remains `c761fa38`; the
primary evidence bundle remains
`../f9febe7771c4faaf184eb133494136f21f6121e0/` (golden, gates, red, green,
probes) — its artifacts are unmodified.

## Commits covered

| Commit | Content |
|---|---|
| `ba56922c` | Documentation fix only: CONTRACT §1 option-bucket classification reconciled with the actual extractor / spec outcome / probes (three-way split: truly unextracted remainder, extracted-but-bounded-gap, probed TiDB vendor refusals; `insert_method` moved to extracted+warn-consumed); CONTRACT §9 records the frozen `42 cases / 470 assertions`; ADR `2026-11-08` annotates the milestone forward-dating convention (task-slot dates run ahead of commit dates). |
| `47043f94` | `gates/README.md` added inside the `f9febe77` bundle's `gates/` directory — required by the committed-range L2 check once that range includes the evidence commit (a `.sh` in a directory requires a module README). Additive documentation only; no recorded evidence was altered. |

## Golden re-run — all seven suites on `47043f94`

Re-executed serially with `python3 scripts/ddl_golden.py run --task <T>`.
Every artifact records `head_sha == cli.build.head_sha == 47043f94`,
`required_case_ids == executed`, zero duplicate case IDs,
`cleanup.compose_down_rc == 0`, zero residual containers.
CLI sha256 `6d6e49f328276c01` for all seven runs.

| Task | Cases | Assertions |
|---|---|---|
| T09 | 125/125 | 3442 |
| T10 | 126/126 | 3175 |
| T11 | 40/40 | 506 |
| T12 | 25/25 | 282 |
| T13 | 47/47 | 697 |
| T14 | 69/69 | 1087 |
| T15 | 42/42 | 470 |

Denominators identical to the `f9febe77` archive — no weakening.

## Gates re-collected

See `gates/INDEX.md`: decision-record, inventory, golden-validator,
docs-example, three-level-doc worktree check, and the committed-range L2
check `f9febe77..47043f94` — all rc=0. Gates unaffected by documentation
edits were not re-run; their records remain in the `f9febe77` bundle.
