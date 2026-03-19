# TiDB Parser Module

TiDB-backed parser adapter for multi-statement SQL parsing and parser-warning collection.

## Files

| File | Responsibility |
|------|---------------|
| parser.go | Parses SQL text and preserves raw statement nodes plus parser warnings |
| parser_test.go | Verifies multi-statement parsing and parse-failure behavior |

## Exports

- `Parser`
- `Result`
- `New()`

## Dependencies
- Upstream: `internal/application/audit`
- Downstream: `github.com/pingcap/tidb/pkg/parser`

## Update Rule
- If members/interfaces/dependencies change, update this file in same change.
