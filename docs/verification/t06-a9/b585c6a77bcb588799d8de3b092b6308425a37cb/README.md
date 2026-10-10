# T06-A9-R2: EXTRA whitelist byte-exact comparison

Code under test: `b585c6a77bcb588799d8de3b092b6308425a37cb`
(`test(ddl): make T06-A9 EXTRA whitelist comparison byte-exact (#85)`)

This evidence commit is the direct child of the tested code commit.

## What was fixed

`t06_a9_extra_case` previously generated `LOWER(<expr>) IN ('...', ...)`.
On MySQL 8.4, `information_schema.COLUMNS.EXTRA` is `utf8mb3` /
`utf8mb3_general_ci`, a PAD SPACE collation: shorter operands are
right-padded with U+0020 for comparison, so four illegal strings —
`" "`, `"default_generated "`, `"on update current_timestamp "`,
`"default_generated on update current_timestamp(0) "` — were accepted
as whitelist hits (see `red-pad-space/`).

The helper now generates `CAST(LOWER(<expr>) AS BINARY) IN
(CAST('...' AS BINARY), ...)`: case normalization first, then an
unbounded binary comparison against byte-exact whitelist constants.
No TRIM/REPLACE, no fixed `BINARY(N)` length, no collation rewriting,
no COALESCE; SQL NULL still yields `<unrecognized-extra>`.

## Directory layout

| Path | Contents |
| --- | --- |
| `red-pad-space/` | R1 red evidence at old HEAD `edc5bf0`: `00-baseline` records `VERSION()`/`@@collation_connection` and `CHARSET(EXTRA)`/`COLLATION(EXTRA)` = `utf8mb3`/`utf8mb3_general_ci`; `10-group-a` = session literals (`utf8mb4_0900_ai_ci`) all correct; `20-group-b` = same inputs under `CONVERT(... USING utf8mb3) COLLATE utf8mb3_general_ci` — all four trailing-space inputs accepted. Originals, unmodified. |
| `green-same-inputs/` | R2 green at `b585c6a7`: `10-group-a`/`20-group-b` rerun all 8 inputs with the new helper on the live mysql84 anchor — legal controls keep created/updated role markers in both contexts, all four trailing-space inputs return `<unrecognized-extra>` in both contexts; `30-literal-control` runs the manifest literal control through the runner's exact mysql invocation (`-N -B -e`, latin1 session); `40-catalog-context` verifies `CONCAT(EXTRA,' ')` keeps `utf8mb3_general_ci` under the runner invocation, `SELECT NULL` returns NULL, and the padded real column values return sentinel. `summary.json` = 16/16 PASS. |
| `golden/` | Fresh four-anchor T06 run at `b585c6a7`: `artifact.json`, all case records, policy files. 256/256, 6211 assertions. |
| `gates/` | argv-captured stdout/stderr/rc for `make ddl-golden-validator-test` (393 cases, 0 failures), `make ddl-golden TASK=T06`, `python3 scripts/ddl_golden.py validate` (binary check enabled), `make decision-record-gate`. |
| `identity/` | `source.txt`: code SHA, tree SHA, worktree state, source file git-object hashes, binary sha256. `hashes.txt`: artifact and manifest sha256. |

## Evidence classification

- **Old HEAD red**: `red-pad-space/` — old helper, `edc5bf0`.
- **R2 green**: `green-same-inputs/` + `golden/` — new helper, `b585c6a7`.
- **Constructed-string SELECTs**: group-a/group-b probes evaluate literals; group-b wraps them in the catalog column's charset/collation. These are read-only SELECTs, not real catalog content.
- **Real catalog observations**: `green-same-inputs/40-catalog-context` and the `extra pad-space context observation` execute step in `golden/cases/*/records.json` — `HEX(EXTRA)`, `CHARSET(EXTRA)`, `COLLATION(EXTRA)`, `COLLATION(CONCAT(EXTRA,' '))`, `HEX(CONCAT(EXTRA,' '))`, and the CASE result on real `created_at`/`updated_at` columns.
- **Synthetic validator checks**: `gates/validator.*` — contract mutations including `t06a9r2b` plain-LOWER revert, catalog-context removal (single/both sides), forged literal outputs.
- **Formal Golden run**: `golden/` — not reused from any earlier artifact.

## Per-anchor catalog context (observed, not assumed)

| Anchor | `CHARSET(EXTRA)` | `COLLATION(EXTRA)` | `COLLATION(CONCAT(EXTRA,' '))` |
| --- | --- | --- | --- |
| mysql57 | utf8 | utf8_general_ci | utf8_general_ci |
| mysql80 | utf8mb3 | utf8mb3_general_ci | utf8mb3_general_ci |
| mysql84 | utf8mb3 | utf8mb3_general_ci | utf8mb3_general_ci |
| tidb85 | utf8mb4 | utf8mb4_bin | utf8mb4_bin |

`CONCAT(EXTRA,' ')` retains each catalog column's own collation under
the runner's invocation on all four anchors; every appended U+0020
suffix is rejected by the binary comparison with `<unrecognized-extra>`.

## Gate summary

| Gate | Result |
| --- | --- |
| `make ddl-golden-validator-test` | rc=0, 393 cases, 0 failures |
| `make ddl-golden TASK=T06` | rc=0, 256/256, 6211 assertions, PASS |
| `ddl_golden.py validate` | rc=0, `artifact valid` (binary check on) |
| `make decision-record-gate` | rc=0, PASS |

Go/PG/SDK/HTTP and T02–T05 suites were not rerun: this change touches
only the proof oracle and its contract tests; reuse is per the task
authorization.
