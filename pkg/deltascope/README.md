# Public Package Module

Stable public package surface for library consumers.

## Files

| File | Responsibility |
|------|---------------|
| doc.go | Declares the public package placeholder |
| audit.go | Exposes the stable public audit API, optional metadata-provider hooks, and public result/request types |
| version.go | Publishes the default semantic version and canonical ASCII logo |
| audit_test.go | Verifies the public audit API with defaults, overrides, multi-statement input, and metadata-aware request plumbing |

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
- `Verdict`
- `DefaultVersion`
- `Logo`

## Notes

- `Request` now carries top-level `Schema` and `MetadataProvider` fields so CLI, HTTP, and library consumers can opt into metadata-aware audits without changing the offline call shape.
- `Result` and `StatementResult` expose an optional `Explanation` field for additive shared result context without changing verdict semantics. The built-in audit flow populates these aggregate fields whenever findings are present.
- `Finding` now exposes an optional `Explanation` field so library consumers can read structured per-finding `why`, `risk`, `suggestion`, and metadata-status notes directly.
- `DefaultVersion` is now `v0.7.0`, which matches the source-build baseline used by the CLI, HTTP server, and official MCP server release artifacts.
- The public package surface stays stable in `v0.7.0`; the release expands product entrypoints, not the exported `pkg/deltascope` contract.

## Dependencies
- Upstream: external library consumers
- Downstream: `context`, `internal/application/audit`, `internal/domain/report`, `internal/domain/rule`, `internal/domain/spec`

## Update Rule
- If members/interfaces/dependencies change, update this file in same change.
