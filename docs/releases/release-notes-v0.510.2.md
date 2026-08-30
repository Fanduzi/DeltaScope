# DeltaScope v0.510.2 Release Notes

## Summary

v0.510.2 ships the existing #60–#62 decisions: otherwise-passing audits with an unaudited unsupported statement floor to `review`; active MCP launcher examples use `npx -y --prefer-online @fanduzi/deltascope-mcp@latest`; and sole `deltascope-mcp version` and `help` invocations exactly alias `-version` and `-help` before MCP stdio starts.

DeltaScope remains static analysis: it does not execute submitted SQL, return query results, or decide authorization. MCP has no Query Access tool. The registered rule catalog remains 373 rules. Supported rule-and-dialect fixture coverage remains 586/586 (100.0%) across 286 YAML fixtures; this is not SQL syntax or grammar coverage.

## Fixes

- #60: an otherwise `pass` audit becomes `review` when an unsupported statement remains unaudited; existing `review` and `reject` verdicts do not downgrade.
- #61: canonical MCP launcher examples refresh npm metadata and select the latest dist-tag without changing launcher runtime behavior.
- #62: sole positional `version` and `help` preserve the stdout, stderr, and exit code of their existing dashed forms and do not start the server.

## Non-Goals

- Not a new verdict enum, parser, command framework, MCP tool, SQL execution, or authorization feature.
- Not a rule catalog, SQL syntax, or grammar coverage claim.
- No severity field is introduced.

## Rule Catalog Facts

| Metric | Count |
|--------|------:|
| Total rules | **373** |
| blocker | 73 |
| warning | 142 |
| notice | 158 |

## Corpus and Catalog Facts

- Supported rule-and-dialect fixture coverage: **586/586**, **100.0%**, **286** YAML fixture files. This is not SQL syntax or grammar coverage.
- PostgreSQL ALTER TABLE config entries: **53**.
- DDL coverage catalog: **407** entries (mysql 62, tidb 55, postgresql 290, parser_upgrade_candidate 18).

## Decision Records

- `docs/decisions/2026-08-30-unsupported-statement-verdict-review-floor.md` (#60)
- `docs/decisions/2026-08-30-mcp-launcher-upgrade-safe-install.md` (#61)
- `docs/decisions/2026-08-30-mcp-positional-meta-invocation.md` (#62)
