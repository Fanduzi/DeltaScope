# Application Module

Application services orchestrate use cases between interfaces, domain logic, and infrastructure.

## Files

| File | Responsibility |
|------|---------------|
| doc.go | Declares the application package placeholder |
| audit/ | Holds SQL parsing and the main offline audit orchestration use case |
| auditmeta/ | Shares metadata-aware audit preparation between interface adapters |
| configstatus/ | Derives single-rule effective status under default policy plus optional YAML config |
| policy/load.go | Loads effective audit policy for application use cases |

## Exports

- Package boundary only; concrete exports live in child application modules

## Dependencies
- Upstream: `internal/interfaces/*`, `pkg/deltascope`
- Downstream: `internal/application/audit`, `internal/application/policy`, `internal/domain`, `internal/infrastructure`

## Update Rule
- If members/interfaces/dependencies change, update this file in same change.
