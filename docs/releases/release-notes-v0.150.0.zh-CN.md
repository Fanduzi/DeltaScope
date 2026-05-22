# DeltaScope v0.150.0 发行说明

## 概要 — PostgreSQL ALTER TABLE 表关系规则

v0.150.0 新增 4 条 PostgreSQL 专用 ALTER TABLE 表关系规则，覆盖继承关系（INHERIT / NO INHERIT）和 typed table 类型关联（OF / NOT OF）。PostgreSQL ALTER TABLE 规则总数从 22 条增至 26 条，残留普查中 `finding_covered` 从 50 提升至 54，`unsupported_boundary` 从 11 减少到 7。

## 新增规则

### 表关系（4 条）

| 规则 ID | 默认级别 | 触发条件 |
|---------|---------|---------|
| `ddl.pg.alter.add_inherit.notice` | notice | `ALTER TABLE ... INHERIT` 添加父表继承关系 |
| `ddl.pg.alter.drop_inherit.notice` | notice | `ALTER TABLE ... NO INHERIT` 移除父表继承关系 |
| `ddl.pg.alter.add_of_type.notice` | notice | `ALTER TABLE ... OF` 将表与 typed table 复合类型关联 |
| `ddl.pg.alter.drop_of_type.notice` | notice | `ALTER TABLE ... NOT OF` 移除 typed table 关联 |

## 普查变化

| 指标 | v0.140.0 之后 | v0.150.0 之后 |
|------|-------------|-------------|
| total | 66 | 66 |
| finding_covered | 50 | 54 |
| normalized_silent | 1 | 1 |
| unsupported_boundary | 11 | 7 |
| parser_error | 4 | 4 |
| unclassified | 0 | 0 |

## SQL 语料

| 指标 | 值 |
|------|---|
| supported_rule_dialect_targets | 529 |
| covered_rule_dialect_targets | 529 |
| coverage_percent | 100.0% |
| expected_yaml_files_total | 237 |

## 无泄漏契约

- **表关系**：发现不输出父表名、类型名、`catalog_state`、`dependency_graph`、`type_shape`、`column_shape` 或 `validation_result`。

## 非目标

- 不是完整的 PostgreSQL ALTER TABLE 支持。
- 不进行实时 catalog 验证。
- 不进行父表/类型兼容性验证。
- 不进行 PostgreSQL 18 parser 支持。
- 剩余 7 个 `unsupported_boundary` 形态推迟到后续里程碑。
- 4 个 `parser_error` 形态等待 `pg_query_go` v7。
