# DeltaScope v0.58.0 发行说明

## 概要

v0.58.0 新增 PostgreSQL composite type lifecycle 窄支持。DeltaScope 现在可以规范化 `CREATE TYPE ... AS (...)`、`ALTER TYPE ... RENAME TO` 和 `ALTER TYPE ... SET SCHEMA`，并配有三条 PostgreSQL-only 新发现用于离线迁移审查。`DROP TYPE` 明确复用 v0.55.0 已有的类型生命周期规则（`ddl.pg.drop_type.advisory`、`ddl.pg.drop_type.cascade.warn`），不引入新的 composite-specific DROP TYPE 规则。属性级操作（`ADD ATTRIBUTE`、`DROP ATTRIBUTE`、`ALTER ATTRIBUTE ... TYPE`、`RENAME ATTRIBUTE`）仍然明确不支持/延迟。

## 规范化形式

| SQL | 规范化操作 |
|-----|-----------|
| `CREATE TYPE address AS (street text, city text)` | `create_type_composite` |
| `CREATE TYPE qualified.address AS (street text, city text)` | `create_type_composite` |
| `CREATE TYPE address AS (street text COLLATE "C", city text)` | `create_type_composite`（collation 被记录但不做解释） |
| `ALTER TYPE address RENAME TO mailing_address` | `alter_type`（action=rename） |
| `ALTER TYPE address SET SCHEMA archive` | `alter_type`（action=set_schema） |

## 新增 PostgreSQL-only 规则

| 规则 ID | 触发条件 | 默认级别 |
|---------|---------|---------|
| `ddl.pg.create_type.composite.notice` | `CREATE TYPE ... AS (...)` | notice |
| `ddl.pg.alter_type.composite_rename.notice` | `ALTER TYPE ... RENAME TO` | notice |
| `ddl.pg.alter_type.composite_set_schema.notice` | `ALTER TYPE ... SET SCHEMA` | notice |

## DROP TYPE：复用已有规则

composite type 的 `DROP TYPE` 语句明确复用 v0.55.0 已有的类型生命周期规则：

| 规则 ID | 触发条件 | 默认级别 |
|---------|---------|---------|
| `ddl.pg.drop_type.advisory` | `DROP TYPE` | warning |
| `ddl.pg.drop_type.cascade.warn` | `DROP TYPE ... CASCADE` | warning |

不存在也不会引入 composite-specific DROP TYPE 规则。已有规则已提供充分覆盖。

## 明确不支持/延迟的操作

| SQL | 不支持的特性 |
|-----|------------|
| `ALTER TYPE ... ADD ATTRIBUTE` | `alter_type_add_attribute` |
| `ALTER TYPE ... DROP ATTRIBUTE` | `alter_type_drop_attribute` |
| `ALTER TYPE ... ALTER ATTRIBUTE ... TYPE` | `alter_type_alter_attribute_type` |
| `ALTER TYPE ... RENAME ATTRIBUTE ... TO ...` | `alter_type_rename_attribute` |

## Collation 与类型语义边界

DeltaScope 在结构层级上可以识别 composite type 属性定义中的 `COLLATE` 注解（例如 `CREATE TYPE address AS (street text COLLATE "C", city text)`），但不渲染、解释或校验 collation 语义。这是一个明确的设计决策。

## 测试覆盖

- AST 普查测试，记录所有 composite type lifecycle 形式的稳定 parser 事实。
- Parser/extractor 规范化测试，覆盖所有支持的 composite DDL 变体。
- Corpus fixtures 覆盖三条新规则的触发形式。
- 服务层测试，通过 `AuditSQL` 覆盖代表性 composite lifecycle 变体。
- 公共接口测试，覆盖全部四个面：`pkg/deltascope` Audit、CLI Execute、HTTP handler 和 MCP audit_sql tool。

## Non-Goals

- DeltaScope 不对 composite type 做实时依赖校验。
- DeltaScope 不建模完整 PostgreSQL 类型系统语义。
- 这是窄 composite type lifecycle 支持，不是完整 PostgreSQL 类型系统支持。
- 属性级操作（`ADD ATTRIBUTE`、`DROP ATTRIBUTE`、`ALTER ATTRIBUTE ... TYPE`、`RENAME ATTRIBUTE`）仍然明确延迟。
- 无 MySQL/TiDB 行为变更。
- 除三条新 PostgreSQL-only 规则外，无默认策略变更。

## 安装

**macOS（推荐）：**

```bash
brew tap fanduzi/deltascope
brew install --cask deltascope
```

**通用安装器：**

```bash
curl -fsSL https://raw.githubusercontent.com/Fanduzi/DeltaScope/v0.58.0/install.sh | \
  DELTASCOPE_VERSION=v0.58.0 sh
```
