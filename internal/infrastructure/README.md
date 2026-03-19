# Infrastructure Module

Infrastructure adapters for parser, config loading, and output rendering.

## Files

| File | Responsibility |
|------|---------------|
| doc.go | Declares the infrastructure package placeholder |
| config/viper/loader.go | Loads YAML policy overrides through Viper |
| parser/ | Holds parser adapter modules |

## Exports

- No exported API yet

## Dependencies
- Upstream: `internal/application`
- Downstream: `internal/infrastructure/config/viper`, `internal/infrastructure/parser`, external libraries and runtimes

## Update Rule
- If members/interfaces/dependencies change, update this file in same change.
