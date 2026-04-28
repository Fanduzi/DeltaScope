# DeltaScope v0.48.0 发行说明

## 摘要

v0.48.0 发布 PostgreSQL DDL Coverage Census & Gap Closure Pack。通过系统性地审核 56 个代表性 PostgreSQL DDL 语句形式的全管线行为，识别了覆盖盲区并以四条新的 PostgreSQL-only 规则、`ALTER TABLE ADD COLUMN` NOT NULL / DEFAULT 约束的提取器修复以及扩展的 SQL 语料库覆盖来闭合这些盲区。

## 变更

- 新增四条 PostgreSQL-only DDL 规则，注册在 `ddl.pg.*` 命名空间下：
  - `ddl.pg.drop_index.advisory` — 当 `DROP INDEX` 移除索引时发出 notice，建议审查依赖查询（默认级别：`notice`）。
  - `ddl.pg.alter.add_column.non_null_no_default.warn` — 当 `ALTER TABLE ADD COLUMN` 添加 `NOT NULL` 列但未指定 `DEFAULT` 子句时发出 warning，因为可能导致大表全表重写（默认级别：`warning`）。
  - `ddl.pg.alter.add_unique_constraint.concurrent_index.advisory` — 当 `ALTER TABLE ADD UNIQUE CONSTRAINT` 不包含 `NOT VALID` 且后续没有 `CREATE UNIQUE INDEX CONCURRENTLY` 时发出 notice，建议使用并发索引创建以实现零停机部署（默认级别：`notice`）。
  - `ddl.pg.alter.drop_constraint.advisory` — 当 `ALTER TABLE DROP CONSTRAINT` 移除 CHECK、UNIQUE 或 FOREIGN KEY 约束时发出 notice，建议审查依赖查询和数据完整性影响（默认级别：`notice`）。
- PostgreSQL 提取器修复：`ALTER TABLE ADD COLUMN` 上的 `CONSTR_NOTNULL` 和 `CONSTR_DEFAULT` 约束现在正确填充规范化 spec 中列的 `NotNull` 和 `Default` 字段，使依赖这些事实的规则能正确评估。
- SQL 语料库扩展，新增覆盖全部四条新规则的 PostgreSQL DDL finding 用例。
- Census 特征化测试锁定 56 条形式清单：总计 56，可解析 56，已分类 34，已规范化 34，有 finding 覆盖 31，规范化静默通过 3，显式不支持 22，解析错误 0。

## 验证

- `make release-contract-gates VERSION=v0.48.0` — 所有门控通过
- `make sql-corpus-gates` — 语料库覆盖门控通过
- `make sql-corpus-report` — 报告当前支持规则覆盖清单
- `go test ./... -count=1` — 所有单元测试通过

## 非目标

- 不涉及新的 MySQL 或 TiDB 规则 ID、解析器特性或策略变更。
- 不涉及 `CREATE INDEX CONCURRENTLY`、`ALTER TABLE VALIDATE CONSTRAINT` 或 `ALTER TABLE DROP COLUMN` 行为变更——它们在本里程碑保持规范化静默通过。
- 不涉及公共 API 契约变更。`hasColumnConstraint` 是内部辅助函数，不是导出符号。
- 不涉及发布产物命名或 npm launcher 行为变更。

## 安装

**macOS（推荐）：**

```bash
brew tap fanduzi/deltascope
brew install --cask deltascope
```

**通用安装脚本：**

```bash
curl -fsSL https://raw.githubusercontent.com/Fanduzi/DeltaScope/v0.48.0/install.sh | \
  DELTASCOPE_VERSION=v0.48.0 sh
```
