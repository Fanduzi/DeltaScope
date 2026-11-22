# MySQL and TiDB repeated ADD COLUMN declarations

Status Accepted

## Decision

Use `ddl.alter.add_column.exists.forbid` to report the first repeated column name in an ALTER TABLE containing only ADD COLUMN actions. The parser already produces an ordered action for every column in `ADD (...)` and for each separate ADD clause. The rule compares decoded names case-insensitively. It can establish the declaration conflict without a table snapshot. With a known pre-state, the ordered model keeps that exact state after the refusal.

MySQL 5.7.44, 8.0.46 and 8.4.10 refuse a repeated grouped addition with `1060`. TiDB 8.5.0 refuses repeated grouped and separate additions with `8200`, including a repeated name with `IF NOT EXISTS` on one ADD action. The case does not generalize to mixed action lists. Ordered uses of a previous ADD as an AFTER target keep their dialect-specific handling.

## Verification

- [T05 column and ordered-state fixtures](../../testdata/ddl-golden/T05.json)
- [Repeated name corpus](../../testdata/sql-corpus/mysql/ddl/findings/alter_table_add_list_duplicate.expected.yaml)
- [MySQL ALTER TABLE](https://dev.mysql.com/doc/refman/8.4/en/alter-table.html)
- [TiDB ALTER TABLE](https://docs.pingcap.com/tidb/v8.5/sql-statement-alter-table/)

## Inventory cells

### GENERATED ALWAYS AS (expr), VIRTUAL and STORED

T07 verifies direct references, quoted identifiers, virtual chains and stored dependencies. The audit does not claim general expression evaluation.

### expression DEFAULT (8.0.13+) and INVISIBLE column (8.0.23+)

T07 compares the observed version gates, valid defaults and visibility effects. The declaration check uses parsed structure.

### ADD COLUMN, ADD COLUMN FIRST/AFTER and ADD (col, col, ...)

T05 compares first-path addition, FIRST, AFTER, distinct grouped additions and repeated declarations. T08 includes missing AFTER targets.

### DROP COLUMN

T07 contrasts dropping a generated leaf with a base column still used by another generated column. Other dependencies retain their existing fact requirements.

### MODIFY COLUMN, type change, NULL/NOT NULL transitions and FIRST/AFTER

T05 and T08 compare narrowing, widening, explicit nullability and positioning. A valid declaration can still fail when executed against particular data.

### comma-separated action lists

T05 and T08 compare explicit action order and published column order. Mixed statements remain subject to the existing ordered-state and incomplete-evidence rules.

### ADD/DROP/MODIFY/CHANGE/RENAME COLUMN and TiDB same-object modification limits

The TiDB controls compare each modeled action and show refusal when a name is repeated. In a separate native comparison, a MODIFY list targeting the same column twice returned `8200` and kept the table intact. Version and per-action catalog evidence still control other combinations.

The isolated T05 preview passed 268 cases and 5,512 assertions. 80 canonical references are bound to 17 authored cells.
