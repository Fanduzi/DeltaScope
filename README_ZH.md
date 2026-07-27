<div align="center">

# DeltaScope

[![Release](https://img.shields.io/github/v/release/Fanduzi/DeltaScope?display_name=tag)](https://github.com/Fanduzi/DeltaScope/releases)
![Platform](https://img.shields.io/badge/platform-darwin%20%7C%20linux-blue)
![Go Version](https://img.shields.io/badge/go-1.26.1-00ADD8?logo=go)
[![License](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](LICENSE)

[![English](https://img.shields.io/badge/docs-English-blue)](README.md) [![简体中文](https://img.shields.io/badge/docs-简体中文-yellow)](README_ZH.md)

[![变更记录](https://img.shields.io/badge/变更记录-informational)](CHANGELOG.md) [![安全策略](https://img.shields.io/badge/安全策略-important)](SECURITY.md) [![许可证](https://img.shields.io/badge/许可证-blue)](LICENSE) [![发行说明](https://img.shields.io/badge/发行说明-success)](docs/releases/README.md)
</div>

DeltaScope 是一个离线优先的 SQL 审核和数据库变更风险检查工具，支持 MySQL、TiDB、PostgreSQL 的 DDL/DML 变更审核。主产品面已经统一为 `deltascope`、`deltascope-server` 和 `deltascope-mcp`；PostgreSQL offline 能力已经直接收敛到受支持的 macOS 和 Linux 主 archive 上。它给 DBA、应用工程师、CI 流水线和 AI agent 提供同一套 DDL / DML 审核入口，在 SQL 真正落库之前先把风险暴露出来。

**常用搜索入口：**
- [MySQL DDL审核工具](https://deltascope.pages.dev/zh/mysql-ddl-audit-tool) — 上线前检查表结构变更风险
- [PostgreSQL / PGSQL DDL审核工具](https://deltascope.pages.dev/zh/postgresql-ddl-audit-tool) — PG 表结构变更和权限分配审核
- [SQL上线审核工具](https://deltascope.pages.dev/zh/sql-release-audit-tool) — 在 CI 中检查数据库变更风险

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
curl -fsSL https://raw.githubusercontent.com/Fanduzi/DeltaScope/v0.450.0/install.sh | \
  DELTASCOPE_VERSION=v0.450.0 sh
```

### 方言选择 & 发布产物

每个 tag 都会产出 archive `deltascope_<version>_<os>_<arch>.tar.gz`，包含 `deltascope`、`deltascope-server` 和 `deltascope-mcp`。所有 archive 均支持 MySQL、TiDB 和 PostgreSQL 离线审核，通过 `--dialect mysql|tidb|postgresql` 选择方言。installer script、Homebrew Cask 和 npm MCP launcher 都从 GitHub Release 解析对应平台的 archive。详见 [审核能力矩阵](docs/reference/audit-capability-matrix.zh-CN.md) 了解各方言覆盖面，[发行说明](docs/releases/README.md) 了解版本演进。

## 快速开始

审核高风险 DML：

```bash
deltascope audit --sql "delete from users"
```

示例摘录：

```text
Verdict: reject
Statements: 1
Blockers: 1
Warnings: 0
Notices: 0

Statement 1: DELETE
- [blocker] dml.where.require: UPDATE and DELETE statements must include a WHERE clause
```

审核 `CREATE TABLE` 语句：

```bash
deltascope audit --sql "create table tbl_users (id bigint unsigned not null auto_increment comment 'id', created_at datetime not null default current_timestamp comment 'created', updated_at datetime not null default current_timestamp on update current_timestamp comment 'updated', primary key (id)) comment='users' engine=InnoDB default charset=utf8mb4"
```

示例摘录：

```text
Verdict: review
Statements: 1
Blockers: 0
Warnings: 1
Notices: 0

Statement 1: CREATE TABLE
- [warning] ddl.column.default.require: column "id" should define a default value
```

审核 SQL 文件：

```bash
deltascope audit --file ./migrations/20260328_add_column.sql
```

在 CI 中使用 JSON 输出：

```bash
deltascope audit \
  --sql "create table tbl_users (id bigint unsigned not null auto_increment comment 'id', created_at datetime not null default current_timestamp comment 'created', updated_at datetime not null default current_timestamp on update current_timestamp comment 'updated', primary key (id)) comment='users' engine=InnoDB default charset=utf8mb4" \
  --format json \
  --fail-on warning
```

示例 JSON 结构：

```json
{
  "verdict": "review",
  "summary": {
    "statements": 1,
    "blockers": 0,
    "warnings": 1,
    "notices": 0
  },
  "statements": [ ... ],
  "context": {
    "mode": "offline",
    "dialect": "mysql",
    "dialect_source": "default"
  }
}
```

审核 TiDB 语句：

```bash
deltascope audit --dialect tidb --sql "alter table users add column email varchar(255) not null"
```

审核 PostgreSQL 带约束的 `CREATE TABLE`：

```bash
deltascope audit \
  --dialect postgresql \
  --sql "create table orders (id bigint primary key, user_id bigint references users(id), amount numeric not null check (amount >= 0))"
```

当 SQL 看起来像 PostgreSQL 但方言设为 MySQL 时，DeltaScope 会发出 advisory notice 而不是自动切换：

```bash
deltascope audit --sql "insert into users(id) values (1) returning id;"
```

显式指定 PostgreSQL 方言审核：

```bash
deltascope audit --dialect postgresql --sql "insert into users(id) values (1) returning id;"
```

生成 SARIF 报告用于 GitHub Code Scanning：

```bash
deltascope audit --file ./migrations.sql --format sarif > deltascope.sarif
```

使用 CI 原生输出（支持任意方言）：

```bash
deltascope audit --dialect postgresql --file ./migrations/20260409_add_index.sql --format github-actions
```

向 GitHub Actions 运行页面写入简短审核摘要，使用 `--format github-summary`：

```bash
deltascope audit --file ./migrations.sql --format github-summary --fail-on none >> "$GITHUB_STEP_SUMMARY"
```

完整的 GitHub Actions 工作流（结合 `config lint --strict` 门禁、`github-actions` 内联注解和 `github-summary` 任务摘要）见 [github-actions.yml](docs/examples/github-actions.yml)。各输出格式见 [cli.zh-CN.md](docs/reference/cli.zh-CN.md)。

在 GitLab CI 中使用 `--format gitlab-codequality` 并将 `gl-code-quality-report.json` 发布为 Code Quality 制品；详见 [use-deltascope-in-gitlab-ci.zh-CN.md](docs/recipe/use-deltascope-in-gitlab-ci.zh-CN.md)。

在 CI 中校验策略配置。干净的配置打印 `Config OK`；仅有替换风险告警的配置打印 `Config OK with warnings` 并以退出码 0 结束。加上 `--strict` 可在这些告警上失败：

```bash
deltascope config lint --file ./deltascope.yaml --strict
```

随后可用 `deltascope config status` 查看单条规则的生效 ON/OFF 状态。

## DML 影响估算

对于 `DELETE FROM users WHERE id = 42` 这类选择性较强的 DML，DeltaScope 可能会在该语句结果上附加一个 `impact` 对象。这个对象以保守估算为原则，包含 `estimated_rows`、`estimated_ratio`、`risk_level`、`confidence`、`source`、`reason_codes`，以及可选的 `notes`。

```json
{
  "raw_sql": "DELETE FROM users WHERE id = 42",
  "impact": {
    "estimated_rows": 1,
    "estimated_ratio": 0.0001,
    "risk_level": "low",
    "confidence": "high",
    "source": "metadata",
    "reason_codes": ["pk_equality"],
    "notes": ["refined with table statistics"]
  }
}
```

离线模式只使用 SQL 形状做估算。metadata-aware 模式可以基于只读表统计信息进一步收敛估算。DeltaScope 不会执行 DML，也不会运行 `EXPLAIN ANALYZE`。

metadata-aware 审核（需要数据库连接）：

```bash
deltascope audit \
  --sql "alter table orders add column status tinyint not null comment 'order status'" \
  --host 127.0.0.1 --port 3306 --user root --ask-password --schema app
```

带连接超时的元数据感知审核（MySQL）：

```bash
deltascope audit \
  --sql "alter table users add column email varchar(255)" \
  --dialect mysql \
  --host 127.0.0.1 --port 3306 --user root --ask-password --schema app \
  --metadata-connect-timeout 5s
```

带连接超时的 PostgreSQL 元数据感知审核：

```bash
deltascope audit \
  --sql "alter table orders add column status text not null" \
  --dialect postgresql \
  --host 127.0.0.1 --port 5432 --user root --ask-password --schema app \
  --metadata-connect-timeout 5s
```

通过 TLS 的元数据感知审核：

```bash
deltascope audit \
  --sql "alter table orders add column status text not null" \
  --dialect postgresql \
  --host pg.example.com --port 5432 --user root --ask-password \
  --tls-mode enabled --tls-ca-file /etc/ssl/certs/pg-ca.pem \
  --schema app --metadata-connect-timeout 5s
```

查看所有内置规则：

```bash
deltascope rules list
```

## 为什么是 DeltaScope

SQL 错误在落库前发现代价极低，落库后代价极高。DeltaScope 在本地开发、CI、HTTP 服务和 MCP 四个环节提供同一套审核引擎，同一套策略全局生效——无需在各工具间重复配置规则，无需担心方言差异。

## 关键能力

- 建表治理：标识符、注释、主键、审计列、字符集/排序规则、索引、表选项。
- 改表治理：破坏性操作、兼容性检查、存在性校验、合并建议。
- 对象生命周期检查：`CREATE VIEW`、`DROP TABLE`、`TRUNCATE TABLE`，以及 MySQL/TiDB/PostgreSQL 跨方言的 database/schema 生命周期 DDL。
- DML 保护：`WHERE`、`LIMIT`、`ORDER BY`、子查询、JOIN 条件、批量写入模式、黑名单对象，以及保守的影响行数估算。
- 稳定产品接口：`deltascope` CLI、`deltascope-server`、`deltascope-mcp`、`pkg/deltascope`。
- `deltascope-mcp` 是官方 MCP stdio 服务，暴露 `audit_sql`、`describe_rule`、`list_rules`、`get_capabilities`。
- CI 输出保留源文件路径和语句起始行号（支持 GitHub Actions、SARIF 和 GitLab Code Quality 格式）。

## MCP 快速接入

> **无需手动安装二进制。** npm launcher 会自动为当前平台下载并运行对应的 `deltascope-mcp` 二进制文件。

launcher 的前提：

- Node.js 24 或更高版本
- 当前原生目标只支持 `darwin` 或 `linux`，以及 `amd64` 或 `arm64`

推荐 launcher：

```bash
claude mcp add --scope user deltascope -- npx -y @fanduzi/deltascope-mcp
codex mcp add deltascope -- npx -y @fanduzi/deltascope-mcp
```

如果你需要通用 stdio TOML、原生 `deltascope-mcp`、direct connection、`connection_ref`、代理配置和常见错误说明，请看 [使用 DeltaScope MCP](docs/recipe/use-deltascope-mcp.zh-CN.md)。

### MCP 使用 runtime config

使用 runtime config 启动 `deltascope-mcp` 以设置日志和元数据默认值：

```bash
deltascope-mcp -runtime-config /etc/deltascope/runtime.yaml
```

MCP 禁止 stdout 日志输出以保护 stdio 协议。Runtime config 可以设置 `output: file` 或 `output: stderr`，不能设置 `stdout`。

### MCP 命名连接带 connect_timeout

```yaml
# ~/.config/deltascope/connections.yaml
connections:
  local_mysql:
    host: 127.0.0.1
    port: 3306
    user: root
    password_env: MYSQL_PASSWORD
    schema: app
    dialect: mysql
    connect_timeout: 5s
```

命名连接和直接连接输入都支持 `connect_timeout`。留空或 `0s` 使用 runtime config 默认值。MySQL、TiDB 和 PostgreSQL 都支持 metadata connect timeout。

## AI Agent Skill

> **支持 Claude Code、Codex、Cursor 及 40+ AI 编码工具。**
> 安装一次，在所有 AI 编码会话中获得内联 SQL 审核能力。

DeltaScope 提供通用 AI Agent Skill，可在 AI 编码会话中直接审核 SQL。Skill 会自动检测本地是否安装了 DeltaScope，调用它审核你的 SQL，并给出修复建议——无需离开 AI 编码会话。

```bash
# 通过 npx skills 安装（支持 Claude Code、Codex、Cursor 等 40+ AI 工具）
npx skills add Fanduzi/DeltaScope --skill deltascope-review -a claude-code
```

全局安装（所有项目均可使用）：

```bash
npx skills add Fanduzi/DeltaScope --skill deltascope-review -a claude-code -g
```

保持 Skill 更新：

```bash
npx skills update
```

在任意支持的 AI 会话中调用：

```
/deltascope-review
```

粘贴 SQL 片段或指定文件路径——AI 会用 DeltaScope 审核并给出修复建议。详见 [skills/README.md](skills/README.md)。

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
make build              # 构建所有二进制到 bin/
make test               # 单元测试（无 Docker）
make test-e2e-cli       # 端到端测试（需要 Docker）
make pg-unit-test-gates # PostgreSQL tag 单元测试 gate
make pg-e2e-gates       # PostgreSQL CLI / HTTP / MCP 端到端 gate
make pg-confidence-gates # PostgreSQL confidence 总入口
```

详情见 [Dev docs](docs/dev/README.md)。

## HTTP 服务

```bash
deltascope-server --port 8080
```

使用 runtime config 启动（日志和元数据默认值）：

```bash
deltascope-server --port 8080 -runtime-config /etc/deltascope/runtime.yaml
```

完整 runtime config 示例见 [docs/examples/runtime-config.yaml](docs/examples/runtime-config.yaml)。

`POST /v1/audit` 同时支持离线 JSON 审核请求和带 `connection_id` 的元数据感知请求，`connection_id` 引用服务端 runtime config 中定义的命名连接。HTTP 请求不能直接提交凭据。HTTP 响应会保留公开的审核结果主体，并额外返回 `context` 块。完整协议见 [HTTP API 参考](docs/reference/http-api.zh-CN.md)。

> CLI 保留直接连接标志（`--host`、`--port`、`--user`、`--password-env`、`--ask-password`、`--schema`、`--tls-mode`、`--tls-ca-file`）。`connection_id` 边界仅适用于 HTTP。MCP 没有 Query Access 工具，保留其独立的元数据审计连接模型。

### HTTP 元数据感知请求带 connect_timeout

```json
{
  "sql": "alter table users add column email varchar(255)",
  "dialect": "mysql",
  "connection_id": "local_mysql"
}
```

`connection_id` 引用服务端 runtime config 中的命名连接（见 [docs/examples/runtime-config.yaml](docs/examples/runtime-config.yaml)）。命名连接可定义 `connect_timeout`，使用 Go duration 字符串（`500ms`、`5s`、`1m`）。留空或 `0s` 使用 runtime config 默认值。无效或负值返回 `400` 错误。MySQL、TiDB 和 PostgreSQL 都支持 metadata connect timeout。

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
| `internal/domain/rule` | rule/finding/level 模型 | [README](internal/domain/rule/README.md) |
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
