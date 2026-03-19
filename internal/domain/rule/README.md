# Domain Rule Module

Rule findings and severity types for audit evaluation.

## Files

| File | Responsibility |
|------|---------------|
| rule.go | Defines finding severity and finding metadata |

## Exports

- `Level`
- `Finding`
- `Location`

## Dependencies
- Upstream: domain report aggregation and future rule implementations
- Downstream: none inside the current domain model

## Update Rule
- If members/interfaces/dependencies change, update this file in same change.
