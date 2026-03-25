# Application Audit Module

Application orchestration for parsing and, later, evaluating SQL audit requests.

## Files

| File | Responsibility |
|------|---------------|
| parse.go | Builds application-owned parsed statements from infrastructure-backed parser adapters |
| parse_test.go | Verifies that application parsing hides parser-specific AST details |
| extract.go | Converts parsed statements into first-pass domain `Statement` values with explicit DDL operations, separate DDL constraint handling, typed index metadata, create-table shape flags, preserved unnamed-index names for identifier governance, explicit column charset/collation facts, normalized row-format and auto-increment-init table options, richer alter-table column/index/rename/option payloads, object-lifecycle extraction for create-view/drop/truncate, honest statement-local alter change facts, multi-column add normalization, and DML operation plus target-table facts for offline governance |
| extract_test.go | Verifies representative DDL and DML extraction behavior, including create-like/create-as/partition flags plus enriched create-table facts, preserved backticked-keyword and unnamed-index names, extracted column charset/collation facts, normalized row-format and auto-increment-init options, explicit DDL lifecycle operations, richer alter-table detail including explicit statement-local change facts, multi-column add expansion, non-index constraint handling, and extracted DML target tables |
| evaluate.go | Applies registered rules, enriches findings with explanation metadata, and aggregates statement/global findings into report output |
| evaluate_test.go | Verifies application-owned report-flow integration and explanation enrichment over the rule registry |
| explain.go | Joins evaluated findings with shipped catalog metadata and statement metadata availability notes |
| service.go | Orchestrates the full audit flow across policy loading, parsing, extraction, top-level request metadata plumbing, optional metadata enrichment, rule registration, and evaluation |
| service_test.go | Verifies the end-to-end application audit use case with defaults, config overrides, multi-statement SQL, and metadata enrichment behavior, including schema-only context and top-level request plumbing |
| metadata.go | Defines the optional metadata-provider interface and attaches schema, instance, and target-table facts to statements before evaluation |

## Exports

- `Parse(sql string, dialect spec.Dialect)`
- `Extract(parsed ParsedSQL)`
- `EvaluateStatements(registry, statements)`
- `AuditSQL(ctx, request)`
- `Request`
- `MetadataRequest`
- `MetadataProvider`
- `Service`
- `NewService()`
- `Service.Audit(ctx, request)`
- `ParsedStatement`
- `ParsedSQL`

## Dependencies
- Upstream: future CLI and public audit entrypoints
- Downstream: `context`, `internal/application/policy`, `internal/domain/report`, `internal/domain/rule`, `internal/domain/rule/ddl`, `internal/domain/rule/dml`, `internal/domain/spec`, `internal/infrastructure/parser/tidb`, `github.com/pingcap/tidb/pkg/parser/ast`

## Update Rule
- If members/interfaces/dependencies change, update this file in same change.
