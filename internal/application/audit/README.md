# Application Audit Module

Application orchestration for parsing and, later, evaluating SQL audit requests.

## Files

| File | Responsibility |
|------|---------------|
| parse.go | Builds application-owned parsed statements from infrastructure-backed parser adapters |
| parse_test.go | Verifies that application parsing hides parser-specific AST details |

## Exports

- `Parse(sql string, dialect spec.Dialect)`
- `ParsedStatement`
- `ParsedSQL`

## Dependencies
- Upstream: future CLI and public audit entrypoints
- Downstream: `internal/domain/spec`, `internal/infrastructure/parser/tidb`, `github.com/pingcap/tidb/pkg/parser/ast`

## Update Rule
- If members/interfaces/dependencies change, update this file in same change.
