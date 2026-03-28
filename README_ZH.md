<div align="center">

# DeltaScope

[![Release](https://img.shields.io/github/v/release/Fanduzi/DeltaScope?display_name=tag)](https://github.com/Fanduzi/DeltaScope/releases)
![Platform](https://img.shields.io/badge/platform-darwin%20%7C%20linux-blue)
![Go Version](https://img.shields.io/badge/go-1.26.1-00ADD8?logo=go)
[![License](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](LICENSE)

[![English](https://img.shields.io/badge/docs-English-blue)](README.md) [![简体中文](https://img.shields.io/badge/docs-简体中文-yellow)](README_ZH.md)

[![变更记录](https://img.shields.io/badge/变更记录-informational)](CHANGELOG.md) [![安全策略](https://img.shields.io/badge/安全策略-important)](SECURITY.md) [![许可证](https://img.shields.io/badge/许可证-blue)](LICENSE) [![发行说明](https://img.shields.io/badge/发行说明-success)](docs/releases/release-notes-v0.9.0.zh-CN.md)
</div>

DeltaScope 是一个面向 MySQL 和 TiDB 的离线优先 SQL 审核引擎。它给 DBA、应用工程师、CI 流水线和 AI agent 提供同一套 DDL / DML 审核入口，在 SQL 真正落库之前先把风险暴露出来。

## 安装

首选安装入口是仓库内的 installer script，它解析的就是 CI 发布时使用的同一套 release archive 合同。

```bash
curl -fsSL https://raw.githubusercontent.com/Fanduzi/DeltaScope/main/install.sh | sh
```

macOS 用户也可以通过 Homebrew 安装：

```bash
brew tap Fanduzi/deltascope
brew install --cask deltascope
```

固定版本安装：

```bash
curl -fsSL https://raw.githubusercontent.com/Fanduzi/DeltaScope/v0.9.0/install.sh | \
  DELTASCOPE_VERSION=v0.9.0 sh
```

发布产物命名为 `deltascope_0.9.0_<os>_<arch>.tar.gz`。installer 默认安装 `deltascope`、`deltascope-server` 和 `deltascope-mcp`。开发侧命令统一收敛在 [Dev docs](docs/dev/README.md)。

## 快速开始

审核高风险 DML：

```bash
deltascope audit --sql "delete from users"
```

审核 SQL 文件：

```bash
deltascope audit --file ./migrations/20260328_add_column.sql
```

查看所有内置规则：

```bash
deltascope rules
```

## 为什么是 DeltaScope

SQL 错误在落库前发现代价极低，落库后代价极高。DeltaScope 在本地开发、CI、HTTP 服务和 MCP 四个环节提供同一套审核引擎，同一套策略全局生效——无需在各工具间重复配置规则，无需担心方言差异。

## 关键能力

- 建表治理：标识符、注释、主键、审计列、字符集/排序规则、索引、表选项。
- 改表治理：破坏性操作、兼容性检查、存在性校验、合并建议。
- 对象生命周期检查：`CREATE VIEW`、`DROP TABLE`、`TRUNCATE TABLE`。
- DML 保护：`WHERE`、`LIMIT`、`ORDER BY`、子查询、JOIN 条件、批量写入模式、黑名单对象。
- 稳定产品接口：`deltascope` CLI、`deltascope-server`、`deltascope-mcp`、`pkg/deltascope`。
- `deltascope-mcp` 是官方 MCP stdio 服务，暴露 `audit_sql`、`describe_rule`、`list_rules`、`get_capabilities`。

## MCP 快速接入

launcher 的前提：

- Node.js 24 或更高版本
- 当前原生目标只支持 `darwin` 或 `linux`，以及 `amd64` 或 `arm64`

推荐 launcher：

```bash
claude mcp add --scope user deltascope -- npx -y @fanduzi/deltascope-mcp
codex mcp add deltascope -- npx -y @fanduzi/deltascope-mcp
```

如果你需要通用 stdio TOML、原生 `deltascope-mcp`、direct connection、`connection_ref`、代理配置和常见错误说明，请看 [使用 DeltaScope MCP](docs/recipe/use-deltascope-mcp.zh-CN.md)。

## Claude Code Skill

DeltaScope 提供 Claude Code Skill，可在 AI 编码会话中直接审核 SQL。

```bash
# 安装 Skill（支持 Claude Code、Codex、Cursor 等 40+ AI 编码工具）
npx skills add Fanduzi/DeltaScope --skill deltascope-review -a claude-code
```

在 Claude Code 会话中调用：

```
/deltascope-review
```

粘贴 SQL 片段或指定文件路径，Claude 会用 DeltaScope 审核并给出修复建议。详见 [skills/README.md](skills/README.md)。

## 更多文档

- [Recipes](docs/recipe/README.md)
- [Dev docs](docs/dev/README.md)
- [Reference docs](docs/reference/README.md)
- [使用元数据审核 SQL](docs/recipe/audit-sql-with-metadata.zh-CN.md)
- [迁移前审查 DDL](docs/recipe/review-ddl-before-migration.zh-CN.md)
- [在 CI 中防护 DML](docs/recipe/guard-dml-in-ci.zh-CN.md)
- [使用 DeltaScope MCP](docs/recipe/use-deltascope-mcp.zh-CN.md)

## Recipes

常见使用场景：

审核文件：

```bash
deltascope audit --file ./change.sql
```

给 CI 或 agent 使用 JSON 输出：

```bash
deltascope audit \
  --sql "delete from users" \
  --format json \
  --fail-on warning
```

## 文档导航

完整文档见 [docs/](docs/)。

- [Concept docs](docs/concept/README.md) — 架构与设计决策
- [Dev docs](docs/dev/README.md) — 构建、测试、发布指南
- [Reference docs](docs/reference/README.md) — 规则参考、配置 schema
- [Recipe docs](docs/recipe/README.md) — 典型场景的端到端操作指南
- [Release notes](docs/releases/README.md) — 版本发行说明

## 开发工作流

```bash
make build      # 构建所有二进制到 bin/
make test       # 单元测试（无 Docker）
make test-e2e-cli  # 端到端测试（需要 Docker）
```

详情见 [Dev docs](docs/dev/README.md)。

## HTTP 服务

```bash
deltascope-server --port 8080
```

`POST /audit` 接受与 CLI `--sql` / `--file` 相同的参数，返回相同的 JSON 结构。详情见 [HTTP service docs](docs/reference/http-api.md)。

## Library 用法

```go
import "github.com/Fanduzi/DeltaScope/pkg/deltascope"

result, err := deltascope.Audit(ctx, deltascope.Request{
    SQL:     "delete from users",
    Dialect: "mysql",
})
```

详情见 [pkg/deltascope README](pkg/deltascope/README.md)。

## Architecture

高层次架构图和实现级图表见 [docs/concept/architecture.md](docs/concept/architecture.md) 和 [docs/dev/architecture.md](docs/dev/architecture.md)。

### Modules

| 模块 | 描述 | 文档 |
|------|------|------|
| `cmd/deltascope` | CLI 入口 | [README](cmd/deltascope/README.md) |
| `cmd/deltascope-server` | HTTP 服务入口 | [README](cmd/deltascope-server/README.md) |
| `cmd/deltascope-mcp` | MCP stdio 服务入口 | [README](cmd/deltascope-mcp/README.md) |
| `internal/interfaces/cli` | CLI 适配层 | [README](internal/interfaces/cli/README.md) |
| `internal/interfaces/http` | HTTP 适配层 | [README](internal/interfaces/http/README.md) |
| `internal/interfaces/mcp` | MCP 适配层 | [README](internal/interfaces/mcp/README.md) |
| `internal/application/audit` | 审核用例 | [README](internal/application/audit/README.md) |
| `internal/application/auditmeta` | 元数据感知审核用例 | [README](internal/application/auditmeta/README.md) |
| `internal/domain/rule` | rule/finding/severity 模型 | [README](internal/domain/rule/README.md) |
| `internal/domain/rule/catalog` | 面向解释和发现的内置规则目录 | [README](internal/domain/rule/catalog/README.md) |
| `internal/domain/rule/ddl` | DDL 规则目录 | [README](internal/domain/rule/ddl/README.md) |
| `internal/domain/rule/dml` | DML 规则目录 | [README](internal/domain/rule/dml/README.md) |
| `internal/domain/policy` | 策略配置模型 | [README](internal/domain/policy/README.md) |
| `internal/domain/report` | 审核结果与 verdict 聚合 | [README](internal/domain/report/README.md) |
| `internal/infrastructure` | 基础设施适配层 | [README](internal/infrastructure/README.md) |
| `internal/infrastructure/parser/tidb` | TiDB parser 适配 | [README](internal/infrastructure/parser/tidb/README.md) |
| `internal/infrastructure/config/viper` | YAML 配置适配 | [README](internal/infrastructure/config/viper/README.md) |
| `internal/infrastructure/metadata/mysql` | MySQL/TiDB metadata provider | [README](internal/infrastructure/metadata/mysql/README.md) |
| `internal/infrastructure/output/markdown` | Markdown 渲染器 | [README](internal/infrastructure/output/markdown/README.md) |
| `internal/infrastructure/output/json` | JSON 渲染器 | [README](internal/infrastructure/output/json/README.md) |
| `configs` | 示例配置 | [README](configs/README.md) |
| `pkg/deltascope` | 稳定公共包入口 | [README](pkg/deltascope/README.md) |
