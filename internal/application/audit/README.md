# Application Audit Module

Application orchestration for parsing and, later, evaluating SQL audit requests.

## Files

| File | Responsibility |
|------|---------------|
| parse.go | Builds application-owned parsed statements from infrastructure-backed parser adapters |
| parse_test.go | Verifies that application parsing hides parser-specific AST details |
| extract.go | Converts parsed statements into first-pass domain `Statement` values with separate DDL constraint and DML join shape handling |
| extract_test.go | Verifies representative DDL and DML extraction behavior and unknown-statement flow |

## Exports

- `Parse(sql string, dialect spec.Dialect)`
- `Extract(parsed ParsedSQL)`
- `ParsedStatement`
- `ParsedSQL`

## Dependencies
- Upstream: future CLI and public audit entrypoints
- Downstream: `internal/domain/spec`, `internal/infrastructure/parser/tidb`, `github.com/pingcap/tidb/pkg/parser/ast`

## Update Rule
- If members/interfaces/dependencies change, update this file in same change.
