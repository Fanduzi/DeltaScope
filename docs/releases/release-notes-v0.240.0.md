# DeltaScope v0.240.0 Release Notes

## Summary — Release Workflow Idempotency & Recovery Hardening

v0.240.0 hardens the release workflow against partial downstream failures and accidental reruns. It adds release asset and npm package preflight verification helpers, a manual `release-recover.yml` workflow for operator-driven recovery, and an idempotency guard in the full release workflow that prevents rerunning GoReleaser after release assets already exist. It does **not** change SQL audit behavior, parser support, audit rules, fallback parser implementation, parser_error counts, or any product-facing audit output.

## Release Safety Improvements

- **Release recovery preflight helpers** (`scripts/verify_release_assets.py`, `scripts/verify_npm_package_state.sh`): read-only verification of release assets, checksums, and npm package state for a given version. Used by both the full release and recovery workflows.
- **Manual release recovery workflow** (`.github/workflows/release-recover.yml`): operator-triggered workflow that re-runs select downstream release jobs (npm publish, Homebrew, GitHub release body) after a partial failure. Does not re-run GoReleaser or overwrite existing assets.
- **Full release idempotency guard** (`.github/workflows/release.yml`): the full release workflow now checks for existing release assets before running GoReleaser. If assets already exist, the workflow fails early with a message directing the operator to use the recovery workflow instead.
- **Operator docs** (`docs/dev/release-recovery.md`): documentation for release recovery procedures and preflight helper usage.

## New Make Targets

| Target | Purpose |
|--------|---------|
| `make release-recovery-preflight VERSION=vX.Y.Z` | Verify release assets and npm state for a published version |

## Non-Goals

- Not new parser support.
- Not new SQL audit rules.
- Not fallback parser implementation.
- Not reduced parser_error counts.
- Not full MySQL/TiDB/PostgreSQL DDL support.
- Not dialect parity.
- Not runtime/catalog validation.
- Not SQL audit behavior changes of any kind.

## Parser-Error Counts (unchanged)

| Dialect | Parser Error |
|---------|:------------:|
| MySQL | 15 |
| TiDB | 9 |
| PostgreSQL | 5 |
| **Total** | **29** |

## Parser-Error Feasibility Buckets (unchanged)

| Bucket | MySQL | TiDB | PostgreSQL | Total |
|--------|-------|------|------------|-------|
| `parser_upgrade_candidate` | 5 | 0 | 5 | 10 |
| `bounded_fallback_candidate` | 1 | 3 | 0 | 4 |
| `product_unsupported_or_inapplicable` | 0 | 6 | 0 | 6 |
| `unsafe_fallback_defer` | 9 | 0 | 0 | 9 |
| `needs_research` | 0 | 0 | 0 | 0 |

## DDL Coverage Census (unchanged)

| Dialect | Total | Finding | Silent | Unsupported | Parser Error |
|---------|------:|--------:|-------:|:-----------:|:------------:|
| MySQL | 61 | 46 | 0 | 0 | 15 |
| TiDB | 54 | 45 | 0 | 0 | 9 |
| PostgreSQL (consolidated tracked-case) | 285 | 274 | 6 | 0 | 5 |

## Unchanged Metrics

- SQL corpus: **582/582**, **100.0%**, **245 YAML** fixture files.
- PostgreSQL ALTER TABLE rule count: **32** (unchanged).
- PostgreSQL consolidated DDL census: **285/274/6/0/5/0** (unchanged).
- MySQL/TiDB DDL Notice section: **27** (unchanged).
- TiDB-Specific subsection: **7** (unchanged).

## Decision Record

`docs/decisions/2026-05-30-v0.240.0-release-workflow-idempotency-recovery-hardening.md`
