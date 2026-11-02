# Gates

Final gate receipts bound to product commit
`f00b3a7d1c71c74f28a4c6b96e80e426c644ffa6` (T19, issue #98). Every receipt
contains the executed command, captured stdout/stderr, return code, and the
working-tree/HEAD identity at run time. All gates completed with rc=0.

- `01-go-test` — full unit suite (`go test ./... -count=1`).
- `02-sql-corpus` — `make sql-corpus-gates` (SQL corpus findings/coverage).
- `03-ddl-inventory` — `make ddl-inventory-gate` (inventory census contract).
- `04-golden-validator` — `make ddl-golden-validator-test` (offline manifest
  contract suite, 758 cases).
- `05-golden-t19` — real Docker `make ddl-golden TASK=T19` across the four
  baseline anchors (MySQL 5.7.44 / 8.0.46 / 8.4.10, TiDB v8.5.0): 23 cases,
  281 assertions, PASS; emitted artifact mirrored to `../green/golden-artifact.json`.
