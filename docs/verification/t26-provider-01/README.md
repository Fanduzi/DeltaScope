# T26-PROVIDER-01 evidence package

Scope: #105 / T26 — provider-round work order: MySQL SERVER/SRS fact
readers + application wiring + public-error boundary + cache identity.
OFFLINE ONLY: no native execution, no live DB, no WKT/lifecycle/
post-state acceptance. Tested candidate: `7c2c1f23`.

| file | contents |
|---|---|
| `acceptance.md` | reviewer final ruling (SCOPED ACCEPTED), accepted identity + commit chain, capability scope registration, closed-issue map, run-record attribution, record corrections |
| `red_zero_row.log` | verbatim red transcript — new verbatim-name assertion on `f1ddab3d` product code (zero-row `ServerState.Name` was trimmed) |
| `red_r2_recapture.log` | labeled five-cell red recapture — new bounded/cache assertions on `471d3d66` product code (not the original 81051e45 run) |
| `green_provider_t26.log` | raw `-count=1` provider T26 suite output on `7c2c1f23` — 18/18 PASS |
| `green_packages.log` | raw `-count=1` affected-package output on `7c2c1f23` — mysql metadata + audit packages PASS |
| `mcp_exit.go` | source-derivation excerpt: real MCP audit error path (`auditSQLWithMetadata` → `toolError(mapAuditToolError(err), err.Error())` → `default: bad_request` → verbatim bounded message in TextContent + StructuredContent) |

No historical record was rewritten by this directory; attribution per
`acceptance.md` §5/§7.
