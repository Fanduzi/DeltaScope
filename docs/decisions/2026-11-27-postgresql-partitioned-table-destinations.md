# PostgreSQL 17 partitioned-table destination refusals

Status: Accepted

The existing access-method, tablespace and owner blockers now also cover
six destination refusals on a fresh, schema-qualified persistent partitioned
table (`relkind = p`) in a single ALTER TABLE action. PostgreSQL 17.7 refuses
an absent access method (`42704`), an index access method (`55000`), an absent
tablespace (`42704`), `pg_global` (`22023`), an absent named owner (`42704`),
and `PUBLIC` (`42704`). The lookup rechecks relation kind and persistence in
the same catalog query as the destination. Findings reuse the existing
reason codes and do not publish private catalog facts.

Ordinary-table checks and advisories are unchanged. A valid destination
on a partitioned table retains its evidence gap, even when the current
setting or owner matches. Bare `DEFAULT`, owner aliases, ordinary leaf
partitions, multi-action statements, permissions, physical movement and
changes to child partitions are outside this extension.

The [T37 cases](../../testdata/ddl-golden/T37.json) named
`pg-partition-destination-*` bind to `SET ACCESS METHOD`, `SET TABLESPACE`
and `OWNER TO` at the PostgreSQL 17 anchor. The inventory classifications
and denominator are unchanged. See the [PostgreSQL 17 ALTER TABLE reference](https://www.postgresql.org/docs/17/sql-altertable.html).
