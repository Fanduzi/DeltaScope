# TiDB Parser Module

TiDB-backed parser adapter for multi-statement SQL parsing and coarse statement classification.

## Files

| File | Responsibility |
|------|---------------|
| parser.go | Parses SQL text, preserves parser warnings, and classifies statements |
| parser_test.go | Verifies multi-statement parsing and parse-failure behavior |

## Exports

- `Parser`
- `ParsedStatement`
- `Result`
- `New()`

## Dependencies
- Upstream: `internal/application/audit`
- Downstream: `internal/domain/spec`, `github.com/pingcap/tidb/pkg/parser`

## Update Rule
- If members/interfaces/dependencies change, update this file in same change.
