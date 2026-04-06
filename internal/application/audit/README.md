# Application Audit Module

Application orchestration for parsing and, later, evaluating SQL audit requests.

## Files

| File | Responsibility |
|------|---------------|
| parse.go | Dispatches by dialect, builds application-owned parsed statements from infrastructure-backed parser adapters via parser-neutral extractors, and leaves PostgreSQL build-tagged support behind the Phase 3 adapter seam |
| parse_pg.go | Implements PostgreSQL parsing when built with the `postgresql` tag |
| parse_pg_stub.go | Returns the PG-capable build guidance error when PostgreSQL support is not compiled in |
| parse_test.go | Verifies that application parsing hides parser-specific AST details |
| extract.go | Converts parsed statements into first-pass domain `Statement` values by invoking parser-neutral extractors and attaching shape-only DML impact facts |
| extract_test.go | Verifies representative DDL and DML extraction behavior, including create-like/create-as/partition flags plus enriched create-table facts, preserved backticked-keyword and unnamed-index names, extracted column charset/collation facts, normalized row-format and auto-increment-init options, explicit DDL lifecycle operations, richer alter-table detail including explicit statement-local change facts, multi-column add expansion, non-index constraint handling, and extracted DML target-table plus predicate-shape facts |
| impact.go | Maps extracted DML predicate shapes to conservative offline impact estimates, populates statement `impact` objects with `estimated_rows`, `estimated_ratio`, `risk_level`, `confidence`, `source`, `reason_codes`, and optional `notes`, upgrades shape-derived sources to metadata after enrichment, and refines the narrow primary-key-on-`id` case when metadata snapshots confirm `PRIMARY(id)` plus optional `table_rows` facts |
| impact_test.go | Verifies shape-only impact estimation plus post-enrichment metadata source upgrades, unique-equality refinement, and offline preservation behavior for representative UPDATE and DELETE shapes and their additive `impact` payloads |
| evaluate.go | Applies registered rules, enriches findings with explanation metadata, and aggregates statement/global findings into report output while preserving statement-level DML `impact` estimates |
| evaluate_test.go | Verifies application-owned report-flow integration and explanation enrichment over the rule registry |
| explain.go | Joins evaluated findings with shipped catalog metadata and statement metadata availability notes |
| service.go | Orchestrates the full audit flow across policy loading, parsing, extraction, top-level request metadata plumbing, optional metadata enrichment, post-enrichment DML impact attachment/refinement, rule registration, evaluation, and partial-support error propagation for unsupported statements |
| service_test.go | Verifies the end-to-end application audit use case with defaults, config overrides, multi-statement SQL, PostgreSQL validation-boundary acceptance, mixed supported/unsupported partial results, metadata enrichment behavior, metadata-backed DML `impact` surfacing, and schema-only context/top-level request plumbing |
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
- Downstream: `context`, `fmt`, `internal/application/policy`, `internal/domain/report`, `internal/domain/rule`, `internal/domain/rule/ddl`, `internal/domain/rule/dml`, `internal/domain/spec`, `internal/infrastructure/parser/postgresql`, `internal/infrastructure/parser/tidb`

## Update Rule
- If members/interfaces/dependencies change, update this file in same change.
