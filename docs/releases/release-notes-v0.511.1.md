# DeltaScope v0.511.1 Release Notes

Release date: 2026-09-16

## Summary

v0.511.1 shares three transport helpers that CLI and HTTP already owned twice: catalog YAML scalar rendering, `--sql`/`--file`/stdin loading, and online Query Access session attach. Public Audit, Query Access, and MCP contracts stay the same.

DeltaScope remains static analysis: it does not execute submitted SQL, return query results, or decide authorization. MCP still has no Query Access tool and no TLS fields. The Rule Catalog is **376** rules, including three default-disabled `dml.impact.*` opt-in rules. Default Policy does not enable those three. Supported rule-and-dialect fixture coverage remains 586/586 (100.0%) across 286 YAML fixtures; this is not SQL syntax or grammar coverage.

## Changes

- `catalog.FormatYAMLScalar` is the one YAML scalar renderer. CLI `rules explain` uses it and no longer keeps a local copy.
- CLI `audit` and `query-access analyze` share `resolveCLISQL`. Empty-SQL copy stays command-named (`audit:` vs `query-access:`). File-read errors stay as they were.
- CLI and HTTP attach an opened online session through `metadata.AttachOnlineQueryAccessSession`. Observed Server Identity is still reused; transports still inject their from-conn constructor.

## Non-Goals

- Not a public SDK attach helper in `pkg/deltascope`.
- Not caching Default Policy or the per-audit rule registry.
- Not unifying CLI, HTTP, and MCP user-facing failure sentences.
- Not an MCP Query Access tool.
- Not SQL execution, authorization, or a SQL syntax or grammar coverage claim.

## Rule Catalog Facts

| Metric | Count |
|--------|------:|
| Total rules | **376** |
| blocker | 73 |
| warning | 144 |
| notice | 159 |

The total includes three default-disabled `dml.impact.*` catalog rows. It is not the Default Policy count and not Loaded.

## Corpus and Catalog Facts

- Supported rule-and-dialect fixture coverage: **586/586**, **100.0%**, **286** YAML fixture files. This is not SQL syntax or grammar coverage.
- PostgreSQL ALTER TABLE config entries: **53**.
- DDL coverage catalog: **407** entries (mysql 62, tidb 55, postgresql 290, parser_upgrade_candidate 18).

## Decision Record

No new decision record. This patch does not change a public contract.
