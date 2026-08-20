# DeltaScope v0.490.0 Release Notes

## Summary - Audit Contract and Unified Online Query Access

v0.490.0 publishes the operator and agent audit-contract fixes that landed after v0.480.0, plus a unified Query Access online session. Offline ALTER no longer claims existence was checked. MCP `audit_sql` text carries findings, and `list_rules` returns compact catalog rows. CLI empty `--sql`, metadata connection exits, generated empty-string YAML, and the embedded DDL-coverage catalog match the documented contract. Query Access SDK, CLI, and HTTP share one online entry; dialect-specific session constructors stay available but are deprecated.

This is still static analysis. DeltaScope does not execute submitted SQL, retrieve query results, or decide authorization, grants, RLS, or masking. MCP still has no Query Access tool. The registered audit rule catalog is unchanged. No-leak Query Access evidence is unchanged.

## What Changed

### Unified Query Access Online Session

- New public constructor: `NewOnlineQueryAccessSessionFromConn`. CLI and HTTP online Query Access route through that shared entry.
- Dialect-specific session APIs remain but are deprecated in favor of the unified constructor.
- PostgreSQL resolver ownership is connection-only. DB-backed metadata resolvers are removed. Proof promotion is orchestrated in one pipeline.
- Default/offline SDK, CLI, and HTTP stay fail-closed / indeterminate for function-bearing Query Access until a supported online session exists.
- No-leak: submitted SQL is never executed, prepared, or explained. Public results and logs do not expose SQL/literal markers, connection data, credentials, catalog data, or raw driver errors.

### MCP Agent Surfaces

- `audit_sql` `content[0].text` is a compact finding summary (verdict, counts, `[level] rule_id: message`, suggestion). It is not a second JSON copy of `structuredContent`.
- Offline `audit_sql` structured `context` includes `note` / `unproven` when existence was not checked, and the compact text carries the same one-line caveat.
- `list_rules` returns compact catalog rows (`rule_id`, `level`, `dialect`, `kind`, `summary`). Use `describe_rule` for the full body.
- MCP Registry discovery metadata is published (`server.json` / npm `mcpName`).

### CLI and HTTP Audit Contract

- Offline CLI, HTTP, and MCP `context` emit `note: existence not checked (no database connection)` and `unproven: ["column_exists","table_exists"]`. Metadata-aware results omit those fields. `pkg/deltascope.Result` has no `context`.
- Offline `ALTER ... DROP COLUMN` notice wording is hypothetical (`would drop column … if it exists`). Verdict stays `pass` for notice-only offline ALTER.
- Explicit empty `--sql` is rejected without reading stdin.
- Metadata connection failures map to bounded exit `3`; unknown flags and parser-error SQL map to exit `2`.
- `config init` encodes empty string params as YAML `""`.
- The published CLI binary embeds the DDL-coverage catalog.
- Markdown skip reasons are aggregated; skipped rule IDs are not dumped.

## What Stayed the Same

- SQL audit rule evaluation and the registered audit rule catalog are unchanged except the offline DROP COLUMN notice copy. `level` remains the public audit priority field; no severity field is introduced.
- MCP tools remain `audit_sql`, `describe_rule`, `list_rules`, and `get_capabilities` only.
- Query Access still does not authenticate callers, evaluate grants, enforce RLS, mask columns, rewrite SQL, auto-grant privileges, or guarantee a later execution snapshot.
- Existing MySQL/TiDB Query Access admission envelopes from earlier releases are unchanged.
- Existing release tags, GitHub Releases, npm packages, and Homebrew casks are untouched until this tag publishes.

## Non-Goals

- Not an MCP Query Access tool.
- Not a change to the CREATE policy pack, default rule levels, or a `--host` requirement for offline audit.
- Not putting `context` on `pkg/deltascope.Result`.
- Not hypothetical wording for every existence-flavored notice (DROP INDEX still says it removes an existing index).
- Not general PostgreSQL Query Access expansion beyond the already-published envelopes.
- Not authorization, grants, roles, RLS, masking, rewrite, SQL execution, or data-returning APIs.
- Not a severity field; not a change to the registered audit rule catalog counts.
- Not a change to any previously published artifact or existing tag.

## Rule Catalog Facts

The registered audit rule catalog is unchanged from v0.480.0.

| Metric | Count |
|--------|------:|
| Total rules | **371** |
| blocker | 72 |
| warning | 142 |
| notice | 157 |

| Dialect scope | Rules |
|---------------|------:|
| common | 177 |
| postgresql | 191 |
| mysql | 1 |
| tidb | 2 |

| Statement kind | Rules |
|----------------|------:|
| ddl | 361 |
| dml | 10 |

## Unchanged Metrics

- SQL corpus: **582/582**, **100.0%**, **247** YAML fixture files.
- PostgreSQL ALTER TABLE config entries: **53**.
- DDL coverage catalog: **400** entries (mysql 61, tidb 54, postgresql 285, parser_upgrade_candidate 18).

## Decision Records

- `docs/decisions/2026-08-20-offline-existence-caveat-context.md` (this release)
- `docs/decisions/2026-08-18-mcp-audit-sql-text.md` (this release)
- `docs/decisions/2026-08-18-mcp-list-rules-compact-output.md` (this release)
- `docs/decisions/2026-08-18-mcp-registry-discovery.md` (this release)
- `docs/decisions/2026-08-18-cli-ddl-coverage-embedded-catalog.md` (this release)
- `docs/decisions/2026-08-17-markdown-rule-summary-aggregation.md` (this release)
- `docs/decisions/2026-08-17-cli-explicit-empty-sql-input-source.md` (this release)
- `docs/decisions/2026-08-17-cli-metadata-connection-exit-mapping.md` (this release)
- `docs/decisions/2026-08-17-cli-user-input-exit-mapping.md` (this release)
- `docs/decisions/2026-08-17-generated-config-empty-string-encoding.md` (this release)
- `docs/decisions/2026-08-16-query-access-proof-orchestration.md` (this release)
- `docs/decisions/2026-08-16-query-access-remove-db-backed-resolvers.md` (this release)
- `docs/decisions/2026-08-14-query-access-dialect-session-api-deprecation.md` (this release)
- `docs/decisions/2026-08-12-query-access-online-analysis-entry.md` (this release)
- `docs/decisions/2026-08-11-query-access-postgresql-resolver-core.md` (this release)
- `docs/decisions/2026-08-03-query-access-pg17-count-online-surface-contract.md` (v0.480.0)
