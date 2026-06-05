# DeltaScope v0.270.0 Release Notes

## Summary — DDL Coverage Catalog

v0.270.0 introduces a DDL coverage catalog that consolidates DeltaScope's verified DDL coverage across MySQL, TiDB, and PostgreSQL into a single machine-readable JSON artifact and bilingual user documentation. This release does **not** add parser support, change parser behavior, add SQL audit rules, implement a fallback parser, reduce `parser_error` counts, change audit verdict or finding semantics, or alter SQL audit behavior in any way.

## DDL Coverage Catalog

A machine-readable catalog at `docs/reference/ddl-coverage-catalog.json` now lists every DDL form DeltaScope has verified:

- **400 entries** across MySQL (61), TiDB (54), and PostgreSQL (285)
- Each entry includes dialect, family, form label, classification, guidance metadata, rule IDs, and a human-readable note
- Generated from the same census data that drives existing coverage tests — not hand-maintained

### Classification Vocabulary

| Classification | Meaning |
|---|---|
| `finding_covered` | Parses and produces one or more audit findings under current rules |
| `normalized_silent` | Parses but current default policy produces no finding |
| `unsupported_boundary` | Recognized product boundary; returns unsupported diagnostics |
| `parser_error` | Selected dialect parser cannot parse the form |
| `unclassified` | Reserved; release gate requires zero |

## Parser-Upgrade Candidate Metadata

18 entries carry `guidance_code=parser_upgrade_candidate` with `evidence_ref` documentation links. All 10 required candidates are included:

| Dialect | DDL Form | guidance_code |
|---------|----------|---------------|
| MySQL | `ALTER VIEW` | `parser_upgrade_candidate` |
| MySQL | `ALTER PROCEDURE` | `parser_upgrade_candidate` |
| MySQL | `CREATE FUNCTION` | `parser_upgrade_candidate` |
| MySQL | `ALTER FUNCTION` | `parser_upgrade_candidate` |
| MySQL | `DROP FUNCTION` | `parser_upgrade_candidate` |
| PostgreSQL | `DROP SUBSCRIPTION WITH drop_slot` | `parser_upgrade_candidate` |
| PostgreSQL | `NOT NULL NOT VALID` | `parser_upgrade_candidate` |
| PostgreSQL | `ALTER CONSTRAINT NOT ENFORCED` | `parser_upgrade_candidate` |
| PostgreSQL | `ALTER CONSTRAINT INHERIT` | `parser_upgrade_candidate` |
| PostgreSQL | `ALTER CONSTRAINT NO INHERIT` | `parser_upgrade_candidate` |

## Drift Gate

`make ddl-coverage-catalog-test` verifies the catalog JSON stays aligned with census tests. If the checked-in JSON is stale, the gate fails.

## User Documentation

- `docs/reference/ddl-coverage.md` — English reference
- `docs/reference/ddl-coverage.zh-CN.md` — Chinese reference (full parity)

## Non-Goals

- No parser support added.
- No fallback parser.
- No new SQL audit rules.
- No parser_error count reduction.
- No audit verdict or finding semantic changes.
- No SQL audit behavior change.
- No runtime behavior changes.
- No npm/Homebrew behavior change.

## DDL Coverage Census (unchanged)

| Dialect | Total | Finding | Silent | Unsupported | Parser Error |
|---------|------:|--------:|-------:|:-----------:|:------------:|
| MySQL | 61 | 46 | 0 | 0 | 15 |
| TiDB | 54 | 45 | 0 | 0 | 9 |
| PostgreSQL (consolidated tracked-case) | 285 | 274 | 6 | 0 | 5 |

### PostgreSQL ALTER TABLE Residual (unchanged)

`66/60/2/0/4/0`

## Unchanged Metrics

- SQL corpus: **582/582**, **100.0%**, **245 YAML** fixture files.
- PostgreSQL ALTER TABLE rule count: **32** (unchanged).
- PostgreSQL consolidated DDL census: **285/274/6/0/5/0** (unchanged).
- Parser-error total: **29** cases across all dialects (unchanged).
- MySQL/TiDB DDL Notice section: **27** (unchanged).
- TiDB-Specific subsection: **7** (unchanged).

## Decision Record

`docs/decisions/2026-06-05-v0.270.0-ddl-coverage-catalog.md`
