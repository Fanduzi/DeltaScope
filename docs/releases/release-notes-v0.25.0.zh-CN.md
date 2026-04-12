# DeltaScope v0.25.0 发行说明

发布日期：2026-04-12

## 概述

DeltaScope `v0.25.0` 是 **SQL 语料库与边界置信度包**。它引入了一套持久的、表驱动的 SQL 语料库，覆盖 MySQL、TiDB 和 PostgreSQL 的代表性基线用例，并通过双层断言验证预期结果。本版本不新增规则、CLI 标志、HTTP 负载契约、MCP 工具契约或公共 Go API 契约。

语料库回答的是一个实际的发布信心问题：哪些代表性 SQL 语句实际通过了 DeltaScope，预期结果是什么？

## 变更内容

### SQL 语料库

新增 `testdata/sql-corpus/` 目录，包含按方言组织的 SQL 示例及其预期审计结果：

- 每个用例由 `.sql` 文件 + `.expected.yaml` 文件对组成。
- 语料库通过现有 `AuditSQL` 应用层驱动——不引入新的运行时代码路径。
- 语料测试通过 `go test ./internal/application/audit` 运行。

### 双层断言

每个语料用例在两个层面进行验证：

1. **报告层断言**——运行完整审计管线，检查 `unsupported.count`、`statement_kind`、`findings.include` 和 `findings.exclude`。
2. **语义解析/提取层断言**——通过内部 `Parse` + `Extract` 路径访问报告未暴露的 `spec.Statement` 字段，断言 `operation`（DDL/DML 操作名）和 `facts.constraints`（约束类型、名称、列、被引用表/列）。

两个层都由同一个 `.expected.yaml` 文件驱动。语义层仅在预期文件包含 `operation` 或 `facts` 字段时运行。

### 语料覆盖范围

| 方言 | DDL | DML | 类别 |
|------|-----|-----|------|
| MySQL | `CREATE TABLE`（主键、外键） | `UPDATE`、`DELETE` | supported、findings、clean |
| TiDB | `CREATE TABLE`（主键） | `UPDATE`、`DELETE` | supported、findings、clean |
| PostgreSQL | `CREATE TABLE`（CHECK、UNIQUE、FOREIGN KEY、REFERENCES）、`CREATE OR REPLACE VIEW` | — | supported、unsupported、findings、boundary |

### 边界发现

PostgreSQL 语料记录了两个当前边界用例：

- `GENERATED ... AS IDENTITY`——作为边界 finding 记录；本版本未修复。
- `PARTITION BY`——作为边界 finding 记录。

这些在 `v0.25.0` 中有意不做修复。语料库捕获当前行为，以便后续修复时可以对照稳定基线验证。

### 未变更内容

- 不新增规则或规则配置项。
- 不新增 CLI 标志或输出格式。
- 不改变 HTTP、MCP 或公共 Go API 契约。
- 不改变解析器行为或规则评估逻辑。
- `GENERATED ... AS IDENTITY` 仍是不支持的边界。

## 后续待办

以下作为独立的后续里程碑跟踪：

- **PostgreSQL CREATE TABLE Unsupported Boundary Pack**——负责修复 `GENERATED ... AS IDENTITY`、生成存储列、分区建表、排他约束和 schema 限定外键引用等边界。此项有意与语料置信度工作分离。

## 兼容性

无破坏性变更。

- 现有 MySQL、TiDB 和 PostgreSQL 审计行为不变。
- 语料测试是开发者/发布信心资产，不影响终端用户审计行为。

## 安装 / 升级

```bash
curl -fsSL https://raw.githubusercontent.com/Fanduzi/DeltaScope/v0.25.0/install.sh | \
  DELTASCOPE_VERSION=v0.25.0 sh
```

macOS 用户可通过 Homebrew 安装：

```bash
brew tap Fanduzi/deltascope
brew install --cask deltascope
```
