# PostgreSQL 17 ALTER POLICY targets, roles and clauses

Status Accepted

## Decision

Enable `ddl.pg.alter_policy.target.valid` for qualified PostgreSQL 17 policy
changes on ordinary and partitioned tables. Resolve named roles in authored
order up to PUBLIC. Roles after PUBLIC are ignored. Bind each policy and
rename destination to its exact relation. PostgreSQL tests rename occupancy
before looking for the old policy name, including a rename to the same name.

The audit checks simple one-part predicate column references, Boolean and NULL
constants and null tests on those column references. A SELECT or DELETE
policy forbids WITH CHECK. An INSERT policy forbids USING. Qualifying a
predicate's type or behavior beyond the observed syntax and catalog facts
requires more evidence. Policy names and authored table identities can appear
in findings. Catalog column and role directories and predicate bodies stay
internal.

Session relation lookup, general expression resolution, privileges and
resulting row access require separate evidence. The combined TO and RENAME
form retains its PostgreSQL 17 parser diagnostic.

## Verification

- [T42 ALTER POLICY controls](../../testdata/ddl-golden/T42.json)
- [SELECT WITH CHECK refusal](../../testdata/sql-corpus/postgresql/ddl/findings/alter_policy_select_with_check.expected.yaml)
- [Policy declaration tests](../../internal/infrastructure/parser/postgresql/parser_policy_change_postgresql_tag_test.go)
- [PostgreSQL 17.7 policy commands](https://github.com/postgres/postgres/blob/REL_17_7/src/backend/commands/policy.c)

T42 preview passed 65 cases and 1,261 assertions. Twenty-one references bind the two exact PostgreSQL 17 policy cells.
