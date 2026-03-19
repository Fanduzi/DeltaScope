# Application Audit Module

Application orchestration for parsing and, later, evaluating SQL audit requests.

## Files

| File | Responsibility |
|------|---------------|
| parse.go | Delegates SQL parsing to infrastructure-backed parser adapters |

## Exports

- `Parse(sql string, dialect spec.Dialect)`

## Dependencies
- Upstream: future CLI and public audit entrypoints
- Downstream: `internal/domain/spec`, `internal/infrastructure/parser/tidb`

## Update Rule
- If members/interfaces/dependencies change, update this file in same change.
