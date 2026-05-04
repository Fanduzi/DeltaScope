# DeltaScope v0.56.0 发行说明

## 概要

v0.56.0 增加 PostgreSQL ALTER TABLE logged-state 覆盖，并改进 ALTER COLUMN TYPE USING 元数据提取。DeltaScope 现在会规范化 `ALTER TABLE ... SET LOGGED` 和 `ALTER TABLE ... SET UNLOGGED`，新增两条 PostgreSQL-only 发现来提示 logged-state 转换，同时在 ALTER COLUMN TYPE 规范化中捕获 USING 表达式。SET TABLESPACE 保持显式不支持边界。

## 标准化形式

| SQL | 标准化操作 |
|-----|-----------|
| `ALTER TABLE users SET LOGGED` | `alter` (action=set_logged) |
| `ALTER TABLE users SET UNLOGGED` | `alter` (action=set_unlogged) |
| `ALTER TABLE users ALTER COLUMN name TYPE varchar(100) USING name::varchar(100)` | `alter` (action=alter_column_type, using=name::varchar(100)) |

## 新增 PostgreSQL-only 规则

| 规则 ID | 触发条件 | 默认级别 |
|---------|---------|---------|
| `ddl.pg.alter.set_logged.notice` | `ALTER TABLE ... SET LOGGED` | notice |
| `ddl.pg.alter.set_unlogged.notice` | `ALTER TABLE ... SET UNLOGGED` | notice |

## 改进

- `ALTER TABLE ... ALTER COLUMN TYPE ... USING ...` 现在会在规范化 alter 元数据中捕获 USING 表达式，使不安全类型转换在审计输出中可见。

## 显式不支持边界

| SQL | 不支持特性 |
|-----|-----------|
| `ALTER TABLE users SET TABLESPACE pg_default` | `alter_set_tablespace` |

## 测试覆盖

- AST 普查测试记录 logged-state 形式的稳定解析器事实。
- 解析器/提取器标准化测试覆盖 SET LOGGED、SET UNLOGGED 和 ALTER COLUMN TYPE USING 变体。
- 语料库 fixture 覆盖两条新规则的触发形式。
- 通过 `AuditSQL` 对 logged-state 变体进行服务级测试。
- 四个公共面测试：`pkg/deltascope` Audit、CLI Execute、HTTP handler 和 MCP audit_sql tool。

## 非目标

- DeltaScope 不会验证目标表当前是否为 logged 或 unlogged 状态。
- DeltaScope 不会评估 logged-state 转换对 WAL 或复制的潜在影响。
- 这不是完整的 PostgreSQL ALTER TABLE 语法支持。
- 不影响 MySQL/TiDB 行为。
- 除新增两条 PostgreSQL-only 规则外，不改变默认策略。

## 安装

**macOS（推荐）：**

```bash
brew tap fanduzi/deltascope
brew install --cask deltascope
```

**通用安装脚本：**

```bash
curl -fsSL https://raw.githubusercontent.com/Fanduzi/DeltaScope/v0.56.0/install.sh | \
  DELTASCOPE_VERSION=v0.56.0 sh
```
