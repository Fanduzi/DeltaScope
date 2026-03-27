# DeltaScope MCP Launcher

Bootstrap package for launching the native `deltascope-mcp` stdio server from npm-based MCP clients.

## What This Package Does

This package does not implement DeltaScope MCP tools itself. It downloads the matching native `deltascope-mcp` release binary, caches it locally, and then starts the real stdio server.

The launcher verifies the downloaded archive against the official DeltaScope release checksums before it updates the local cache.

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
npx -y @fanduzi/deltascope-mcp
```
