# DeltaScope v0.220.0 Release Notes

## Summary — Parser-Error Unsupported Contract Hardening

v0.220.0 hardens the public unsupported-contract diagnostic across all four product surfaces (SDK, CLI, HTTP, MCP). Parser-error statements now emit a standardized diagnostic — `statement was not audited because the selected dialect parser could not parse it; no audit findings were inferred` — replacing raw parser `near ...` fragments and tracked forbidden payloads. It does **not** add new parser support, new SQL audit rules, fallback parser implementation, or reduce parser_error counts.

## Standard Diagnostic

```
statement was not audited because the selected dialect parser could not parse it; no audit findings were inferred
```

This diagnostic is now emitted consistently across SDK, CLI, HTTP, and MCP surfaces whenever a parser-error statement is encountered. Raw parser `near ...` fragments and tracked forbidden payloads are no longer exposed in public output.

## Public Surface Coverage

| Surface | Diagnostic Standardized | Raw Fragments Removed |
|---------|:-----------------------:|:---------------------:|
| SDK (`pkg/deltascope`) | Yes | Yes |
| CLI (`deltascope`) | Yes | Yes |
| HTTP (`deltascope-server`) | Yes | Yes |
| MCP (`deltascope-mcp`) | Yes | Yes |

## Parser-Error Counts (unchanged)

| Dialect | Parser Error |
|---------|:------------:|
| MySQL | 15 |
| TiDB | 9 |
| PostgreSQL | 5 |
| **Total** | **29** |

## Parser-Error Feasibility Buckets (unchanged)

| Bucket | MySQL | TiDB | PostgreSQL | Total |
|--------|-------|------|------------|-------|
| `parser_upgrade_candidate` | 5 | 0 | 5 | 10 |
| `bounded_fallback_candidate` | 1 | 3 | 0 | 4 |
| `product_unsupported_or_inapplicable` | 0 | 6 | 0 | 6 |
| `unsafe_fallback_defer` | 9 | 0 | 0 | 9 |
| `needs_research` | 0 | 0 | 0 | 0 |

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

## Non-Goals

- Not new parser support.
- Not new SQL audit rules.
- Not fallback parser implementation.
- Not reduced parser_error counts.
- Not full MySQL/TiDB/PostgreSQL DDL support.
- Not dialect parity.
- Not runtime/catalog validation.
- PostgreSQL capability boundary error remains preserved.

## Decision Record

`docs/decisions/2026-05-28-v0.220.0-parser-error-unsupported-contract-hardening.md`
