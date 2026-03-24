<div align="center">

# DeltaScope

[![Release](https://img.shields.io/github/v/release/Fanduzi/DeltaScope?display_name=tag)](https://github.com/Fanduzi/DeltaScope/releases)
![Platform](https://img.shields.io/badge/platform-darwin%20%7C%20linux-blue)
![Go Version](https://img.shields.io/badge/go-1.26.1-00ADD8?logo=go)
[![License](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](LICENSE)

[English](README.md) | 中文 | [Changelog](CHANGELOG.md) | [Security](SECURITY.md) | [License](LICENSE) | [Release Notes](docs/releases/release-notes-v0.6.1.zh-CN.md)
</div>

DeltaScope 是一个面向 MySQL 和 TiDB 的离线优先 SQL 审核引擎。它给 DBA、应用工程师、CI 流水线和 AI agent 提供同一套 DDL / DML 审核入口，在 SQL 真正落库之前先把风险暴露出来。

## 安装

首选安装入口是仓库内的 installer script，它解析的就是 CI 发布时使用的同一套 release archive 合同。

```bash
curl -fsSL https://raw.githubusercontent.com/Fanduzi/DeltaScope/main/install.sh | sh
```

固定版本安装：

```bash
curl -fsSL https://raw.githubusercontent.com/Fanduzi/DeltaScope/v0.6.1/install.sh | \
  DELTASCOPE_VERSION=v0.6.1 sh
```

发布产物命名为 `deltascope_0.6.1_<os>_<arch>.tar.gz`。开发侧命令统一收敛在 [Dev docs](docs/dev/README.md)。

## 快速开始

审核高风险 DML：

```bash
deltascope audit --sql "delete from users"
```

示例输出：

```text
Verdict: reject
Statements: 1
Blockers: 1
Warnings: 0
Notices: 0

Statement 1: DELETE
- [blocker] dml.where.require: DELETE or UPDATE must include a WHERE clause
```

审核 `CREATE TABLE`：

```bash
deltascope audit --sql "create table users (id bigint unsigned not null auto_increment, primary key (id), name varchar(255) not null comment 'user name') comment='user table'"
```

示例输出：

```text
Verdict: review
Statements: 1
Blockers: 0
Warnings: 1
Notices: 0

Statement 1: CREATE TABLE
- [warning] ddl.column.comment.require: column `id` must have a comment
```

审核文件：

```bash
deltascope audit --file ./change.sql
```

给 CI 或 agent 使用 JSON 输出：

```bash
deltascope audit \
  --sql "alter table users drop column age" \
  --format json \
  --fail-on warning
```

JSON 结构示例：

```json
{
  "verdict": "review",
  "summary": {
    "blockers": 0,
    "warnings": 1,
    "notices": 0
  },
  "statements": [
    {
      "index": 1,
      "kind": "ALTER TABLE",
      "findings": [
        {
          "rule_id": "ddl.alter.drop_column.forbid",
          "level": "warning",
          "message": "dropping columns should be reviewed carefully"
        }
      ]
    }
  ]
}
```

对在线实例执行 metadata-aware 审核：

```bash
deltascope audit \
  --sql "alter table users add column email varchar(255)" \
  --host 127.0.0.1 --port 3306 --user root --ask-password --schema app
```

## 为什么是 DeltaScope

- 使用稳定的 `blocker`、`warning`、`notice` finding 模型审核 DDL 和 DML。
- 默认离线可用，适合本地开发、CI 和 agent 自动化。
- CLI、HTTP、library 共用同一套规则引擎，而不是三套分裂行为。
- 只有在确实需要实例事实或现有 schema 状态时，才启用 metadata-aware 增强。

## 关键能力

- `CREATE TABLE` 治理：标识符、注释、主键、审计列、字符集/排序规则、索引、表级 option。
- `ALTER TABLE` 治理：破坏性动作、兼容性检查、存在性验证、merge 提示。
- 对象生命周期检查：`CREATE VIEW`、`DROP TABLE`、`TRUNCATE TABLE`。
- DML 防护：`WHERE`、`LIMIT`、`ORDER BY`、子查询、join 条件、批量插入模式和 denylist。
- 稳定产品面：`deltascope` CLI、`deltascope-server`、`pkg/deltascope`。

## Recipes

- [离线审核 SQL](docs/recipe/audit-sql-offline.zh-CN.md)
- [连接数据库审核 SQL](docs/recipe/audit-sql-with-metadata.zh-CN.md)
- [迁移前审核 DDL](docs/recipe/review-ddl-before-migration.zh-CN.md)
- [在 CI 中拦截 DML](docs/recipe/guard-dml-in-ci.zh-CN.md)
- [与 AI Agent 集成](docs/recipe/use-with-ai-agents.zh-CN.md)
- [查看规则与配置](docs/recipe/inspect-rules-and-config.zh-CN.md)
- [排查 metadata-aware 审核问题](docs/recipe/troubleshoot-metadata-aware-audit.zh-CN.md)

## 文档导航

- [管理文档](docs/admin/README.md)
- [概念文档](docs/concept/README.md)
- [开发文档](docs/dev/README.md)
- [参考文档](docs/reference/README.md)
- [审核能力矩阵](docs/reference/audit-capability-matrix.zh-CN.md)

## 开发工作流

- `make test` 执行 `go test ./...`
- `make build` 在 `bin/` 下产出两个本地二进制
- `make build-linux` 在 `bin/` 下产出 Linux amd64 二进制
- `make test-e2e-cli` 执行基于 Docker 的 metadata CLI smoke
- [docs/dev/testing.md](docs/dev/testing.md) 汇总了完整目标集

## HTTP 服务

HTTP 适配层复用同一条审核主路径：

```bash
deltascope-server -listen 127.0.0.1:8083
```

接口：

- `GET /healthz`
- `GET /version`
- `POST /v1/audit`

## Library 用法

```go
result, err := deltascope.Audit(ctx, deltascope.Request{
    SQL:     "delete from users",
    Dialect: deltascope.DialectMySQL,
})
```

稳定公共 API 位于 [pkg/deltascope](pkg/deltascope/README.md)。

## Architecture

DeltaScope 保持一条共享审核主路径，再通过多个入口暴露给用户。产品层和实现层 ASCII 架构图分别位于 [docs/concept/architecture.md](docs/concept/architecture.md) 和 [docs/dev/architecture.md](docs/dev/architecture.md)。

### Modules

| Module | Description | Doc |
|--------|-------------|-----|
| `cmd/deltascope` | CLI 入口 | [README](cmd/deltascope/README.md) |
| `cmd/deltascope-server` | HTTP 服务入口 | [README](cmd/deltascope-server/README.md) |
| `internal/interfaces` | 传输适配层命名空间 | [README](internal/interfaces/README.md) |
| `internal/interfaces/cli` | CLI 适配层 | [README](internal/interfaces/cli/README.md) |
| `internal/interfaces/http` | HTTP 适配层 | [README](internal/interfaces/http/README.md) |
| `internal/application` | 用例编排层 | [README](internal/application/README.md) |
| `internal/application/audit` | 解析与审核编排 | [README](internal/application/audit/README.md) |
| `internal/application/policy` | 配置加载 | [README](internal/application/policy/README.md) |
| `internal/domain` | 核心领域对象与规则 | [README](internal/domain/README.md) |
| `internal/domain/spec` | 归一化 statement 模型 | [README](internal/domain/spec/README.md) |
| `internal/domain/rule` | rule/finding/severity 模型 | [README](internal/domain/rule/README.md) |
| `internal/domain/rule/catalog` | 面向解释和发现的内置规则目录 | [README](internal/domain/rule/catalog/README.md) |
| `internal/domain/rule/ddl` | DDL 规则目录 | [README](internal/domain/rule/ddl/README.md) |
| `internal/domain/rule/dml` | DML 规则目录 | [README](internal/domain/rule/dml/README.md) |
| `internal/domain/policy` | 策略配置模型 | [README](internal/domain/policy/README.md) |
| `internal/domain/report` | 审核结果与 verdict 聚合 | [README](internal/domain/report/README.md) |
| `internal/infrastructure` | 基础设施适配层 | [README](internal/infrastructure/README.md) |
| `internal/infrastructure/parser` | parser 适配命名空间 | [README](internal/infrastructure/parser/README.md) |
| `internal/infrastructure/parser/tidb` | TiDB parser 适配 | [README](internal/infrastructure/parser/tidb/README.md) |
| `internal/infrastructure/config/viper` | YAML 配置适配 | [README](internal/infrastructure/config/viper/README.md) |
| `internal/infrastructure/metadata/mysql` | MySQL/TiDB metadata provider | [README](internal/infrastructure/metadata/mysql/README.md) |
| `internal/infrastructure/output` | 输出渲染命名空间 | [README](internal/infrastructure/output/README.md) |
| `internal/infrastructure/output/markdown` | Markdown 渲染器 | [README](internal/infrastructure/output/markdown/README.md) |
| `internal/infrastructure/output/json` | JSON 渲染器 | [README](internal/infrastructure/output/json/README.md) |
| `configs` | 示例配置 | [README](configs/README.md) |
| `pkg/deltascope` | 稳定公共包入口 | [README](pkg/deltascope/README.md) |
