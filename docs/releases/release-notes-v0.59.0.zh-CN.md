# DeltaScope v0.59.0 发行说明

## 概要

v0.59.0 新增 PostgreSQL extension lifecycle 窄支持。DeltaScope 现在可以规范化 `CREATE EXTENSION`、`ALTER EXTENSION`（`UPDATE`、`UPDATE TO`、`SET SCHEMA`）和 `DROP EXTENSION`，并配有六条 PostgreSQL-only 新发现用于离线迁移审查。Extension 成员变更（`ALTER EXTENSION ... ADD/DROP TABLE`）仍然明确不支持/延迟。不做 extension 可用性、已安装包、版本兼容性或依赖图的实时校验。

## 规范化形式

| SQL | 规范化操作 |
|-----|-----------|
| `CREATE EXTENSION pg_trgm` | `create_extension` |
| `CREATE EXTENSION IF NOT EXISTS pg_trgm` | `create_extension`（if_not_exists=true） |
| `CREATE EXTENSION pg_trgm WITH SCHEMA utils` | `create_extension`（schema=utils） |
| `CREATE EXTENSION pg_trgm WITH VERSION '1.5'` | `create_extension`（version=1.5） |
| `CREATE EXTENSION pg_trgm CASCADE` | `create_extension`（cascade=true） |
| `ALTER EXTENSION pg_trgm UPDATE` | `alter_extension`（action=update） |
| `ALTER EXTENSION pg_trgm UPDATE TO '1.6'` | `alter_extension`（action=update_to） |
| `ALTER EXTENSION pg_trgm SET SCHEMA utils` | `alter_extension`（action=set_schema） |
| `DROP EXTENSION pg_trgm` | `drop_extension` |
| `DROP EXTENSION IF EXISTS pg_trgm` | `drop_extension`（if_exists=true） |
| `DROP EXTENSION pg_trgm CASCADE` | `drop_extension`（cascade=true） |

## 新增 PostgreSQL-only 规则

| 规则 ID | 触发条件 | 默认级别 |
|---------|---------|---------|
| `ddl.pg.create_extension.notice` | `CREATE EXTENSION` | notice |
| `ddl.pg.create_extension.cascade.warn` | `CREATE EXTENSION ... CASCADE` | warning |
| `ddl.pg.alter_extension.update.notice` | `ALTER EXTENSION ... UPDATE` / `UPDATE TO` | notice |
| `ddl.pg.alter_extension.set_schema.notice` | `ALTER EXTENSION ... SET SCHEMA` | notice |
| `ddl.pg.drop_extension.advisory` | `DROP EXTENSION` | warning |
| `ddl.pg.drop_extension.cascade.warn` | `DROP EXTENSION ... CASCADE` | warning |

## CASCADE 重复发现

`CREATE EXTENSION ... CASCADE` 会同时触发 `ddl.pg.create_extension.notice` 和 `ddl.pg.create_extension.cascade.warn`。`DROP EXTENSION ... CASCADE` 会同时触发 `ddl.pg.drop_extension.advisory` 和 `ddl.pg.drop_extension.cascade.warn`。这些重复发现是刻意为之——每条规则关注不同层面（操作本身 vs. CASCADE 副作用风险）。

## 明确不支持/延迟的操作

| SQL | 不支持的特性 |
|-----|------------|
| `ALTER EXTENSION ... ADD TABLE` | `alter_extension_add_member` |
| `ALTER EXTENSION ... DROP TABLE` | `alter_extension_drop_member` |

## 实时校验边界

DeltaScope 不对 extension 做任何形式的实时校验：
- 不检查 extension 可用性或已安装包
- 不做版本兼容性校验
- 不做依赖图解析
- 不做 schema 存在性验证

## 测试覆盖

- AST 普查测试，记录所有 extension lifecycle 形式的稳定 parser 事实。
- Parser/extractor 规范化测试，覆盖所有支持的 extension DDL 变体。
- Corpus fixtures 覆盖六条新规则的触发形式。
- 服务层测试，通过 `AuditSQL` 覆盖代表性 extension lifecycle 变体。
- 公共接口测试，覆盖全部四个面：`pkg/deltascope` Audit、CLI Execute、HTTP handler 和 MCP audit_sql tool。

## Non-Goals

- DeltaScope 不对 extension 做实时依赖校验。
- DeltaScope 不建模完整 PostgreSQL extension 系统语义。
- 这是窄 extension lifecycle 支持，不是广泛的治理或 admin DDL 支持。
- Extension 成员变更（`ADD/DROP TABLE`）仍然明确延迟。
- 无 MySQL/TiDB 行为变更。
- 除六条新 PostgreSQL-only 规则外，无默认策略变更。

## 安装

**macOS（推荐）：**

```bash
brew tap fanduzi/deltascope
brew install --cask deltascope
```

**通用安装器：**

```bash
curl -fsSL https://raw.githubusercontent.com/Fanduzi/DeltaScope/v0.59.0/install.sh | \
  DELTASCOPE_VERSION=v0.59.0 sh
```
