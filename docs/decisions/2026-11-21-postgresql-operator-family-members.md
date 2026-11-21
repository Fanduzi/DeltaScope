# PostgreSQL 17 B-tree search-operator members

Status Accepted

`ddl.pg.alter_operator_family.member.valid` checks one qualified B-tree search-operator ADD or DROP. The catalog binds the authored family and access method before evaluating the member. The strategy must be in 1–5. ADD checks the exact binary operator and its Boolean result before checking whether the strategy and operand slot is occupied. DROP checks the same slot and refuses a pinned system or internally owned member. Findings use the authored family identity. Catalog object identifiers and discovered dependency names stay private.

Twelve PostgreSQL 17.7 and CLI comparisons cover a valid ADD and DROP, slot presence and absence, strategy bounds, source precedence, qualified missing operator and type, and a pinned catalog member. Unqualified operands or operators require session resolution evidence. Other methods, ordering operators, support functions, multiple members, and broader DROP dependencies require separate checks.

- [PostgreSQL 17 ALTER OPERATOR FAMILY](https://www.postgresql.org/docs/17/sql-alteropfamily.html)
- [PostgreSQL 17.7 member processing](https://github.com/postgres/postgres/blob/REL_17_7/src/backend/commands/opclasscmds.c#L817)
- [PostgreSQL 17.7 pinned objects](https://github.com/postgres/postgres/blob/REL_17_7/src/backend/catalog/catalog.c#L343)
- [T39 controls](../../testdata/ddl-golden/T39.json)

The joint T39 preview passed all 130 cases and 2,948 assertions at base `d4cf7b6c`.
