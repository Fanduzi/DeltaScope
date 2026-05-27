# DeltaScope v0.210.0 Release Notes

## Summary — Cross-Dialect Parser-Error Feasibility Census

v0.210.0 classifies all 29 parser-error cases across MySQL (15), TiDB (9), and PostgreSQL (5) into feasibility buckets and adds a `make ddl-parser-error-feasibility-report` gate. It does **not** add new parser support, new SQL audit rules, fallback parser implementation, or reduce parser_error counts.

## Parser-Error Feasibility Buckets

| Bucket | MySQL | TiDB | PostgreSQL | Total |
|--------|-------|------|------------|-------|
| `parser_upgrade_candidate` | 5 | 0 | 5 | 10 |
| `bounded_fallback_candidate` | 1 | 3 | 0 | 4 |
| `product_unsupported_or_inapplicable` | 0 | 6 | 0 | 6 |
| `unsafe_fallback_defer` | 9 | 0 | 0 | 9 |
| `needs_research` | 0 | 0 | 0 | 0 |
| **Total** | **15** | **9** | **5** | **29** |

### MySQL Parser-Error Cases (15)

| Family | Form | Bucket |
|--------|------|--------|
| view | ALTER VIEW | unsafe_fallback_defer |
| routine | ALTER PROCEDURE | unsafe_fallback_defer |
| routine | CREATE FUNCTION | parser_upgrade_candidate |
| routine | ALTER FUNCTION | unsafe_fallback_defer |
| routine | DROP FUNCTION | unsafe_fallback_defer |
| trigger | CREATE TRIGGER | unsafe_fallback_defer |
| trigger | DROP TRIGGER | unsafe_fallback_defer |
| event | CREATE EVENT | unsafe_fallback_defer |
| event | ALTER EVENT | unsafe_fallback_defer |
| event | DROP EVENT | unsafe_fallback_defer |
| tablespace | CREATE TABLESPACE | parser_upgrade_candidate |
| tablespace | ALTER TABLESPACE | parser_upgrade_candidate |
| tablespace | DROP TABLESPACE | parser_upgrade_candidate |
| resource_group | CREATE RESOURCE GROUP | parser_upgrade_candidate |
| resource_group | ALTER RESOURCE GROUP | bounded_fallback_candidate |

### TiDB Parser-Error Cases (9)

| Family | Form | Bucket |
|--------|------|--------|
| view | ALTER VIEW | bounded_fallback_candidate |
| table_option | ALTER TABLE TTL | bounded_fallback_candidate |
| table_option | ALTER TABLE LOCALITY | bounded_fallback_candidate |
| trigger | CREATE TRIGGER | product_unsupported_or_inapplicable |
| trigger | DROP TRIGGER | product_unsupported_or_inapplicable |
| routine | CREATE FUNCTION | product_unsupported_or_inapplicable |
| routine | DROP FUNCTION | product_unsupported_or_inapplicable |
| event | CREATE EVENT | product_unsupported_or_inapplicable |
| event | DROP EVENT | product_unsupported_or_inapplicable |

### PostgreSQL Parser-Error Cases (5)

| Family | Form | Bucket |
|--------|------|--------|
| subscription | DROP SUBSCRIPTION WITH drop_slot | parser_upgrade_candidate |
| constraints | NOT NULL NOT VALID | parser_upgrade_candidate |
| constraints | ALTER CONSTRAINT NOT ENFORCED | parser_upgrade_candidate |
| constraints | ALTER CONSTRAINT INHERIT | parser_upgrade_candidate |
| constraints | ALTER CONSTRAINT NO INHERIT | parser_upgrade_candidate |

## DDL Coverage Census (unchanged)

| Dialect | Total | Finding | Silent | Unsupported | Parser Error |
|---------|------:|--------:|-------:|:-----------:|:------------:|
| MySQL | 61 | 46 | 0 | 0 | 15 |
| TiDB | 54 | 45 | 0 | 0 | 9 |
| PostgreSQL (consolidated tracked-case) | 285 | 274 | 6 | 0 | 5 |

## Unchanged Metrics

- SQL corpus: **582/582**, **100.0%**, **245 YAML** fixture files.
- PostgreSQL ALTER TABLE rule count: **32** (unchanged).
- PostgreSQL consolidated DDL census: **285/274/6/0/5/0** (unchanged).

## Reporting Gate

- `make ddl-parser-error-feasibility-report` — prints classified parser-error feasibility census for MySQL, TiDB, and PostgreSQL.

## Non-Goals

- Not new parser support.
- Not new SQL audit rules.
- Not fallback parser implementation.
- Not reduced parser_error counts.
- Not full MySQL/TiDB/PostgreSQL DDL support.
- Not dialect parity.
- Not runtime/catalog validation.

## Decision Record

`docs/decisions/2026-05-27-v0.210.0-cross-dialect-parser-error-feasibility-census.md`
