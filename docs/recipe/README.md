# Recipe Docs

Task-oriented examples for common DeltaScope workflows.

These docs are intended to be copied into real terminal sessions. Each recipe should show:

- what problem it solves
- the exact command to run
- the kind of output or verdict to expect

## Recipes

| Recipe | Purpose |
| --- | --- |
| [audit-sql-offline.md](audit-sql-offline.md) | Review SQL without any live database dependency |
| [audit-sql-with-metadata.md](audit-sql-with-metadata.md) | Run metadata-aware audits against a live MySQL or TiDB-compatible instance |
| [review-ddl-before-migration.md](review-ddl-before-migration.md) | Gate migration DDL before rollout |
| [guard-dml-in-ci.md](guard-dml-in-ci.md) | Fail CI when risky DML crosses the configured threshold |
| [use-deltascope-mcp.md](use-deltascope-mcp.md) | Connect DeltaScope to MCP clients through the launcher or native binary |
| [use-with-ai-agents.md](use-with-ai-agents.md) | Feed DeltaScope JSON into scripts and agent loops |
| [inspect-rules-and-config.md](inspect-rules-and-config.md) | Explore shipped rules and config defaults quickly |
| [troubleshoot-metadata-aware-audit.md](troubleshoot-metadata-aware-audit.md) | Diagnose schema inference, connection, and metadata-aware audit failures |
