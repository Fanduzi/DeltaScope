# Public Package Module

Stable public package surface for library consumers.

## Files

| File | Responsibility |
|------|---------------|
| doc.go | Declares the public package placeholder |
| audit.go | Exposes the stable public audit API and public result/request types |
| version.go | Publishes the default semantic version and canonical ASCII logo |
| audit_test.go | Verifies the public audit API with defaults, overrides, and multi-statement input |

## Exports

- `Audit(ctx, request)`
- `Request`
- `Result`
- `StatementResult`
- `Finding`
- `Level`
  Public finding severity type for `blocker`, `warning`, and `notice`
- `Summary`
- `Location`
- `Dialect`
- `Verdict`
- `DefaultVersion`
- `Logo`

## Dependencies
- Upstream: external library consumers
- Downstream: `internal/application/audit`, `internal/domain/report`, `internal/domain/rule`, `internal/domain/spec`

## Update Rule
- If members/interfaces/dependencies change, update this file in same change.
