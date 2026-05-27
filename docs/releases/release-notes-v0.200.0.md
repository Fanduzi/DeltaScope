# DeltaScope v0.200.0 Release Notes

## Summary — MySQL/TiDB DDL Normalized-Silent Coverage Milestone

v0.200.0 promotes all previously normalized-silent MySQL and TiDB DDL forms to finding-covered status through a 28-rule DDL notice pack. It does **not** add new parser support, change PostgreSQL audit behavior, or claim full DDL coverage.

## What Changed

- 28 new DDL notice rules covering MySQL/TiDB lifecycle DDL events (CREATE/DROP/ALTER DATABASE, CREATE/DROP/ALTER TABLE, TRUNCATE, RENAME TABLE), MySQL/TiDB ALTER TABLE action notices (ADD/DROP/MODIFY COLUMN, CHANGE COLUMN, ADD/DROP INDEX, RENAME INDEX/COLUMN, ADD/DROP FOREIGN KEY, ADD/DROP/SET DEFAULT, ORDER BY, CONVERT/MODIFY CHARSET), TiDB-specific rules (CREATE/ALTER/DROP PLACEMENT POLICY, CREATE/ALTER/DROP SEQUENCE), and MySQL-specific DROP RESOURCE GROUP.
- MySQL normalized_silent dropped from 25 to **0**; finding_covered rose from 21 to **46**.
- TiDB normalized_silent dropped from 27 to **0**; finding_covered rose from 18 to **45**.
- SQL corpus expanded to **582/582** targets, **100.0%**, **245 YAML** fixture files.
- Decision record: `docs/decisions/2026-05-27-v0.200.0-mysql-tidb-ddl-normalized-silent-coverage.md`.

## DDL Coverage Census

### MySQL

| Status | Before | After |
|--------|-------:|------:|
| Total | 61 | 61 |
| Finding Covered | 21 | **46** |
| Normalized Silent | 25 | **0** |
| Unsupported Boundary | 0 | 0 |
| Parser Error | 15 | 15 |
| Unclassified | 0 | 0 |

### TiDB

| Status | Before | After |
|--------|-------:|------:|
| Total | 54 | 54 |
| Finding Covered | 18 | **45** |
| Normalized Silent | 27 | **0** |
| Unsupported Boundary | 0 | 0 |
| Parser Error | 9 | 9 |
| Unclassified | 0 | 0 |

### PostgreSQL (unchanged)

| Status | Count |
|--------|------:|
| Total | 285 |
| Finding Covered | 274 |
| Normalized Silent | 6 |
| Unsupported Boundary | 0 |
| Parser Error | 5 |
| Unclassified | 0 |

PostgreSQL `285` is a consolidated tracked-case total that includes overlap between source census lists. It is not a unique SQL grammar denominator.

### Remaining Gaps

- MySQL parser gaps (15 forms): triggers, events, functions, ALTER VIEW, ALTER PROCEDURE, tablespace, CREATE/ALTER RESOURCE GROUP.
- TiDB parser gaps (9 forms): triggers, events, functions, ALTER VIEW, ALTER TABLE TTL, ALTER TABLE LOCALITY.

## Unchanged Metrics

- PostgreSQL ALTER TABLE rule count: **32** (unchanged).
- PostgreSQL consolidated DDL census: **285/274/6/0/5/0** (unchanged).

## Non-Goals

- Not new parser support.
- Not full MySQL DDL support.
- Not full TiDB DDL support.
- Not full PostgreSQL DDL support.
- Not dialect parity.
- Not runtime/catalog validation.
- Not a parser-error fix release.
