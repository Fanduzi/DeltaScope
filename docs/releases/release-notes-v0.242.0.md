# DeltaScope v0.242.0 Release Notes

## Summary — Release Recovery CI Polish

v0.242.0 formally ships the post-v0.241.0 `GH_TOKEN` preflight fix and adds a regression gate so the release recovery workflow's preflight authentication wiring cannot be silently removed. The `make release-recovery-contract-test` target now statically verifies that `GH_TOKEN: ${{ secrets.GITHUB_TOKEN }}` is present in the preflight step of `.github/workflows/release-recover.yml`. This release does **not** change SQL audit behavior, parser support, audit rules, fallback parser implementation, parser_error counts, or any product-facing audit output.

## Release Recovery Preflight Auth Fix

- **GH_TOKEN wiring in preflight** (`.github/workflows/release-recover.yml`): the release recovery workflow's preflight step now includes `GH_TOKEN: ${{ secrets.GITHUB_TOKEN }}` so that `gh release download` and `gh release view` commands succeed. This fix was committed as `0e3f31c` after v0.241.0 was tagged; v0.242.0 is the first tagged release to include it.
- **Regression gate** (`Makefile`): `make release-recovery-contract-test` now statically checks that `GH_TOKEN` is wired in the workflow file. If the wiring is removed in a future edit, the contract test will fail.

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

`docs/decisions/2026-06-01-v0.242.0-release-recovery-ci-polish.md`
