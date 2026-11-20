# PostgreSQL 17 default operator class resolution for constraint attachment

Status Accepted

F09 D41 extends the existing `ddl.pg.alter.constraint_using_index.valid`
check. PostgreSQL reduces domains to their base types and selects a unique
exact default first. It then selects a unique preferred binary-compatible
class in the same type category, or a unique remaining binary-compatible
class. Catalog observations include implicit binary casts and the PostgreSQL
17 polymorphic, range and record cases. The selected class is compared with
the installed class in each key slot. INCLUDE columns are outside that
comparison. Ambiguous or unavailable candidates retain the existing gap.

Nine PostgreSQL 17.7 controls exercise preferred varchar selection, a different
compatible installed class, a nondefault class, nested domains, enums, arrays,
ranges, multiranges and composite types. Seven succeed and two return `42809`.
The CLI reports only the authored attachment and bounded failure reason.
Operator class identifiers and type catalog rows remain private. Permissions,
lock availability and checks of existing table rows require separate facts.

- `/tmp/ds-pg-f09-opclass-fallback-cli-683d84ac/evidence.json`
- [PostgreSQL 17.7 default operator class selection](https://github.com/postgres/postgres/blob/REL_17_7/src/backend/commands/indexcmds.c)
- [PostgreSQL 17.7 binary coercion](https://github.com/postgres/postgres/blob/REL_17_7/src/backend/parser/parse_coerce.c)

The joint T37 preview passed 535 cases with 15,589 assertions.
