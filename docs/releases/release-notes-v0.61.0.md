# DeltaScope v0.61.0 Release Notes

## Summary

v0.61.0 delivers comprehensive codebase quality improvements: database connection pool leak fixes, MCP server panic recovery with three-layer protection, static analysis integration fixing 903 code-quality issues, context propagation support for timeout and cancellation, parallel test execution for 1522 tests, and performance optimizations including slice preallocation, strings.Builder string concatenation, and builder pool reuse in the markdown renderer. All files in the codebase are now under 800 lines. No new rules, parser features, or public API changes.

## Quality Improvements

| Area | Change |
|------|--------|
| Database connections | Connection pool leak fixes with proper lifecycle management (`SetMaxOpenConns`, `SetMaxIdleConns`, `SetConnMaxLifetime`) |
| MCP stability | Three-layer panic recovery: tool handler, server handler, and process-level |
| Static analysis | golangci-lint v2 integration with 15 active linters; 903 issues auto-fixed |
| Context propagation | `context.Context` timeout and cancellation support across all audit layers |
| Test performance | Parallel test execution (`t.Parallel()`) for 1522 tests |
| Runtime performance | Slice preallocation, `strings.Builder` for string concatenation, builder pool for markdown renderer reuse |

## Performance Benchmarks

Hot-path optimizations include preallocated slices in rule evaluation, `strings.Builder` replacing `fmt.Sprintf` concatenation in output renderers, and a sync.Pool-based builder pool for the markdown renderer.

## Non-Goals

- No new rule IDs, parser features, or public API changes.
- No MySQL/TiDB/PostgreSQL audit behavior changes.
- No release asset naming or install workflow changes.

## Install

**macOS (recommended):**

```bash
brew tap fanduzi/deltascope
brew install --cask deltascope
```

**Generic installer:**

```bash
curl -fsSL https://raw.githubusercontent.com/Fanduzi/DeltaScope/v0.61.0/install.sh | \
  DELTASCOPE_VERSION=v0.61.0 sh
```
