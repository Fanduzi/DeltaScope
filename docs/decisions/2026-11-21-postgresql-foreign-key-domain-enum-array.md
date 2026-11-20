# PostgreSQL 17 foreign-key equality for domains, enums and arrays

Status Accepted

F09 D42 extends `ddl.pg.alter.add_foreign_key.targets.valid` on fresh,
exact PostgreSQL 17 ordinary tables. The catalog query uses the selected
reference key and preserves authored pair order. Exact equality operator
pairs use source-domain base types. The native `anyenum` fallback compares
the actual enum identities, so a domain over an enum can still refuse. The
`anyarray` fallback compares true array types after unwrapping domains.
Other polymorphic and coercion cases retain `foreign_key.pg_equality_compatibility`.

Eighteen matched PostgreSQL 17.7 and CLI controls returned nine successes
and nine `42804` refusals. They include nested domains on both sides,
domains with incompatible bases, matching and different enums, matching and
different arrays, domains over enums or arrays, and authored-order controls. Only the authored
column and bounded reason are public. Existing row validity, runtime actions,
locks and permissions require separate evidence.

- `/tmp/ds-pg-f09-fk-domain-polymorphic-cli-v2-683d84ac/evidence.json`
- [PostgreSQL 17.7 foreign-key attachment](https://github.com/postgres/postgres/blob/REL_17_7/src/backend/commands/tablecmds.c)
- [PostgreSQL 17.7 polymorphic coercion](https://github.com/postgres/postgres/blob/REL_17_7/src/backend/parser/parse_coerce.c)

The joint T37 preview passed 535 cases with 15,589 assertions.
