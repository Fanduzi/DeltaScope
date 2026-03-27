# DeltaScope v0.7.0 Release Notes

## Overview

DeltaScope `v0.7.0` adds the official MCP stdio server as a stable product surface. This release keeps the existing offline-first audit engine and explainable result model, then exposes them through a structured MCP contract with rule-discovery tools, metadata-aware database access, and real end-to-end validation against MySQL and TiDB.

## Highlights

- Official `deltascope-mcp` stdio server
- Stable MCP tools: `audit_sql`, `describe_rule`, `list_rules`, `get_capabilities`
- Structured MCP tool errors and published output schemas
- Metadata-aware MCP flows for direct connections and `connection_ref`
- Real MySQL and TiDB MCP metadata e2e coverage

## What's New

### Official MCP server

- DeltaScope now ships `deltascope-mcp` as the official MCP stdio entrypoint
- MCP clients can audit SQL, inspect shipped rules, list rules, and discover the server contract through `get_capabilities`
- the MCP adapter stays thin and reuses the same audit path as the CLI, HTTP service, and public Go API

### Metadata-aware MCP support

- `audit_sql` supports both direct inline `connection` objects and named `connection_ref` entries
- metadata-aware MCP runs return explicit `context` fields such as mode, dialect, schema, schema source, and metadata source
- schema inference hints and connection errors now return stable machine-readable MCP error codes

### Shared preparation path

- shared metadata-aware preparation now lives in `internal/application/auditmeta`
- CLI and MCP both use the same dialect detection, schema inference, and connection-opening flow
- CLI-specific semantics such as `schema_source:"flag"` remain preserved

### Release and validation updates

- GoReleaser archives now include `deltascope-mcp`
- the default installer now installs `deltascope`, `deltascope-server`, and `deltascope-mcp`
- Docker-backed MCP metadata e2e now proves direct and `connection_ref` flows against real MySQL and TiDB targets

## Install / Upgrade

Install the latest release:

```bash
curl -fsSL https://raw.githubusercontent.com/Fanduzi/DeltaScope/main/install.sh | sh
```

Install this exact release:

```bash
curl -fsSL https://raw.githubusercontent.com/Fanduzi/DeltaScope/v0.7.0/install.sh | \
  DELTASCOPE_VERSION=v0.7.0 sh
```

The published archive format remains `deltascope_0.7.0_<os>_<arch>.tar.gz`.

## Compatibility

- Supported OS targets: `darwin`, `linux`
- Supported architectures: `amd64`, `arm64`
- Supported database dialects: `MySQL`, `TiDB`

## Known Limitations

- metadata-aware live checks still depend on live schema access and are not available in offline mode
- MCP transport is currently stdio only
- `connection_ref` still expects a local YAML configuration file unless clients send inline connection details
