# DeltaScope v0.6.1 Release Notes

## Overview

DeltaScope `v0.6.1` is the first polished public release after the initial `v0.5.0` baseline. It completes the product-facing documentation surface, closes the CLI metadata-aware rollout, and ships a tag-driven release/install path for MySQL and TiDB SQL auditing workflows.

## Highlights

- Release-ready CLI for offline and metadata-aware SQL auditing
- Docker-backed MySQL and TiDB live smoke coverage for metadata-aware CLI paths
- Product-facing documentation tree with recipes, reference docs, and ASCII architecture guides
- Tag-driven GitHub Actions release workflow plus `install.sh`
- Apache License 2.0 licensing and bilingual release documentation

## What’s New

### CLI completion

- `deltascope audit` now exposes the shipped metadata-aware audit path with MySQL-style connection flags
- `rules list`, `rules show`, `rules search`, `config lint`, `config show-default`, and `capabilities` are now part of the public CLI surface
- password prompting, schema inference, and schema-qualified SQL handling were hardened

### Metadata-aware confidence

- live smoke coverage now runs against real MySQL and TiDB Docker fixtures
- the e2e harness validates dialect auto-detect, schema inference, ambiguity handling, qualified-schema SQL, and metadata-backed checks

### Documentation and product surface

- `README.md` and `README_ZH.md` now act as product landing pages
- docs are now organized under `docs/admin`, `docs/concept`, `docs/dev`, `docs/recipe`, `docs/reference`, and `docs/releases`
- the audit capability matrix moved into stable reference docs

### Release and installation

- GitHub Actions now owns the single trusted tag-driven release path
- GoReleaser packages both `deltascope` and `deltascope-server`
- `install.sh` installs published release archives directly

## Install / Upgrade

Install the latest release:

```bash
curl -fsSL https://raw.githubusercontent.com/Fanduzi/DeltaScope/main/install.sh | sh
```

Install this exact release:

```bash
curl -fsSL https://raw.githubusercontent.com/Fanduzi/DeltaScope/v0.6.1/install.sh | \
  DELTASCOPE_VERSION=v0.6.1 sh
```

## Compatibility

- Supported OS targets: `darwin`, `linux`
- Supported architectures: `amd64`, `arm64`
- Supported database dialects: `MySQL`, `TiDB`

## Known Limitations

- metadata-aware live smoke currently targets local Docker single-instance scenarios
- HTTP service hardening is intentionally out of scope for this release
- MCP Server is not part of this release line
