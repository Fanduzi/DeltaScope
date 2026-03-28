# DeltaScope v0.8.0 Release Notes

## Overview

DeltaScope `v0.8.0` turns MCP onboarding into a first-class release surface. This release keeps the official `deltascope-mcp` server from `v0.7.0`, then adds a publishable npm launcher, clearer quick-start documentation, and release workflow integration so users can connect MCP clients through a copy-and-use `npx` flow or keep using the native binary directly.

## Highlights

- Publishable npm launcher package: `@fanduzi/deltascope-mcp`
- Copy-and-use MCP quick start for Claude Code, Codex, and generic stdio clients
- Dedicated DeltaScope MCP onboarding guide in English and Chinese
- Release workflow now validates and publishes the npm launcher
- Launcher checksum verification, cache metadata, and lock recovery

## What's New

### npm launcher for MCP clients

- DeltaScope now publishes `@fanduzi/deltascope-mcp` as the recommended launcher package for MCP clients that prefer `npx`
- the launcher downloads the matching DeltaScope release archive, verifies it against the official checksums file, caches the native `deltascope-mcp` binary, and then forwards stdio to it
- the launcher supports version and release-base overrides for controlled environments while still validating the archive against official DeltaScope checksums

### Faster onboarding documentation

- README now exposes MCP quick-start snippets for Claude Code, Codex, and generic stdio configuration
- a dedicated MCP guide now explains launcher requirements, proxy setup, direct connection usage, `connection_ref`, and local YAML configuration
- English and Chinese onboarding docs now describe the native binary path and the npm launcher path as two explicit product-supported options

### Release pipeline integration

- the release workflow now validates the launcher package contract before publishing a tag
- tagged releases now publish the npm launcher package with provenance in the same delivery flow as Go release assets
- package version and Git tag are now enforced to match during release

### Launcher hardening

- launcher bootstrap logs now go to `stderr` so MCP `stdout` remains protocol-only
- first-run archive downloads are verified against the official DeltaScope checksums file before extraction
- cache metadata, lock timeout, and stale-lock recovery reduce the chance of stuck or corrupted first-run installs

## Install / Upgrade

Recommended launcher for MCP clients:

```bash
npx -y @fanduzi/deltascope-mcp --version
```

Native binary install:

```bash
curl -fsSL https://raw.githubusercontent.com/Fanduzi/DeltaScope/v0.8.0/install.sh | \
  DELTASCOPE_VERSION=v0.8.0 sh
```

## Compatibility

- Launcher runtime: Node.js `24+`
- Supported native targets: `darwin`, `linux`
- Supported architectures: `amd64`, `arm64`
- Supported database dialects: `MySQL`, `TiDB`

## Known Limitations

- the launcher still depends on GitHub release reachability, Node 24+, and a system `tar` command on first run
- proxy environments may still require `NODE_USE_ENV_PROXY=1`
- MCP transport remains stdio only
