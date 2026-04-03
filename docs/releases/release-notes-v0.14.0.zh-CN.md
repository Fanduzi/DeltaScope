# DeltaScope v0.14.0 发布说明

发布日期：2026-04-03

## 概览

DeltaScope `v0.14.0` 为 `UPDATE` 和 `DELETE` 新增了保守的 DML 影响范围估计能力。现在 CLI、HTTP、MCP 和公共 Go API 都可以为单条语句返回 `impact` 对象，包含预计影响行数、预计影响比例、风险等级、置信度、数据来源、原因码和说明备注。

## 更新内容

### DML 影响范围估计

DeltaScope 现在会在规则执行前附加 DML impact facts：

- `shape` 模式只基于 SQL 结构估计风险
- `metadata` 模式会结合只读表统计信息细化估计
- 语句结果现在可返回：
  - `estimated_rows`
  - `estimated_ratio`
  - `risk_level`
  - `confidence`
  - `source`
  - `reason_codes`
  - `notes`

### 新增 DML Impact 规则族

这个版本新增了以下 DML impact 规则：

- `dml.impact.estimate`
- `dml.impact.rows.max_count`
- `dml.impact.ratio.max_percent`

这让团队可以在不真正执行 DML 的前提下，基于保守的预计影响行数或影响比例，对高风险语句做策略拦截。

### 输出与文档更新

共享的 `impact` 契约现已在以下产品面统一可见：

- Markdown 输出
- CLI JSON
- HTTP 响应
- MCP 结构化结果
- `pkg/deltascope`

相关 reference docs、capability matrix 和模块 README 也已同步更新。

### 构建与发布链路加固

自 `v0.13.1` 之后，这个版本还吸收了几项发布链路改进：

- 本地 Go 二进制默认使用 `CGO_ENABLED=0`
- CI 会校验 Linux 二进制是否为静态链接
- 发布自动化升级到 `goreleaser-action@v7`
- 增加了手动 release smoke workflow

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
curl -fsSL https://raw.githubusercontent.com/Fanduzi/DeltaScope/v0.14.0/install.sh | \
  DELTASCOPE_VERSION=v0.14.0 sh
```

## 兼容性

没有破坏性变更。`v0.14.0` 是建立在 `v0.13.1` 之上的兼容性功能版本。
