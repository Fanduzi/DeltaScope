# DeltaScope v0.38.0 发行说明

## 概述

DeltaScope v0.38.0 扩展了 PostgreSQL unique/index 审计覆盖范围，支持语句局部的唯一约束和简单 btree `CREATE INDEX` 形态。已有的索引规则现在可以为已批准的 PostgreSQL 形态产生 findings，并提供 corpus、公共接口和 Docker-backed e2e 覆盖。

## 变更内容

### PostgreSQL Unique/Index 规则覆盖

已有的通用索引规则现在适用于独立的 PostgreSQL `CREATE INDEX` 和 `CREATE UNIQUE INDEX` 语句：

| PostgreSQL 形态 | 示例 |
|----------------|------|
| 普通索引 | `CREATE INDEX idx_users_email ON users (email)` |
| 唯一索引 | `CREATE UNIQUE INDEX uniq_users_email ON users (email)` |
| 并发构建 | `CREATE INDEX CONCURRENTLY idx_users_email ON users (email)` |
| 内联 UNIQUE 约束 | `CREATE TABLE t (email text UNIQUE)` |

### 规则覆盖解锁

| Rule ID | 触发条件 |
|---------|---------|
| `ddl.index.secondary.prefix.require` | 普通索引名未以要求的前缀开头（默认：`idx_`） |
| `ddl.index.unique.prefix.require` | 唯一索引名未以要求的前缀开头（默认：`uniq_`） |
| `ddl.index.columns.max_count` | 索引包含的列数超过允许的最大值 |

### 公共接口

四条产品面均产生显式 `rule_id` 发现：

| 接口 | 行为 |
|------|------|
| CLI | 正常审核输出，包含 `rule_id` findings |
| HTTP（`POST /v1/audit`） | 正常审核响应，findings 含明确 `rule_id` |
| MCP（`audit_sql`） | 正常 tool result，findings 含明确 `rule_id` |
| `pkg/deltascope` | `Audit()` 返回带 findings 的 result，含明确 `rule_id` |

### Docker-backed E2E 覆盖

PostgreSQL CLI e2e 通过 Docker-backed 测试路径覆盖了 `ddl.index.unique.prefix.require` 的语句局部 `CREATE UNIQUE INDEX` 审计。

## 未变更的部分

- 无完整 PostgreSQL 索引支持。
- 无 partial index 支持（`WHERE` 子句）。
- 无 expression index 支持（`((lower(email)))`）。
- 无 INCLUDE 子句支持。
- 无 operator class 支持。
- 无非 btree 访问方法支持（`USING hash` 等）。
- 无 NULLS NOT DISTINCT 支持。
- 无在线 schema 索引内省。
- 无新增索引规则 ID。
- 无 MySQL 或 TiDB 行为变更。

## 安装

```bash
curl -fsSL https://raw.githubusercontent.com/Fanduzi/DeltaScope/v0.38.0/install.sh | \
  DELTASCOPE_VERSION=v0.38.0 sh
```
