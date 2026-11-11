# T26-SEMANTICS-01 §B — prior materials and approval status

Status classes: **approved** (ruling-accepted contract or shipped,
accepted implementation) · **applied-ruling** (reviewer-issued wording
binding this round) · **proposal** (authored, awaiting ruling) ·
**revised** (superseded by a later document in the same archive set) ·
**missing** (original material not recoverable).

No original material was found missing. No proposal below is relabeled
as an approved contract; README summaries and current behavior were not
used as semantic basis — the archived originals are cited.

| # | material | path | record identity | status | notes |
|---|---|---|---|---|---|
| B1 | R1 scope + sources | `docs/verification/t26-r1/r1-scope-and-sources.md` | R1 package, HEAD `4896137f` era | approved (evidence) | official per-form inventory, 5.7/8.0/8.4/TiDB-8.5 matrix, inventory ownership (T26) |
| B2 | R1 baseline | `docs/verification/t26-r1/r1-baseline.md` | same | approved (evidence) | probe matrix, leak-surface analysis, metadata baseline |
| B3 | R1 decision-table proposals | `docs/verification/t26-r1/r1-decision-table.md` | same | **revised** | SRS 19-row table superseded by R2 33-row; SERVER A.1–A.5 and ALTER-INSTANCE E absorbed into R2 docs; open-rulings list still binding |
| B4 | R1 native plan | `docs/verification/t26-r1/r1-native-plan.md` | same | **revised** | superseded by `r2-native-plan.md` 29-case matrix |
| B5 | R2 official sources | `docs/verification/t26-r2/r2-official-sources.md` | R2 package | approved (evidence anchors) | pinned DOC/RN/WL/SRC/PEND evidence types; 8.0.13 SRS tightening split; 5.7-vs-TiDB boundary separation |
| B6 | R2 ALTER-INSTANCE-EXT registry | `docs/verification/t26-r2/r2-alter-instance-ext.md` | R2 ruling 三/D | approved (tracking record) | surplus subactions removed from T26 implementation proposal by ruling; two inventoried subactions stay T26-owned |
| B7 | R2 SERVER semantics map | `docs/verification/t26-r2/r2-server-semantics.md` | R2 §四 answer | **proposal** (S1–S10 awaiting ruling) | R3 header registers "awaiting reviewer ruling"; R3/R6 shipped only the recognition-level subset (kind, verbatim name, wrapper, option keys/kinds/counts, dup keys, spelling count, boundary mapping) — the promised-judgment clauses S5–S8 stay unapproved |
| B8 | R2 SRS decision table | `docs/verification/t26-r2/r2-srs-decision-table.md` | R2 §F answer | **proposal** — all 33 rows unapproved; R4 addendum = **applied-ruling** for the 7-point `complete` contract | PENDING ORACLE rows 20/21 + row-11 refusal set; row 13 RULING NEEDED (suspended R6) |
| B9 | R2 output redaction | `docs/verification/t26-r2/r2-output-redaction.md` | R2 §G answer | approved + shipped (R3–R6) | internal/external split, PublicSQL seam, 10-exit inventory |
| B10 | R2 metadata fact scheme | `docs/verification/t26-r2/r2-metadata-facts.md` | R2 §H answer | fact-level contract **approved** ("Ruling applied: no zero-row→proven-absent"); per-fact queries later realized in provider round; adjudication *usage* stays proposal | zero-row/error/not-loaded per fact; §5 ordered-state channel separation; §6 prohibitions |
| B11 | R2 native plan | `docs/verification/t26-r2/r2-native-plan.md` | R2 §I answer | **proposal** (unexecuted) | 29-case matrix; 8 `db_*` required cells |
| B12 | R3 decision record | `docs/decisions/2026-11-15-mysql-server-srs-recognition.md` | committed ADR | approved + shipped | recognition + named boundary + redaction stage; lifecycle semantics explicitly deferred |
| B13 | T26 stage manifest | `testdata/ddl-golden/T26.json` | SHA-256 `67e7044d…` (R6 record) | approved (23/23 `cli_*` verified on `16eba8a0`); 8 `db_*` cells unexecuted | db cells reserved for separately-authorized native round |
| B14 | T26-R6 acceptance | `docs/verification/t26-r6/` | archive `1e130f1d`, tested `16eba8a0` | approved | scoped offline phase acceptance; deferred-scope list binding |
| B15 | T26-PROVIDER-01 acceptance | `docs/verification/t26-provider-01/` | archive `dfa7852e`, tested `7c2c1f23` | approved | five provider capabilities; three statement-path wirings; two provider-only conflict capabilities; bounded public-error contract |

Related rulings inherited verbatim: T04/#83 incomplete-coverage
contract (gap vs failing-provider, bounded `required_facts`, no payload
echo) — `docs/decisions/2026-09-23-ddl-incomplete-coverage-contract.md`;
T25 tablespace boundary/ordered-state precedent —
`docs/decisions/2026-11-14-mysql-tablespace-family.md`.

Registered absences: none. Every clause below traces to a file above.
