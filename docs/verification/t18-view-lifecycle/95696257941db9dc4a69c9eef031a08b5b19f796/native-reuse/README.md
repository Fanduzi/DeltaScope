# Native MySQL 8.0.46 global table reuse boundary

Probe receipts for `lex->query_tables`/`find_table_in_global_list` boundaries: a unique top-level physical table reused through later global physical-chain references is `IS_UPDATABLE=NO`; derived/CTE/view first nodes do not inherit that check. `probe.stdout.txt` records an initial catalog column typo; `probe2.stdout.txt` is the corrected catalog receipt.
