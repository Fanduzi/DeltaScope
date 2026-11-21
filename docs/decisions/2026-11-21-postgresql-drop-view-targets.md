# PostgreSQL 17 DROP VIEW targets and direct dependencies

Status Accepted

The default blocker `ddl.pg.drop_view.target.valid` checks a single, fresh, schema-qualified DROP VIEW target on an observed PostgreSQL 17 server. IF EXISTS skips a proven missing relation. A present table or materialized view still fails the kind check. Under RESTRICT, a normal rewrite dependency from another view or materialized view proves refusal. The catalog query retains neither the dependent name nor its definition. The separate CASCADE warning and DROP VIEW forbid remain available.

Nine PostgreSQL 17.7 and CLI comparisons verify exact case and spacing, guarded absence, wrong kind, both dependent view kinds, and native CASCADE. Their rollback checks preserve relation identity, row data, and an independent sequence. The audit requires a fresh catalog and reports an evidence gap when the exact target state is unavailable. Search-path resolution, temporary relations, multi-target DROP VIEW, other dependency classes and broader CASCADE effects require separate evidence.

- [PostgreSQL 17 DROP VIEW](https://www.postgresql.org/docs/17/sql-dropview.html)
- [T40 native and audit controls](../../testdata/ddl-golden/T40.json)

The combined preview passed 68 T40 cases with 1,482 assertions. Eleven selectors bind the two DROP VIEW inventory subactions.
