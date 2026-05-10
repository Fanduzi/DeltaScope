# DeltaScope v0.63.0 Release Notes

## Summary

v0.63.0 adds runtime configuration for server and MCP operations, public metadata connect timeout inputs across CLI, HTTP, and MCP surfaces, and adoption documentation. Runtime config is a separate YAML layer from policy config (`--config`), designed for operational settings like logging and metadata defaults. No new SQL audit rules, parser features, or rule behavior changes.

## New Features

| Feature | Description |
|---------|-------------|
| Runtime config loader | Server and MCP accept `-runtime-config` flag to load a YAML file with logging and metadata defaults |
| Runtime config logging | Configure `level`, `format`, `output`, `file`, and `rotation` (max size, max age, max backups, compress) |
| Runtime config metadata | Set `metadata.connect_timeout` default for server and MCP metadata connections |
| CLI metadata connect timeout | `--metadata-connect-timeout` flag for explicit per-request metadata connection timeout |
| HTTP metadata connect timeout | `connection.connect_timeout` field in the JSON request body |
| MCP metadata connect timeout | `connect_timeout` on direct and named connection inputs |
| Adoption docs | Runtime config examples, CLI/HTTP/MCP metadata timeout examples, SQL audit MCP server and TiDB schema change audit pages |

## Runtime Config

The runtime config YAML is loaded via `-runtime-config` and applies to `deltascope-server` and `deltascope-mcp`. It is separate from policy config (`--config`).

```yaml
logging:
  level: info
  format: json
  output: file
  file: /var/log/deltascope/server.log
  rotate:
    enabled: true
    max_size_mb: 100
    max_backups: 5
    max_age_days: 30
    compress: true

metadata:
  connect_timeout: 10s
```

### MCP stdout logging restriction

MCP stdout logging is forbidden to protect the stdio protocol. Runtime config can set `output: file` or `output: stderr`, but not `stdout`.

## Metadata Connect Timeout

### Precedence

```
request-level connect_timeout
  > runtime metadata.connect_timeout
    > opener internal default
```

- CLI: `--metadata-connect-timeout 5s`
- HTTP: `"connection": {"connect_timeout": "5s"}`
- MCP direct: `"connect_timeout": "5s"` in the tool input
- MCP named connection: `connect_timeout: 5s` in the connections YAML

Empty string or `0s` means unset (falls through to the next precedence level). MySQL, TiDB, and PostgreSQL all support metadata connect timeout.

## Quality

- Public surface coverage verified across CLI, HTTP, MCP, and SDK surfaces
- Full E2E matrix: CLI/HTTP/MCP x MySQL/TiDB/PostgreSQL
- SQL corpus 400/400 targets and fixture execution verified

## Documentation

- Runtime config example for server and MCP
- CLI, HTTP, and MCP metadata connect timeout examples
- SQL audit MCP server adoption page
- TiDB schema change audit adoption page

## Non-Goals

- No new SQL rules in v0.63.0.
- No SQL rule severity or default policy changes.
- Parser cache remains deferred.
- Runtime config currently applies to `deltascope-server` and `deltascope-mcp`, not ordinary CLI logging.
- SDK `deltascope.Request` does NOT have `MetadataConnectTimeout`; SDK callers that pass their own `MetadataProvider` manage connection behavior themselves.
- DeltaScope does not execute migrations and is not a database proxy or runtime query firewall.
- No live privilege/role validation expansion.

## Install

**macOS (recommended):**

```bash
brew tap fanduzi/deltascope
brew install --cask deltascope
```

**Generic installer:**

```bash
curl -fsSL https://raw.githubusercontent.com/Fanduzi/DeltaScope/v0.63.0/install.sh | \
  DELTASCOPE_VERSION=v0.63.0 sh
```

## Upgrade

If you previously installed v0.62.0:

```bash
# Homebrew
brew upgrade --cask deltascope

# Generic installer (re-run with new version)
curl -fsSL https://raw.githubusercontent.com/Fanduzi/DeltaScope/v0.63.0/install.sh | \
  DELTASCOPE_VERSION=v0.63.0 sh
```

## Verification

```bash
deltascope --version
# Should output v0.63.0

deltascope audit --sql "delete from users" --metadata-connect-timeout 5s
# Should work normally with the new timeout flag
```
