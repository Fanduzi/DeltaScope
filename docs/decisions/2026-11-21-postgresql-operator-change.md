# PostgreSQL ALTER OPERATOR owner, schema and estimator changes

Status Accepted

## Decision

Enable `ddl.pg.alter_operator.change.target.valid` for PostgreSQL 17 OWNER TO
and SET SCHEMA, and for RESTRICT and JOIN estimator declarations. The rule requires an exact qualified operator signature and
literal operand identities from the AST. A unary operator uses the absent
left operand. Catalog lookup resolves qualified scalar and array operand
identities before the operator and checks occupancy by the full signature.

OWNER TO resolves the requested role first. SET SCHEMA resolves the source
before the destination and treats an existing source in its current namespace
as a no-op. A second operator occupying the same signature in the destination
is refused. A different signature can use the same name. Catalog OIDs and
role identities stay internal. Missing observations, unqualified operands,
session namespaces and execution privileges require further evidence.

The parser retains the authored RESTRICT and JOIN options and checks the last
value for each. NONE clears an option. Qualified estimators are checked against
PostgreSQL 17's exact signatures. RESTRICT takes `(internal, oid, internal,
integer)`. JOIN takes `(internal, oid, internal, smallint)` with an optional
fifth `internal` parameter. The presence of both JOIN versions is ambiguous.
The functions must return double precision. Nonempty estimators require a
Boolean operator result, and JOIN requires a binary operator. Other SET
options retain their existing unsupported boundary. Function selection through
`search_path` and execution permissions require more evidence.

## Verification

- [T41 operator controls](../../testdata/ddl-golden/T41.json)
- [Operator collision corpus](../../testdata/sql-corpus/postgresql/ddl/findings/alter_operator_destination_occupied.expected.yaml)
- [Exact operand extraction](../../internal/infrastructure/parser/postgresql/parser_operator_change_postgresql_tag_test.go)
- [PostgreSQL 17.7 operator commands](https://github.com/postgres/postgres/blob/REL_17_7/src/backend/commands/operatorcmds.c)

T41 preview passed 186 cases and 5,028 assertions. Twenty-seven references bind the exact owner, namespace and estimator cells.
