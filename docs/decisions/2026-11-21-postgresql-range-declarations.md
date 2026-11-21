# PostgreSQL CREATE TYPE AS RANGE declarations

Status Accepted

## Decision

Enable `ddl.pg.create_type.range.valid` for qualified PostgreSQL 17 range declarations. The rule checks the creation namespace and existing type before inspecting options. It accepts the type name of an undefined shell or a movable automatic array. It preserves option order and resolves the first qualified scalar subtype through the existing type catalog query.

The rule detects missing subtypes, absent or undefined subtype targets, pseudo-types, duplicate and unknown options, collation on an uncollatable subtype and a canonical function specified without a pre-created shell. The finding carries the authored range target and a reason code.

Default operator class selection, explicit operator classes and collation names, multirange names, support function signatures and execution privileges require more evidence. The audit records those unresolved premises even when the native range declaration succeeds. `CREATE TYPE ... AS RANGE ()` preserves the native `42601` parser boundary.

## Verification

- [T41 native and CLI controls](../../testdata/ddl-golden/T41.json)
- [Missing subtype corpus](../../testdata/sql-corpus/postgresql/ddl/findings/create_range_no_subtype.expected.yaml)
- [Range assessment tests](../../internal/domain/rule/ddl/postgresql_range_rules_test.go)
- [PostgreSQL 17.7 range definition](https://github.com/postgres/postgres/blob/REL_17_7/src/backend/commands/typecmds.c)

T41 preview passed 204 cases and 5,655 assertions. Nineteen references bind the exact PostgreSQL 17 RANGE cell.
