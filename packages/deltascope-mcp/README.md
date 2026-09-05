# DeltaScope MCP Launcher

Bootstrap package for launching the native `deltascope-mcp` stdio server from npm-based MCP clients.

## What This Package Does

This package does not implement DeltaScope MCP tools itself. It downloads the matching native `deltascope-mcp` release binary, caches it locally, and then starts the real stdio server.

The launcher verifies the downloaded archive against the official DeltaScope release checksums before it updates the local cache.

## Supported MCP Surface

The launched `deltascope-mcp` server exposes four tools:

- `audit_sql` — audit SQL statements with DeltaScope. Text is a compact finding summary; `structuredContent` is the full result.
- `describe_rule` — describe one shipped DeltaScope rule by rule ID.
- `list_rules` — list shipped DeltaScope rules as compact catalog rows, with optional filters.
- `get_capabilities` — return a concise capability summary for MCP clients.

The `audit_sql` tool supports:

- MySQL offline audit
- TiDB offline audit
- PostgreSQL offline audit on the main PG-capable release binaries across the supported macOS and Linux platforms

Connection-backed metadata-aware audit supports MySQL/TiDB-compatible instances and PostgreSQL on the main PG-capable release binaries across the supported macOS and Linux platforms.

## Node.js runtime

The launcher requires Node.js 24 or newer (`engines.node` is `>=24`). Below 24
it fails closed with a clear error before downloading or starting the native
binary.

`lib/launcher.js` exports `requireSupportedNodeVersion` and `bootstrapLauncher`
for that gate.

## Version Contract

- npm package version should track the DeltaScope release version it boots
- by default the launcher resolves the native binary from its own package version
- `DELTASCOPE_MCP_VERSION` overrides the target DeltaScope version
- `DELTASCOPE_MCP_BASE_URL` overrides only the archive download base URL before the version and archive name are appended
- checksum verification still uses the official GitHub release checksums file

Default download base:

```text
https://github.com/Fanduzi/DeltaScope/releases/download
```

Example override:

```bash
DELTASCOPE_MCP_BASE_URL=https://mirror.example.com/deltascope/releases/download \
npx -y --prefer-online @fanduzi/deltascope-mcp@latest
```
