# DeltaScope v0.7.0 发布说明

## 概览

DeltaScope `v0.7.0` 将官方 MCP stdio 服务带入稳定产品面。这次发布保持现有离线优先审计引擎与 explainable result 模型不变，并通过结构化 MCP 合同把它们暴露给智能体客户端，同时补齐规则发现工具、metadata-aware 数据库访问，以及面向 MySQL / TiDB 的真实端到端验证。

## 亮点

- 正式的 `deltascope-mcp` stdio 服务
- 稳定 MCP 工具：`audit_sql`、`describe_rule`、`list_rules`、`get_capabilities`
- 结构化 MCP 工具错误与显式输出 schema
- direct connection 与 `connection_ref` 两条 metadata-aware MCP 路径
- 面向 MySQL 与 TiDB 的真实 MCP metadata e2e 覆盖

## 新增与改进

### 官方 MCP 服务

- DeltaScope 现在正式提供 `deltascope-mcp` 作为 MCP stdio 入口
- MCP 客户端可以通过它审计 SQL、查询内置规则、搜索规则目录，并通过 `get_capabilities` 获取服务合同摘要
- MCP 适配层保持轻量，复用与 CLI、HTTP 服务、公共 Go API 相同的审计主路径

### Metadata-aware MCP 支持

- `audit_sql` 同时支持内联 `connection` 和命名 `connection_ref`
- metadata-aware MCP 结果会返回显式 `context` 字段，包括 mode、dialect、schema、schema_source 和 metadata_source
- schema 推断提示与连接错误现在都会映射为稳定的机器可读 MCP 错误码

### 共享准备流程

- 共享的 metadata-aware 准备逻辑现在收敛在 `internal/application/auditmeta`
- CLI 与 MCP 共用同一套方言检测、schema 推断与连接建立流程
- CLI 自有语义，例如 `schema_source:"flag"`，仍被保留

### 发布与验证更新

- GoReleaser archive 现在包含 `deltascope-mcp`
- 默认 installer 现在会同时安装 `deltascope`、`deltascope-server` 与 `deltascope-mcp`
- Docker-backed MCP metadata e2e 已证明 direct 与 `connection_ref` 在真实 MySQL / TiDB 上都能跑通

## 安装 / 升级

安装最新版：

```bash
curl -fsSL https://raw.githubusercontent.com/Fanduzi/DeltaScope/main/install.sh | sh
```

安装当前版本：

```bash
curl -fsSL https://raw.githubusercontent.com/Fanduzi/DeltaScope/v0.7.0/install.sh | \
  DELTASCOPE_VERSION=v0.7.0 sh
```

发布 archive 命名保持为 `deltascope_0.7.0_<os>_<arch>.tar.gz`。

## 兼容性

- 支持操作系统：`darwin`、`linux`
- 支持架构：`amd64`、`arm64`
- 支持数据库方言：`MySQL`、`TiDB`

## 已知限制

- metadata-aware 联机检查仍依赖实时 schema 访问，离线模式下不可用
- MCP transport 当前仅支持 stdio
- 如果客户端不发送内联连接参数，`connection_ref` 仍要求本地存在 YAML 配置文件
