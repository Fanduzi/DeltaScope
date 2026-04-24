# DeltaScope v0.43.0 发行说明

## 概览

DeltaScope v0.43.0 引入默认策略方言隔离。当设置 `--dialect postgresql` 时，DeltaScope 不再发出 MySQL/TiDB-only 规则 ID 或 MySQL 特有的修复建议文本。当设置 `--dialect mysql` 或 `--dialect tidb` 时，DeltaScope 不再发出 PostgreSQL-only 规则 ID。隔离在规则 `AppliesTo` 门控层实现，不是后期过滤。

## 新增内容

- 默认策略按 `--dialect` 隔离规则，覆盖 MySQL、TiDB 和 PostgreSQL。
- PostgreSQL 审核跳过 MySQL-family 规则：
  - `ddl.table.engine.allowlist`
  - `ddl.table.charset.allowlist`
  - `ddl.table.row_format.allowlist`
  - `ddl.table.auto_increment.init_value.require`
  - `ddl.table.primary_key.unsigned.require`
  - `ddl.table.primary_key.auto_increment.require`
  - `ddl.table.primary_key.not_null.require`
  - `ddl.table.partition.forbid`
  - `ddl.table.create_as.forbid`
  - `ddl.table.create_like.forbid`
  - `ddl.column.charset.allowlist`
  - `ddl.column.collation.allowlist`
  - `ddl.column.charset_collation.match.require`
  - `ddl.alter.change_column.forbid`
  - `ddl.alter.modify_column.forbid`
- PostgreSQL `CREATE TABLE` 审核不再建议 MySQL-only 的 `ON UPDATE CURRENT_TIMESTAMP` 更新时间审计列。
- MySQL/TiDB 审核排除所有 `ddl.pg.*` 规则和 PostgreSQL-only 的无前缀方言门控规则。
- 服务级测试断言跨方言规则隔离：
  - `TestPostgreSQLDefaultAuditExcludesMySQLFamilyRules`
  - `TestPostgreSQLDefaultAuditExcludesMySQLRemediationText`
  - `TestMySQLDefaultAuditExcludesPostgreSQLRules`
- SQL corpus PostgreSQL 探针新增负面 `exclude:` 块，列出不允许出现的 MySQL-family 规则。

## 示例

v0.43.0 之前的 PostgreSQL 审核可能发出 MySQL-only findings：

```text
[blocker] ddl.table.charset.allowlist: table charset is not in the allowlist
[blocker] ddl.table.primary_key.unsigned.require: single primary-key column should be unsigned bigint
```

v0.43.0 的 PostgreSQL 审核只产生方言合适的 findings：

```bash
deltascope audit \
  --dialect postgresql \
  --sql "CREATE TABLE users (id bigint PRIMARY KEY, name varchar(64) NOT NULL);"
```

## 规则契约

| 字段 | 值 |
|------|----|
| 类型 | 默认策略方言隔离 |
| 实现层 | 规则 `AppliesTo` 门控 |
| 范围 | MySQL-family 规则从 PostgreSQL 排除；PostgreSQL-only 规则从 MySQL/TiDB 排除 |
| 共享规则 | 在所有方言下保持活跃（注释、命名、PK 存在性、PK 列数） |

## 非目标

- 新规则 ID
- 新解析器功能
- 新 public API contract
- Live schema 校验
- 跨数据库或跨部署窗口追踪
- 除方言隔离外的 MySQL/TiDB 行为变更
- PostgreSQL 类型规范化（`pg_catalog.int8` 标准化）

## 安装

```bash
curl -fsSL https://raw.githubusercontent.com/Fanduzi/DeltaScope/v0.43.0/install.sh | \
  DELTASCOPE_VERSION=v0.43.0 sh
```
