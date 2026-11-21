# PostgreSQL CREATE TABLE primary and unique keys

Status Accepted

## Decision

Enable `ddl.pg.create_table.keys.valid` as a blocker for proved refusal on a
qualified PostgreSQL 17 CREATE TABLE with direct primary or unique key
constraints. Preserve every declaration and the ordered key and INCLUDE names.
The rule checks schema and guard precedence, multiple primary declarations,
`USING INDEX`, missing or repeated key members, missing INCLUDE members and
system columns. Repeated INCLUDE and overlap between key and INCLUDE remain
valid. `NULLS NOT DISTINCT` is retained from the parser and covered by a native
index catalog witness.

Only declarations with a potential refusal need catalog binding. Missing or
stale destination evidence leaves a gap for those declarations. A different
relation can occupy the name before system-column indexing would occur. The
rule reports the observed occupancy in that case. Earlier DDL can invalidate
the destination catalog evidence. Typed, inherited, temporary and LIKE
declarations can depend on columns
or namespaces outside the direct declaration. Permissions, access methods,
operator classes and constraint execution require separate evidence.

Findings carry the authored table target and a reason code. The catalog lookup
shares the destination witness with the `ON COMMIT` rule.

## Verification

- [T36 native constraint controls](../../testdata/ddl-golden/T36.json)
- [Missing unique-key member corpus](../../testdata/sql-corpus/postgresql/ddl/findings/create_unique_absent_column.expected.yaml)
- [Exact key declaration extraction](../../internal/infrastructure/parser/postgresql/parser_create_keys_postgresql_tag_test.go)
- [PostgreSQL 17.7 creation transform](https://github.com/postgres/postgres/blob/REL_17_7/src/backend/parser/parse_utilcmd.c)

The joint T36 preview passed 144 cases and 3,518 assertions. 28 references bind five exact PostgreSQL 17 cells.
