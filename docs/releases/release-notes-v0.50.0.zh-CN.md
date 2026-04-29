# DeltaScope v0.50.0 发行说明

## 概要

v0.50.0 发布 PostgreSQL Object Lifecycle DDL Pack。DeltaScope 现在可以规范化和审核 PostgreSQL schema、sequence 和 materialized view 生命周期 DDL——`CREATE SCHEMA`、`DROP SCHEMA`、`CREATE SEQUENCE`、`ALTER SEQUENCE`、`DROP SEQUENCE`、`CREATE MATERIALIZED VIEW` 和 `DROP MATERIALIZED VIEW`——并新增九条 PostgreSQL-only 规则，覆盖级联删除、sequence 循环和 sequence 重置场景。

## 新增

- PostgreSQL 对象生命周期 DDL 规范化：schema、sequence 和 materialized view 现在走正常审核管线而非返回 unsupported。
- 新增规范化操作：
  - `CREATE SCHEMA`
  - `DROP SCHEMA`
  - `CREATE SEQUENCE`
  - `ALTER SEQUENCE`
  - `DROP SEQUENCE`
  - `CREATE MATERIALIZED VIEW`
  - `DROP MATERIALIZED VIEW`
- 九条新 PostgreSQL-only 规则：
  - `ddl.pg.drop_schema.advisory` — `DROP SCHEMA` 移除 schema 时发出提示 (notice)
  - `ddl.pg.drop_schema.cascade.warn` — `DROP SCHEMA ... CASCADE` 使用级联删除时发出警告 (warning)
  - `ddl.pg.create_sequence.cycle.warn` — `CREATE SEQUENCE ... CYCLE` 可能导致序列值回绕时发出警告 (warning)
  - `ddl.pg.alter_sequence.restart.warn` — `ALTER SEQUENCE ... RESTART` 重置序列计数器时发出警告 (warning)
  - `ddl.pg.alter_sequence.cycle.warn` — `ALTER SEQUENCE ... CYCLE` 在已有序列上启用值回绕时发出警告 (warning)
  - `ddl.pg.drop_sequence.advisory` — `DROP SEQUENCE` 移除序列时发出提示 (notice)
  - `ddl.pg.drop_sequence.cascade.warn` — `DROP SEQUENCE ... CASCADE` 使用级联删除时发出警告 (warning)
  - `ddl.pg.drop_materialized_view.advisory` — `DROP MATERIALIZED VIEW` 移除物化视图时发出提示 (notice)
  - `ddl.pg.drop_materialized_view.cascade.warn` — `DROP MATERIALIZED VIEW ... CASCADE` 使用级联删除时发出警告 (warning)
- 全部生命周期操作的服务层测试（通过 `AuditSQL`）。
- schema、sequence 和 materialized view 生命周期形式的语料库 fixture。
- 四个公共表面的测试覆盖：`pkg/deltascope` Audit、CLI Execute、HTTP handler 和 MCP audit_sql 工具。

## 非目标

- `REFRESH MATERIALIZED VIEW` 仍为 unsupported/deferred。
- 这不是完整的 PostgreSQL 对象生命周期覆盖。剩余 unsupported DDL 形式（trigger、function、extension 等）仍为显式边界。
- 没有 MySQL/TiDB 行为变更。
- 除九条新 PostgreSQL-only 规则条目外，没有默认策略变更。
- 除生命周期 DDL 规范化扩展外，没有解析器语法变更。

## 安装

**macOS（推荐）：**

```bash
brew tap fanduzi/deltascope
brew install --cask deltascope
```

**通用安装脚本：**

```bash
curl -fsSL https://raw.githubusercontent.com/Fanduzi/DeltaScope/v0.50.0/install.sh | \
  DELTASCOPE_VERSION=v0.50.0 sh
```
