# DeltaScope v0.54.0 发行说明

## 概要

v0.54.0 收尾 PostgreSQL ALTER TABLE 的高价值残余覆盖：DeltaScope 现在会规范化 `ENABLE/DISABLE TRIGGER ALL|USER` 与 `REPLICA IDENTITY` 变体，新增三条 PostgreSQL-only replica identity 发现，并让 `REPLICA IDENTITY DEFAULT` 作为干净的规范化通过路径。

## 标准化形式

| SQL | 标准化操作 |
|-----|-----------|
| `ALTER TABLE t ENABLE TRIGGER ALL` | `enable_trigger` (scope=`all`) |
| `ALTER TABLE t ENABLE TRIGGER USER` | `enable_trigger` (scope=`user`) |
| `ALTER TABLE t DISABLE TRIGGER ALL` | `disable_trigger` (scope=`all`) |
| `ALTER TABLE t DISABLE TRIGGER USER` | `disable_trigger` (scope=`user`) |
| `ALTER TABLE t REPLICA IDENTITY DEFAULT` | `replica_identity` (identity=`default`) |
| `ALTER TABLE t REPLICA IDENTITY FULL` | `replica_identity` (identity=`full`) |
| `ALTER TABLE t REPLICA IDENTITY NOTHING` | `replica_identity` (identity=`nothing`) |
| `ALTER TABLE t REPLICA IDENTITY USING INDEX idx` | `replica_identity` (identity=`using_index`, index=`idx`) |

## 新增 PostgreSQL-only 规则

| 规则 ID | 触发条件 | 默认级别 |
|---------|---------|---------|
| `ddl.pg.alter.replica_identity_full.warn` | `ALTER TABLE ... REPLICA IDENTITY FULL` | warning |
| `ddl.pg.alter.replica_identity_nothing.warn` | `ALTER TABLE ... REPLICA IDENTITY NOTHING` | warning |
| `ddl.pg.alter.replica_identity_using_index.notice` | `ALTER TABLE ... REPLICA IDENTITY USING INDEX ...` | notice |

- 触发器范围形式（`ALL`/`USER`）复用现有 `ddl.pg.alter.enable_trigger.notice` 和 `ddl.pg.alter.disable_trigger.warn` 规则。
- `REPLICA IDENTITY DEFAULT` 已规范化，故意静默通过——不触发任何规则。

## 测试覆盖

- AST 普查测试记录所有八种残余形式的稳定解析器事实。
- 解析器/提取器标准化测试覆盖触发器范围与 replica identity 变体。
- 语料库 fixture 覆盖三条新规则的触发形式。
- 通过 `AuditSQL` 对 replica identity 和触发器范围变体进行服务级测试。
- 四个公共面测试：`pkg/deltascope` Audit、CLI Execute、HTTP handler 和 MCP audit_sql tool。

## 非目标

- DeltaScope 不会检查在线触发器状态，也不验证触发器定义或函数。
- DeltaScope 不会验证 `REPLICA IDENTITY USING INDEX` 所引用的索引是否有效、唯一或非部分索引。
- 这不是完整的 PostgreSQL ALTER TABLE 语法支持。
- 不影响 MySQL/TiDB 行为。
- 除新增三条 PostgreSQL-only 规则外，不改变默认策略。

## 安装

**macOS（推荐）：**

```bash
brew tap fanduzi/deltascope
brew install --cask deltascope
```

**通用安装脚本：**

```bash
curl -fsSL https://raw.githubusercontent.com/Fanduzi/DeltaScope/v0.54.0/install.sh | \
  DELTASCOPE_VERSION=v0.54.0 sh
```
