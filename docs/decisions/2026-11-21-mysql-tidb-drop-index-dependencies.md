# MySQL and TiDB standalone index dependencies

Status Accepted

## Decision

Enable `ddl.drop_index.foreign_key.warn` for a standalone `DROP INDEX` whose ordered pre-state proves that the named secondary member is the sole eligible index supporting a foreign key. Use the same dependency classification as ordered state. Emit the warning without foreign key names or catalog member details. The warning is based on positive evidence, and an unknown dependency remains outside the positive finding. Earlier audit steps can establish a referenced-side dependency when the referencing table has been loaded. A parent-only catalog read does not establish the full inbound directory.

Honor TiDB `DROP INDEX IF EXISTS` in the standalone existence rule. A missing index is an accepted no-op on native TiDB 8.5.0. The positive dependency warning can still apply when a guarded DROP names an existing required index. Preserve the existing MySQL and ALTER TABLE rule contracts.

## Verification

T09 compares native refusal of a sole supporting index, successful drop with an alternate index, and missing or guarded targets. Standalone CREATE INDEX uses the existing target-column, duplicate-name and configured prefix-length checks. Index options and broader inbound directory proofs require separate evidence.

- [T09 index lifecycle controls](../../testdata/ddl-golden/T09.json)
- [MySQL foreign key requirements](https://dev.mysql.com/doc/refman/8.4/en/create-table-foreign-keys.html)
- [TiDB DROP INDEX](https://docs.pingcap.com/tidb/v8.5/sql-statement-drop-index/)

## Inventory cells

### CREATE INDEX

T09 verifies exact prefix creation and duplicate name refusal at all three MySQL anchors.

### CREATE UNIQUE INDEX

The standalone unique-index control creates the authored five-character prefix and retains the stored member.

### key parts

T09 compares an absent explicit column with a successful single-column key.

### prefix lengths

The configured existing length rule detects an excessive prefix. A full declared column width produces a NULL `SUB_PART` in the catalog. A shorter prefix retains its numeric `SUB_PART`.

### DROP INDEX name ON table

T09 compares sole local support, alternate support and parent-only reads on MySQL and TiDB. The latter cannot establish the inbound dependence. TiDB's standalone guarded missing case succeeds without an index existence blocker.

The isolated T09 preview passed 157 cases and 4,440 assertions. 41 case references bind six authored cells.
