# DeltaScope v0.28.0 发行说明

发布日期：2026-04-13

## 概述

DeltaScope `v0.28.0` 是 **Referenced-Object Metadata Surface Pack**。它将 PostgreSQL 被引用对象事实（`ReferencedSchema`、`ReferencedTable`、`ReferencedColumns`）从共享语义契约中以 additive 方式暴露到 FK forbid 规则的 finding metadata，覆盖 CLI、HTTP、MCP 和 `pkg/deltascope` 四条传输面。本版本不新增规则、CLI 标志或公共 API 契约，也不代表 schema-aware FK 策略支持或完整的 PostgreSQL 外键支持。

## 变更内容

`ddl.table.foreign_key.forbid` finding metadata 现在在底层约束携带被引用对象事实时，会包含 referenced-object 字段：

```sql
-- Schema-qualified FK 会触发带 referenced-object metadata 的 finding
CREATE TABLE orders (
    id bigint PRIMARY KEY,
    approver_id bigint,
    CONSTRAINT fk_orders_approver FOREIGN KEY (approver_id) REFERENCES public.users(id)
);
```

Finding metadata 现在包含：

| 字段 | 值 |
|------|-----|
| `referenced_schema` | `"public"` |
| `referenced_table` | `"users"` |
| `referenced_columns` | `["id"]` |

`referenced_table` 不会拼接成 `"public.users"`。schema 和 table 始终是独立字段。

## Metadata 契约

Metadata widening 是 additive 且有条件的：

- `referenced_schema` — 当 FK 引用 schema-qualified 表时出现（如 `public.users`）。无 schema 限定符时省略。
- `referenced_table` — 当 FK 约束有被引用表时出现（所有 FK 约束的标准行为）。
- `referenced_columns` — 当 FK 约束有被引用列时出现（所有 FK 约束的标准行为）。

现有 metadata 字段（`table`、`constraint`、`columns`）不变。

## 未变更内容

- 没有新增 rule ID。`ddl.table.foreign_key.forbid` 规则不变，仅 finding metadata 更宽。
- 没有新增 CLI 标志、HTTP payload 契约、MCP tool 契约或公共 Go API 契约。
- 没有修改 PostgreSQL 解析器或提取器；`ReferencedSchema` 保留在 `v0.27.0` 已发布。
- 没有修改不支持边界契约。
- 没有修改 MySQL 或 TiDB 审核面。
- 没有引入 schema-aware FK 策略决策或跨 schema 校验。

## 后续

- Schema-aware FK 策略/规则工作仍是未来决策点。
- `ALTER TABLE ... GENERATED` 边界覆盖尚未确定。

## 安装 / 升级

```bash
# macOS（推荐）
brew tap Fanduzi/deltascope
brew install --cask deltascope

# 通用安装脚本
curl -fsSL https://raw.githubusercontent.com/Fanduzi/DeltaScope/v0.28.0/install.sh | \
  DELTASCOPE_VERSION=v0.28.0 sh
```
