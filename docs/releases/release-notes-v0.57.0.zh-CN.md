# DeltaScope v0.57.0 发行说明

## 概要

v0.57.0 增加 PostgreSQL domain lifecycle 覆盖。DeltaScope 现在会规范化 `CREATE DOMAIN`、`ALTER DOMAIN`（constraint、default、not null、rename）和 `DROP DOMAIN`，新增 7 条 PostgreSQL-only 发现用于离线迁移审查。`CHECK` 和 `DEFAULT` 表达式文本明确不渲染——规则只暴露布尔事实（`has_check`、`has_default`）和约束名称。

## 标准化形式

| SQL | 标准化操作 |
|-----|-----------|
| `CREATE DOMAIN email AS text CHECK (VALUE <> '')` | `create_domain` |
| `CREATE DOMAIN email AS text NOT NULL DEFAULT 'n/a' CONSTRAINT chk CHECK (...)` | `create_domain` |
| `ALTER DOMAIN email SET DEFAULT 'unknown@example.com'` | `alter_domain` (action=set_default) |
| `ALTER DOMAIN email DROP DEFAULT` | `alter_domain` (action=drop_default) |
| `ALTER DOMAIN email SET NOT NULL` | `alter_domain` (action=set_not_null) |
| `ALTER DOMAIN email DROP NOT NULL` | `alter_domain` (action=drop_not_null) |
| `ALTER DOMAIN email ADD CONSTRAINT email_not_empty CHECK (...)` | `alter_domain` (action=add_constraint) |
| `ALTER DOMAIN email DROP CONSTRAINT email_not_empty` | `alter_domain` (action=drop_constraint) |
| `ALTER DOMAIN email VALIDATE CONSTRAINT email_not_empty` | `alter_domain` (action=validate_constraint) |
| `ALTER DOMAIN email RENAME TO contact_email` | `alter_domain` (action=rename) |
| `DROP DOMAIN email` | `drop_domain` |
| `DROP DOMAIN IF EXISTS email CASCADE` | `drop_domain` |

## 新增 PostgreSQL-only 规则

| 规则 ID | 触发条件 | 默认级别 |
|---------|---------|---------|
| `ddl.pg.create_domain.notice` | `CREATE DOMAIN` | notice |
| `ddl.pg.alter_domain.constraint.notice` | `ALTER DOMAIN ... ADD/DROP/VALIDATE CONSTRAINT` | notice |
| `ddl.pg.alter_domain.default.notice` | `ALTER DOMAIN ... SET/DROP DEFAULT` | notice |
| `ddl.pg.alter_domain.not_null.notice` | `ALTER DOMAIN ... SET/DROP NOT NULL` | notice |
| `ddl.pg.alter_domain.rename.notice` | `ALTER DOMAIN ... RENAME TO` | notice |
| `ddl.pg.drop_domain.advisory` | `DROP DOMAIN` | warning |
| `ddl.pg.drop_domain.cascade.warn` | `DROP DOMAIN ... CASCADE` | warning |

> `DROP DOMAIN IF EXISTS ... CASCADE` 会同时触发两条发现：`ddl.pg.drop_domain.advisory` 和 `ddl.pg.drop_domain.cascade.warn`，属于有意设计。

## 显式不支持边界

| SQL | 不支持特性 |
|-----|-----------|
| `CREATE TYPE address AS (street text, city text)` | `create_type_composite` |

## 表达式渲染边界

DeltaScope 不渲染 `CHECK` 或 `DEFAULT` 表达式文本。规则只暴露布尔事实（`has_check`、`has_default`、`not_null`）和约束名称，不包含表达式正文。这是有意的设计决定，避免在离线审查中产生虚假精确性。

## 测试覆盖

- AST 普查测试记录全部 15 种 domain lifecycle 形式的稳定解析器事实。
- 解析器/提取器标准化测试覆盖所有支持的 domain DDL 变体。
- 语料库 fixture 覆盖全部 7 条新规则的触发形式。
- 通过 `AuditSQL` 对 12 种 domain lifecycle 变体进行服务级测试。
- 四个公共面测试：`pkg/deltascope` Audit、CLI Execute、HTTP handler 和 MCP audit_sql tool。

## 非目标

- DeltaScope 不渲染 `CHECK` 或 `DEFAULT` 表达式文本。
- DeltaScope 不对 domain 执行在线依赖验证。
- `CREATE TYPE ... AS (...)` composite types 保持显式不支持，标记为 `create_type_composite`。
- 不影响 MySQL/TiDB 行为。
- 除新增 7 条 PostgreSQL-only 规则外，不改变默认策略。

## 安装

**macOS（推荐）：**

```bash
brew tap fanduzi/deltascope
brew install --cask deltascope
```

**通用安装脚本：**

```bash
curl -fsSL https://raw.githubusercontent.com/Fanduzi/DeltaScope/v0.57.0/install.sh | \
  DELTASCOPE_VERSION=v0.57.0 sh
```
