# Public Package Module

Stable public package surface for library consumers.

## Files

| File | Responsibility |
|------|---------------|
| doc.go | Declares the public package placeholder |
| audit.go | Exposes the stable public audit API, optional metadata-provider hooks, and public result/request types |
| query_access.go | Exposes the stable public query access analysis API, schema resolver interface, and public result/request types |
| version.go | Publishes the default semantic version and canonical ASCII logo |
| audit_test.go | Verifies the public audit API with defaults, overrides, multi-statement input, PostgreSQL request routing, and metadata-aware request plumbing |
| query_access_test.go | Verifies the public query access API with dialect routing, mode handling, JSON structure parity, and context cancellation |

## Exports

- `Audit(ctx, request)`
- `Request`
- `MetadataProvider`
- `Metadata`
- `InstanceFacts`
- `TableSnapshot`
- `Table`
- `Column`
- `Index`
- `Constraint`
- `Result`
- `StatementResult`
- `Explanation`
- `Finding`
- `FindingExplanation`
- `ExplanationMetadata`
- `Level`
  Public finding level type for `blocker`, `warning`, and `notice`
- `Summary`
- `Location`
- `Dialect`
  Includes `DialectPostgreSQL` for PostgreSQL request routing support
- `Verdict`
- `DefaultVersion`
- `Logo`
- `AnalyzeQueryAccess(ctx, request)`
  Performs query access analysis and returns read classification, admission, and permission requirements
- `QueryAccessRequest`
  Input for query access analysis with SQL, dialect, mode, default schema, and optional schema resolver
- `QueryAccessResult`
  Output of query access analysis with structured JSON fields for dialect, mode, classification, admission, relations, columns, outputs, requirements, unresolved references, and warnings
- `QueryAccessMode`
  Controls which column references become requirements: `strict` or `projection_only`
- `QueryAccessReadClassification`
  Describes whether SQL is read-only: `read_only`, `not_read_only`, or `indeterminate`
- `QueryAccessAdmission`
  Describes whether SQL is eligible for authorization: `admissible`, `rejected`, or `indeterminate`
- `QueryAccessSchemaResolver`
  Optional interface for resolving relation metadata during analysis

## Notes

- `Request` now carries top-level `Schema` and `MetadataProvider` fields so CLI, HTTP, and library consumers can opt into metadata-aware audits without changing the offline call shape.
- Public `MetadataProvider` stays minimal; standalone PostgreSQL index-owner resolution remains an internal optional seam behind the application metadata enrichment layer.
- `Result` and `StatementResult` expose an optional `Explanation` field for additive shared result context without changing verdict semantics. The built-in audit flow populates these aggregate fields whenever findings are present.
- `Result` now also exposes `Unsupported` (`[]spec.UnsupportedDetail`) and `Diagnostics` (`[]spec.Diagnostic`) arrays so library consumers can inspect structured partial-support and parser-error/unsupported-statement outcomes.
- `ErrUnsupportedStatement` is returned when unsupported statements are present, while still returning a populated `Result` for supported statements.
- `Finding` now exposes an optional `Explanation` field so library consumers can read structured per-finding `why`, `risk`, `suggestion`, and metadata-status notes directly.
- `DefaultVersion` is `v0.370.0`, matching the current repository release baseline for source builds.
- Release surface gates verify that `DefaultVersion` stays aligned with the release tag so source-built binaries do not drift behind published artifacts.

## Dependencies
- Upstream: external library consumers
- Downstream: `context`, `internal/application/audit`, `internal/application/queryaccess`, `internal/domain/queryaccess`, `internal/domain/report`, `internal/domain/rule`, `internal/domain/spec`

## Update Rule
- If members/interfaces/dependencies change, update this file in same change.
