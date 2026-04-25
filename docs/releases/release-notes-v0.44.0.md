# DeltaScope v0.44.0 Release Notes

## Summary

DeltaScope v0.44.0 hardens the release verification pipeline with centralized version surface checks, binary version smoke, npm launcher contract gates, default policy dialect hygiene smoke, and a unified `make release-contract-gates` target. Every tag now passes a single pre-release gate that verifies source constants, package versions, README install pins, release notes, landing page surfaces, binary version output, npm launcher tests, archive naming contracts, and dialect isolation before the tag is pushed.

## Added

- Centralized version surface verification script (`scripts/verify_release_version_surfaces.sh`) checks all release-facing version references in one pass: source constants, npm package, README install pins, release notes H1, release index links, landing DOM hero/release-version/footer, and landing JS i18n strings.
- Binary version smoke target (`make release-local-version-smoke`) builds all three binaries with version ldflags and asserts `deltascope --version`, `deltascope-server --version`, and `deltascope-mcp -version` report the expected tag.
- npm launcher archive and checksum naming contract tests in `packages/deltascope-mcp/test/platform.test.js` verify `resolveArchiveName` and `resolveChecksumsName` follow the `deltascope_<version>_<os>_<arch>` contract.
- Archive verifier (`scripts/verify_release_archive.sh`) now runs dialect hygiene against the extracted binary after the PG audit smoke, catching cross-dialect rule leaks inside packaged release artifacts.
- Release dialect hygiene smoke script (`scripts/verify_release_dialect_hygiene.sh`) verifies default policy dialect isolation: PostgreSQL audits must not emit MySQL/TiDB-only rules or remediation text, and MySQL/TiDB audits must not emit PostgreSQL-only rules.
- Unified release contract gate target (`make release-contract-gates VERSION=vX.Y.Z`) composes version surface gates, local binary version smoke, dialect hygiene gates, npm launcher tests, and goreleaser config check into a single pre-release entry point.
- Release workflow now runs `make release-contract-gates` before GoReleaser publishes, blocking tag pushes that have stale runtime versions, missing release notes, or dialect isolation regressions.

## Example

Run the full release contract gate before tagging:

```bash
make release-contract-gates VERSION=v0.44.0
```

This single command verifies version constants, package versions, README install pins, release notes, landing surfaces, binary version output, npm launcher tests, and dialect isolation — all before the tag is pushed.

## Release Contract

| Field | Value |
|------|-------|
| Kind | Release contract hardening |
| Scope | Pre-release verification pipeline only |
| New gate | `make release-contract-gates VERSION=vX.Y.Z` |
| Surfaces | version.go, package.json, README, release notes, landing, binary output, npm launcher, archive naming, dialect hygiene |

## Non-Goals

- New rule IDs
- New parser features
- New public API contracts
- Live schema validation
- Domain logic changes
- MySQL/TiDB or PostgreSQL audit behavior changes
- Release artifact structure changes

## Install

```bash
curl -fsSL https://raw.githubusercontent.com/Fanduzi/DeltaScope/v0.44.0/install.sh | \
  DELTASCOPE_VERSION=v0.44.0 sh
```
