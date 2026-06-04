# DeltaScope v0.260.0 Release Notes

## Summary — Unsupported Diagnostics Guidance Codes

v0.260.0 adds `guidance_code` and `evidence_ref` to parser_error diagnostics so users can tell which unsupported boundary a statement falls into and find detailed evidence documentation. This release does **not** add parser support, change parser behavior, add SQL audit rules, implement a fallback parser, reduce `parser_error` counts, change audit verdict or finding semantics, or alter SQL audit behavior in any way.

## Diagnostics Guidance Codes

When DeltaScope encounters SQL it cannot parse, the diagnostic output now includes:

- **`guidance_code`** — a stable machine-readable string identifying the unsupported boundary category (e.g., `parser_upgrade_candidate`).
- **`evidence_ref`** — a GitHub documentation URL pointing to the relevant evidence section.

Example diagnostic output for a parser_error statement:

```json
{
  "classification": "parser_error",
  "guidance_code": "parser_upgrade_candidate",
  "evidence_ref": "https://github.com/Fanduzi/DeltaScope/blob/main/docs/reference/cli.md#parser-upgrade-candidate-evidence-v02500",
  "audited": false
}
```

## First Verified Parser-Upgrade Candidates

| Dialect | DDL Form | guidance_code |
|---------|----------|---------------|
| MySQL | `ALTER VIEW` | `parser_upgrade_candidate` |
| PostgreSQL | `DROP SUBSCRIPTION ... WITH (drop_slot = true)` | `parser_upgrade_candidate` |

## Surface Parity

`guidance_code` and `evidence_ref` are surfaced consistently across all public surfaces:

- **SDK** — `spec.Diagnostic` JSON struct tags
- **CLI JSON** — automatic via struct serialization
- **CLI text (markdown)** — appended to diagnostic block
- **CLI text (quiet)** — appended to diagnostic line
- **HTTP** — automatic via struct serialization
- **MCP** — automatic via struct serialization

## No-Leak Guarantee

`guidance_code` is a fixed string from a controlled vocabulary. `evidence_ref` is a fixed GitHub documentation URL. Neither field contains raw SQL, parser near-text, object names, function bodies, default expressions, or any user payload.

## Non-Goals

- No parser support added.
- No fallback parser.
- No new SQL audit rules.
- No parser_error count reduction.
- No DDL census change.
- No audit verdict or finding semantic changes.
- No SQL audit behavior change.
- No npm/Homebrew behavior change.

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

`docs/decisions/2026-06-04-v0.260.0-unsupported-diagnostics-guidance-codes.md`
