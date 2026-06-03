# DeltaScope v0.250.0 Release Notes

## Summary — Parser Upgrade Candidate Evidence Pack

v0.250.0 is a parser-upgrade candidate evidence milestone. It documents which of the 29 remaining `parser_error` forms would benefit from parser/library upgrades, which non-candidate boundaries must not be crossed, and what user-facing implications exist. This release does **not** add parser support, change parser behavior, add SQL audit rules, implement a fallback parser, reduce `parser_error` counts, change public output shapes, or alter SQL audit behavior in any way.

## Parser-Upgrade Candidate Evidence

All 29 remaining `parser_error` cases across MySQL, TiDB, and PostgreSQL have been classified into feasibility buckets:

| Bucket | MySQL | TiDB | PostgreSQL | Total |
|--------|-------|------|------------|-------|
| `parser_upgrade_candidate` | 5 | 0 | 5 | 10 |
| `bounded_fallback_candidate` | 1 | 3 | 0 | 4 |
| `product_unsupported_or_inapplicable` | 0 | 6 | 0 | 6 |
| `unsafe_fallback_defer` | 9 | 0 | 0 | 9 |
| `needs_research` | 0 | 0 | 0 | 0 |
| **Total** | **15** | **9** | **5** | **29** |

### Parser-Upgrade Candidates (10 forms)

These forms would become parseable if the upstream parser library adds syntax support:

**MySQL (5):**

| # | Family | Case |
|---|--------|------|
| 1 | view | ALTER VIEW |
| 2 | routine | ALTER PROCEDURE |
| 3 | routine | CREATE FUNCTION |
| 4 | routine | ALTER FUNCTION |
| 5 | routine | DROP FUNCTION |

**PostgreSQL (5):**

| # | Family | Case |
|---|--------|------|
| 1 | subscription | DROP SUBSCRIPTION ... WITH (drop_slot = true) |
| 2 | constraints | NOT NULL NOT VALID |
| 3 | constraints | ALTER CONSTRAINT NOT ENFORCED |
| 4 | constraints | ALTER CONSTRAINT INHERIT |
| 5 | constraints | ALTER CONSTRAINT NO INHERIT |

### User Impact

- `parser_error` statements are not audited. No findings are generated for unparseable SQL.
- No audit findings are inferred from failed parses.
- Raw SQL text and parser `near ...` error fragments are not copied into user-facing guidance.
- Users should manually review `parser_error` statements.
- This evidence helps future parser upgrade planning.

## Non-Goals

- No parser support added.
- No parser adapter changes.
- No fallback parser.
- No new SQL audit rules.
- No policy default changes.
- No parser_error count reduction.
- No public output shape change.
- No full DDL support claim.
- No dialect parity claim.
- No SQL audit behavior change.

## Parser-Error Counts (unchanged)

| Dialect | Parser Error |
|---------|:------------:|
| MySQL | 15 |
| TiDB | 9 |
| PostgreSQL | 5 |
| **Total** | **29** |

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
- MySQL/TiDB DDL Notice section: **27** (unchanged).
- TiDB-Specific subsection: **7** (unchanged).

## Decision Record

`docs/decisions/2026-06-02-v0.250.0-parser-upgrade-candidate-evidence-pack.md`
