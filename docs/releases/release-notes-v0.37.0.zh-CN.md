# DeltaScope v0.37.0 发行说明

## 概述

DeltaScope v0.37.0 新增 PostgreSQL `CREATE TABLE` 主键事实支持。内联、表级、命名和复合主键声明现在会填充 DeltaScope 标准化主键契约，使已有的主键规则可以审计 PostgreSQL `CREATE TABLE` 语句。

## 变更内容

### PostgreSQL 主键事实提取

PostgreSQL 提取器现在为 `CREATE TABLE` 语句填充共享的 `DDL.PrimaryKey` 契约，支持以下形态：

| PostgreSQL 形态 | 示例 |
|----------------|------|
| 内联 | `CREATE TABLE t (id bigint PRIMARY KEY)` |
| 表级 | `CREATE TABLE t (id bigint, PRIMARY KEY (id))` |
| 命名 | `CREATE TABLE t (id bigint, CONSTRAINT t_pkey PRIMARY KEY (id))` |
| 复合 | `CREATE TABLE t (a int, b int, PRIMARY KEY (a, b))` |

主键列被视为有效 `NOT NULL`——与 PostgreSQL 语义一致，即主键列始终不为 null，无论是否显式声明 `NOT NULL`。

### 规则覆盖解锁

已有的主键规则现在适用于 PostgreSQL `CREATE TABLE`：

| Rule ID | 触发条件 |
|---------|---------|
| `ddl.table.primary_key.bigint.require` | PostgreSQL 主键列类型不是 BIGINT |
| `ddl.table.primary_key.columns.max_count` | PostgreSQL 复合主键超过配置的列数上限 |

`ddl.table.primary_key.not_null.require` 不作为 PostgreSQL 的稳定负例，因为在 PostgreSQL 语义下 PK 列被有效视为 NOT NULL。

### 公共接口

四条产品面均对 PostgreSQL 主键违规产生显式 `rule_id` 发现：

| 接口 | 行为 |
|------|------|
| CLI | 正常审核输出，包含 `rule_id` findings |
| HTTP（`POST /v1/audit`） | 正常审核响应，findings 含明确 `rule_id` |
| MCP（`audit_sql`） | 正常 tool result，findings 含明确 `rule_id` |
| `pkg/deltascope` | `Audit()` 返回带 findings 的 result，含明确 `rule_id` |

## 未变更的部分

- 无完整 PostgreSQL 索引支持。
- 无 `ALTER TABLE ADD PRIMARY KEY` 支持。
- 无在线 schema 主键内省。
- 无新增主键规则 ID。
- 无完整 PostgreSQL 约束/索引对等。
- 无 MySQL 或 TiDB 行为变更。

## 安装

```bash
curl -fsSL https://raw.githubusercontent.com/Fanduzi/DeltaScope/v0.37.0/install.sh | \
  DELTASCOPE_VERSION=v0.37.0 sh
```
