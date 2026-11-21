# PostgreSQL 17 object privilege declarations

Status Accepted

## Decision

Enable `ddl.pg.grant.declaration.valid` for local PostgreSQL 17 object privilege declarations. Preserve authored target, privilege and column order. Resolve each exact qualified relation and each named schema, database and tablespace. Compare privilege names to the allowed object classes and resolve each explicit column, including cataloged system columns. A qualified view and an ordinary table can expose different system columns. Quoted names and repeated columns preserve native behavior.

Resolve GRANTED BY identity before object and grantee checks. The observed catalog session supplies the current role for that comparison. Roles, column directories and ACL values stay outside new findings. The rule checks declared identity and compatibility only. Existing table privilege policy rules still apply.

ALL TABLES IN SCHEMA can verify its authored schemas. Expansion of schema members needs further evidence. A PUBLIC grant option needs evidence that the requested privilege survives grantor checks. Effective grant chains, cascading revoke effects, membership and actor authorization require separate facts. Unqualified relation names require effective search path evidence.

## Verification

- [Canonical T47](../../testdata/ddl-golden/T47.json)
- [Missing grant column corpus](../../testdata/sql-corpus/postgresql/ddl/findings/grant_missing_column.expected.yaml)
- [PostgreSQL 17.7 privilege statement processing](https://github.com/postgres/postgres/blob/REL_17_7/src/backend/catalog/aclchk.c)

The combined T47 preview passed 125 cases and 3,324 assertions. 38 references bind the thirteen privilege cells.
