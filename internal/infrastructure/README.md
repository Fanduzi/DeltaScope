# Infrastructure Module

Infrastructure adapters for parser, config loading, metadata loading, and output rendering.

## Files

| File | Responsibility |
|------|---------------|
| doc.go | Declares the infrastructure package placeholder |
| config/viper/loader.go | Loads YAML policy overrides through Viper |
| logger/logger.go | Constructs `*slog.Logger` instances for server and MCP surfaces |
| metadata/ | Holds optional metadata-provider adapters |
| output/ | Holds result rendering adapters |
| parser/ | Holds parser adapter modules |

## Exports

- `logger.NewLogger` — creates `*slog.Logger` for server/MCP surfaces (default: stderr, JSON, info)
- `logger.NewStdLogger` — bridges `*slog.Logger` to `*log.Logger` for legacy middleware
- `logger.Config` — level, format, output, file path, and optional rotation configuration
- `logger.RotateConfig` — rotation settings: enabled, max size, max backups, max age, compress (defaults: 100MB, 3, 30 days, true)

## Dependencies
- Upstream: `internal/application`
- Downstream: `internal/infrastructure/config/viper`, `internal/infrastructure/metadata`, `internal/infrastructure/output`, `internal/infrastructure/parser`, external libraries and runtimes

## Update Rule
- If members/interfaces/dependencies change, update this file in same change.
