# T06-A1 evidence — CREATE TABLE primary-key golden proof (#85)

Code under test: `06de7475a4f702fac8940d4b6b23c254b18a36a8`
(`test(ddl): pin T06-A1 CREATE TABLE primary-key golden path (#85)`).
This directory is evidence only; it changes no behavior.

## Scope claim

The slice proves the existing `ddl.table.primary_key.require` first path on
the `mysql.create-table`/`tidb.create-table` inventory rows. 32 manifest
cases passed on all four locked anchors (mysql 5.7.44 / 8.0.46 / 8.4.10,
tidb v8.5.0) with 574 assertions. It does not complete #85 or #79, and no
inventory status was widened.

## Layout

- `red/` — pre-implementation baseline: `ddl_golden.py` failed with
  `missing task manifest` before `T06.json` existed (the honest red —
  a missing proof entry, not a product-logic failure).
- `a0-baseline/` — T06-A0 offline originals: 8 CLI runs (argv/rc/stderr/
  stdout.json) plus the rules catalog used to build the isolated policy.
- `golden/` — four-anchor `make ddl-golden TASK=T06` artifact copy:
  `artifact.json` (32 executed cases, per-case expected/actual/assertions),
  `cases/*.json` per-case raw evidence (raw stdout/stderr, recorded
  commands, setup/post_verify/execute/structure/teardown records,
  `instance_facts`), `policies/` the three generated isolated profiles
  (all-off, `t06-pk-presence-isolated`, `t06-pk-required-false`), and
  `bin-sha256.txt` (the audit binary's digest; the binary itself is not
  committed). Runner summary: `head=06de7475a4f7 cases=32 passed=32
  assertions=574 PASS`; compose teardown rc=0 with zero residual
  containers.
- `gates/` — final-SHA gate captures, each as `<name>.argv`/`.stdout`/
  `.stderr`/`.rc` plus `head.txt` and `status.txt`.

## Key assertions read directly from `golden/artifact.json`

- 4 no-PK cases (`t06-{mysql57,mysql80,mysql84,tidb85}-no-pk`): product
  exit 1 / `reject` / one `ddl.table.primary_key.require` blocker AND
  driver-side `CREATE TABLE` rc 0 with real zero PRIMARY KEY constraints
  and zero `PRIMARY` index parts — policy and legality proven
  independently.
- 8 PK cases (inline + table-level × 4 anchors): product exit 0 /
  `pass` AND real PK constraint count 1 with member `id:1`.
- GIPK facts recorded live on mysql80/mysql84 for every case:
  `sql_require_primary_key=OFF`, `sql_generate_invisible_primary_key=OFF`,
  `show_gipk_in_create_table_and_information_schema=ON` — no
  server-generated invisible PK could be mistaken for input-declared.
- 10 CLI controls: no-PK reject (both dialects), inline/table-level pass,
  `all-rules-disabled` pass, `t06-pk-required-false` pass.

## Gates (all rc=0 on the code SHA)

gofmt, focus-app/sdk/http (`-run T06A1`), `python3
scripts/test_ddl_golden.py` (234 contract cases, 0 failures), inventory
gate, sql-corpus gates, coverage-catalog test, docs-example gates,
decision-record gate, `GOFLAGS=-count=1 make test`, `make
pg-unit-test-gates`, three-level doc check, task-range diff scope.
