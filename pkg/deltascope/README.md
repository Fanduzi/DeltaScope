# Public Package Module

Stable public package surface for library consumers.

## Files

| File | Responsibility |
|------|---------------|
| doc.go | Declares the public package placeholder |
| audit.go | Exposes the stable public audit API, optional metadata-provider hooks, and public result/request types |
| version.go | Publishes the default semantic version and canonical ASCII logo |
| audit_test.go | Verifies the public audit API with defaults, overrides, multi-statement input, PostgreSQL request routing, and metadata-aware request plumbing |

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
  Public finding severity type for `blocker`, `warning`, and `notice`
- `Summary`
- `Location`
- `Dialect`
  Includes `DialectPostgreSQL` for PostgreSQL request routing support
- `Verdict`
- `DefaultVersion`
- `Logo`

## Notes

- `Request` now carries top-level `Schema` and `MetadataProvider` fields so CLI, HTTP, and library consumers can opt into metadata-aware audits without changing the offline call shape.
- `Result` and `StatementResult` expose an optional `Explanation` field for additive shared result context without changing verdict semantics. The built-in audit flow populates these aggregate fields whenever findings are present.
- `Result` now also exposes an `Unsupported` array so library consumers can inspect structured partial-support PostgreSQL outcomes.
- `ErrUnsupportedStatement` is returned when unsupported statements are present, while still returning a populated `Result` for supported statements.
- `Finding` now exposes an optional `Explanation` field so library consumers can read structured per-finding `why`, `risk`, `suggestion`, and metadata-status notes directly.
- `DefaultVersion` is now `v0.15.0`, which matches the source-build baseline used by the CLI, HTTP server, official MCP server release artifacts, and the dedicated PostgreSQL CLI release path.
- The public package surface stays stable in `v0.15.0`; this release adds PostgreSQL request routing and release-surface alignment while keeping the exported `pkg/deltascope` contract additive.

## Dependencies
- Upstream: external library consumers
- Downstream: `context`, `internal/application/audit`, `internal/domain/report`, `internal/domain/rule`, `internal/domain/spec`

## Update Rule
- If members/interfaces/dependencies change, update this file in same change.
