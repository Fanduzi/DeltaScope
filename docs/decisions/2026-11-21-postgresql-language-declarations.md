# PostgreSQL 17 procedural language declarations

Status Accepted

## Decision

Enable `ddl.pg.create_language.handler.valid` for explicit handler-form declarations. A qualified handler must resolve to a function with zero arguments and a `language_handler` return type. An INLINE callback resolves with one `internal` argument. A VALIDATOR resolves with one `oid` argument. PostgreSQL checks these signatures before it checks for an occupied language name. `OR REPLACE` preserves the existing language identity and owner. The audit reports the authored language name and a bounded reason.

Unqualified callbacks require the effective search path. Actor privileges, callback execution and dependencies require separate evidence. The catalog query reads callback identities and signatures without invoking them.

The PostgreSQL grammar emits a `CreateExtensionStmt` for parameterless `CREATE LANGUAGE`. Its `OR REPLACE` sets the extension `IF NOT EXISTS` flag and `TRUSTED` has no effect. Extension availability, installation permissions and scripts remain governed by the extension path. The native fixtures compare installed and unavailable extensions and preserve the existing extension classification.

## Verification

- [Canonical T47](../../testdata/ddl-golden/T47.json)
- [Missing callback corpus](../../testdata/sql-corpus/postgresql/ddl/findings/create_language_missing_handler.expected.yaml)
- [PostgreSQL 17.7 CREATE LANGUAGE grammar](https://github.com/postgres/postgres/blob/REL_17_7/src/backend/parser/gram.y#L4986-L5016)
- [PostgreSQL 17.7 handler and replacement semantics](https://github.com/postgres/postgres/blob/REL_17_7/src/backend/commands/proclang.c)

T47 preview passed 93 cases and 1,674 assertions. Twenty references bind the three original language cells.
