# DeltaScope v0.18.0 发布说明

发布日期：2026-04-09

## 概览

DeltaScope `v0.18.0` 新增 **PostgreSQL metadata-aware 审计**，在 CLI、HTTP 和 MCP 三个接口上实现完整传输对等。这是 DeltaScope 首次支持连接到运行中的 PostgreSQL 实例，获取 schema 元数据，通过 `EXPLAIN` 进行 DML 影响估算，并基于真实数据库状态执行规则校验——与现有 MySQL/TiDB metadata-aware 体验对齐。

## 更新内容

### PostgreSQL Metadata-Aware 审计

DeltaScope 现在支持 PostgreSQL 12+ 的 metadata-aware 审计，与现有 MySQL 和 TiDB 支持并行：

- **Schema 解析**：自动解析 SQL 中的限定表名（`app.users`）；非限定表名通过 `--schema` 参数或跨 schema 唯一匹配推断解析
- **实例信息**：从 `pg_catalog` 获取 PostgreSQL 版本、数据库名称和 schema 目录
- **表快照**：通过 `information_schema` 和 `pg_indexes` 获取列定义、约束、主键、唯一约束和索引
- **DML 影响估算**：使用 PostgreSQL `EXPLAIN`（非 `EXPLAIN ANALYZE`）估算 `UPDATE` 和 `DELETE` 语句的受影响行数——保守、只读、不执行 DML

### 传输对等

三个传输面现在都支持 PostgreSQL metadata-aware 审计：

- **CLI**：`deltascope audit --dialect postgresql --host ... --port 5432 --user ...`
- **HTTP**：`POST /v1/audit`，带 `"dialect": "postgresql"` 和 `connection` 块
- **MCP**：`audit_sql` 工具，连接输入中带 `"dialect": "postgresql"`

### 新增 PostgreSQL Metadata-Aware 规则

针对 PostgreSQL 的规则覆盖：

- `ddl.alter.drop_primary_key.forbid` — 通过 `pg_constraint` 映射检测主键上的 `DROP CONSTRAINT`
- `ddl.alter.rename_column.exists.require` — 重命名前验证列是否存在
- `ddl.alter.rename_index.forbid` — 通过 `pg_indexes` owner 解析标记索引重命名
- `ddl.alter.drop_column.exists.require` — 删除前验证列是否存在
- `ddl.table.exists.create.forbid` — 通过元数据在 `CREATE TABLE` 前检查表是否存在

### E2E 测试覆盖

针对三个传输面使用真实 PostgreSQL 17 容器的完整端到端测试：

- CLI：通过 shell harness 执行 9 个测试用例（`test_cli_metadata_e2e_postgresql.sh`）
- HTTP：通过 Go `e2e && postgresql` 构建标签执行 9 个子测试
- MCP：通过 Go `e2e && postgresql` 构建标签执行 9 个子测试

### 文档

所有参考文档和概念文档已更新以反映 PostgreSQL metadata-aware 支持，包括 CLI 参数、HTTP API 示例、MCP 用法、能力矩阵和故障排除指南。

## 安装 / 升级

```bash
curl -fsSL https://raw.githubusercontent.com/Fanduzi/DeltaScope/v0.18.0/install.sh | \
  DELTASCOPE_VERSION=v0.18.0 sh
```

macOS 用户可使用 Homebrew 安装：

```bash
brew tap Fanduzi/deltascope
brew install --cask deltascope
```

## 兼容性

没有破坏性变更。`v0.18.0` 通过增量的 PostgreSQL metadata-aware 能力扩展了现有审计合同：

- 所有现有 MySQL/TiDB offline 和 metadata-aware 行为不变
- PostgreSQL offline 审计继续正常工作
- 新的 metadata-aware 模式通过连接参数按需启用
- `drop_primary_key` 规则现在会对 PostgreSQL `ALTER TABLE ... DROP CONSTRAINT` 主键触发
