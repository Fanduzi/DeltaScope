# 使用 DeltaScope MCP

DeltaScope 提供 `deltascope-mcp` 作为规范的 MCP stdio server。对于首次接入，推荐 launcher 入口是 `npx -y @fanduzi/deltascope-mcp`。

如果你想要“复制即可用”的体验，用 launcher。若你需要固定本地二进制或自定义 `-connections-path`，则使用原生 binary。

## 前提

- npm launcher 需要 Node.js 20 或更高版本
- 当前原生目标只支持 `darwin` 或 `linux`，以及 `amd64` 或 `arm64`
- launcher 会先用 DeltaScope 官方 release checksums 校验 archive，再执行其中的二进制

## 服务暴露的工具

DeltaScope MCP 暴露四个官方工具：

- `audit_sql`
- `describe_rule`
- `list_rules`
- `get_capabilities`

## Claude Code

用推荐 launcher 添加服务：

```bash
claude mcp add --scope user deltascope -- npx -y @fanduzi/deltascope-mcp
```

## Codex

在 Codex 里也用同一个 launcher：

```bash
codex mcp add deltascope -- npx -y @fanduzi/deltascope-mcp
```

## 通用 Stdio TOML

如果你的 MCP 客户端需要原始 stdio 配置，可以把 `command` 指向 launcher：

```toml
[mcp_servers.deltascope]
command = "npx"
args = ["-y", "@fanduzi/deltascope-mcp"]
startup_timeout_sec = 20
```

有些客户端的 table 名不同，但 `command` 和 `args` 值应保持不变，只需映射到那个客户端自己的 stdio server 段落里。

## 原生 Binary

如果你已经安装了 release archive，或者用过仓库里的 installer，也可以直接运行原生二进制：

```toml
[mcp_servers.deltascope]
command = "deltascope-mcp"
args = []
startup_timeout_sec = 20
```

如果你需要自定义 `connections.yaml` 路径，可继续使用原生 binary，并传入已有 flag：

```toml
[mcp_servers.deltascope]
command = "deltascope-mcp"
args = ["-connections-path", "/path/to/connections.yaml"]
startup_timeout_sec = 20
```

## 代理配置

npm launcher 当前用 Node 的 `fetch()` 下载 DeltaScope release archive。在需要代理的网络里，请同时设置常规代理变量，并开启 Node 的环境代理支持：

```bash
export HTTP_PROXY=http://127.0.0.1:7891
export HTTPS_PROXY=http://127.0.0.1:7891
export NODE_USE_ENV_PROXY=1
```

如果 launcher 仍无法访问 GitHub，可先手动安装 release archive，再走原生 binary 路径。

如果你需要 release mirror，可以只覆盖 archive 下载基址，同时继续使用 GitHub 官方 checksums 做校验：

```bash
export DELTASCOPE_MCP_BASE_URL=https://mirror.example.com/deltascope/releases/download
```

## Direct Connection

当客户端能在请求里直接传 metadata，并且你不想保存 profile 时，使用 direct connection。

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

注意：

- `connection` 和 `connection_ref` 互斥。
- 如果同时提供 `dialect` 和 `connection.dialect`，以顶层 `dialect` 为准。
- direct connection 适合一次性审核，或者客户端本来就知道目标数据库详情的场景。

## `connection_ref`

当你希望使用可复用、具名的连接配置时，使用 `connection_ref`。

```json
{
  "sql": "delete from users where id = 1",
  "connection_ref": "prod_readonly"
}
```

默认情况下，`deltascope-mcp` 读取 `~/.config/deltascope/connections.yaml`。原生 `-connections-path` flag 可以覆盖该路径。

期望的文件结构：

```yaml
connections:
  prod_readonly:
    host: 10.0.0.12
    port: 3306
    user: audit_bot
    schema: app
    dialect: mysql
    password_env: PROD_DB_PASSWORD
```

适合 `connection_ref` 的场景：

- 多个客户端共享同一个保存好的 profile
- 不想把 host、user、schema、secret lookup 放进请求体
- 需要一个可在多次 agent run 间复用的稳定名字

## 常见错误

| Code | 含义 | 常见修复 |
| --- | --- | --- |
| `bad_request` | 请求结构无效，或缺少必填字段。 | 检查 tool input JSON 和字段名。 |
| `connection_invalid` | inline `connection` 或引用的 connection profile 无效。 | 检查 `host`、`user`、`socket`、`dialect` 和 profile 内容。 |
| `connection_failed` | DeltaScope 无法连到数据库。 | 检查网络、socket 路径、凭据和目标库。 |
| `config_invalid` | config file 或 `connections.yaml` 无法解析/加载。 | 检查文件路径和 YAML 语法。 |

如果你需要客户端视角的精确 contract，请先调用 `get_capabilities`。它会返回顶层输入字段、互斥规则，以及 server 公布的稳定错误码。
