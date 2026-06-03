# DeltaScope v0.251.0 Release Notes

## Summary — Release Tag Annotation Guard

v0.251.0 adds a release tag annotation guard that prevents lightweight git tags from passing through the release pipeline. This release does **not** add parser support, change parser behavior, add SQL audit rules, implement a fallback parser, reduce `parser_error` counts, change public output shapes, or alter SQL audit behavior in any way.

## Tag Annotation Guard

- **`scripts/verify_release_tag_annotation.py`** — verifies a local tag is annotated (`git cat-file -t` returns `tag`), not lightweight.
- **`make release-tag-annotation-test`** — runs offline unit tests (no tag required).
- **`make release-tag-annotation-gate VERSION=vX.Y.Z`** — verifies an existing tag after creation (post-tag only).
- **`.github/workflows/release.yml`** — early guard step rejects lightweight tags before any artifacts are published.

## Non-Goals

- No parser support added.
- No fallback parser.
- No new SQL audit rules.
- No parser_error count reduction.
- No DDL census change.
- No public output shape change.
- No SQL audit behavior change.
- No npm/Homebrew behavior change.

## Parser-Error Counts (unchanged)

| Dialect | Parser Error |
|---------|:------------:|
| MySQL | 15 |
| TiDB | 9 |
| PostgreSQL | 5 |
| **Total** | **29** |

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

`docs/decisions/2026-06-03-v0.251.0-release-tag-annotation-guard.md`
