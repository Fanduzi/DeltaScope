# DeltaScope v0.27.0 发行说明

发布日期：2026-04-13

## 概述

DeltaScope `v0.27.0` 是 **Schema-Qualified Reference Semantics Pack**。它在共享 `spec.Constraint` 契约中保留了 PostgreSQL schema-qualified 被引用对象事实，由语料用例和服务层语义测试锁定。本版本不新增规则、CLI 标志或公共 API 契约，也不代表完整的 PostgreSQL 外键支持或 schema-aware 规则决策。

## 变更内容

PostgreSQL 提取器现在会保留 schema-qualified `REFERENCES` 和 `FOREIGN KEY ... REFERENCES` 形式中的 schema 部分：

```sql
-- 两种形式现在都会保留 ReferencedSchema = "public"、ReferencedTable = "users"
CREATE TABLE orders (
    user_id bigint REFERENCES public.users(id)
);

CREATE TABLE orders (
    user_id bigint,
    FOREIGN KEY (user_id) REFERENCES public.users(id)
);
```

`spec.Constraint` 结构体新增 additive `ReferencedSchema` 字段。规范化表示始终分离 schema 和 table：

| 字段 | 值 |
|------|-----|
| `ReferencedSchema` | `"public"` |
| `ReferencedTable` | `"users"` |

`ReferencedTable` 不会被拼接成 `"public.users"`。

## 语义契约

Schema-qualified 被引用对象语义由三层锁定：

1. **解析器/提取器**：PostgreSQL 提取器在已有的 `ReferencedTable` 和 `ReferencedColumns` 旁填充 `ReferencedSchema` 字段。
2. **语料层**：`testdata/sql-corpus/postgresql/` 包含专门的 schema-qualified reference 用例，`.expected.yaml` 断言 `ReferencedSchema` 和 `ReferencedTable`。
3. **服务层**：语义测试断言 schema-qualified 被引用对象事实经过审计管线后仍然保留。

## Surface 契约

当前公共 finding 元数据不变：

- **CLI**：FK forbid finding 输出不包含 `referenced_schema`。
- **HTTP** 和 **MCP**：finding 元数据不暴露 `referenced_schema`。
- **`pkg/deltascope`**：公共 `Result` 类型的 finding 元数据不携带 `referenced_schema`。

共享语义契约（`spec.Constraint`）在底层更丰富，但公共传输面保持既有支持行为。

## 未变更内容

- 没有新增规则 ID。`ReferencedSchema` 是提取器/共享语义事实，不是规则 ID。
- 没有新增 CLI 标志、HTTP 负载契约、MCP tool 契约或公共 Go API 契约。
- `v0.26.0` 的 PostgreSQL 不支持边界契约没有变化。
- MySQL 和 TiDB 审核面没有变化。
- 现有 FK forbid 规则元数据不包含 `referenced_schema` 字段。

## 后续工作

- 决定公共 finding 元数据是否应暴露被引用对象的 schema 事实。
- Schema-aware FK 策略/规则工作仍是未来决策点，非已提交里程碑。
- `v0.26.0` 后续的 `ALTER TABLE ... GENERATED` 边界覆盖仍未提交。

## 安装 / 升级

```bash
# macOS（推荐）
brew tap Fanduzi/deltascope
brew install --cask deltascope

# 通用安装器
curl -fsSL https://raw.githubusercontent.com/Fanduzi/DeltaScope/v0.27.0/install.sh | \
  DELTASCOPE_VERSION=v0.27.0 sh
```
