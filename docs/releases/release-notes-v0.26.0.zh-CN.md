# DeltaScope v0.26.0 发行说明

发布日期：2026-04-12

## 概述

DeltaScope `v0.26.0` 是 **PostgreSQL CREATE TABLE 不支持边界收口包**。它在提取器层收口了 PostgreSQL `CREATE TABLE` 中明确不在支持范围内的语法边界，每条边界都有语料用例和表面对等测试锁定。本版本不新增规则、CLI 标志或公共 API 契约，也不代表完整的 PostgreSQL `CREATE TABLE` 支持。

## 变更内容

PostgreSQL 提取器现在将四条 `CREATE TABLE` 边界标记为显式 unsupported：

| 特性 | 提取器标签 | 示例 |
|------|-----------|------|
| Identity 列 | `generated_as_identity` | `id bigint GENERATED ALWAYS AS IDENTITY` |
| Generated stored 列 | `generated_column` | `full_name text GENERATED ALWAYS AS (...) STORED` |
| Exclusion 约束 | `exclusion_constraint` | `EXCLUDE USING gist (...)` |
| 分区表 | `partitioning` | `PARTITION BY RANGE (created_at)` |

此前，部分语法被静默接受或部分处理。现在它们会被显式拒绝并附带清晰的 `unsupported` 原因，使边界契约稳定且可测试。

## 边界契约

每条边界由三层锁定：

1. **提取器层**：PostgreSQL 提取器返回包含特性标签和原因字符串的 `UnsupportedDetail`。
2. **语料层**：`testdata/sql-corpus/postgresql/` 包含专用用例，`.expected.yaml` 断言 `unsupported.count` 和 `unsupported.include`。
3. **表面层**：CLI、HTTP、MCP 和 `pkg/deltascope` 的表面对等测试验证每条传输通道上的 unsupported 契约。

## Surface 契约

不支持语句在不同传输通道上的暴露方式不同：

- **CLI** 和 **`pkg/deltascope`**：返回带 `unsupported` 数组（包含 `feature` 和 `reason` 字段）的部分结果，以及 `ErrUnsupportedStatement` 哨兵错误。CLI 渲染结果（包括 unsupported 部分）并以审计退出码退出。
- **HTTP** 和 **MCP**：将不支持语句作为传输层错误暴露（HTTP 错误响应、MCP tool error 并设置 `IsError: true` 及结构化错误内容）。底层 `deltascope.Audit` 对不支持边界返回错误，传输适配器将其传播。

## 未变更内容

- 没有新增规则 ID。这些边界是提取器层的 unsupported 契约，不是规则 finding。
- 没有新增 CLI 标志、HTTP 负载契约、MCP 工具契约或公共 Go API 契约。
- `v0.23.0` 和 `v0.24.0` 中已支持的 PostgreSQL `CREATE TABLE` 语义（命名 CHECK、UNIQUE、FOREIGN KEY、内联 REFERENCES、内联 UNIQUE）未变更。
- MySQL 和 TiDB 审核面未变更。

## 后续

- **带 schema 限定的外键引用**仍为决策点。如果保留 schema 需要扩展共享契约，`ReferencedSchema` 将在后续里程碑中处理，而非混入边界收口。
- **`ALTER TABLE ... GENERATED`** 边界覆盖是潜在后续工作，但尚未承诺。

## 安装 / 升级

```bash
# macOS（推荐）
brew tap Fanduzi/deltascope
brew install --cask deltascope

# 通用安装脚本
curl -fsSL https://raw.githubusercontent.com/Fanduzi/DeltaScope/v0.26.0/install.sh | \
  DELTASCOPE_VERSION=v0.26.0 sh
```
