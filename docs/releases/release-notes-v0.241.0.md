# DeltaScope v0.241.0 Release Notes

## Summary — Release Recovery Dry-Run Patch

v0.241.0 makes the `release-recover.yml` workflow default-safe by adding a `dry_run` input (default `true`). In dry-run mode, the workflow exercises the full recovery preflight pipeline — release asset/checksum verification, Homebrew cask render/verify with tap clone/diff, and npm package state check — without executing any destructive or publishing side effects. Real recovery still requires explicitly setting `dry_run=false`. This patch does **not** change SQL audit behavior, parser support, audit rules, fallback parser implementation, parser_error counts, or any product-facing audit output.

## Release Recovery Dry-Run

- **Default-safe `dry_run` input** (`.github/workflows/release-recover.yml`): the recovery workflow now accepts a required `dry_run` boolean input defaulting to `true`. Operators must explicitly set `dry_run=false` to execute real recovery actions.
- **Dry-run verification scope** (what runs in dry-run mode):
  - Release asset and checksum preflight checks.
  - Homebrew cask render/verify: clones the tap, renders the cask, and shows `git diff --stat` against the current cask file.
  - npm package state check: verifies whether the target version already exists on the registry.
- **Dry-run does not execute** (what is suppressed in dry-run mode):
  - Homebrew tap push.
  - Homebrew install verification.
  - npm publish.
  - GitHub release upload or delete.
  - Git tag operations.
- **Real recovery**: operators must explicitly set `dry_run=false` to perform actual Homebrew tap push, npm publish, or Homebrew install verification.

## Updated Make Targets

| Target | Purpose |
|--------|---------|
| `make release-recovery-contract-test` | Verify dry-run default and contract gates for the recovery workflow |

## Non-Goals

- Not new parser support.
- Not new SQL audit rules.
- Not fallback parser implementation.
- Not reduced parser_error counts.
- Not full MySQL/TiDB/PostgreSQL DDL support.
- Not dialect parity.
- Not runtime/catalog validation.
- Not SQL audit behavior changes of any kind.
- Not DDL census change.
- Not PG ALTER TABLE rule count change.

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

`docs/decisions/2026-05-31-v0.241.0-release-recovery-dry-run-patch.md`
