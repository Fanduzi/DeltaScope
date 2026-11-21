# PostgreSQL shell and base type declarations

Status Accepted

## Decision

Enable `ddl.pg.type.base.valid` for qualified PostgreSQL 17 shell declarations,
base type definitions and SET property changes. Shell creation checks namespace
and type occupancy. An existing shell occupies the name. PostgreSQL can move
an automatic array out of the way. A full base type definition needs a shell,
input and output declarations, and unique recognized parameters.

The SET checks bind the exact source type and its storage. Fixed-size types
require PLAIN. Existing non-PLAIN storage cannot change to PLAIN. Other
supported storage strategies retain their authored order. PostgreSQL rejects
unrecognized or immutable properties and SET on types other than non-array
base types. Support function resolution remains a gap where it is required.
Findings expose the authored type target and the reason code.

The rule leaves namespace selection, function signatures and ownership,
representation compatibility, dependent type changes and execution privileges
open. Native probes for built-in storage changes use rollback in an isolated
fixture.

## Verification

- [T41 native type controls](../../testdata/ddl-golden/T41.json)
- [Missing input function corpus](../../testdata/sql-corpus/postgresql/ddl/findings/create_base_type_missing_io.expected.yaml)
- [PostgreSQL 17.7 shell, base and ALTER TYPE commands](https://github.com/postgres/postgres/blob/REL_17_7/src/backend/commands/typecmds.c)

T41 preview passed 221 cases and 6,241 assertions. Eighteen references bind the three PostgreSQL 17 type declaration and property cells.
