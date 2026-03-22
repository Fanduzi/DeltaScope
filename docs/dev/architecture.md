# Implementation Architecture

The repository keeps transport concerns at the edges, application orchestration in the middle, domain logic isolated from adapters, and infrastructure packages responsible for parser/config/output/metadata bindings.

## Layering

ASCII diagram:

```text
cmd/
  deltascope
  deltascope-server
        |
        v
internal/interfaces/
  cli            http
        |
        v
internal/application/
  audit          policy
        |
        v
internal/domain/
  spec           policy
  rule           report
        |
        v
internal/infrastructure/
  parser/tidb
  config/viper
  metadata/mysql
  output/json
  output/markdown
        |
        v
pkg/deltascope
  stable public library facade
```

## Practical Boundaries

- `cmd` packages stay thin and mostly bind process flags or command startup.
- `internal/interfaces` owns CLI and HTTP request/response translation.
- `internal/application` coordinates parse, metadata enrichment, config loading, and rule execution.
- `internal/domain` owns normalized SQL specs, rule semantics, and final reports.
- `internal/infrastructure` adapts external dependencies without redefining product rules.
- `pkg/deltascope` exposes the stable public API on top of the same audit path.
