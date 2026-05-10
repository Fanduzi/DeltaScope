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
| runtimeconfig/ | Loads non-policy process settings (logging, metadata timeout) from YAML |

## Exports

- `logger.NewLogger` — creates `*slog.Logger` for server/MCP surfaces (default: stderr, JSON, info)
- `logger.NewStdLogger` — bridges `*slog.Logger` to `*log.Logger` for legacy middleware
- `logger.Config` — level, format, output, file path, and optional rotation configuration
- `logger.RotateConfig` — rotation settings: enabled, max size, max backups, max age, compress (defaults: 100MB, 3, 30 days, true)
- `runtimeconfig.Load` — reads runtime config YAML, returns Config (empty path returns zero config)
- `runtimeconfig.ParseConnectTimeout` — parses duration string for metadata connect timeout (empty/zero = unset)
- `runtimeconfig.Config` — holds logging and metadata runtime settings
- `runtimeconfig.LoggingConfig` — level, format, output, file, and rotation for runtime logging
- `runtimeconfig.RotateConfig` — rotation settings (pointer fields distinguish unset from explicit zero)
- `runtimeconfig.MetadataConfig` — metadata connection settings (connect_timeout duration string)

## Dependencies
- Upstream: `internal/application`
- Downstream: `internal/infrastructure/config/viper`, `internal/infrastructure/metadata`, `internal/infrastructure/output`, `internal/infrastructure/parser`, external libraries and runtimes

## Update Rule
- If members/interfaces/dependencies change, update this file in same change.
