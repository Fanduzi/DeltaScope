# DeltaScope v0.510.4 Release Notes

Release date: 2026-09-06

## Summary

v0.510.4 names split public signals so operators and agents do not confuse Verdict with Fail Threshold, Rule Catalog with Loaded, or silent MCP/CLI/HTTP gaps. GitHub issues [#65](https://github.com/Fanduzi/DeltaScope/issues/65)–[#76](https://github.com/Fanduzi/DeltaScope/issues/76) land in the published Official Distribution.

DeltaScope remains static analysis: it does not execute submitted SQL, return query results, or decide authorization. MCP has no Query Access tool. The Rule Catalog is **376** rules, including three default-disabled `dml.impact.*` opt-in rules. Default Policy does not enable those three. Supported rule-and-dialect fixture coverage remains 586/586 (100.0%) across 286 YAML fixtures; this is not SQL syntax or grammar coverage.

## Fixes

- [#65](https://github.com/Fanduzi/DeltaScope/issues/65): CLI JSON adds sibling `fail_on_triggered`. Notices-only `--fail-on notice` still has Verdict `pass` and a non-zero process exit. SDK, HTTP, and MCP Result omit the field.
- [#66](https://github.com/Fanduzi/DeltaScope/issues/66): `install.sh` prefixes bare `MAJOR.MINOR.PATCH` with `v` before composing the GitHub download URL.
- [#67](https://github.com/Fanduzi/DeltaScope/issues/67): Catalog, Default Policy, Loaded, and `fk_forbid` suppression are named as different facts. `config status` reports FK naming suppression instead of a missing rule.
- [#68](https://github.com/Fanduzi/DeltaScope/issues/68): empty `--sql` stays exit 2 on `audit` and exit 3 on `query-access analyze`. Error texts are `audit:` versus `query-access:`.
- [#69](https://github.com/Fanduzi/DeltaScope/issues/69): MCP launcher keeps `engines.node >=24` and fails closed below Node 24 before download or spawn.
- [#70](https://github.com/Fanduzi/DeltaScope/issues/70): untagged / `go install @main` builds report Go module or VCS info. `DefaultVersion` is fallback only.
- [#71](https://github.com/Fanduzi/DeltaScope/issues/71): pull requests and `main` run `go test ./...` and PostgreSQL unit gates. Heavy e2e stays on release.
- [#72](https://github.com/Fanduzi/DeltaScope/issues/72): `get_capabilities` declares `query_access: { available: false, surfaces: ["cli", "http"] }`. No Query Access MCP tool.
- [#73](https://github.com/Fanduzi/DeltaScope/issues/73): GitHub Action example pin tracks the current stable tag.
- [#74](https://github.com/Fanduzi/DeltaScope/issues/74): `dml.impact.*` rules are in the Rule Catalog as default-disabled. Default Policy does not enable them. Statement impact objects remain on default UPDATE/DELETE results.
- [#75](https://github.com/Fanduzi/DeltaScope/issues/75): `audit -h` and `query-access analyze -h` print help. Host is `-H` / `--host`.
- [#76](https://github.com/Fanduzi/DeltaScope/issues/76): HTTP `/v1/audit` treats MCP `connection_ref` as `invalid_request` naming `connection_id`, not opaque `invalid_json`.

## Non-Goals

- Not raising Verdict when Fail Threshold trips.
- Not unifying empty-SQL exit codes.
- Not lowering npm `engines` to Node 20.
- Not an MCP Query Access tool.
- Not enabling `dml.impact.*` in Default Policy.
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

- [2026-09-04 named public signals](../decisions/2026-09-04-named-public-signals.md) ([#65](https://github.com/Fanduzi/DeltaScope/issues/65)–[#74](https://github.com/Fanduzi/DeltaScope/issues/74)) is the accepted boundary record for this patch.
