# MySQL 8.x INSTANT ADD COLUMN table prerequisites

Status Accepted

## Decision

Use the existing `ddl.alter.online_ddl.applicability` rule and the shared online DDL classifier for observed table restrictions. On MySQL 8.x, an ALTER with explicit `ALGORITHM=INSTANT` and only ADD COLUMN actions returns the native refusal when the bound pre-state positively identifies a MyISAM table, an InnoDB table with COMPRESSED row format, or an InnoDB table with a FULLTEXT index. The classifier preserves the prior version, lock and action checks. On a refusal, ordered audit preserves the pre-statement schema so a later statement cannot bind a column that would never land.

The decision uses three observed restrictions and preserves the existing version and engine behavior elsewhere. Instant row-version limits, maximum record size, temporary tables, and dictionary tablespace placement need separate evidence. The rule reports the existing refusal code without exposing index identities.

## Verification

- [T15 native comparisons](../../testdata/ddl-golden/T15.json)
- [Compressed-table corpus](../../testdata/sql-corpus/mysql/ddl/findings/alter_table_instant_compressed.expected.yaml)
- [MySQL 8.0 online DDL operations](https://dev.mysql.com/doc/refman/8.0/en/innodb-online-ddl-operations.html)
- [MySQL 8.4 online DDL operations](https://dev.mysql.com/doc/refman/8.4/en/innodb-online-ddl-operations.html)

The T15 preview passed 94 cases and 1,778 assertions. The three source rows bind 88 exact references.

### ORDER BY

On MySQL 5.7.44 and 8.4.10, `ALTER TABLE f05_fixture.t0 ORDER BY id` succeeded against an InnoDB table with an integer primary key and a `VARCHAR(32)` column. The audit returned `ddl.alter.rebuild.warn`, verified unchanged pre-state, and had no evidence gap. The native command preserved the `SHOW CREATE TABLE` definition and the witness row. Physical order and actual work require separate evidence.

### FORCE

On MySQL 5.7.44 and 8.4.10, `ALTER TABLE f05_fixture.t1 FORCE` succeeded against the same table shape. The audit returned `ddl.alter.rebuild.warn`, verified unchanged pre-state, and had no evidence gap. The native command preserved the `SHOW CREATE TABLE` definition and the witness row.

### CONVERT TO CHARACTER SET

On MySQL 5.7.44 and 8.4.10, `ALTER TABLE f05_fixture.t2 CONVERT TO CHARACTER SET utf8mb4 COLLATE utf8mb4_bin` succeeded against the same table shape with `DEFAULT CHARACTER SET latin1`. The audit returned `ddl.alter.charset_conversion.warn`, verified unchanged pre-state, and had no evidence gap. The native command preserved the row and `information_schema.COLUMNS` reported `utf8mb4` and `utf8mb4_bin` for the character column.

The six controls used fresh copies of `CREATE TABLE f05_fixture.tN (id INT NOT NULL AUTO_INCREMENT PRIMARY KEY, label VARCHAR(32)) ENGINE=InnoDB DEFAULT CHARACTER SET latin1` and inserted `(7, 'witness')` before audit. The independent catalog query selected `CHARACTER_SET_NAME, COLLATION_NAME` from `information_schema.COLUMNS` for `TABLE_SCHEMA='f05_fixture'`, the exact fixture table, and `COLUMN_NAME='label'`.

### ENABLE/DISABLE KEYS

On MySQL 5.7.44, 8.0.46 and 8.4.10, both ALTER statements succeeded on the MyISAM fixture below. After `DISABLE KEYS`, `SHOW INDEX` reported `Comment=disabled` for `secondary_v` and an empty comment for `PRIMARY` and `uq`. After `ENABLE KEYS`, all three comments were empty. Both native steps preserved the two rows. The CLI emitted `ddl.alter.keys_toggle.notice` with the MyISAM engine and the authored action. Reading the index state before and after each audit proved the audit preserved it.

```sql
CREATE TABLE d.t (id INT PRIMARY KEY, u INT NOT NULL, v INT NOT NULL,
                  UNIQUE KEY uq (u), KEY secondary_v (v)) ENGINE=MyISAM;
INSERT INTO d.t VALUES (1,10,100),(2,20,200);
SHOW INDEX FROM d.t;
ALTER TABLE d.t DISABLE KEYS;
SHOW INDEX FROM d.t;
ALTER TABLE d.t ENABLE KEYS;
SHOW INDEX FROM d.t;
SELECT * FROM d.t ORDER BY id;
```

The independent witness uses the `Comment` column in `SHOW INDEX`. MySQL 5.7.44's `INFORMATION_SCHEMA.STATISTICS.COMMENT` returned an empty value in the disabled state. The fixture proves the index-state transition and preserves the separate InnoDB T15 comparison. The MyISAM notice now distinguishes stopping maintenance of nonunique indexes from rebuilding disabled nonunique indexes. Lock duration and physical work depend on execution conditions.
