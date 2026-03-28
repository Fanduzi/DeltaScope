# DeltaScope v0.8.1 Release Notes

## Overview

DeltaScope `v0.8.1` is a metadata fix release for the MCP launcher publishing path. It keeps the `v0.8.0` functionality unchanged and corrects the npm package repository metadata required by sigstore provenance validation during CI package publishing.

## Highlights

- Fixed npm provenance validation for `@fanduzi/deltascope-mcp`
- No runtime or contract changes to the DeltaScope engine or MCP server
- Release-facing version links now point to `v0.8.1`

## What's Fixed

### npm launcher package metadata

- `packages/deltascope-mcp/package.json` now declares the canonical repository URL as `https://github.com/Fanduzi/DeltaScope`
- this aligns the package metadata with the GitHub Actions provenance bundle emitted during `npm publish --provenance`
- CI package publishing no longer fails on repository URL validation

## Install / Upgrade

Recommended launcher for MCP clients:

```bash
npx -y @fanduzi/deltascope-mcp --version
```

Native binary install:

```bash
curl -fsSL https://raw.githubusercontent.com/Fanduzi/DeltaScope/v0.8.1/install.sh | \
  DELTASCOPE_VERSION=v0.8.1 sh
```

## Compatibility

- Launcher runtime: Node.js `24+`
- Supported native targets: `darwin`, `linux`
- Supported architectures: `amd64`, `arm64`
- Supported database dialects: `MySQL`, `TiDB`
