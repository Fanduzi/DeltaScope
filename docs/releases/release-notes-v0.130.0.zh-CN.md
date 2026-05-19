# DeltaScope v0.130.0 发行说明

## 概要 — PostgreSQL ALTER TABLE 残留覆盖 / 语义深度

v0.130.0 深化 PostgreSQL ALTER TABLE 残留覆盖，新增 10 条规则，覆盖三个语义家族：存储/布局、trigger/rule 残留和 reloptions。PostgreSQL ALTER TABLE 残留普查现在显示 66 个形态中有 40 个 `finding_covered`（此前为 29），`unsupported_boundary` 从 32 减少到 21。

## 新增规则

### 存储 / 布局（2 条）

| 规则 ID | 默认级别 | 触发条件 |
|---------|---------|---------|
| `ddl.pg.alter.set_tablespace.notice` | notice | `ALTER TABLE ... SET TABLESPACE ...` |
| `ddl.pg.alter.set_access_method.warn` | warning | `ALTER TABLE ... SET ACCESS METHOD ...` |

### Trigger / Rule 残留（6 条）

| 规则 ID | 默认级别 | 触发条件 |
|---------|---------|---------|
| `ddl.pg.alter.enable_replica_trigger.notice` | notice | `ALTER TABLE ... ENABLE REPLICA TRIGGER ...` |
| `ddl.pg.alter.enable_always_trigger.notice` | notice | `ALTER TABLE ... ENABLE ALWAYS TRIGGER ...` |
| `ddl.pg.alter.enable_rule.notice` | notice | `ALTER TABLE ... ENABLE RULE ...` |
| `ddl.pg.alter.disable_rule.warn` | warning | `ALTER TABLE ... DISABLE RULE ...` |
| `ddl.pg.alter.enable_replica_rule.notice` | notice | `ALTER TABLE ... ENABLE REPLICA RULE ...` |
| `ddl.pg.alter.enable_always_rule.notice` | notice | `ALTER TABLE ... ENABLE ALWAYS RULE ...` |

### Reloptions（2 条）

| 规则 ID | 默认级别 | 触发条件 |
|---------|---------|---------|
| `ddl.pg.alter.set_reloptions.warn` | warning | `ALTER TABLE ... SET (...)` 包含 reloption 键 |
| `ddl.pg.alter.reset_reloptions.notice` | notice | `ALTER TABLE ... RESET (...)` 包含 reloption 键 |

## 普查变化

| 指标 | v0.130.0 之前 | v0.130.0 之后 |
|------|-------------|-------------|
| total | 66 | 66 |
| finding_covered | 29 | 40 |
| normalized_silent | 1 | 1 |
| unsupported_boundary | 32 | 21 |
| parser_error | 4 | 4 |
| unclassified | 0 | 0 |

## SQL 语料

| 指标 | 值 |
|------|---|
| policy_rule_ids | 326 |
| supported_rule_dialect_targets | 517 |
| covered_rule_dialect_targets | 517 |
| coverage_percent | 100.0% |
| expected_yaml_files_total | 225 |

## 无泄漏契约

- **存储/布局**：发现不输出原始 tablespace 名称、access method 名称或实时验证声明。
- **Trigger/rule 残留**：发现不输出 trigger 函数名、trigger 函数体、rule 查询文本或 rule 命令文本。
- **Reloptions**：发现不输出选项名或值（如 `fillfactor`、`autovacuum_enabled`、`70`、`false`）。

## 非目标

- 不是完整的 PostgreSQL ALTER TABLE 支持。
- 不进行实时 catalog 验证。
- 不提供 rewrite 耗时估算。
- 不进行运行时行为验证。
- 不扩展 DCL。
- 剩余 `unsupported_boundary` 形态推迟到后续里程碑。
- 不声称 v1.0/稳定 API 契约。
