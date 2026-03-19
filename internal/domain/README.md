# Domain Module

Core domain boundary for audit concepts, statement models, policy, rules, and report aggregation.

## Files

| File | Responsibility |
|------|---------------|
| doc.go | Declares the domain package placeholder |

## Exports

- Package boundary only; concrete exports live in child domain modules

## Dependencies
- Upstream: `internal/application`
- Downstream: `internal/domain/spec`, `internal/domain/rule`, `internal/domain/policy`, `internal/domain/report`

## Update Rule
- If members/interfaces/dependencies change, update this file in same change.
