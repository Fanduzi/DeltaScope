# PostgreSQL 17 foreign-key scalar equality prerequisites

Status Accepted

F09 D37 extends the existing `ddl.pg.alter.add_foreign_key.targets.valid` blocker. After exact relation and column identity, reference-key eligibility, and arity are established, compare each pair of authored columns using the operator class selected by the actual referenced key. For an omitted reference list, use the eligible primary key's ordered slots. For an explicit list, match the key as a set and map back to the authored order. If several immediate live valid keys qualify, PostgreSQL takes the first eligible index in OID order. The provider reads this ordering, equality strategy entries and applicable implicit casts in one catalog statement. The private observation binds both qualified tables and both ordered lists. It carries only the first incompatible position, with zero reserved for a proved acceptable set.

The bounded comparison includes scalar base and enum input types when the operator class uses a scalar base input. It checks the exact cross-type and same-source-type equality entries, followed by implicit cast paths for both actual types to the operator-class input type. Assignment-only casts do not decide this premise. Domains, arrays and polymorphic operator-class inputs retain `foreign_key.pg_equality_compatibility`. Permission, collation compatibility beyond this premise, live data validation, referential actions and execution outcomes still require separate observations. The finding contains the authored source column and a bounded reason, with catalog operator, index and dependent identities kept private.

Fourteen PostgreSQL 17.7 native transactions returned seven successes and seven `42804` refusals. Actual CLI and catalog controls distinguished cross-integer operator-family equality, implicit varchar conversion, assignment-only numeric casts, exact second-column mapping when the index order differs, and the scoped domain, enum and array gaps. Catalog and data identities matched after audit and native rollback.

- `/tmp/ds-pg-f09-fk-scalar-equality-cli-b49be2d4/evidence.json`
- `/tmp/ds-pg-f09-fk-scalar-equality-query-b49be2d4/catalog-read.json`
- [PG17.7 foreign-key operator compatibility and referenced index selection](https://github.com/postgres/postgres/blob/REL_17_7/src/backend/commands/tablecmds.c)
- [PG17.7 relation index ordering](https://github.com/postgres/postgres/blob/REL_17_7/src/backend/utils/cache/relcache.c)

The combined preview passed all 497 T37 cases with 14,257 assertions. The inventory records nine exact PostgreSQL 17 selectors for this subaction.
