# T16 native probe archive

Each archive records the exact SQL, rc, error output, `SHOW WARNINGS`,
and catalog reads its steps produced on the four frozen anchors
(mysql 5.7.44 / 8.0.46 / 8.4.10, tidb v8.5.0). Temporary-table probes
run inside a single session per label — a local temporary object
cannot outlive its connection, so every temp label cleans, creates,
mutates, and re-reads within one `mysql` invocation. Scope notes
record what each log actually proves — read the log text, not this
file, for evidence.

| Archive | Scope |
|---|---|
| `p16_lifecycle_matrix.log` (`probe16_lifecycle.sh`) | 368-label lifecycle matrix: `CREATE [GLOBAL] TEMPORARY` (MySQL GLOBAL is a syntax refusal; TiDB `ON COMMIT DELETE ROWS` lands as a persistent catalog object, `PRESERVE ROWS` refused 8200), temp-shadow semantics (plain CREATE lands on the catalog under a temp shadow; INSERT/SELECT bind the temp object), `CREATE TABLE ... LIKE` member fidelity (columns+PK+indexes+checks copy; no FK constraint, no auto_increment counter; TiDB local-temp source refused 8006), CTAS (MySQL lands declared-prefix + select-derived tail; TiDB refused 1105), multi-target DROP (MySQL ≥8.0 atomic veto on missing/duplicate names, 5.7 sequential partial, TiDB sequential persistent + veto-shaped temp slot, duplicate lists 1066 on MySQL vs sequential-1051 on TiDB), `DROP TEMPORARY` binding the session slot only, `DROP GLOBAL TEMPORARY` hitting the catalog, TRUNCATE (temp-first binding, auto_increment reset, inbound-FK refusal 1701 on MySQL and TiDB), standalone `RENAME TABLE` pair ordering (left-to-right work set — `t→u, u→v` blocks on the occupied destination while `u→v, t→u` rotates cleanly; TiDB local-temp endpoints refused 8200; MySQL RENAME is catalog-oriented and ignores the temp slot), `ALTER TABLE` on local temp (accepted on MySQL, refused 8200 on TiDB), `IF [NOT] EXISTS` conditional no-ops, and `FLASHBACK`/`RECOVER` staying unsupported entries. |
| `p16_tail_supplement.log` (`probe16_supplement.sh`) | Bounded continuation probes filling cells the matrix left open: `--force` continuation after failed multi-DROP proving the temp slot's fate per anchor (MySQL ≥8.0 atomic veto keeps the temp object; 5.7 sequential drops resolvable members including temp; TiDB drops the resolvable persistent member yet keeps the temp object — veto-shaped, not unconditional), same-name `RENAME TABLE t TO t` (1050 refusal standalone vs `ALTER ... RENAME TO t` no-op), rotation sequences proving destination-vacancy at pair-evaluation time, FK-parent TRUNCATE refusals, and global-temporary RENAME/DROP catalog binding. |
| `probe16_lifecycle.sh` | The generating script for the matrix log: per-label cleanup + schema prep under `golden`, single-session temp probes, per-step rc + stderr + `SHOW WARNINGS` + information_schema reads, idempotent cleanup. Re-runnable against the compose anchors only — it executes real DDL and is not part of any test suite. |
| `probe16_supplement.sh` | The generating script for the supplement log: stdin-fed multi-statement sessions (so `--force` continuation actually proceeds past a refused statement), TiDB client-side execution via `deltascope-ddl-golden-tidb85-client`, and the same idempotent cleanup contract. |
