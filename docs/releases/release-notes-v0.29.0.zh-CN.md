# DeltaScope v0.29.0 发行说明

发布日期：2026-04-14

## 概述

DeltaScope `v0.29.0` 是 **Schema-Aware FK Policy Pack**。它是第一个 schema-aware FK policy 步骤：针对显式 cross-schema 外键新增 PostgreSQL-only notice 规则 `ddl.pg.table.foreign_key.cross_schema.advisory`。它不代表完整的 PostgreSQL 外键支持，不是跨 schema 校验引擎，也不建模 `search_path`。

## 变更内容

当以下条件全部满足时，DeltaScope 会发出一条额外 advisory finding：

1. 审核方言是 PostgreSQL。
2. owning table schema 显式存在。
3. referenced schema 显式存在。
4. owning table schema 与 referenced schema 不同。

```sql
-- 会发出 ddl.pg.table.foreign_key.cross_schema.advisory
CREATE TABLE billing.orders (
    id bigint PRIMARY KEY,
    approver_id bigint,
    CONSTRAINT fk_orders_approver FOREIGN KEY (approver_id) REFERENCES auth.users(id)
);
```

默认策略下，现有 FK forbid 规则仍然生效。新 advisory 只是增加 schema 上下文，不替代 `ddl.table.foreign_key.forbid`。

## Rule 契约

| 字段 | 契约 |
|------|------|
| Rule ID | `ddl.pg.table.foreign_key.cross_schema.advisory` |
| 默认级别 | `notice` |
| 方言 | 仅 PostgreSQL |
| 触发条件 | owning table schema 与 referenced schema 都显式存在且两者不同 |
| Same-schema FK | 不触发 |
| 裸引用 | `REFERENCES users(id)` 不触发，因为 referenced schema unknown |

## Metadata 契约

Advisory finding 可以包含这些 metadata 字段：

| 字段 | 示例 |
|------|------|
| `table_schema` | `"billing"` |
| `referenced_schema` | `"auth"` |
| `referenced_table` | `"users"` |
| `referenced_columns` | `["id"]` |

`referenced_table` 继续规范化为 `"users"`，不会写成 `"auth.users"`。schema 与 table 始终是独立字段。

## 未变更内容

- 裸引用仍然是 schema unknown。DeltaScope 不推断 `public`，也不建模 PostgreSQL `search_path` 语义。
- Same-schema 外键不触发新 advisory。
- 本发行说明不暗示 parser、extractor、corpus 或 public API 扩展。
- MySQL 和 TiDB 审核行为不变。
- `v0.28.0` 的 metadata widening 仍然是底层 surface：在这些事实存在时暴露 `referenced_schema`、`referenced_table` 和 `referenced_columns`。

## 后续

- 决定 schema-aware FK policy 是否应扩展到这条显式 cross-schema advisory 之外。
- `ALTER TABLE ... GENERATED` 边界覆盖尚未确定。

## 安装 / 升级

```bash
# macOS（推荐）
brew tap Fanduzi/deltascope
brew install --cask deltascope

# 通用安装脚本
curl -fsSL https://raw.githubusercontent.com/Fanduzi/DeltaScope/v0.29.0/install.sh | \
  DELTASCOPE_VERSION=v0.29.0 sh
```
