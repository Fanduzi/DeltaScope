# Application Audit Module

Application orchestration for parsing and, later, evaluating SQL audit requests.

## Files

| File | Responsibility |
|------|---------------|
| parse.go | Builds application-owned parsed statements from infrastructure-backed parser adapters |
| parse_test.go | Verifies that application parsing hides parser-specific AST details |
| extract.go | Converts parsed statements into first-pass domain `Statement` values with separate DDL constraint handling and richer DML operation facts |
| extract_test.go | Verifies representative DDL and DML extraction behavior, including insert-select and on-duplicate metadata |
| evaluate.go | Applies registered rules and aggregates statement/global findings into report output |
| evaluate_test.go | Verifies application-owned report-flow integration over the rule registry |
| service.go | Orchestrates the full offline audit flow across policy loading, parsing, extraction, rule registration, and evaluation |
| service_test.go | Verifies the end-to-end application audit use case with defaults, config overrides, and multi-statement SQL |

## Exports

- `Parse(sql string, dialect spec.Dialect)`
- `Extract(parsed ParsedSQL)`
- `EvaluateStatements(registry, statements)`
- `AuditSQL(ctx, request)`
- `Request`
- `Service`
- `NewService()`
- `Service.Audit(ctx, request)`
- `ParsedStatement`
- `ParsedSQL`

## Dependencies
- Upstream: future CLI and public audit entrypoints
- Downstream: `internal/application/policy`, `internal/domain/report`, `internal/domain/rule`, `internal/domain/rule/ddl`, `internal/domain/rule/dml`, `internal/domain/spec`, `internal/infrastructure/parser/tidb`, `github.com/pingcap/tidb/pkg/parser/ast`

## Update Rule
- If members/interfaces/dependencies change, update this file in same change.
