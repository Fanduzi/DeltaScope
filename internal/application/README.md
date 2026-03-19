# Application Module

Application services orchestrate use cases between interfaces, domain logic, and infrastructure.

## Files

| File | Responsibility |
|------|---------------|
| doc.go | Declares the application package placeholder |
| policy/load.go | Loads effective audit policy for application use cases |

## Exports

- No exported API yet

## Dependencies
- Upstream: `internal/interfaces/*`
- Downstream: `internal/application/policy`, `internal/domain`, `internal/infrastructure`

## Update Rule
- If members/interfaces/dependencies change, update this file in same change.
