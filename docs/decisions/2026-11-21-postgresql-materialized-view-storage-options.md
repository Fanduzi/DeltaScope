# PostgreSQL 17 materialized-view storage options

Status Accepted

`ddl.pg.alter_materialized_view.options.valid` checks a single SET or RESET on an exactly qualified materialized view. The source catalog binds relation kind, heap access method and the existence of an attached TOAST relation. A proved absent source under IF EXISTS is silent. A present relation of the wrong kind blocks before the option check.

RESET entries with values block. Unknown or repeated names without values are accepted by PostgreSQL. SET rejects an unrecognized namespace. On a heap materialized view, the audit checks integer fillfactor bounds, `autovacuum_enabled` Boolean validity and duplicates for those recognized properties. PostgreSQL ignores `toast` entries when the materialized view has no attached TOAST relation. For a present TOAST relation the audit checks `toast.autovacuum_enabled` separately. Other option names, quoted fillfactor values, custom access methods, unqualified source resolution and execution authorization require separate facts. The findings carry the authored target and a reason code. Private catalog identities and option values stay in the audit process.

The native test uses PostgreSQL 17.7 with both integer-only and text materialized views. It compares successful SET and RESET effects, option and target refusal codes, and catalog, data and sequence state before and after audit. T40 includes the native controls and the matching audit responses.

- [PostgreSQL 17 ALTER MATERIALIZED VIEW](https://www.postgresql.org/docs/17/sql-altermaterializedview.html)
- [PostgreSQL 17.7 option transformation](https://github.com/postgres/postgres/blob/REL_17_7/src/backend/access/common/reloptions.c)
- [T40 controls](../../testdata/ddl-golden/T40.json)

The joint T40 preview passed 147 cases and 4,224 assertions.
