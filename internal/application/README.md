# Application Module

Application services orchestrate use cases between interfaces, domain logic, and infrastructure.

## Files

| File | Responsibility |
|------|---------------|
| doc.go | Declares the application package placeholder |
| audit/ | Holds SQL parsing and the main offline audit orchestration use case |
| auditmeta/ | Shares metadata-aware audit preparation between interface adapters |
| configstatus/ | Derives single-rule effective status under default policy plus optional YAML config |
| configlint/ | Derives deterministic rule-level replacement warnings for a YAML config file |
| policy/load.go | Loads effective audit policy for application use cases |
| sql_input.go | Removes one leading UTF-8 BOM at the shared SQL-input boundary |
| queryaccess/ | Defines application contracts for query access analysis |

## Exports

- Shared input normalization; concrete use-case exports live in child application modules
- `NormalizeSQLInput()` removes exactly one leading UTF-8 BOM before use-case validation and parsing

## Dependencies
- Upstream: `internal/interfaces/*`, `pkg/deltascope`
- Downstream: `internal/application/audit`, `internal/application/policy`, `internal/application/queryaccess`, `internal/domain`, `internal/infrastructure`

## Update Rule
- If members/interfaces/dependencies change, update this file in same change.
