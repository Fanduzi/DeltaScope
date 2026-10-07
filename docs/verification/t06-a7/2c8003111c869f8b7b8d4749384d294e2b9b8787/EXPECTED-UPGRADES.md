# T06-A7 expected-output upgrade table

Every intentional old→new expectation change caused by promoting declared
`CREATE TABLE ... COLLATE` from `create_table.option.collate` unsupported
evidence to governed semantics under `ddl.table.collation.allowlist`.
Statement text and case/fixture IDs are unchanged; only the expected result
derived from the removed unsupported item was recomputed.

## Corpus expectations (`testdata/sql-corpus/`)

| file | statement / field | old (f8326032) | new (2c800311) | causation |
|---|---|---|---|---|
| `mysql/ddl/findings/rule_coverage_offline.expected.yaml` | `CREATE TABLE bad_collation ... COLLATE=latin1_swedish_ci` → `unsupported.count` | `1` incl. `create_table.option.collate` | `0` | collate consumed by new table rule; no other unsupported aspect on that statement |
| same | `findings.include` | no `ddl.table.collation.allowlist` | `ddl.table.collation.allowlist` added | enabled with `values=[utf8mb4_bin]`; declared `latin1_swedish_ci` is disallowed → policy finding |
| same | `config.rules` | rule absent | `ddl.table.collation.allowlist` enabled, `values=[utf8mb4_bin]`, `require_explicit=true` | corpus must exercise the new rule like its `ddl.table.charset.allowlist` sibling |
| `tidb/ddl/findings/rule_coverage_offline.expected.yaml` | `unsupported.count` | `4` incl. `create_table.option.collate` | `3` (keeps `create_table_select`, `create_table.index.fulltext`, `alter_table.add_index.fulltext`) | only the collate item removed; other unsupported aspects untouched |
| same | `findings.include` / `config.rules` | as mysql old | as mysql new | same causation |

## Shared audit tests

| file | case | old | new | causation |
|---|---|---|---|---|
| `internal/application/audit/coverage_t03_test.go` `TestAuditSQLT03RecognizedUnauditedAspectsIncomplete` | row `tidb create table collate` (`CREATE TABLE t (id INT) COLLATE utf8mb4_bin;`) | expected `create_table.option.collate` unsupported → incomplete | row deleted | the aspect is now consumed; T06A7 tests assert complete/pass for the same shape. CREATE/ALTER DATABASE COLLATE rows kept (schema boundary unchanged) |

## Golden manifest & validator

| file | item | old | new | causation |
|---|---|---|---|---|
| `testdata/ddl-golden/T06.json` | denominator | 124 cases | 188 cases (+32 CLI `t06-a7-{mysql,tidb}-*`, +32 metadata `t06-a7-{anchor}-*`) | A7 contract; all 124 old cases/IDs preserved byte-identical semantics |
| same | `policies` | 9 profiles | 14 profiles (+5 A7 isolated profiles) | new isolated profiles per spec |
| `scripts/ddl_golden.py` | T06 contract | 124-case contract | 188-case contract + A7 case/policy builders | same |
| `scripts/test_ddl_golden.py` | synthetic fixture / mutations | 314 contract cases | 335 contract cases; `t06-a7-` excluded from older generic mutation loops whose variants predate the A7 shapes; A7 rules added to fake catalog | validator must pin new contract |

## Inventory & traceability

| file | item | old | new | causation |
|---|---|---|---|---|
| `testdata/ddl-inventory/inventory.yaml` | collate-related rows | evidence refs pointed at T03 unsupported classification | updated evidence refs/status notes for the governed path | capability reclassification must be reflected in the inventory |
| `testdata/ddl-inventory/T06-base-traceability.md` | charset/collation rows + denominator | A6 state | A7 rows: column rules proven isolated; table COLLATE policy added; declared-vs-inherited distinction recorded | traceability sync |

## CLI behavior (same input, both dialects)

Input: `CREATE TABLE t (c INT) COLLATE=utf8mb4_bin;`, all-rules-disabled
policy, `--fail-on blocker`.

| | old HEAD f8326032 | tested HEAD 2c800311 |
|---|---|---|
| verdict / coverage | `review` / `incomplete` | `pass` / `complete` |
| `unsupported[]` | 1 item `create_table.option.collate` | empty |
| exit code | `1` (review; `fail_on_triggered=false`) | `0` |

Originals: `baseline-f8326032/runs/S4-{mysql,tidb}.*` vs
`s4-green/S4-{mysql,tidb}.*`.
