# DeltaScope v0.52.0 发行说明

## 概要

v0.52.0 将之前 unsupported 的 6 类 PostgreSQL ALTER TABLE 动作纳入审核管线。DeltaScope 现在对 `ALTER TABLE ... SET SCHEMA`、`ALTER TABLE ... OWNER TO`、`ALTER TABLE ... ENABLE/DISABLE TRIGGER name` 以及 `ALTER TABLE ... ATTACH/DETACH PARTITION` 进行标准化处理和规则匹配，不再返回 unsupported 结果。

## 新增

- 六条新 PostgreSQL-only 规则：

| 规则 ID | 触发动作 | 默认级别 |
|---------|---------|---------|
| `ddl.pg.alter.set_schema.advisory` | `ALTER TABLE ... SET SCHEMA` | notice |
| `ddl.pg.alter.owner.advisory` | `ALTER TABLE ... OWNER TO` | notice |
| `ddl.pg.alter.enable_trigger.notice` | `ALTER TABLE ... ENABLE TRIGGER name` | notice |
| `ddl.pg.alter.disable_trigger.warn` | `ALTER TABLE ... DISABLE TRIGGER name` | warning |
| `ddl.pg.alter.attach_partition.advisory` | `ALTER TABLE ... ATTACH PARTITION` | notice |
| `ddl.pg.alter.detach_partition.warn` | `ALTER TABLE ... DETACH PARTITION` | warning |

- 针对全部六类动作的解析器/提取器标准化处理。
- 每条规则触发形式的语料库 fixture。
- 全部六条规则的服务层测试（通过 `AuditSQL`）。
- 四个公共表面的测试覆盖：`pkg/deltascope` Audit、CLI Execute、HTTP handler 和 MCP audit_sql 工具。
- AST census 测试记录每类动作的稳定解析器事实。

## 非目标

- 这不是完整的 PostgreSQL ALTER TABLE 语法支持。剩余 ALTER TABLE 子命令（如 `ALTER COLUMN TYPE`、`ADD CONSTRAINT ... NOT VALID`、`ENABLE/DISABLE TRIGGER ALL/USER`、`REPLICA IDENTITY` 等）仍为显式边界。
- 不对分区边界进行语义分析。`ATTACH PARTITION ... FOR VALUES` 的边界不会与父分区方案进行校验。
- `ENABLE/DISABLE TRIGGER ALL` 和 `ENABLE/DISABLE TRIGGER USER` 变体仍为延期项。
- 没有 MySQL/TiDB 行为变更。
- 除六条新 PostgreSQL-only 规则条目外，没有默认策略变更。

## 安装

**macOS（推荐）：**

```bash
brew tap fanduzi/deltascope
brew install --cask deltascope
```

**通用安装脚本：**

```bash
curl -fsSL https://raw.githubusercontent.com/Fanduzi/DeltaScope/v0.52.0/install.sh | \
  DELTASCOPE_VERSION=v0.52.0 sh
```
