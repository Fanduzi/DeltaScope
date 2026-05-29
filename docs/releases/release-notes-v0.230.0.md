# DeltaScope v0.230.0 Release Notes

## Summary — Unsupported Diagnostics Evidence

v0.230.0 introduces structured diagnostics evidence for parser-error and unsupported-statement outcomes across all four product surfaces (SDK, CLI, HTTP, MCP). Every statement that DeltaScope cannot fully audit now carries a `spec.Diagnostic` with `classification`, `reason`, `action_hint`, `audited`, and `dialect` fields. It does **not** add new parser support, new SQL audit rules, fallback parser implementation, or reduce parser_error counts.

## New Result Contract

- `spec.Diagnostic` — structured diagnostic per statement.
- `report.Result.Diagnostics` — diagnostics array on the public result.

### Diagnostic Fields

| Field | Purpose |
|-------|---------|
| `classification` | `parser_error` or `unsupported_statement` |
| `reason` | Human-readable explanation of why the statement was not audited |
| `action_hint` | Guidance on what the user should do next |
| `audited` | `false` — the statement was not fully audited |
| `dialect` | The selected dialect for the audit request |

## Public Surface Coverage

| Surface | Diagnostics Emitted | Action Hints Present |
|---------|:-------------------:|:--------------------:|
| SDK (`pkg/deltascope`) | Yes | Yes |
| CLI (`deltascope`) | Yes | Yes |
| HTTP (`deltascope-server`) | Yes | Yes |
| MCP (`deltascope-mcp`) | Yes | Yes |

## Diagnostic Examples

### Parser-Error Diagnostic

| Field | Value |
|-------|-------|
| `classification` | `parser_error` |
| `reason` | `statement was not audited because the selected dialect parser could not parse it; no audit findings were inferred` |
| `action_hint` | contains `verify the selected dialect` |
| `audited` | `false` |

### Unsupported-Statement Diagnostic

| Field | Value |
|-------|-------|
| `classification` | `unsupported_statement` |
| `reason` | `DeltaScope recognized this statement or feature but does not audit it yet` |
| `action_hint` | contains manual review / future support guidance |
| `audited` | `false` |

No raw SQL or payload is copied into diagnostics.

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
- No raw SQL or payload copied into diagnostics.

## Decision Record

`docs/decisions/2026-05-28-v0.230.0-unsupported-diagnostics-evidence.md`
