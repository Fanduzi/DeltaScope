# DeltaScope v0.23.0 发行说明

发布日期：2026-04-11

## 概览

DeltaScope `v0.23.0` 是 **PostgreSQL CREATE TABLE Coverage Pack**。该版本扩展了可进入共享审核管线的 PostgreSQL `CREATE TABLE` 结构，使常见的富约束建表语句能够返回正常审计结果，而不再落在未支持边界之外。

本版本不宣称已经完整支持 PostgreSQL DDL；也不新增 PostgreSQL 规则 ID、严重级别、CLI 标志、HTTP payload 契约、MCP tool 契约或公共 Go API 契约。

## 变更内容

### 更广的 PostgreSQL `CREATE TABLE` 覆盖范围

DeltaScope 现在支持并可审计更多常见 PostgreSQL `CREATE TABLE` 约束形式：

- 表级命名 `CHECK`
- 列级内联 `CHECK`
- 表级命名 `UNIQUE`
- 列级内联 `UNIQUE`
- 表级命名 `FOREIGN KEY`
- 列级内联 `REFERENCES`

### 共享规则复用

这是覆盖范围扩展，不是新的规则包。

- 当策略启用相应规则族时，既有结构化命名治理可用于命名 PostgreSQL `CHECK`、`UNIQUE` 和 `FOREIGN KEY` 约束。
- 现有共享索引规则可以消费内联 `UNIQUE` 产出的标准化索引事实。
- 内联 `REFERENCES` 仅作为 parser-owned 的共享结构暴露；不应被解读为新增 metadata-aware 外键语义。

### 接口一致性

扩展后的 PostgreSQL `CREATE TABLE` 覆盖已在以下接口确认一致：

- `deltascope` CLI
- HTTP `POST /v1/audit`
- MCP `audit_sql`
- 公共 Go API `pkg/deltascope`

## 兼容性

无破坏性变更。

- 既有 MySQL、TiDB 与 PostgreSQL 审计行为保持兼容。
- 不新增规则 ID、严重级别或触发条件。
- CLI、HTTP、MCP 与 `pkg/deltascope` 公共契约保持不变。
- `v0.22.0` 引入的 release-surface 入口仍是 package/docs 校验的规范路径。

## 安装 / 升级

```bash
curl -fsSL https://raw.githubusercontent.com/Fanduzi/DeltaScope/v0.23.0/install.sh | \
  DELTASCOPE_VERSION=v0.23.0 sh
```

macOS 用户可通过 Homebrew 安装：

```bash
brew tap Fanduzi/deltascope
brew install --cask deltascope
```
