# DeltaScope v0.190.0 Release Notes

## Summary — Cross-Dialect DDL Coverage Census Consolidation

v0.190.0 is a release engineering / coverage inventory milestone that establishes a consolidated DDL coverage census across MySQL, TiDB, and PostgreSQL. It does **not** add new SQL audit rules, new parser support, or new product behavior.

## What Changed

- `make ddl-census-report` now prints the tracked DDL coverage census for MySQL, TiDB, and PostgreSQL. This is an inventory/reporting gate — it shows how many tracked DDL forms are finding-covered, silently normalized, explicitly unsupported, or parser-blocked for each dialect. It is not a full SQL grammar coverage claim.
- MySQL/TiDB tracked DDL census expanded from 18 to 61/54 forms covering 10 DDL families each.
- PostgreSQL tracked DDL census consolidated from 5 source-specific tests into a single consolidated test with machine-verifiable arithmetic.
- Decision record: `docs/decisions/2026-05-26-v0.190.0-cross-dialect-ddl-census-consolidation.md`.

## DDL Coverage Census

| Dialect | Total | Finding Covered | Normalized Silent | Unsupported Boundary | Parser Error | Unclassified |
|---------|------:|----------------:|------------------:|:--------------------:|:------------:|:------------:|
| MySQL | 61 | 21 | 25 | 0 | 15 | 0 |
| TiDB | 54 | 18 | 27 | 0 | 9 | 0 |
| PostgreSQL (consolidated tracked-case) | 285 | 274 | 6 | 0 | 5 | 0 |

The PostgreSQL `285` is a consolidated tracked-case total that includes overlap between source census lists (representative, completion, deep, long-tail, ALTER TABLE residual). It is not a unique SQL grammar denominator.

### Top MySQL/TiDB Gaps

- MySQL normalized_silent future-rule candidates: RENAME TABLE, standalone CREATE INDEX, ALTER DATABASE, CREATE/DROP PROCEDURE, privilege/DCL.
- MySQL parser gaps (15 forms): triggers, events, functions, ALTER VIEW, ALTER PROCEDURE, tablespace, CREATE/ALTER RESOURCE GROUP.
- TiDB normalized_silent future-rule candidates: RENAME TABLE, ALTER TABLE ADD/DROP INDEX, standalone CREATE INDEX, PLACEMENT POLICY, SEQUENCE, ALTER TABLE PLACEMENT POLICY, privilege/DCL.
- TiDB parser gaps (9 forms): triggers, events, functions, ALTER VIEW, ALTER TABLE TTL, ALTER TABLE LOCALITY.

## Unchanged Metrics

v0.190.0 does not change SQL parser, rule, or product behavior:

- PostgreSQL ALTER TABLE rule count: **32** (unchanged).
- Residual census: **66/60/2/0/4/0** (unchanged).
- SQL corpus: **535/535**, **100.0%**, **243 YAML files** (unchanged).

## Non-Goals

- Not a new SQL audit rule release.
- Not new parser support.
- Not full MySQL DDL support.
- Not full TiDB DDL support.
- Not full PostgreSQL DDL support.
- Not dialect parity.
- Not runtime/catalog validation.
