# DeltaScope v0.511.0 Release Notes

Release date: 2026-09-07

## Summary

v0.511.0 shares Transport Connection Resolution for metadata-aware Audit and Online Query Access before open, then names Catalog vs Loaded vs Suppression, Mutation Target, incomplete-audit Markdown completeness, and a single Observed Server Identity probe. CLI, HTTP, and MCP keep their own failure sentences. MCP still has no Query Access tool and no TLS fields.

DeltaScope remains static analysis: it does not execute submitted SQL, return query results, or decide authorization. The Rule Catalog is **376** rules, including three default-disabled `dml.impact.*` opt-in rules. Default Policy does not enable those three. Supported rule-and-dialect fixture coverage remains 586/586 (100.0%) across 286 YAML fixtures; this is not SQL syntax or grammar coverage.

## New Features

- One shared connection-resolution path owns password, TLS shape, timeout, PostgreSQL omitted-port default, MySQL/TiDB catalog alias, and Connection Failure Class. CLI, HTTP, and MCP use it. The path stops before open. Metadata-aware Audit still opens a pool; Online Query Access still pins a session.
- `rulepresence.Of` answers Catalog, Default Policy, Loaded, and Suppression through actual `ddl.Register` / `dml.Register` and `rule.Registry.Contains`. Config status renders those four facts without collapsing them.
- `markdown.Render` owns Unsupported and Diagnostics. The CLI Markdown path only prepends Audit Context. CI adapters compare `spec.DiagnosticParserError`.
- `spec.DML.MutationTargets` names the table a DML statement writes. Metadata, auditmeta, and `dml.table.exists.require` read `MutationTargetTables()`.
- Transports that already identified a pinned connection call `deltascope.NewOnlineQueryAccessSessionFromIdentifiedConn`. That constructor does not ping or query `VERSION` again. `NewOnlineQueryAccessSessionFromConn` remains for callers that only have a `*sql.Conn`.
- Identity-resolver helpers with no PostgreSQL adapter caller are unexported. Adapter-called functions stay exported. No proof-engine seam.

## Non-Goals

- Not adding TLS fields to the MCP public connection contract.
- Not unifying CLI, HTTP, and MCP user-facing failure sentences.
- Not merging the Audit pool opener with the Query Access pinned opener.
- Not an MCP Query Access tool.
- Not hiding `Parse`/`Extract` or deleting the unused JSON output adapter.
- Not unifying `online.ErrPostgreSQLQueryAccessVersionUnsupported` with `deltascope.ErrOnlineQueryAccessPostgreSQLVersionUnsupported`.
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

- [2026-09-06 transport connection resolution](../decisions/2026-09-06-transport-connection-resolution.md) is the accepted boundary for the shared path before open.
- [2026-09-07 architecture-review follow-through](../decisions/2026-09-07-architecture-review-follow-through.md) is the accepted boundary for the five remaining named facts.
