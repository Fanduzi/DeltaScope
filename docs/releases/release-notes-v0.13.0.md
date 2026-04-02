# DeltaScope v0.13.0 Release Notes

## Overview

DeltaScope `v0.13.0` makes the HTTP surface metadata-aware. This release keeps the existing offline-first audit contract, then adds direct MySQL and TiDB metadata preparation to `POST /v1/audit` so deployed HTTP services can return the same policy-driven findings that CLI and MCP users already get from live schema context.

## What's Changed

### HTTP Metadata-Aware Audit

The HTTP adapter now accepts direct `connection` inputs on `POST /v1/audit` and can prepare live metadata before evaluating SQL. The response keeps the stable verdict/findings schema and adds additive context fields that report:

- resolved `dialect`
- `dialect_source`
- resolved `schema`
- `schema_source`
- `metadata_source`

This gives HTTP clients the same metadata-aware audit shape already used by the CLI and MCP surfaces.

### Shared Direct-Connection Helpers

Direct connection validation and password resolution are now centralized in shared interface helpers:

- HTTP and MCP now use the same host / socket validation rules
- `password`, `password_env`, and `password_file` follow the same mutually exclusive contract across adapters
- direct credential lookup failures are mapped to the stable `connection_invalid` HTTP error code

### Real-Server HTTP End-to-End Coverage

The HTTP release surface now has Docker-backed end-to-end coverage that builds and launches the real `deltascope-server` binary, then exercises metadata-aware requests against:

- MySQL fixtures
- TiDB fixtures

This closes the gap between the documented HTTP contract and the tested deployment path.

## Install / Upgrade

**macOS (recommended):**

```bash
brew tap Fanduzi/deltascope
brew install --cask deltascope
```

Or upgrade:

```bash
brew upgrade --cask deltascope
```

**Linux / other environments:**

```bash
curl -fsSL https://raw.githubusercontent.com/Fanduzi/DeltaScope/v0.13.0/install.sh | \
  DELTASCOPE_VERSION=v0.13.0 sh
```

## Compatibility

No breaking changes. `v0.13.0` extends the HTTP adapter with additive metadata-aware inputs and response context while preserving the stable CLI, HTTP, MCP, and Go library contracts.
