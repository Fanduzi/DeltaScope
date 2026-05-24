# DeltaScope v0.170.0 发行说明

## 概要 — PostgreSQL ALTER TABLE 最终可解析边界规则

v0.170.0 新增 4 条 PostgreSQL 专用 ALTER TABLE notice 规则，覆盖最终可解析边界形式：SET EXPRESSION、ADD IDENTITY、ADD EXCLUSION CONSTRAINT 和 ALL IN TABLESPACE。PostgreSQL ALTER TABLE 残留普查显示 66 种形式中 60 种已 `finding_covered`（从 56 上升），`unsupported_boundary` 从 4 降至 0。PostgreSQL ALTER TABLE 规则总数升至 32 条。

## 新增规则

### 最终可解析边界（4 条）

| 规则 ID | 默认级别 | 触发条件 |
|---------|---------|---------|
| `ddl.pg.alter.set_expression.notice` | notice | `ALTER TABLE ... ALTER COLUMN ... SET EXPRESSION` 设置生成列表达式 |
| `ddl.pg.alter.add_identity.notice` | notice | `ALTER TABLE ... ALTER COLUMN ... ADD GENERATED ... AS IDENTITY` 为已有列添加身份 |
| `ddl.pg.alter.add_exclusion_constraint.notice` | notice | `ALTER TABLE ... ADD CONSTRAINT ... EXCLUDE USING` 添加排除约束 |
| `ddl.pg.alter.move_all_tablespace.notice` | notice | `ALTER TABLE ALL IN TABLESPACE ... SET TABLESPACE ...` 移动表空间中的所有表 |

## 普查变化

| 指标 | v0.170.0 之前 | v0.170.0 之后 |
|------|-------------|-------------|
| total | 66 | 66 |
| finding_covered | 56 | 60 |
| normalized_silent | 2 | 2 |
| unsupported_boundary | 4 | 0 |
| parser_error | 4 | 4 |
| unclassified | 0 | 0 |

### 剩余 parser_error（4 项）

四种 PostgreSQL 18 语法形式仍超出范围，等待 `pg_query_go v7`（受 libpg_query 18 支持阻塞）。

## SQL 语料库

| 指标 | 值 |
|------|-----|
| policy_rule_ids | 344 |
| supported_rule_dialect_targets | 535 |
| covered_rule_dialect_targets | 535 |
| coverage_percent | 100.0 |
| expected_yaml_files_total | 243 |
| missing_rule_dialect_targets | 0 |

## 无泄漏合同

最终可解析边界 finding 仅输出有限元数据：

| 键 | 描述 |
|----|------|
| `operation` | `alter_table` |
| `action` | `set_expression`、`add_identity`、`add_exclusion_constraint` 或 `move_all_tablespace` |
| `table` | 目标表名 |
| `column` | 目标列名（仅 SET EXPRESSION 和 ADD IDENTITY） |
| `constraint` | 约束名（仅 ADD EXCLUSION CONSTRAINT） |

禁止元数据键（永不输出）：`raw_sql`、`expression_body`、`sequence_options`、`exclusion_operators`、`exclusion_predicates`、`operator_class`、`catalog_state`、`tablespace_name`。

## 非目标

- 非完整 PostgreSQL ALTER TABLE 支持。
- 无 PostgreSQL 18 解析器支持。
- 无运行时/实时验证。
- 无重写耗时估算。
- 无 v1.0/稳定 API 合同声明。
