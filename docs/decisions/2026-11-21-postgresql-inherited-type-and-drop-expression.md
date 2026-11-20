# PostgreSQL 17 inherited TYPE and DROP EXPRESSION targets

Status Accepted

D35 shares the D33 exact catalog observation for one PostgreSQL 17 ALTER action. The source rule checks inherited attributes and direct partition keys before destination evaluation for TYPE and TYPE USING. On a stored generated source, USING still refuses first. DROP EXPRESSION refuses a direct inherited attribute even with IF EXISTS. ONLY on a parent with attached children has a separate refusal. The existing rules retain their current levels and availability. The facts exclude detached-pending children, catalog identifiers and expression text.

Native PostgreSQL 17.7 and the actual CLI matched the covered refusals. The recursive controls preserve the table data. Remaining TYPE parent-only traversal depends on earlier type and coercion prerequisites. ADD COLUMN recursion and the identity declaration on an ordinary inheritance parent have separate native conditions.

- `/tmp/ds-pg-f09-inherited-column-actions-cli-b49be2d4/evidence.json`

- [PostgreSQL 17.7 ALTER table implementation](https://github.com/postgres/postgres/blob/REL_17_7/src/backend/commands/tablecmds.c)

The corrected combined T37 preview passed 481/481 with 13,697 assertions. Its 22 new native commands returned five successes and seventeen refusals. The `exists` field on the ADD findings describes the observed column presence.
