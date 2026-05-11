# DeltaScope v0.64.0 发行说明

## 概要

v0.64.0 新增跨方言 DDL 覆盖普查，并关闭首个 database/schema 生命周期对等差距。MySQL 和 TiDB 的 `CREATE DATABASE`、`CREATE SCHEMA`、`DROP DATABASE`、`DROP SCHEMA` 现在会产出显式审计发现，不再静默通过。PostgreSQL 的 `CREATE SCHEMA` 和 `CREATE SCHEMA IF NOT EXISTS` 现在会产出显式通知。三条新审计规则，SQL 语料库扩展至 405/405 目标，CLI/HTTP/MCP/SDK 全面公共表面覆盖。

## 跨方言 DDL 普查

v0.64.0 以 MySQL、TiDB、PostgreSQL 各方言的代表性 DDL 形式系统性普查开始。每种形式被分类为 `parser_error`、`unsupported_boundary`、`normalized_silent` 或 `finding_covered`。普查使各方言在表生命周期、视图生命周期、database/schema 生命周期、索引生命周期、约束生命周期、trigger 生命周期、routine 生命周期和权限/DCL 生命周期上的覆盖情况一目了然。

## 新增规则

| 规则 ID | 方言 | 级别 | 触发条件 |
|---------|------|:----:|---------|
| `ddl.database.create.notice` | MySQL/TiDB | notice | `CREATE DATABASE`、`CREATE DATABASE IF NOT EXISTS`、`CREATE SCHEMA` |
| `ddl.database.drop.warn` | MySQL/TiDB | warning | `DROP DATABASE`、`DROP DATABASE IF EXISTS`、`DROP SCHEMA` |
| `ddl.pg.create_schema.notice` | PostgreSQL | notice | `CREATE SCHEMA`、`CREATE SCHEMA IF NOT EXISTS` |

### MySQL/TiDB Database/Schema 生命周期

在 MySQL 和 TiDB 中，`SCHEMA` 是 `DATABASE` 的同义词。两种形式触发相同规则：

```bash
deltascope audit --sql "create database app"
# → [notice] ddl.database.create.notice

deltascope audit --sql "drop database app"
# → [warning] ddl.database.drop.warn

deltascope audit --sql "create schema app"
# → [notice] ddl.database.create.notice

deltascope audit --sql "drop schema app"
# → [warning] ddl.database.drop.warn
```

`IF NOT EXISTS` 和 `IF EXISTS` 变体仍然会发出发现。

### PostgreSQL CREATE SCHEMA

PostgreSQL `CREATE SCHEMA` 现在会发出通知。已有 `DROP SCHEMA` 规则（`ddl.pg.drop_schema.advisory`、`ddl.pg.drop_schema.cascade.warn`）不变。

```bash
deltascope audit --dialect postgresql --sql "create schema app"
# → [notice] ddl.pg.create_schema.notice
```

## 标准化

| 方言 | 语句 | 标准化操作 | 对象类型 |
|------|------|-----------|---------|
| MySQL/TiDB | `CREATE DATABASE app` | `create_schema` | `database` |
| MySQL/TiDB | `CREATE SCHEMA app` | `create_schema` | `database` |
| MySQL/TiDB | `DROP DATABASE app` | `drop_schema` | `database` |
| MySQL/TiDB | `DROP SCHEMA app` | `drop_schema` | `database` |
| PostgreSQL | `CREATE SCHEMA app` | `create_schema` | `schema` |

MySQL/TiDB `CREATE DATABASE ... CHARACTER SET`/`COLLATE` 选项作为解析器事实保留，但无策略规则对其进行治理。

## 质量

- SQL 语料库：214 条策略规则，405/405 受支持目标，100% 覆盖
- 公共表面覆盖已验证：CLI、HTTP、MCP、SDK
- AST 特征测试记录 database/schema 生命周期形式的稳定解析器事实
- 默认策略方言隔离：MySQL/TiDB 审计不发出 `ddl.pg.*`；PostgreSQL 审计不发出 `ddl.database.*`

## 非目标

- 不宣称完整 DDL 支持。
- 本里程碑不支持 trigger 生命周期。
- 本里程碑不支持 routine/function/procedure 生命周期。
- 不支持 event 生命周期。
- 不支持数据库权限/DCL 对等。
- 不执行在线 database/schema 存在性验证。
- 不校验 charset/collation/tablespace/owner。
- PostgreSQL `CREATE SCHEMA AUTHORIZATION` 和嵌套 `CREATE SCHEMA ... CREATE TABLE ...` 仍不支持/推迟。
- 已有 PostgreSQL `DROP SCHEMA` 行为不变。
- DeltaScope 不执行迁移。

## 安装

**macOS（推荐）：**

```bash
brew tap fanduzi/deltascope
brew install --cask deltascope
```

**通用安装器：**

```bash
curl -fsSL https://raw.githubusercontent.com/Fanduzi/DeltaScope/v0.64.0/install.sh | \
  DELTASCOPE_VERSION=v0.64.0 sh
```

## 升级

如果之前安装了 v0.63.0：

```bash
# Homebrew
brew upgrade --cask deltascope

# 通用安装器（使用新版本重新运行）
curl -fsSL https://raw.githubusercontent.com/Fanduzi/DeltaScope/v0.64.0/install.sh | \
  DELTASCOPE_VERSION=v0.64.0 sh
```

## 验证

```bash
deltascope --version
# 应输出 v0.64.0

deltascope audit --sql "create database app"
# 应产出 ddl.database.create.notice 发现

deltascope audit --dialect postgresql --sql "create schema app"
# 应产出 ddl.pg.create_schema.notice 发现
```
