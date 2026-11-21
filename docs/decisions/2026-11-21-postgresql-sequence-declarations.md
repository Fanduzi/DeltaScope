# PostgreSQL sequence declarations and OWNED BY

Status Accepted

## Decision

Enable `ddl.pg.sequence.declaration.valid` for exact qualified PostgreSQL 17
CREATE and ALTER SEQUENCE statements. CREATE IF NOT EXISTS checks the target
namespace and occupied relation before the numeric options. Unguarded CREATE
validates its declaration first. ALTER IF EXISTS can skip an absent relation.
A relation of another kind is refused for ALTER.

Ordered options retain repeated declarations and exact signed values. The rule
checks eligible data types, nonzero increments, ascending and descending
bounds, explicit and default START, positive CACHE and explicit or bare
RESTART. ALTER uses stable definition facts from `pg_sequence`. A declaration
that tightens bounds without a replacement restart value requires current
value evidence.

OWNED BY preserves exact relation and column names. The catalog observation
checks the owner relation kind, matching owner and schema, then column
existence. Existing identity dependencies forbid changing the ownership link.
Findings report the authored sequence and a reason code. Catalog owners and
object identities stay internal. The metadata reader neither queries nor
advances sequence issuance state.

RENAME checks the relation name slot. Renaming an owned or identity sequence
preserves its ownership link. SET SCHEMA checks automatic and identity ownership
before destination lookup. PostgreSQL refuses to move an owned sequence even to
its current schema. A free sequence can remain in its current schema, and a
move to a different schema checks namespace eligibility and relation occupancy.

Unqualified and session-dependent names, execution privileges and concurrent
definition changes require separate evidence.

## Verification

- [T43 native sequence controls](../../testdata/ddl-golden/T43.json)
- [Zero increment refusal](../../testdata/sql-corpus/postgresql/ddl/findings/create_sequence_zero_increment.expected.yaml)
- [PostgreSQL 17.7 sequence commands](https://github.com/postgres/postgres/blob/REL_17_7/src/backend/commands/sequence.c)

T43 preview passed 86 cases and 2,951 assertions. Fifty-one references bind all seven original PostgreSQL 17 sequence dimensions.
