# Use DeltaScope MCP

DeltaScope ships `deltascope-mcp` as the canonical MCP stdio server. For day-one onboarding, the recommended launcher is `npx -y @fanduzi/deltascope-mcp`.

Use the launcher when you want a copy-and-use setup. Use the native binary when you need a fixed local executable or a custom `-connections-path`.

## Requirements

- Node.js 24 or newer for the npm launcher
- supported native targets: `darwin` or `linux`, `amd64` or `arm64`
- launcher downloads are verified against the official DeltaScope release checksums before execution

## What The Server Exposes

DeltaScope MCP exposes four official tools:

- `audit_sql`
- `describe_rule`
- `list_rules`
- `get_capabilities`

## Discovery Files

These files list the existing stdio server. They do not change audit behavior.

- Repo-root [`.mcp.json`](../../.mcp.json) is the stdio launcher config (`npx -y @fanduzi/deltascope-mcp`) for catalogs that auto-detect a repo-root MCP config.
- [`server.json`](../../server.json) is official MCP Registry metadata for namespace `io.github.fanduzi/deltascope`. The npm ownership marker is `mcpName` in [`packages/deltascope-mcp/package.json`](../../packages/deltascope-mcp/package.json). Listing on the official registry still requires a separate `mcp-publisher` publish after that marker is present on the published npm package.

## Claude Code

Add the server with the recommended launcher:

```bash
claude mcp add --scope user deltascope -- npx -y @fanduzi/deltascope-mcp
```

## Codex

Add the same launcher through Codex:

```bash
codex mcp add deltascope -- npx -y @fanduzi/deltascope-mcp
```

## Generic Stdio TOML

If your MCP client wants raw stdio configuration, point `command` at the launcher:

```toml
[mcp_servers.deltascope]
command = "npx"
args = ["-y", "@fanduzi/deltascope-mcp"]
startup_timeout_sec = 20
```

Some clients use a different table name. Keep the same `command` and `args` values and map them into that client's stdio server section.

## Native Binary

If you already installed the release archive or used the repository installer, you can run the native binary directly:

```toml
[mcp_servers.deltascope]
command = "deltascope-mcp"
args = []
startup_timeout_sec = 20
```

If you need a custom `connections.yaml` path, use the native binary and pass the existing flag:

```toml
[mcp_servers.deltascope]
command = "deltascope-mcp"
args = ["-connections-path", "/path/to/connections.yaml"]
startup_timeout_sec = 20
```

## Proxy Setup

The npm launcher currently uses Node's `fetch()` to download the DeltaScope release archive. In proxy-restricted networks, set the normal proxy variables and enable Node's environment-proxy support:

```bash
export HTTP_PROXY=http://127.0.0.1:7891
export HTTPS_PROXY=http://127.0.0.1:7891
export NODE_USE_ENV_PROXY=1
```

If the launcher still cannot reach GitHub, retry with the native binary path after installing the release archive manually.

If you need a release mirror, you can override only the archive base URL while still verifying the archive against the official GitHub checksums:

```bash
export DELTASCOPE_MCP_BASE_URL=https://mirror.example.com/deltascope/releases/download
```

## Direct Connection

Use direct connection when the client can send metadata inline with the request and you do not want a saved profile.

```json
{
  "sql": "delete from users where id = 1",
  "connection": {
    "host": "127.0.0.1",
    "port": 3306,
    "user": "deltascope",
    "schema": "app",
    "dialect": "mysql",
    "password_env": "DELTASCOPE_PASSWORD"
  }
}
```

### PostgreSQL Direct Connection

```json
{
  "sql": "delete from users where id = 1",
  "connection": {
    "host": "127.0.0.1",
    "port": 5432,
    "user": "deltascope",
    "schema": "public",
    "dialect": "postgresql",
    "password_env": "DELTASCOPE_PASSWORD"
  }
}
```

Notes:

- `connection` and `connection_ref` are mutually exclusive.
- If both `dialect` and `connection.dialect` are present, the top-level `dialect` wins.
- Use direct connection for one-off audits or when the MCP client already knows the live target details.
- **PostgreSQL requires `dialect: "postgresql"`** — unlike MySQL/TiDB, the dialect is not auto-detected. Always include it explicitly in the connection block or as a top-level field.

## `connection_ref`

Use `connection_ref` when you want reusable named connection profiles.

```json
{
  "sql": "delete from users where id = 1",
  "connection_ref": "prod_readonly"
}
```

By default, `deltascope-mcp` reads `~/.config/deltascope/connections.yaml`. The native `-connections-path` flag overrides that path.

Expected file shape:

```yaml
connections:
  prod_readonly:
    host: 10.0.0.12
    port: 3306
    user: audit_bot
    schema: app
    dialect: mysql
    password_env: PROD_DB_PASSWORD
  pg_readonly:
    host: 10.0.0.20
    port: 5432
    user: audit_bot
    schema: public
    dialect: postgresql
    password_env: PG_DB_PASSWORD
```

Use `connection_ref` when:

- several clients should share the same saved profile
- you want to keep host, user, schema, and secret lookup out of the request body
- you need a stable name that can be reused across agent runs

## Common Errors

| Code | Meaning | Usual fix |
| --- | --- | --- |
| `bad_request` | The request shape is invalid or missing required fields. | Check the tool input JSON and field names. |
| `connection_invalid` | The inline `connection` block or referenced connection profile is invalid. | Check `host`, `user`, `socket`, `dialect`, and the profile contents. |
| `connection_failed` | DeltaScope could not reach the database. | Check network access, socket path, credentials, and the target database. |
| `config_invalid` | The config file or `connections.yaml` could not be parsed or loaded. | Check file path and YAML syntax. |

If you need the exact client-facing contract, use `get_capabilities` first. It returns the top-level inputs, mutual-exclusion rules, and the stable error codes the server advertises.
