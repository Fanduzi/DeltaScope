# Changelog

All notable changes to DeltaScope will be documented in this file.

The format follows Keep a Changelog and the project uses semantic versioning for release tags.

## [Unreleased]

Release target: `v0.6.0`

### Added

- Product docs IA under `docs/admin`, `docs/concept`, `docs/dev`, `docs/recipe`, and `docs/reference`
- Product-facing English and Chinese landing pages
- Product-level and implementation-level ASCII architecture docs
- Task-oriented recipe docs and stable reference docs
- Tag-driven GitHub Actions release workflow with GoReleaser packaging
- `install.sh` for release-archive installation
- Stable local operator targets in `Makefile` for test/build/CLI e2e flows
- Metadata-aware lifecycle checks for `DROP TABLE` and `TRUNCATE TABLE`
- Explicit DDL operation modeling for create-view, drop-table, drop-view, and truncate-table extraction
- Product-surface docs including bilingual README, changelog, and security policy

### Changed

- Release-facing artifact contract now centers on `deltascope_<version>_<os>_<arch>.tar.gz`
- Default source-build version target now points to `v0.6.0`
- Metadata-backed existence rules now advertise `requires_metadata: true` instead of the misleading legacy `required` param
- Capability matrix statuses now reflect shipped metadata-aware and lifecycle coverage

## [v0.5.0] - 2026-03-21

### Added

- Stable `pkg/deltascope` public API
- `deltascope` CLI with `audit`, `config init`, and `version`
- ASCII-logo version command and explicit `--version` behavior
- `deltascope-server` HTTP service with `healthz`, `version`, and `v1/audit`
- Offline-first DDL and DML rule catalog for MySQL and TiDB
- Optional metadata-aware instance facts and table snapshots
