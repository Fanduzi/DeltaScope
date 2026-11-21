# PostgreSQL 17 publication and subscription changes

Status Accepted

## Decision

Enable `ddl.pg.alter_publication.target.valid` and `ddl.pg.alter_subscription.target.valid` for bounded PostgreSQL 17 catalog premises. Keep the existing lifecycle notices and warnings. ENABLE and DISABLE come from the AST Boolean, including when the subscription name contains the word DISABLE.

Publication checks bind the exact database-local publication and ordered selectors. They check explicit relation eligibility and direct membership, named schemas and their membership, the FOR ALL TABLES boundary, and the bounded `publish` and `publish_via_partition_root` options. An existing partitioned root row filter or column list can prevent disabling publication via the root. Inheritance expansion, row-filter analysis, column-list validation, CURRENT_SCHEMA and unqualified relation binding require additional evidence.

Subscription checks use the connected database identity. They cover exact local target names, rename and owner resolution, ENABLE with an associated slot, bounded SET options, local publication-list changes, disabled refresh, and SKIP location syntax. Slotless DROP uses local catalog facts. A missing subscription with IF EXISTS takes the guarded result. PostgreSQL 17 rejects `DROP SUBSCRIPTION ... WITH (drop_slot=...)` at parse time.

Connection validation, publisher refresh and slot effects, and a valid nonzero SKIP location require additional evidence. The audit reads local catalogs. Catalog publication names, slot identifiers and connection options stay outside findings and gaps. Existing SQL echo fields preserve the supplied SQL.

## Verification

- [Canonical T46](../../testdata/ddl-golden/T46.json)
- [Publication option corpus](../../testdata/sql-corpus/postgresql/ddl/findings/alter_publication_unknown_option.expected.yaml)
- [Subscription slot corpus](../../testdata/sql-corpus/postgresql/ddl/findings/alter_subscription_enable_without_slot.expected.yaml)
- [PostgreSQL 17.7 publication implementation](https://github.com/postgres/postgres/blob/REL_17_7/src/backend/commands/publicationcmds.c)
- [PostgreSQL 17.7 subscription implementation](https://github.com/postgres/postgres/blob/REL_17_7/src/backend/commands/subscriptioncmds.c)

T46 preview passed 96 cases and 3,981 assertions. The inventory binds 69 references across the 12 PostgreSQL 17 cells.
