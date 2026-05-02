# DeltaScope v0.55.0 发行说明

## 概要

v0.55.0 增加 PostgreSQL 类型生命周期覆盖，聚焦 enum type 和 drop type。DeltaScope 现在会规范化 `CREATE TYPE ... AS ENUM`、`ALTER TYPE ... ADD VALUE` 和 `DROP TYPE`，新增五条 PostgreSQL-only 发现来提示 enum 与 drop type 风险，并继续将 composite type 与 domain 作为显式不支持边界。

## 标准化形式

| SQL | 标准化操作 |
|-----|-----------|
| `CREATE TYPE color AS ENUM ('red', 'green', 'blue')` | `create_type` (type_kind=enum, labels=red,green,blue) |
| `ALTER TYPE color ADD VALUE 'yellow'` | `alter_type` (type_kind=enum, action=add_value, value=yellow) |
| `ALTER TYPE color ADD VALUE IF NOT EXISTS 'yellow'` | `alter_type` (type_kind=enum, action=add_value, value=yellow, if_not_exists=true) |
| `ALTER TYPE color ADD VALUE 'yellow' BEFORE 'green'` | `alter_type` (type_kind=enum, action=add_value, value=yellow, placement=before, neighbor=green) |
| `ALTER TYPE color ADD VALUE 'yellow' AFTER 'green'` | `alter_type` (type_kind=enum, action=add_value, value=yellow, placement=after, neighbor=green) |
| `DROP TYPE color` | `drop_type` |
| `DROP TYPE IF EXISTS color CASCADE` | `drop_type` (if_exists=true, cascade=true) |

## 新增 PostgreSQL-only 规则

| 规则 ID | 触发条件 | 默认级别 |
|---------|---------|---------|
| `ddl.pg.create_type.enum.notice` | `CREATE TYPE ... AS ENUM` | notice |
| `ddl.pg.alter_type.add_value.advisory` | `ALTER TYPE ... ADD VALUE` | warning |
| `ddl.pg.alter_type.add_value.position.notice` | `ALTER TYPE ... ADD VALUE ... BEFORE/AFTER` | notice |
| `ddl.pg.drop_type.advisory` | `DROP TYPE` | warning |
| `ddl.pg.drop_type.cascade.warn` | `DROP TYPE ... CASCADE` | warning |

## 显式不支持边界

| SQL | 不支持特性 |
|-----|-----------|
| `CREATE TYPE address AS (street text, city text)` | `create_type_composite` |
| `CREATE DOMAIN email AS text CHECK (...)` | `create_domain` |

## 测试覆盖

- AST 普查测试记录所有七种受支持的 type lifecycle 形式的稳定解析器事实。
- 解析器/提取器标准化测试覆盖 enum 创建、加值变体和 drop type 变体。
- 语料库 fixture 覆盖五条新规则的触发形式。
- 通过 `AuditSQL` 对 type lifecycle 变体进行服务级测试。
- 四个公共面测试：`pkg/deltascope` Audit、CLI Execute、HTTP handler 和 MCP audit_sql tool。

## 非目标

- DeltaScope 不会检查在线依赖对象。
- DeltaScope 不会验证 enum 值是否已被数据或应用代码使用。
- DeltaScope 不会建模完整的 PostgreSQL 类型系统语义。
- 这不是完整的 PostgreSQL 类型生命周期支持。
- 不影响 MySQL/TiDB 行为。
- 除新增五条 PostgreSQL-only 规则外，不改变默认策略。

## 安装

**macOS（推荐）：**

```bash
brew tap fanduzi/deltascope
brew install --cask deltascope
```

**通用安装脚本：**

```bash
curl -fsSL https://raw.githubusercontent.com/Fanduzi/DeltaScope/v0.55.0/install.sh | \
  DELTASCOPE_VERSION=v0.55.0 sh
```
