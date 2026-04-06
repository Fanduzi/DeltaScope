# TiDB Parser Module

TiDB-backed parser adapter for multi-statement SQL parsing and parser-warning collection.

## Files

| File | Responsibility |
|------|---------------|
| parser.go | Parses SQL text and preserves raw statement nodes plus parser warnings |
| extractor.go | Wraps TiDB AST nodes in parser-neutral extractors and performs TiDB-specific statement extraction |
| parser_test.go | Verifies multi-statement parsing, parse-failure behavior, and extractor-backed wrapping |

## Exports

- `Parser`
- `Result`
- `ExtractedStatement`
- `New()`
- `WrapStatements()`

## Dependencies
- Upstream: `internal/application/audit`, `internal/domain/spec`
- Downstream: `github.com/pingcap/tidb/pkg/parser`

## Update Rule
- If members/interfaces/dependencies change, update this file in same change.
