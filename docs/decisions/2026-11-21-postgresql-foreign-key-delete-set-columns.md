# PostgreSQL 17 ON DELETE SET foreign-key column lists

Status Accepted

D32 extends the existing `ddl.pg.alter.add_foreign_key.targets.valid` blocker for table-level `ADD FOREIGN KEY` on a fresh, exact PostgreSQL 17 ordinary table. Preserve the ordered decoded identifiers from the parser, including repetitions. PostgreSQL resolves every action-column identity before verifying the set of allowed members. An observed missing column, system column or ordinary column outside the declared foreign-key columns supplies a distinct refusal. Valid duplicate members follow native acceptance. `NOT VALID` still checks declaration legality. The findings expose the authored target and a bounded reason. Catalog attributes and private declaration members are excluded from serialized metadata.

A PostgreSQL 17.7 fixture compared eight native commands with real CLI audits. A valid subset and repeated names succeeded. The observed refusals were `42P10` for a column outside the key, `42703` for a case-distinct missing column and `0A000` for a system column. The `ON UPDATE` column-list and `MATCH PARTIAL` inputs retained parser errors. These checks establish declaration eligibility. Runtime action effects, existing row validity and the referenced key's equality conversion require independent facts.

- `/tmp/ds-pg-f09-fk-delete-actions-cli-b6d73f04/evidence.json`
- [PostgreSQL 17.7 native subset check](https://github.com/postgres/postgres/blob/REL_17_7/src/backend/commands/tablecmds.c)

The combined T37 preview passed 447 cases and 12,506 assertions.
