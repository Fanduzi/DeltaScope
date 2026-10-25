# T11-R1 native-anchor probes

Targeted probes run on the live ddl-golden compose anchors during the
R1 hardening pass. Raw session logs (verbatim client output) are
preserved beside this file and under
`docs/work-plans/t11-unified-constraints/r1/`:

- `r1-probe-mysql57-80.log` — referenced-key eligibility, refer actions,
  parent-side drop dependency on MySQL 5.7.44 / 8.0.46
- `r1-probe-mysql84-tidb85.log` — same matrix on MySQL 8.4.10 and
  TiDB v8.5.0 (via the mysql-client sidecar)
- `r1-probe2.log` — landed-state verification (KEY_COLUMN_USAGE after
  each DROP CONSTRAINT/CHECK arm), `restrict_fk_on_non_standard_key`
  observation, `tidb_enable_check_constraint` OFF/ON arms
- `r1-probe3.log` — TiDB nonclustered PK sole-coverer drop (1553),
  multi-coverer rebind semantics, FULLTEXT referenced-key refusal
- `r1-probe4.log` — MySQL 8.x `DROP CONSTRAINT` landed-state control

## Facts pinned by these probes (consumed by the R1 contract)

- No usable referenced key refuses the ADD: 1215 (5.7), 1822 (8.0),
  6125 (8.4 with `restrict_fk_on_non_standard_key=ON`, observed ON),
  1822 (TiDB v8.5.0). A non-unique plain index qualifies on 5.7/8.0 and
  TiDB; MySQL 8.4 refuses it under the observed restrict flag.
- Leftmost order is required — a composite key whose referenced columns
  are not a leftmost in-order prefix is not a usable referenced key.
- `ON DELETE|UPDATE SET NULL` on a NOT NULL local column: 1830 on
  8.0/8.4/TiDB (5.7 refuses at 1215). `SET DEFAULT`: 1215 on 5.7,
  accepted on 8.0/8.4 and TiDB v8.5.
- Sole-coverer drops refuse 1553 on every child- and parent-side entry
  (member index, `primary` spellings, DROP PRIMARY KEY — including
  TiDB nonclustered PK members; TiDB clustered refuses earlier at 8200).
  With multiple eligible coverers the drop rebinds instead of refusing.
- TiDB `DROP CONSTRAINT` / `DROP CHECK` never removes an FK: warned
  no-op under `tidb_enable_check_constraint=OFF` (factory default),
  error 3940 under ON. The earlier probe note claiming it drops the FK
  was a misread of rc=0 — KCU verified the member survives.
- MySQL 8.x `DROP CONSTRAINT` removes a named FK; `DROP CHECK` on the
  same name refuses 3821. MySQL 5.7 rejects the syntax (1064).
- FULLTEXT (and by class spatial/vector/columnar) indexes never serve
  as the referenced key (TiDB 1822 verified).

## Corrections to the prior bundle's notes

The `11d64391` probe README recorded two items the R1 probes disprove —
kept verbatim for history, corrected here: the non-standard referenced
key refusal is 6125 on **MySQL 8.4** (TiDB answers 1822), and TiDB
`DROP CONSTRAINT` does **not** drop an FK under either flag state.
