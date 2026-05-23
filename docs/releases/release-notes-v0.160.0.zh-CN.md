# DeltaScope v0.160.0 发行说明

## 概要 — PostgreSQL ALTER TABLE 约束可延迟性规则

v0.160.0 新增 2 条 PostgreSQL 专用 ALTER TABLE 规则，覆盖外键约束可延迟性变更，并静默规范化 `SET WITHOUT OIDS`。PostgreSQL ALTER TABLE 残留普查显示 66 种形式中 56 种已 `finding_covered`（从 54 上升），`unsupported_boundary` 从 7 降至 4。PostgreSQL ALTER TABLE 规则总数升至 28 条。

## 新增规则

### 约束可延迟性（2 条）

| 规则 ID | 默认级别 | 触发条件 |
|---------|---------|---------|
| `ddl.pg.alter.constraint_deferrable.notice` | notice | `ALTER TABLE ... ALTER CONSTRAINT ... DEFERRABLE` 将外键约束标记为可延迟 |
| `ddl.pg.alter.constraint_initially_deferred.notice` | notice | `ALTER TABLE ... ALTER CONSTRAINT ... INITIALLY DEFERRED` 将外键约束标记为初始延迟 |

## 静默规范化

| 动作 | SQL 形式 | 行为 |
|------|---------|------|
| `set_without_oids` | `ALTER TABLE ... SET WITHOUT OIDS` | 静默规范化，不产生 finding。自 PostgreSQL 12 起已废弃（OID 已从用户表中移除）。 |

## 普查变化

| 指标 | v0.160.0 之前 | v0.160.0 之后 |
|------|-------------|-------------|
| total | 66 | 66 |
| finding_covered | 54 | 56 |
| normalized_silent | 1 | 2 |
| unsupported_boundary | 7 | 4 |
| parser_error | 4 | 4 |
| unclassified | 0 | 0 |

### 剩余 unsupported_boundary（4 项）

| SQL 形式 | 原因 |
|---------|------|
| `ALTER TABLE ... SET EXPRESSION AS (...)` | 表达式主体无法与列身份分离 |
| `ALTER TABLE ... ADD GENERATED ... AS IDENTITY` | 序列选项和身份语义需要仔细建模 |
| `ALTER TABLE ... EXCLUDE USING gist (...)` | 排除约束涉及操作符类和复杂谓词表达式 |
| `ALTER TABLE ALL IN TABLESPACE ... SET TABLESPACE ...` | 批量表空间移动是不同的语句类型（`AlterTableMoveAllStmt`） |

### 剩余 parser_error（4 项）

四种 PostgreSQL 18 语法形式仍超出范围，等待 `pg_query_go v7`（受 libpg_query 18 支持阻塞）。

## SQL 语料库

| 指标 | 值 |
|------|-----|
| policy_rule_ids | 340 |
| supported_rule_dialect_targets | 531 |
| covered_rule_dialect_targets | 531 |
| coverage_percent | 100.0 |
| expected_yaml_files_total | 239 |
| missing_rule_dialect_targets | 0 |

## 无泄漏合同

约束可延迟性 finding 仅输出有限元数据：

| 键 | 描述 |
|----|------|
| `operation` | `alter_table` |
| `action` | `alter_constraint_deferrable` 或 `alter_constraint_initially_deferred` |
| `table` | 目标表名 |
| `constraint` | 外键约束名 |
| `constraint_type` | `foreign_key` |
| `deferrable` | 布尔标志（`"true"` / `"false"`） |
| `initially_deferred` | 布尔标志（`"true"` / `"false"`） |

禁止元数据键（永不输出）：`raw_sql`、`expression`、`predicate`、`operator_class`、`exclusions`、`sequence_options`、`catalog_state`、`validation_result`、`dependency_graph`。

## 非目标

- 非完整 PostgreSQL ALTER TABLE 支持。
- 无 PostgreSQL 18 解析器支持。
- 无实时目录验证。
- 无运行时外键行为验证。
- 无锁/重写耗时估算。
- 尚不支持 `SET EXPRESSION`、`ADD GENERATED ... AS IDENTITY`、`EXCLUDE USING` 或 `ALTER TABLE ALL IN TABLESPACE ... SET TABLESPACE ...`。
- 无 v1.0/稳定 API 合同声明。
