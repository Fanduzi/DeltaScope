# Infrastructure Module

Infrastructure adapters for parser, config loading, metadata loading, and output rendering.

## Files

| File | Responsibility |
|------|---------------|
| doc.go | Declares the infrastructure package placeholder |
| config/viper/loader.go | Loads YAML policy overrides through Viper |
| metadata/ | Holds optional metadata-provider adapters |
| output/ | Holds result rendering adapters |
| parser/ | Holds parser adapter modules |

## Exports

- No exported API yet

## Dependencies
- Upstream: `internal/application`
- Downstream: `internal/infrastructure/config/viper`, `internal/infrastructure/metadata`, `internal/infrastructure/output`, `internal/infrastructure/parser`, external libraries and runtimes

## Update Rule
- If members/interfaces/dependencies change, update this file in same change.
