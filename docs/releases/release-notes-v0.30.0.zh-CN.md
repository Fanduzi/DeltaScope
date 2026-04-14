# DeltaScope v0.30.0 发行说明

发布日期：2026-04-14

## 概述

DeltaScope `v0.30.0` 是 **PostgreSQL ALTER TABLE GENERATED Boundary Pack**。它收紧了 PostgreSQL `ALTER TABLE ... ADD COLUMN` 在 generated stored / identity 语义下的不支持边界契约。它不代表 generated-column 支持、identity-column 支持，也不是广义的 PostgreSQL `ALTER TABLE` 支持。

## 变更内容

DeltaScope 现在会对以下 PostgreSQL `ALTER TABLE ... ADD COLUMN` 形态返回显式 unsupported 结果：

1. `GENERATED ALWAYS AS (...) STORED` → `generated_column`
2. `GENERATED ALWAYS AS IDENTITY` → `generated_as_identity`

```sql
ALTER TABLE users ADD COLUMN full_name text GENERATED ALWAYS AS (first_name || ' ' || last_name) STORED;
ALTER TABLE users ADD COLUMN id bigint GENERATED ALWAYS AS IDENTITY;
```

这些形态不再看起来像普通的受支持 add-column 路径。

## Unsupported 契约

| 形态 | unsupported feature |
|------|---------------------|
| `ALTER TABLE ... ADD COLUMN ... GENERATED ALWAYS AS (...) STORED` | `generated_column` |
| `ALTER TABLE ... ADD COLUMN ... GENERATED ALWAYS AS IDENTITY` | `generated_as_identity` |

各传输面的行为继续保持既有 unsupported 契约的一致分工：

- **CLI** 和 **`pkg/deltascope`** 返回带 `unsupported` 数组的部分结果，以及 `ErrUnsupportedStatement`。
- **HTTP** 和 **MCP** 将不支持语句作为传输层错误暴露，因为底层审计调用会返回错误。

## 置信度表面

本次发布在所有相关置信度层面锁定了这套边界契约：

- SQL 语料覆盖
- service 层检查
- CLI、HTTP、MCP 和 `pkg/deltascope` 的表面对等

本版本不新增 rule ID、不新增 CLI 标志，也不新增公共 API 契约。

## 未变更内容

- 相邻的 PostgreSQL generated / identity alteration 形态，如 `DROP EXPRESSION`、`SET GENERATED`、`DROP IDENTITY`，仍保持 generic unsupported 边界。
- DeltaScope 仍未将 generated expression 或 identity 语义建模为受支持的共享契约能力。
- MySQL 和 TiDB 的审核行为不变。
- `v0.26.0` 中针对 PostgreSQL `CREATE TABLE` 的 generated stored 列、identity 列、exclusion 约束和分区表 unsupported 边界仍然有效。

## 后续

- 决定是否还有其他 PostgreSQL `ALTER TABLE` generated / identity 形态需要稳定的显式 unsupported 子类型。
- 以后再决定这些显式 unsupported 边界是否应演进为真正的语义支持。

## 安装 / 升级

```bash
# macOS（推荐）
brew tap Fanduzi/deltascope
brew install --cask deltascope

# 通用安装脚本
curl -fsSL https://raw.githubusercontent.com/Fanduzi/DeltaScope/v0.30.0/install.sh | \
  DELTASCOPE_VERSION=v0.30.0 sh
```
