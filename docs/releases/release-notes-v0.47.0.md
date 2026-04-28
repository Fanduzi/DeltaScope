# DeltaScope v0.47.0 Release Notes

## Summary

v0.47.0 brings source location fidelity to all CI renderers. GitHub Actions, SARIF, and GitLab Code Quality outputs now carry the original file path and statement-start line number for each finding, so inline annotations point at the exact SQL statement that triggered the finding instead of the first line of the migration file.

## Changed

- The audit pipeline now populates `Line` and `Column` fields on each parsed statement using a progressive source mapper that scans the original SQL buffer forward, matching each `RawSQL` text and counting newlines. This replaces the previous statement-index fallback and produces correct line numbers for multi-statement migration files.
- `Finding.Location` is now populated from the statement location in the evaluation layer (only when the rule does not already provide a custom location), so every CI renderer automatically picks up source coordinates without per-renderer changes.
- GitHub Actions output (`--format github-actions`) now emits `file=<path>,line=N,col=N` with the correct statement-start line. When no `--file` path is provided, the `file=` key is omitted entirely instead of falling back to an empty value.
- SARIF output (`--format sarif`) now includes `artifactLocation.uri` with the file path and `startLine`/`startColumn` for each result. When no `--file` path is provided, `artifactLocation` is omitted.
- GitLab Code Quality output (`--format gitlab-codequality`) already propagated `--file` into `location.path`; the source mapper now ensures `location.lines.begin` carries the correct statement-start line number.
- Added `make release-source-location-smoke` gate that validates source location propagation across GitHub Actions, SARIF, GitLab Code Quality, and TiDB SARIF outputs. This gate is included in `make release-contract-gates`.
- Dedicated unit tests lock the progressive source mapper behavior: multi-line second-statement location, leading-newline handling, repeated-statement progressive match, blank-line skipping, and no-match fallback.
- Public API (`pkg/deltascope`) tests verify that `Audit()` returns `Finding.Location` with correct `Line` and `Column` for MySQL, TiDB, and PostgreSQL dialects.
- CLI integration tests verify TiDB SARIF and TiDB GitLab Code Quality source location fidelity.
- HTTP and MCP integration tests verify that structured responses include `location.line` and `location.column` in findings.

## Verification

- `make release-contract-gates VERSION=v0.47.0` — all gates pass
- `make release-source-location-smoke` — source location smoke passes
- `make test` — all unit tests pass
- Progressive source mapper produces line 9 for a `DELETE` on the second statement of a multi-line migration file (not line 2 from statement-index fallback)

## Non-Goals

- No new rule IDs, parser features, or policy changes.
- No domain logic changes beyond statement location propagation.
- No MySQL/TiDB/PostgreSQL audit behavior changes.
- No HTTP/MCP transport protocol changes beyond auto-serialized `location` field.
- No release asset naming changes.

## Install

**macOS (recommended):**

```bash
brew tap fanduzi/deltascope
brew install --cask deltascope
```

**Generic installer:**

```bash
curl -fsSL https://raw.githubusercontent.com/Fanduzi/DeltaScope/v0.47.0/install.sh | \
  DELTASCOPE_VERSION=v0.47.0 sh
```
