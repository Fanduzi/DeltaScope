# DeltaScope v0.22.0 Release Notes

Release date: 2026-04-11

## Overview

DeltaScope `v0.22.0` is the **E2E & Release Confidence Pack**. It closes confidence around the existing PostgreSQL product surface with canonical unit, E2E, package, and versioned release-surface gates.

This release does not add new PostgreSQL SQL rule semantics. No rule IDs, severity levels, trigger conditions, CLI flags, HTTP payload contracts, MCP tool contracts, or public Go API contracts change.

## What's Changed

### Canonical PostgreSQL Confidence Gates

Maintainers now have one documented set of repository entrypoints for the PostgreSQL confidence loop:

- `make pg-unit-test-gates` — runs PostgreSQL-tagged unit packages without Docker
- `make pg-e2e-gates` — runs Docker-backed PostgreSQL CLI, HTTP, and MCP end-to-end suites
- `make pg-confidence-gates` — runs the canonical combined PostgreSQL confidence closure

### Release-Surface Verification

The release path now separates package/release contract checks from versioned documentation and install-surface checks:

- `make release-surface-gates VERSION=v0.22.0` verifies the MCP launcher package/release contract.
- `make release-version-surface-gates VERSION=v0.22.0` verifies versioned README install snippets, bilingual release notes, and landing-page release-note links.

The GitHub release workflow runs both release-surface gates before packaging release artifacts.

### Documentation Alignment

Bilingual docs, CI recipes, CLI references, the audit capability matrix, scripts guide, changelog, and landing page now describe `v0.22.0` as a confidence and release-surface tightening milestone rather than a PostgreSQL SQL semantics release.

## Compatibility

No breaking changes.

- Existing MySQL, TiDB, and PostgreSQL audit behavior is unchanged.
- No new rule IDs, severity levels, or trigger conditions are introduced.
- CLI, HTTP, MCP, and `pkg/deltascope` public API contracts are unchanged.
- The PostgreSQL SQL semantics baseline remains the `v0.21.0` DDL coverage pack; `v0.22.0` verifies confidence around that existing surface.

## Install / Upgrade

```bash
curl -fsSL https://raw.githubusercontent.com/Fanduzi/DeltaScope/v0.22.0/install.sh | \
  DELTASCOPE_VERSION=v0.22.0 sh
```

macOS users can install with Homebrew:

```bash
brew tap Fanduzi/deltascope
brew install --cask deltascope
```
