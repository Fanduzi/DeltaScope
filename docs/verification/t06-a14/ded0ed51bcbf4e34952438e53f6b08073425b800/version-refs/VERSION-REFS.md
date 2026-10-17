# T06-A14 version material references

Retrieved: 2026-10-17 (UTC, see retrieved-at.txt). Full pages not committed;
key excerpts below. These are the basis for the frozen per-product catalog
oracles, not execution records.

## TiDB 8.5 — CREATE TABLE (pingcap/docs release-8.5)

URL: https://raw.githubusercontent.com/pingcap/docs/release-8.5/sql-statements/sql-statement-create-table.md
Saved: tidb85-create-table.md (310 lines, markdown)

> "The following *table_options* are supported. Other options such as
> `AVG_ROW_LENGTH`, `CHECKSUM`, `COMPRESSION`, `CONNECTION`,
> `DELAY_KEY_WRITE`, `ENGINE`, `KEY_BLOCK_SIZE`, `MAX_ROWS`, `MIN_ROWS`,
> `ROW_FORMAT` and `STATS_PERSISTENT` are parsed but ignored." (line 162)

## TiDB 8.5 — INFORMATION_SCHEMA.TABLES (pingcap/docs release-8.5)

URL: https://raw.githubusercontent.com/pingcap/docs/release-8.5/information-schema/information-schema-tables.md
Saved: tidb85-info-tables.md (138 lines, markdown)

> "ENGINE: The type of the storage engine. The value is currently `InnoDB`."
> "ROW_FORMAT: The row format. The value is currently `Compact`."
> (lines 108–110; example block shows `ENGINE: InnoDB`, `ROW_FORMAT: Compact`,
> `TABLE_COLLATION: utf8mb4_bin`)

## MySQL 8.4 — INFORMATION_SCHEMA.TABLES (Oracle docs)

URL: https://docs.oracle.com/cd/E17952_01/mysql-8.4-en/information-schema-tables-table.html
Saved: mysql84-tables-excerpt.md (10450 bytes text-excerpt, original HTML not committed, retrieval snapshot only)

> "ENGINE — The storage engine for the table."
> "ROW_FORMAT — The row-storage format (Fixed, Dynamic, Compressed,
> Redundant, Compact). … The actual row format of the table is reported
> in the ROW_FORMAT column."
> "TABLE_COLLATION — The table default collation. The output does not
> explicitly list the table default character set, but the collation
> name begins with the character set name."
> (the A14 oracle uses the real COLLATION_CHARACTER_SET_APPLICABILITY
> join, never the name prefix)

## MySQL 8.4 — Table Character Set and Collation (Oracle docs §12.3.4)

URL: https://docs.oracle.com/cd/E17952_01/mysql-8.4-en/charset-table.html
Saved: mysql84-charset-table-excerpt.md (2348 bytes text-excerpt, original HTML not committed, retrieval snapshot only)

> "Every table has a table character set and a table collation. The
> CREATE TABLE and ALTER TABLE statements have optional clauses for
> specifying the table character set and collation."
