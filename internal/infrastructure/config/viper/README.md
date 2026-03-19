# Viper Config Module

Viper-backed YAML configuration loading for DeltaScope policy.

## Files

| File | Responsibility |
|------|---------------|
| loader.go | Loads default policy and applies optional YAML overrides |
| loader_test.go | Verifies default loading and YAML override behavior |

## Exports

- `LoadPolicy(path string)`

## Dependencies
- Upstream: `internal/application/policy`
- Downstream: `internal/domain/policy`, `github.com/spf13/viper`, `github.com/go-viper/mapstructure/v2`

## Update Rule
- If members/interfaces/dependencies change, update this file in same change.
