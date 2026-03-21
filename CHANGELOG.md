# Changelog

All notable changes to DeltaScope will be documented in this file.

The format follows Keep a Changelog and the project uses semantic versioning for release tags.

## [Unreleased]

### Added

- Metadata-aware lifecycle checks for `DROP TABLE` and `TRUNCATE TABLE`
- Explicit DDL operation modeling for create-view, drop-table, drop-view, and truncate-table extraction
- Product-surface docs including bilingual README, changelog, and security policy

### Changed

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
