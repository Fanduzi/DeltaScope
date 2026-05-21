# DeltaScope v0.140.0 发行说明

## 概要 — PostgreSQL ALTER TABLE 有界残留包

v0.140.0 新增 8 条 PostgreSQL 专用 ALTER TABLE 规则，覆盖列属性变更（5 条）和 CLUSTER / DETACH FINALIZE 操作（3 条）。PostgreSQL ALTER TABLE 残留普查现在显示 66 个形态中有 50 个 `finding_covered`（此前为 40），`unsupported_boundary` 从 21 减少到 11。

## 新增规则

### 列属性（5 条）

| 规则 ID | 默认级别 | 触发条件 |
|---------|---------|---------|
| `ddl.pg.alter.set_column_statistics.notice` | notice | `ALTER TABLE ... ALTER COLUMN ... SET STATISTICS` |
| `ddl.pg.alter.set_column_options.notice` | notice | `ALTER TABLE ... ALTER COLUMN ... SET (...)` 包含属性选项 |
| `ddl.pg.alter.reset_column_options.notice` | notice | `ALTER TABLE ... ALTER COLUMN ... RESET (...)` 包含属性选项 |
| `ddl.pg.alter.set_column_storage.notice` | notice | `ALTER TABLE ... ALTER COLUMN ... SET STORAGE` |
| `ddl.pg.alter.set_column_compression.notice` | notice | `ALTER TABLE ... ALTER COLUMN ... SET COMPRESSION` |

### CLUSTER / DETACH FINALIZE（3 条）

| 规则 ID | 默认级别 | 触发条件 |
|---------|---------|---------|
| `ddl.pg.alter.cluster_on.notice` | notice | `ALTER TABLE ... CLUSTER ON` |
| `ddl.pg.alter.set_without_cluster.notice` | notice | `ALTER TABLE ... SET WITHOUT CLUSTER` |
| `ddl.pg.alter.detach_partition_finalize.notice` | notice | `ALTER TABLE ... DETACH PARTITION ... FINALIZE` |

## 普查变化

| 指标 | v0.140.0 之前 | v0.140.0 之后 |
|------|-------------|-------------|
| total | 66 | 66 |
| finding_covered | 40 | 50 |
| normalized_silent | 1 | 1 |
| unsupported_boundary | 21 | 11 |
| parser_error | 4 | 4 |
| unclassified | 0 | 0 |

## SQL 语料

| 指标 | 值 |
|------|---|
| policy_rule_ids | 334 |
| supported_rule_dialect_targets | 525 |
| covered_rule_dialect_targets | 525 |
| coverage_percent | 100.0% |
| expected_yaml_files_total | 233 |

## 无泄漏契约

- **列属性**：发现不输出原始 SQL、选项名或值（如 `n_distinct`、`-1`、`100`）、存储参数名（`plain`、`external`、`extended`、`main`）、压缩方法名（`lz4`、`pglz`）或 `compression_kind`。
- **CLUSTER / DETACH FINALIZE**：发现不输出分区边界、catalog 索引名或实时验证声明。

## 非目标

- 不是完整的 PostgreSQL ALTER TABLE 支持。
- 不进行实时 catalog 验证。
- 不提供 rewrite 耗时估算。
- 不进行运行时行为验证。
- 不扩展 DCL。
- 剩余 `unsupported_boundary` 形态（11 项）推迟到后续里程碑。
- 不声称 v1.0/稳定 API 契约。
