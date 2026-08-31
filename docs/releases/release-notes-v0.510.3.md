# DeltaScope v0.510.3 Release Notes

Release date: 2026-08-31

## Summary

v0.510.3 corrects additive MCP capability discovery: [`get_capabilities.connection_inputs`](../../internal/interfaces/mcp/rule_tools.go) now advertises `connection.connect_timeout`, an existing public `audit_sql.connection` input. [Issue #64](https://github.com/Fanduzi/DeltaScope/issues/64) is guarded by a real in-memory [`tools/list` versus `get_capabilities` tool-call parity test](../../internal/interfaces/mcp/server_test.go).

This is an additive discovery-output correction only. It does not change timeout parsing, defaults, connection behavior, errors, credentials, tools, or input schema.

DeltaScope remains static analysis: it does not execute submitted SQL, return query results, or decide authorization. MCP has no Query Access tool. The registered rule catalog remains 373 rules. Supported rule-and-dialect fixture coverage remains 586/586 (100.0%) across 286 YAML fixtures; this is not SQL syntax or grammar coverage.

## Fixes

- [#64](https://github.com/Fanduzi/DeltaScope/issues/64): `get_capabilities.connection_inputs` includes `connection.connect_timeout` after the password inputs, matching the public connection schema order.
- [`TestGetCapabilitiesConnectionInputsMatchAuditSQLSchema`](../../internal/interfaces/mcp/server_test.go) compares real in-memory `tools/list` connection properties with a real `get_capabilities` tool call; [`TestGetCapabilitiesToolReturnsKnownSummary`](../../internal/interfaces/mcp/rule_tools_test.go) retains the explicit ordering contract.

## Non-Goals

- Not timeout parsing, validation, defaults, connection-opening behavior, error handling, or credential handling.
- Not a new MCP tool, transport, capability version, or input-schema change.
- Not SQL execution, authorization, a registered-rule catalog change, or a SQL syntax or grammar coverage claim.

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

## Decision Record

- [2026-08-31 MCP connect timeout capability](../decisions/2026-08-31-mcp-connect-timeout-capability.md) ([#64](https://github.com/Fanduzi/DeltaScope/issues/64)) is the existing accepted boundary record; no new decision record is needed for this release preparation.
