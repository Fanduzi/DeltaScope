# T06-A17 four-anchor catalog→provider proof

One disposable table per anchor: `golden.t06_a17_type_facts` (24 columns, the
union of baseline fixtures N1/N2/L1/L2/C with L2's `e`/`s` renamed
`e_escaped`/`s_escaped`), `ENGINE=InnoDB DEFAULT CHARSET=utf8mb4
COLLATE=utf8mb4_bin`, no rows, no PK/index, no ALTER. Lifecycle per anchor:
pre-create COUNT=0 → CREATE → ordered information_schema.COLUMNS read
(incl. HEX(COLUMN_TYPE), NUMERIC_PRECISION/SCALE, CHARACTER_SET/COLLATION) →
real `mysqlmeta.NewProvider(db).LoadTableSnapshot` over a query-logging driver
wrapper (per-query ledger) → DROP → post COUNT=0. Credentials via env only.

Raw per-anchor output in `anchors/<anchor>/` (create.sql, session-facts,
catalog-rows, provider-snapshot, provider-query-ledger, catalog-provider-pairs,
pre/post counts, full query ledger).

| anchor | VERSION() | image | rows | target-basis mismatches |
|---|---|---|---|---|
| mysql57 | 5.7.44 | mysql:5.7.44 | 24 | 0 |
| mysql80 | 8.0.46 | mysql:8.0.46 | 24 | 0 |
| mysql84 | 8.4.10 | mysql:8.4.10 | 24 | 0 |
| tidb85 | 8.0.11-TiDB-v8.5.0 | pingcap/tidb:v8.5.0 | 24 | 0 |

"Mismatch" = target column whose provider Type differs from its catalog
COLUMN_TYPE. All 21 target columns matched byte-for-byte on every anchor.

## Server normalization observed (kept, not reconstructed)

- `DECIMAL(12)` (dm): catalog returns `decimal(12,0)` on all four anchors;
  provider keeps `decimal(12,0)` — catalog facts are never rewritten toward
  the original declaration.
- `DECIMAL` (dn): catalog `decimal(10,0)` on all four.
- `FLOAT(10,0)` (f0): MySQL anchors keep `float(10,0)`; TiDB 8.5 normalizes to
  `float` — provider keeps `float` there.
- `DOUBLE(16,0)` (x0): MySQL keeps `double(16,0)`; TiDB normalizes to `double`.
- Display-width: MySQL 5.7 returns `int(11)`/`bigint(20) unsigned`; 8.0/8.4/TiDB
  return `int`/`bigint unsigned`. Non-target columns keep the established
  bare-keyword provider contract (`int`, `varchar`, `bigint`) either way.

Member literals (`enum('','A,B','A)B','O''Reilly')` etc.) were byte-identical in
catalog and provider output on all four anchors, including case, order, the
empty member, the embedded comma, the right paren, and the escaped apostrophe.
