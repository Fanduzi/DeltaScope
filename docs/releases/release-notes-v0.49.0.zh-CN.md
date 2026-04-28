# DeltaScope v0.49.0 发行说明

## 概要

v0.49.0 发布 PostgreSQL 高级 CREATE INDEX 规范化包。PostgreSQL 部分索引、表达式索引、INCLUDE 覆盖索引以及非 btree 索引现在走规范化路径而非返回 unsupported。这些形式现在通过正常审核管线并触发已有的 `ddl.pg.create_index.concurrently.require` 规则（当缺少 `CONCURRENTLY` 时）。

## 新增

- 高级 PostgreSQL `CREATE INDEX` 变体现在规范化为粗粒度索引事实：
  - 部分索引（`WHERE` 子句）— `HasPredicate` 标记
  - 表达式索引（`LOWER(col)` 等）— `HasExpressionKeys` 标记和 `ExpressionCount`
  - INCLUDE 覆盖索引 — `IncludedColumns` 列表
  - 非 btree 访问方法（`USING gin`、`USING hash` 等）— `AccessMethod` 字段
- `spec.Index` 新增五个字段：`AccessMethod`、`IncludedColumns`、`HasPredicate`、`HasExpressionKeys`、`ExpressionCount`。
- 全部五种高级索引形式的服务层测试（通过 `AuditSQL`）。
- 部分索引、表达式索引、INCLUDE 索引和 GIN 索引的语料库 fixture。
- 四个公共表面的测试覆盖：`pkg/deltascope` Audit、CLI Execute、HTTP handler 和 MCP audit_sql 工具。

## 变更

- PostgreSQL extractor 不再对部分索引、表达式索引、INCLUDE 和非 btree 的 `CREATE INDEX` 变体返回 unsupported。
- 从 v0.48 到 v0.49 的普查指标变化：

  | 指标 | v0.48 | v0.49 |
  |------|-------|-------|
  | finding-covered | 31 | 35 |
  | unsupported-explicit | 22 | 18 |
  | classified DDL | 34 | 38 |
  | normalized | 34 | 38 |
  | corpus-covered | 19/56 | 23/56 |
  | parseable | 56 | 56 |
  | parser-error | 0 | 0 |

## 非目标

- 没有新增规则 ID。已有的 `ddl.pg.create_index.concurrently.require` 规则现在覆盖新规范化的形式。
- 没有默认策略变更。
- 没有 MySQL/TiDB 行为变更。
- 没有 predicate SQL 或 expression SQL 语义分析。DeltaScope 仅保留粗粒度存在性/计数标记。
- 公共响应类型（`StatementResult`、CLI JSON、HTTP JSON、MCP 结构化内容）尚未暴露完整的内部 `spec.Index` 高级字段。这属于未来的表面扩展。
- 剩余 18 个 unsupported PG DDL 形式仍为显式边界。

## 安装

**macOS（推荐）：**

```bash
brew tap fanduzi/deltascope
brew install --cask deltascope
```

**通用安装脚本：**

```bash
curl -fsSL https://raw.githubusercontent.com/Fanduzi/DeltaScope/v0.49.0/install.sh | \
  DELTASCOPE_VERSION=v0.49.0 sh
```
