# PostgreSQL 17 enum declarations and label changes

Status Accepted

## Decision

Enable `ddl.pg.create_type.enum.valid` and `ddl.pg.alter_type.enum.valid` for
qualified PostgreSQL 17 enum targets. CREATE checks the namespace, existing
type eligibility, duplicate labels and label byte length. A shell type can
be filled and PostgreSQL can move a conflicting automatic array type. The
catalog lookup distinguishes these cases from an occupied type name.

ALTER checks type existence and kind, new label byte length, source label
existence for RENAME, destination label collisions and placement label
existence for ADD. A duplicate guarded ADD skips placement validation.
Qualified targets and catalog observations must agree. Earlier DDL can make
type or label observations stale.

CREATE preserves every label, including empty strings and repetition. ALTER
uses the parser's JSON AST when its protobuf representation cannot distinguish
an absent field from an empty label. RENAME VALUE uses its own normalized
action, so the existing ADD VALUE advice only applies to ADD. Findings expose
the authored type target and reason code. Resolved labels remain internal.

The byte checks use the observed UTF8 encoding and standard 63-byte server
identifier limit. Other encodings or builds leave an evidence gap. Session
namespace selection, ownership and privileges, and transaction restrictions on
using newly added labels require separate evidence.

## Verification

- [T41 native enum controls](../../testdata/ddl-golden/T41.json)
- [Missing enum label corpus](../../testdata/sql-corpus/postgresql/ddl/findings/alter_enum_missing_label.expected.yaml)
- [Label presence tests](../../internal/infrastructure/parser/postgresql/parser_enum_labels_postgresql_tag_test.go)
- [PostgreSQL 17.7 enum catalog operations](https://github.com/postgres/postgres/blob/REL_17_7/src/backend/catalog/pg_enum.c)
- [PostgreSQL 17.7 enum type commands](https://github.com/postgres/postgres/blob/REL_17_7/src/backend/commands/typecmds.c)

T41 preview passed 107 cases and 1,781 assertions. Twenty-nine references bind the three PostgreSQL 17 enum dimensions.
