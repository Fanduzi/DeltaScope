# T13 native probe archive

Each archive records the exact SQL, rc, error output, and post-state
reads its steps produced on the four frozen anchors (mysql 5.7.44 /
8.0.46 / 8.4.10, tidb v8.5.0). Scope notes record what each log
actually proves — read the log text, not this file, for evidence.

| Archive | Scope |
|---|---|
| `p1_all.log` (`probe.sh`) | DROP/TRUNCATE on RANGE fixtures, `TRUNCATE ALL`, DROP on a non-partitioned table (1505), missing-member DROP (1507), drop-all (1508). |
| `p2_exchange.log` (`probe2.sh`) | EXCHANGE data movement, missing counterpart (1146), partitioned counterpart (1732), definition-mismatched counterpart (1736), missing member (1735), EXCHANGE on a non-partitioned primary (1505). |
| `p3_validate_mixed.log` (`probe3.sh`) | WITH/WITHOUT VALIDATION arms on MySQL anchors and the mixed-action refusals. **Contaminated cell:** the `absent`-validation arm ran after the WITHOUT arm had already emptied the counterpart, so its "absent → rc=0" line proves nothing about default validation — superseded by `p5_validation_arms.log`. The `t13m parts` line printed by that run executed `true`, not a catalog query — it is not post-state evidence. |
| `p4_nonpart.log` (`probe4_nonpart.sh`) | TRUNCATE PARTITION and EXCHANGE PARTITION on non-partitioned tables → 1505 on all four anchors (T13-R1 cells the p1/p2 archives lacked). |
| `p5_validation_arms.log` (`probe5_validation_arms.sh`) | Three-arm rerun with an identical violating fixture reset per arm: absent clause refuses violating rows exactly like `WITH VALIDATION` (MySQL 1526, TiDB 1737); only `WITHOUT VALIDATION` lands the row. Corrects the p3 absent cell. |
| `p5_validation_tidb_post.log` | TiDB arm post-state reads re-run separately after the driver lost query output for that anchor's noval arm. |
| `p6_mixed_matrix.log` (`probe6_mixed_matrix.sh`) | Mixed-action matrix: partition+structural sibling refusals (MySQL 1064 / TiDB 1064-or-8200 by order), `ALGORITHM=INPLACE` + DROP PARTITION (executes on 8.0/8.4/TiDB, refused 1846 on 5.7), HASH-type DROP (1512) vs TRUNCATE/EXCHANGE (execute). |
| `p7_modifiers.log` (`probe7_modifiers.sh`) | Modifier narrowing: `ALGORITHM=INPLACE` with TRUNCATE/EXCHANGE executes on all four anchors; `ALGORITHM=DEFAULT`/`LOCK=DEFAULT` + DROP execute on all anchors; modifiers after the partition action are syntax errors. |
| `p8_tableoption.log` (`probe8_tableoption.sh`) | `ENGINE=InnoDB` mixed with `DROP PARTITION`: MySQL 1064 both orders (grammar tail), TiDB 8200 option-first / 1064 partition-first. Upgrades the table_option sibling cell from unprobed to proven-refused. |
