<div align="center">

# DeltaScope

[![Release](https://img.shields.io/github/v/release/Fanduzi/DeltaScope?display_name=tag)](https://github.com/Fanduzi/DeltaScope/releases)
![Platform](https://img.shields.io/badge/platform-darwin%20%7C%20linux-blue)
![Go Version](https://img.shields.io/badge/go-1.25-00ADD8?logo=go)
![License](https://img.shields.io/badge/license-not%20set-lightgrey)

[English](README.md) | 中文 | [Changelog](CHANGELOG.md) | [Security](SECURITY.md)
</div>

DeltaScope 是一个面向 MySQL 和 TiDB 的 SQL 审核引擎。它以离线优先的 library 和 CLI 起步，在同一套核心规则引擎上再叠加可选的元信息增强能力和轻量 HTTP 服务。

## 文档导航

- [Admin docs](docs/admin/README.md)
- [Concept docs](docs/concept/README.md)
- [Dev docs](docs/dev/README.md)
- [Recipes](docs/recipe/README.md)
- [Reference docs](docs/reference/README.md)
- [Audit capability matrix](docs/reference/audit-capability-matrix.md)

## 为什么是 DeltaScope

- 使用稳定的 `blocker`、`warning`、`notice` finding 模型审核 DDL 和 DML。
- 默认完全离线运行，适合本地开发、脚本、CI 和 AI agent。
- 在提供数据库连接信息时，可以附加元信息增强，而不是走第二套审核流程。
- 目前同时提供 Go package、`deltascope` CLI 和 `deltascope-server` HTTP 服务。

## 当前能力

- `CREATE TABLE`：标识符、注释、主键语义、审计列、类型族策略、字符集/排序规则、索引宽度/前缀/冗余、表级 option，以及基于 metadata 的粗粒度 sizing 检查。
- `ALTER TABLE`：动作禁用、基于 metadata 的兼容性判断、对象存在性检查、alter-added index 生命周期规则，以及全局 merge-alter 提示。
- 对象生命周期：`CREATE VIEW`、`DROP TABLE`、`TRUNCATE TABLE` 的策略控制，以及基于 metadata 的存在性、行数阈值和 adaptive hash 提示。
- DML：`WHERE`、`LIMIT`、`ORDER BY`、子查询、join-`ON`、insert 行数、`REPLACE`、`INSERT ... SELECT`、`ON DUPLICATE KEY`，以及对象级 denylist 规则。

## 快速开始

```bash
go test ./...
make test-e2e-cli
go run ./cmd/deltascope --version
go run ./cmd/deltascope version
go run ./cmd/deltascope-server -version
```

审核内联 SQL：

```bash
go run ./cmd/deltascope audit --sql "delete from users"
```

审核文件或标准输入：

```bash
go run ./cmd/deltascope audit --file ./change.sql
cat ./change.sql | go run ./cmd/deltascope audit
```

给 agent、脚本或 CI 使用 JSON 输出：

```bash
go run ./cmd/deltascope audit --sql "alter table users drop column age" --format json
```

使用 MySQL 风格连接参数执行 metadata 增强审核：

```bash
go run ./cmd/deltascope audit \
  --sql "alter table users add column email varchar(255)" \
  --host 127.0.0.1 --port 3306 --user root --ask-password --schema app
```

查看内置规则目录和产品能力：

```bash
go run ./cmd/deltascope rules list --kind dml --level blocker
go run ./cmd/deltascope rules show dml.where.require
go run ./cmd/deltascope rules search metadata
go run ./cmd/deltascope capabilities
```

控制非零退出阈值：

```bash
go run ./cmd/deltascope audit --sql "create table users (id bigint, primary key (id))" --fail-on warning
```

退出码：

- `0`：审核完成，且未达到失败阈值
- `1`：审核完成，但 finding 达到 `--fail-on`
- `2`：用户输入错误，例如参数、配置文件或输入文件有问题
- `3`：内部运行错误

## 配置

生成默认 YAML 策略：

```bash
go run ./cmd/deltascope config init > deltascope.yaml
```

使用配置文件执行审核：

```bash
go run ./cmd/deltascope audit --config ./deltascope.yaml --sql "update users set name = 'delta'"
```

校验配置并查看内置默认值：

```bash
go run ./cmd/deltascope config lint --file ./deltascope.yaml
go run ./cmd/deltascope config show-default
```

仓库内的 [示例配置](configs/deltascope.example.yaml) 与 `deltascope config init` 输出保持一致。

## Metadata 增强模式

DeltaScope 不要求必须连库。只有在配置 metadata provider 时，审核流程才会附加：

- 实例级事实：`version`、`character_set_database`、`innodb_large_prefix`、`innodb_default_row_format`、`innodb_adaptive_hash_index`
- 目标表快照：列、索引、主键以及表级 option 的归一化定义

这些附加事实目前用于：

- create/alter/drop/truncate 的存在性检查
- source-aware 的 `ALTER COLUMN` 兼容性判断
- destructive table lifecycle 的 adaptive-hash 提示

在 CLI 中，只要传入任意 MySQL 风格连接参数，就会进入 metadata 增强模式。此时 DeltaScope 会：

- 从实例自动识别 MySQL 或 TiDB
- 优先使用显式 `--schema`
- 否则在目标表唯一命中时自动推断 schema
- 当 schema 推断有歧义，或语句确实需要一个现有对象但无法推断时，直接报错而不是假装有 metadata
- 保持 `--quiet` 适合 shell 管道，同时在 JSON 输出里加入 `context` 字段，方便 agent 消费

## CLI Metadata E2E

基于 Docker 的 metadata-aware CLI live smoke 被刻意设计成独立于 `go test ./...` 的一层，这样默认本地开发和 CI 反馈仍然保持快速、无容器依赖。

前置条件：

- Docker Engine 和 `docker compose`
- Go toolchain
- Python 3

可用目标：

```bash
make test-e2e-cli
make test-e2e-cli-mysql
make test-e2e-cli-tidb
```

这些目标会构建本地 CLI、启动临时 MySQL 或 TiDB fixture、灌入确定性 schema，并对公共 JSON 输出和退出码做断言，覆盖 dialect 自动识别、schema 自动推断、歧义报错、qualified-schema SQL、metadata-backed existence 检查，以及至少一条依赖 instance facts 的规则路径。

## HTTP 服务

```bash
go run ./cmd/deltascope-server --listen 127.0.0.1:8083 --config ./deltascope.yaml
```

接口：

- `GET /healthz`
- `GET /version`
- `POST /v1/audit`

示例：

```bash
curl -X POST http://127.0.0.1:8083/v1/audit \
  -H 'Content-Type: application/json' \
  -d '{"sql":"delete from users","dialect":"mysql"}'
```

## Library 用法

```go
result, err := deltascope.Audit(ctx, deltascope.Request{
    SQL:     "delete from users",
    Dialect: deltascope.DialectMySQL,
})
```

公开 API 位于 [pkg/deltascope](pkg/deltascope/README.md)。

## Roadmap

下一批高优先级能力更偏增强而不是补基础缺口：

## Architecture

DeltaScope 使用偏 DDD 的分层结构。接口层处理传输协议，应用层编排审核流程，领域层维护规则模型和 finding，基础设施层负责 parser、配置、输出和 metadata 适配。library、CLI 和 HTTP 服务共用同一条审核主路径。

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
