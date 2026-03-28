# Changelog

All notable changes to DeltaScope will be documented in this file.

The format follows Keep a Changelog and the project uses semantic versioning for release tags.

## [Unreleased]

Release target: `TBD`

## [v0.9.2] - 2026-03-28

### Changed

- Documentation now aligns top-level install, MCP, skill, and architecture guidance with the current multi-surface product story and release metadata
- Release notes index now includes the published `v0.9.0`, `v0.9.1`, and `v0.9.2` entries for stable navigation from the root README

## [v0.9.1] - 2026-03-28

### Fixed

- CI release pipeline: removed redundant `npm publish --dry-run` call that caused false failures when publishing a new tag (npm rejects even dry-run publishes for already-published versions)

## [v0.9.0] - 2026-03-28

### Added

- Homebrew Cask distribution via `brew tap Fanduzi/deltascope && brew install --cask deltascope`
- Claude Code Skill `deltascope-review` for inline SQL review in Claude Code, Codex, Cursor and 40+ AI agents
- `skills/` directory with public Skill file and install documentation
- Install via `npx skills add Fanduzi/DeltaScope --skill deltascope-review`

## [v0.8.1] - 2026-03-28

### Fixed

- npm launcher package metadata now declares the canonical GitHub repository URL so npm provenance validation can accept CI publishes
- source-build default version and release-facing install links now point to `v0.8.1`

## [v0.8.0] - 2026-03-28

### Added

- npm launcher package `@fanduzi/deltascope-mcp` for copy-and-use MCP onboarding through `npx`
- dedicated DeltaScope MCP onboarding guides in English and Chinese with Claude Code, Codex, generic stdio, direct connection, and `connection_ref` examples
- launcher bootstrap diagnostics on `stderr`, GitHub release checksum verification, cache metadata, and override support for release mirrors

### Changed

- release workflow now validates and publishes the MCP launcher package alongside Go release assets
- README and recipe entrypoints now present MCP quick-start guidance and explicit Node 24+ / platform requirements
- launcher cache handling now uses lock timeout and stale-lock recovery to avoid wedged first-run installs

## [v0.7.0] - 2026-03-27

### Added

- Official `deltascope-mcp` stdio server with `audit_sql`, `describe_rule`, `list_rules`, and `get_capabilities`
- Structured MCP tool errors with stable machine-readable codes
- Metadata-aware MCP support for direct `connection` inputs and named `connection_ref` configs
- Shared `internal/application/auditmeta` preparation flow reused by both CLI and MCP adapters
- Explicit MCP output schemas and client-facing capability summaries
- Docker-backed MCP metadata e2e coverage for MySQL and TiDB, including direct and `connection_ref` flows

### Changed

- Release archives and the default installer now ship `deltascope-mcp` alongside `deltascope` and `deltascope-server`
- Source-build default version now points to `v0.7.0`
- English and Chinese README, recipes, release notes, and module docs now describe the official MCP contract and release surface

## [v0.6.2] - 2026-03-25

### Added

- Aggregate `explanation` blocks on audit results and per-statement results
- Structured per-finding explanation fields including `summary`, `why`, `risk`, and `suggestion`
- Metadata-availability notes on explanation metadata for metadata-aware findings
- Public API and HTTP coverage for explainable audit result shapes
- English and Chinese release notes for `v0.6.2`

### Changed

- Markdown CLI output now renders richer explanation details for findings and aggregate audit summaries
- Rule catalog entries now carry explanation-oriented metadata, examples, and remediation hints for discovery commands
- English and Chinese README, recipe, and reference docs now align with runtime output contracts and localized links
- Default source-build version target now points to `v0.6.2`
- Release-facing install examples now target `v0.6.2`

## [v0.5.0] - 2026-03-21

### Added

- Stable `pkg/deltascope` public API
- `deltascope` CLI with `audit`, `config init`, and `version`
- ASCII-logo version command and explicit `--version` behavior
- `deltascope-server` HTTP service with `healthz`, `version`, and `v1/audit`
- Offline-first DDL and DML rule catalog for MySQL and TiDB
- Optional metadata-aware instance facts and table snapshots
