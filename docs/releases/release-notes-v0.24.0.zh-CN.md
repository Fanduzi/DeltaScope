# DeltaScope v0.24.0 发行说明

发布日期：2026-04-11

## 概述

DeltaScope `v0.24.0` 是 **PostgreSQL CREATE TABLE 语义深化包**。它在 `v0.23.0` 已将常见 PostgreSQL 建表形态纳入共享审核管线的基础上，进一步丰富了这些形态所携带的共享语义信息，而不是扩展语法覆盖范围或新增规则 ID。

本版本不声称完整 PostgreSQL DDL 支持，也不新增 PostgreSQL 规则 ID、严重级别、CLI 标志、HTTP 负载契约、MCP 工具契约或公共 Go API 契约。

## 变更内容

### 更丰富的 PostgreSQL 外键语义

PostgreSQL `CREATE TABLE` 外键结构现在保留更多共享语义细节：

- `ReferencedTable` — 从 `REFERENCES table(column)` 形式中提取的被引用表名
- `ReferencedColumns` — 从中提取的被引用列名

具名 `FOREIGN KEY ... REFERENCES` 和列级内联 `REFERENCES` 均通过共享 `spec.Constraint` 模型传递这些事实。这些是**解析器拥有的共享契约事实**，不是实时元数据真相。它们代表 SQL 语句所声明的内容，而非数据库 schema 的当前状态。

### 共享规则复用继续

这是一次语义深化，不是新的规则包。

- 已有的共享命名治理（`ddl.constraint.foreign_key.name.*`）继续适用于携带更丰富形态的具名 `FOREIGN KEY` 约束。
- 已有的 `ddl.table.foreign_key.forbid` 继续对所有外键形式生效，包括携带更丰富语义的内联 `REFERENCES`。
- 当 FK-forbid 处于激活状态（默认策略）时，FK 命名规则仍被抑制。
- 不需要新增规则配置项。

### 不支持边界

相邻的不支持形态仍明确处于支持面之外：

- `GENERATED ... AS IDENTITY` — 在 `CREATE TABLE` 列定义中仍不支持
- `CREATE TABLE ... PARTITION BY` — 仍不支持
- `CREATE OR REPLACE VIEW` — 仍不支持

这些边界现在已通过服务层和公共 Go API 上的显式测试锁定。

### 产品面对齐

更丰富的 PostgreSQL 外键语义已在以下产品面确认对齐：

- `deltascope` CLI
- HTTP `POST /v1/audit`
- MCP `audit_sql`
- 公共 Go API `pkg/deltascope`

## 兼容性

无破坏性变更。

- 现有 MySQL、TiDB 和 PostgreSQL 审计行为保持兼容。
- 不新增规则 ID、严重级别或触发条件。
- CLI、HTTP、MCP 和 `pkg/deltascope` 公共契约保持不变。`ReferencedTable` 和 `ReferencedColumns` 字段是增量的，使用 `omitempty` JSON 编码。

## 安装 / 升级

```bash
curl -fsSL https://raw.githubusercontent.com/Fanduzi/DeltaScope/v0.24.0/install.sh | \
  DELTASCOPE_VERSION=v0.24.0 sh
```

macOS 用户可通过 Homebrew 安装：

```bash
brew tap Fanduzi/deltascope
brew install --cask deltascope
```
