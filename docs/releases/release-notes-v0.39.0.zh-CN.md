# DeltaScope v0.39.0 发行说明

## 概述

DeltaScope v0.39.0 为已批准的 `ALTER TABLE ... ADD CONSTRAINT` 形态保留语句级主键和唯一约束事实，使已有的主键和唯一/索引规则可以在 CLI、HTTP、MCP 和 `pkg/deltascope` 四条产品面上产生 findings。

## 变更内容

### PostgreSQL ALTER TABLE 约束事实支持

已有的主键和唯一/索引规则现在适用于已批准的 PostgreSQL `ALTER TABLE ... ADD CONSTRAINT` 形态：

| PostgreSQL 形态 | 示例 |
|----------------|------|
| 内联主键 | `ALTER TABLE users ADD PRIMARY KEY (id)` |
| 命名主键 | `ALTER TABLE users ADD CONSTRAINT users_pkey PRIMARY KEY (id)` |
| 内联唯一 | `ALTER TABLE users ADD UNIQUE (email)` |
| 命名唯一 | `ALTER TABLE users ADD CONSTRAINT users_email_key UNIQUE (email)` |

### 规则覆盖解锁

| Rule ID | 触发条件 |
|---------|---------|
| `ddl.table.primary_key.bigint.require` | 主键列不是 BIGINT |
| `ddl.table.primary_key.columns.max_count` | 复合主键列数超过配置的最大值 |
| `ddl.alter.add_index.unique.prefix.require` | 唯一约束名未以要求的前缀开头（默认：`uniq_`） |

### 公共接口

四条产品面均产生显式 `rule_id` 发现：

| 接口 | 行为 |
|------|------|
| CLI | 正常审核输出，包含 `rule_id` findings |
| HTTP（`POST /v1/audit`） | 正常审核响应，findings 含明确 `rule_id` |
| MCP（`audit_sql`） | 正常 tool result，findings 含明确 `rule_id` |
| `pkg/deltascope` | `Audit()` 返回带 findings 的 result，含明确 `rule_id` |

### Docker-backed E2E 覆盖

PostgreSQL CLI e2e 通过 Docker-backed 测试路径覆盖了 `ddl.alter.add_index.unique.prefix.require` 的语句局部 `ALTER TABLE ... ADD CONSTRAINT ... UNIQUE` 审计。

## 未变更的部分

- 无完整 `ALTER TABLE ADD CONSTRAINT` 支持——外键、CHECK 约束和排他约束不在范围内。
- 无可延迟约束支持。
- 无约束验证生命周期支持（`VALIDATE CONSTRAINT`、`NOT VALID`）。
- 无 partial 或 expression index 支持。
- 无 operator class 支持。
- 无从约束重建在线 schema。
- 无新增规则 ID——已有规则通过扩展适用性和投影辅助覆盖已批准形态。
- 无 MySQL 或 TiDB 行为变更。

## 安装

```bash
curl -fsSL https://raw.githubusercontent.com/Fanduzi/DeltaScope/v0.39.0/install.sh | \
  DELTASCOPE_VERSION=v0.39.0 sh
```
