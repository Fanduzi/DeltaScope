# Application Policy Module

Application wrapper for loading effective audit policy.

## Files

| File | Responsibility |
|------|---------------|
| load.go | Delegates policy loading to infrastructure-backed config adapters |

## Exports

- `Load(path string)`

## Dependencies
- Upstream: future CLI and application audit use cases
- Downstream: `internal/domain/policy`, `internal/infrastructure/config/viper`

## Update Rule
- If members/interfaces/dependencies change, update this file in same change.
