# DeltaScope v0.13.0 发布说明

## 概览

DeltaScope `v0.13.0` 让 HTTP surface 也具备 metadata-aware 能力。这个版本保持现有的 offline-first 审计契约，同时为 `POST /v1/audit` 增加 direct MySQL / TiDB 元数据准备流程，让部署后的 HTTP 服务也能返回和 CLI、MCP 一致的 live schema 上下文审计结果。

## 更新内容

### HTTP Metadata-Aware Audit

HTTP adapter 现在支持在 `POST /v1/audit` 中传入 direct `connection`，先准备实时元数据，再执行 SQL 审计。返回结构继续保持稳定的 verdict / findings 契约，并额外补充以下 additive context 字段：

- 解析后的 `dialect`
- `dialect_source`
- 解析后的 `schema`
- `schema_source`
- `metadata_source`

这样 HTTP client 也能获得和 CLI、MCP 一样的 metadata-aware 审计形态。

### 共享 Direct Connection Helper

direct connection 的校验和密码解析现在统一收敛到共享接口层 helper：

- HTTP 和 MCP 复用同一套 host / socket 校验规则
- `password`、`password_env` 和 `password_file` 在两个 adapter 上遵循同一套互斥契约
- direct credential lookup 失败现在统一映射到稳定的 HTTP 错误码 `connection_invalid`

### 面向 Real Server 的 HTTP 端到端覆盖

HTTP release surface 现在补上了 Docker 驱动的端到端覆盖：测试会先构建并启动真实 `deltascope-server` 二进制，再对以下目标发起 metadata-aware 请求：

- MySQL fixture
- TiDB fixture

这补齐了文档化 HTTP 契约和实际部署路径之间的验证闭环。

## 安装 / 升级

**macOS（推荐）：**

```bash
brew tap Fanduzi/deltascope
brew install --cask deltascope
```

或升级：

```bash
brew upgrade --cask deltascope
```

**Linux / 其他环境：**

```bash
curl -fsSL https://raw.githubusercontent.com/Fanduzi/DeltaScope/v0.13.0/install.sh | \
  DELTASCOPE_VERSION=v0.13.0 sh
```

## 兼容性

没有破坏性变更。`v0.13.0` 只是在 HTTP adapter 上增加了 additive 的 metadata-aware 输入和响应上下文字段，同时保持 CLI、HTTP、MCP 和 Go library 的稳定契约。
