# DeltaScope v0.513.0 Release Notes

Release date: 2026-11-15

## Summary

v0.513.0 completes the evidence-backed semantic-acceptance programme: PostgreSQL official DDL acceptance, an official DML denominator across all dialect surfaces, two long-standing version-applicability debts discharged, and the first parser-upgrade candidate graduated to real audit. The canonical golden set grew to **50 tasks — 3,360 cases and 59,177 assertions — all passing at a single git HEAD (`c9192cb3`)**.

DeltaScope remains static analysis: it does not execute submitted SQL, return query results, or decide authorization.

## PostgreSQL 17 DDL Acceptance (#115, T33–T49)

- **Official denominator**: 143 inventory rows distilled from the PostgreSQL 17 command list, each carrying per-anchor cells and 568 probe-evidence cells bound to `postgres:17.7` (digest-locked `postgres@sha256:2006...`, linux/arm64).
- **Scoped golden anchors**: manifests can now declare `anchor_scope` — `postgresql` scope proves only the postgres17 anchor + postgresql dialect, `tidb` scope proves tidb85 + tidb dialect, and the default flat scope keeps the four-anchor denominator honest. The baseline fails closed on scope drift.
- **13 family manifests** (T35–T47) cover create-table, alter-table subaction matrices, drop/truncate/cluster/reindex, indexes, views/materialized views, types/domains/routines, triggers/rules/policies/event-triggers, sequences/statistics/extensions, FDW/server/foreign-table, text search, publication/subscription (logical replication on `wal_level=logical`), and roles/grants/annotations/tablespaces/large objects/ALTER SYSTEM.
- **Residual classification** (T48): every silent and parser-error cell classified with golden evidence; the `restart: on-failure` fix removes the TiKV region-unavailable startup race that previously blocked T23S aggregates.

## Official DML Acceptance (#133, T50–T56)

- **44-row DML denominator** (17 MySQL, 18 TiDB, 9 PostgreSQL) with probe-classified cells for every officially documented DML verb.
- **T51** MySQL mutation verbs: INSERT/REPLACE/UPDATE/DELETE/DO on four anchors — 63 cases, 696 assertions.
- **T52** MySQL query/utility verbs: SELECT family, UNION, 8.0+ forms (TABLE/VALUES/WITH/EXCEPT/INTERSECT/FOR SHARE) plus HANDLER/CALL/LOAD DATA/XML/IMPORT TABLE boundaries — 49 cases, 480 assertions.
- **T53** TiDB-native DML on the new `tidb` scope: IMPORT INTO is pinned as an explicit unsupported boundary with a live native refusal — 26 cases, 256 assertions.
- **T54** PostgreSQL DML on postgres17: INSERT/UPDATE/DELETE variants (RETURNING, ON CONFLICT, USING, FROM) are audited; SELECT/MERGE/COPY/TABLE/VALUES/WITH stay explicit unsupported while psql executes each on the live anchor — vendor-capable but auditor-declined, never silent — 32 cases, 346 assertions.

## Debt Discharge & Parser Graduation

- **T57 (#141)**: `ALTER TABLESPACE <name> ENGINE [=] <engine>` graduated from parser-upgrade candidate to audited — `AlterAction="engine"`, `ENGINE=InnoDB` reaches `ddl.mysql.tablespace.alter.warn`, NDB stays an explicit engine boundary, bare `ENGINE` stays a parser error. **MySQL now has zero parser-upgrade candidates**; the five PostgreSQL forms remain candidates pending a libpg_query bump.
- **T58 (#142)**: the `srs.version_applicability` premise is proven on live anchors — observed 5.7.44 and 8.4.10 pin `version_inapplicable` + `gap_fact`, observed 8.0.46 audits (`ddl.mysql.srs.drop.absent`), tidb85 stays a vendor boundary, and an offline `--target-version` pin stays `version_unproven` (a pin is a conflict check, never an observed fact).
- **T59 (#143)**: the `alter_instance.version_applicability` premise is discharged as a pinned boundary — live tidb85 evidence is attached to every subaction cell and the promotion premise is deferred because TiDB documents no audited-applicability version.

## Bug Fixes & Tooling

- **fix(rules)**: evidence-gap `required_facts` is now sorted — a map-iteration flake that could reorder `table.columns`/`table.auto_random_bits` between runs is eliminated.
- **lint baseline cleared**: the golangci-lint CI gate is green again (errcheck/unparam/unused/gocritic/gosec/staticcheck across 48 files) with no semantic changes.

## Verification

- `TASK=all` aggregate at `c9192cb3`: 50 canonical tasks, 3360/3360 cases, 59,177 assertions, single HEAD, artifact valid.
- `go test ./...` green including `-tags postgresql`; 896/896 golden contract cases; `ddl-inventory-gate` green.
- Decision records: DML acceptance closure, ALTER TABLESPACE ENGINE graduation, SRS live version-gate evidence, ALTER INSTANCE premise deferral.

## Breaking Changes

None.
