# Domain Rule Module

Rule contracts, registration, and finding types for audit evaluation.

## Files

| File | Responsibility |
|------|---------------|
| rule.go | Defines finding severity and finding metadata |
| registry.go | Registers statement/global rules and evaluates them in deterministic order |

## Exports

- `Level`
- `Finding`
- `Location`
- `StatementRule`
- `GlobalRule`
- `Registry`
- `NewRegistry()`

## Dependencies
- Upstream: `internal/application/audit`, domain report aggregation, and future rule implementations
- Downstream: `internal/domain/spec`

## Update Rule
- If members/interfaces/dependencies change, update this file in same change.
