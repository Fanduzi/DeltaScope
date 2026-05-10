# DeltaScope v0.63.0 发行说明

## 摘要

v0.63.0 新增 server/MCP runtime 配置、CLI/HTTP/MCP 公开的元数据连接超时输入，以及采纳文档。Runtime config 是独立于策略配置（`--config`）的 YAML 配置层，用于日志和元数据默认值等运维设置。无新增 SQL 审核规则、无解析器变更、无规则行为变更。

## 新特性

| 特性 | 说明 |
|------|------|
| Runtime 配置加载器 | server 和 MCP 接受 `-runtime-config` 参数，从 YAML 文件加载日志和元数据默认值 |
| Runtime 配置日志 | 可配置 `level`、`format`、`output`、`file` 及 `rotation`（最大体积、最大保留天数、最大备份数、压缩） |
| Runtime 配置元数据 | 设置 `metadata.connect_timeout` 作为 server/MCP 元数据连接默认超时 |
| CLI 元数据连接超时 | `--metadata-connect-timeout` 参数，指定单次请求的元数据连接超时 |
| HTTP 元数据连接超时 | JSON 请求体中的 `connection.connect_timeout` 字段 |
| MCP 元数据连接超时 | 直接连接和命名连接输入中的 `connect_timeout` 字段 |
| 采纳文档 | runtime 配置示例、CLI/HTTP/MCP 超时示例、SQL 审核 MCP 服务和 TiDB schema 变更审核页面 |

## Runtime 配置

Runtime config YAML 通过 `-runtime-config` 加载，适用于 `deltascope-server` 和 `deltascope-mcp`。它与策略配置（`--config`）独立。

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

### MCP stdout 日志限制

MCP 禁止 stdout 日志输出以保护 stdio 协议。Runtime config 可以设置 `output: file` 或 `output: stderr`，不能设置 `stdout`。

## 元数据连接超时

### 优先级

```
请求级 connect_timeout
  > runtime metadata.connect_timeout
    > opener 内部默认值
```

- CLI: `--metadata-connect-timeout 5s`
- HTTP: `"connection": {"connect_timeout": "5s"}`
- MCP 直接连接: 工具输入中的 `"connect_timeout": "5s"`
- MCP 命名连接: connections YAML 中的 `connect_timeout: 5s`

空字符串或 `0s` 表示未设置（向下传递到下一个优先级层级）。MySQL、TiDB 和 PostgreSQL 均支持 metadata connect timeout。

## 质量

- 公开接口覆盖已验证：CLI、HTTP、MCP 和 SDK
- 完整 E2E 矩阵：CLI/HTTP/MCP x MySQL/TiDB/PostgreSQL
- SQL 语料库 400/400 目标和 fixture 执行已验证

## 文档

- server 和 MCP runtime 配置示例
- CLI、HTTP 和 MCP 元数据连接超时示例
- SQL 审核 MCP 服务采纳页面
- TiDB schema 变更审核采纳页面

## 非目标

- v0.63.0 无新增 SQL 规则。
- 无 SQL 规则严重级别或默认策略变更。
- 解析器缓存仍延后。
- Runtime config 当前适用于 `deltascope-server` 和 `deltascope-mcp`，不适用于普通 CLI 日志。
- SDK `deltascope.Request` 没有 `MetadataConnectTimeout` 字段；传入自定义 `MetadataProvider` 的 SDK 调用者自行管理连接行为。
- DeltaScope 不执行迁移，不是数据库代理或运行时查询防火墙。
- 无实时权限/角色验证扩展。

## 安装

**macOS（推荐）：**

```bash
brew tap fanduzi/deltascope
brew install --cask deltascope
```

**通用安装脚本：**

```bash
curl -fsSL https://raw.githubusercontent.com/Fanduzi/DeltaScope/v0.63.0/install.sh | \
  DELTASCOPE_VERSION=v0.63.0 sh
```

## 升级

如果之前安装了 v0.62.0：

```bash
# Homebrew
brew upgrade --cask deltascope

# 通用安装脚本（使用新版本重新运行）
curl -fsSL https://raw.githubusercontent.com/Fanduzi/DeltaScope/v0.63.0/install.sh | \
  DELTASCOPE_VERSION=v0.63.0 sh
```

## 验证

```bash
deltascope --version
# 应输出 v0.63.0

deltascope audit --sql "delete from users" --metadata-connect-timeout 5s
# 新超时参数应正常工作
```
