# Public Package Module

Stable public package surface for library consumers.

## Files

| File | Responsibility |
|------|---------------|
| doc.go | Declares the public package placeholder |
| audit.go | Exposes the stable public audit API and public result/request types |
| audit_test.go | Verifies the public audit API with defaults, overrides, and multi-statement input |

## Exports

- `Audit(ctx, request)`
- `Request`
- `Result`
- `StatementResult`
- `Finding`
- `Summary`
- `Location`
- `Dialect`
- `Verdict`

## Dependencies
- Upstream: external library consumers
- Downstream: future application use cases

## Update Rule
- If members/interfaces/dependencies change, update this file in same change.
