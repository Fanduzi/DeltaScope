# T15 native probe archive

Each archive records the exact SQL, rc, error output, `SHOW WARNINGS`,
and catalog reads its steps produced on the four frozen anchors
(mysql 5.7.44 / 8.0.46 / 8.4.10, tidb v8.5.0). Scope notes record what
each log actually proves — read the log text, not this file, for
evidence.

| Archive | Scope |
|---|---|
| `p15_online_matrix.log` (`probe15_online.sh`) | Online-DDL modifier matrix: `ALGORITHM=INSTANT` per action (ADD/DROP/RENAME COLUMN rc=0 on 8.x, refused 1800 on 5.7, refused 1845 on index/constraint actions; `INSTANT`+explicit `LOCK=` → 1221), `COPY`+`LOCK=NONE` → 1846 on all MySQL, `COPY`+`LOCK=EXCLUSIVE` accepted (the frozen seed), `INPLACE` acceptance cells, duplicate-declaration last-wins, `ORDER BY`/`FORCE`/`ENABLE|DISABLE KEYS` acceptance, per-option ALTER matrix (engine/row_format/auto_increment clamp/comment/charset-collate/KBS 1478-on-non-compressed/compression/encryption 3185-keyring/stats_*/tablespace 1812|3510/MyISAM long tail), and `ORDER BY` grammar refusal on multi-spec. |
| `p15_online_matrix.tidb.log` | The same matrix replayed on TiDB v8.5.0 after the client image was provisioned: `ADD INDEX … ALGORITHM=INSTANT` → **1846** (the one probed refusal; column INSTANT is accepted), every other ALGORITHM/LOCK cell accepted-and-ignored, `CONVERT TO` outside utf8mb4 → 8200, `FORCE` → 8200, the full refused option set (8200; `INSERT_METHOD` 8233, `UNION` 8232), `AUTO_INCREMENT=500` ignored (seed stays 0), MyISAM `ENGINE=` accepted as compat display. |
| `p15_tail_supplement.log` | Bounded long-tail sweep filling cells the first pass skipped: `AVG_ROW_LENGTH`/`CHECKSUM`/`TABLE_CHECKSUM`/`MAX_ROWS`/`MIN_ROWS`/`DATA|INDEX DIRECTORY`/`CONNECTION`/`PASSWORD`/`STORAGE MEDIA`/`SECONDARY_ENGINE`/`UNION` per anchor — all accepted rc=0 on MySQL (MyISAM-scoped ones inert on InnoDB), all 8200/8233/8232-class vendor refusals on TiDB, plus `SECONDARY_ENGINE=NULL` grammar confirmation. |
| `probe15_online.sh` | The generating script for the two matrix logs: fixed synthetic InnoDB/MyISAM fixtures under schema `golden`, per-step rc + stderr + `SHOW WARNINGS` + information_schema reads, idempotent cleanup (`DROP TABLE IF EXISTS`). Re-runnable against the compose anchors only — it executes real DDL and is not part of any test suite. |
