# PostgreSQL materialized-view refresh prerequisites

Status Accepted

The F12 baseline on PostgreSQL 17.7 proves target identity, population and usable-index requirements separately from the existing lock advice and WITH NO DATA notice. The parser retains the decoded target schema and name, the catalog-qualification flag, CONCURRENTLY and WITH NO DATA. The identity facts stay private. It checks one fresh, exact schema-qualified materialized-view target on observed PostgreSQL 17.

The system-catalog query checks relation kind, `relispopulated`, and whether at least one local index is live, valid, unique, immediate, btree, nonpartial and defined over positive attribute numbers. Include every physical index attribute, including INCLUDE slots, because PG17 `is_usable_unique_index` checks `indnatts`. There is no NOT NULL condition for these index attributes. Hold index facts, population facts and catalog IDs outside JSON. Match all identity fields before using a returned observation.

Assess missing and wrong-kind target first. For CONCURRENTLY, check population, conflicting WITH NO DATA and the qualifying index in the native order. A known unpopulated view refuses even when a suitable index exists. A populated view with the conflicting flags refuses before testing indexes. An unqualified, catalog-qualified, temporary, stale or unobserved target yields an attributable evidence gap. The existing nonconcurrent lock warning stays independent. WITH NO DATA advice describes the planned state only for the nonconcurrent form. No product rule executes a refresh, reads user data or certifies grants, source query behavior, result cardinality or lock availability.

The existing native baseline records eleven PG17.7 cases and five paired real CLI shapes. Evidence binds only REFRESH MATERIALIZED VIEW at the PostgreSQL 17 anchor. Preserve earlier T40 cases and profiles, with exact inventory and canonical validation on delivery.

- `/tmp/ds-pg-f12-refresh-native-8cff67d4/evidence.json`
- `/tmp/ds-pg-f12-refresh-cli-8cff67d4/evidence.json`
- [PostgreSQL 17 REFRESH MATERIALIZED VIEW](https://www.postgresql.org/docs/17/sql-refreshmaterializedview.html)
- [PG17.7 refresh and index predicates](https://github.com/postgres/postgres/blob/REL_17_7/src/backend/commands/matview.c)

The rebased T40 preview passed all 47 cases and 801 assertions. The 13 new native controls returned four successes and nine refusals. SDK, HTTP and MCP tests confirm the blocker for a populated view without a usable index. Seventeen selectors bind the three exact PostgreSQL 17 subactions.

The invalid-index controls distinguish a live invalid unique index from a separate usable index. PostgreSQL returns `55000` when only the invalid index is present. The fresh view with both indexes refreshes successfully.
