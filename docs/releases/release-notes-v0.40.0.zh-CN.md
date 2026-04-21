# DeltaScope v0.40.0 发行说明

## 概述

DeltaScope v0.40.0 为已批准的 PostgreSQL `ALTER TABLE ... ADD CONSTRAINT FOREIGN KEY` 形态保留语句级外键事实，使已有的 FK 规则可以在 CLI、HTTP、MCP 和 `pkg/deltascope` 四条产品面上产生 findings。

## 变更内容

### PostgreSQL ALTER TABLE 外键事实支持

PostgreSQL `ALTER TABLE ... ADD CONSTRAINT ... FOREIGN KEY` 形态现在通过 PostgreSQL 提取器保留语句级 FK 事实。`DDL.Constraints` 投影使已有的 FK 规则可以触发 ALTER TABLE FK 添加。

| 保留事实 | 说明 |
|---------|------|
| 本地列 | 参与外键的拥有表列 |
| 被引用表 | 外键引用的目标表 |
| 被引用列 | 被引用表中的列 |
| 被引用 Schema | 使用 `schema.table` 形式引用时保留的 schema 限定符 |

### 规则覆盖解锁

| Rule ID | 触发条件 |
|---------|---------|
| `ddl.table.foreign_key.forbid` | 默认策略下外键约束被禁止 |
| `ddl.pg.table.foreign_key.cross_schema.advisory` | 拥有表 schema 和被引用 schema 均为显式且不同（notice） |

### 公共接口

四条产品面均产生显式 `rule_id` 发现：

| 接口 | 行为 |
|------|------|
| CLI | 正常审核输出，包含 `rule_id` findings |
| HTTP（`POST /v1/audit`） | 正常审核响应，findings 含明确 `rule_id` |
| MCP（`audit_sql`） | 正常 tool result，findings 含明确 `rule_id` |
| `pkg/deltascope` | `Audit()` 返回带 findings 的 result，含明确 `rule_id` |

### Docker-backed E2E 覆盖

PostgreSQL CLI e2e 通过 Docker-backed 测试路径覆盖了 `ddl.table.foreign_key.forbid` 的语句局部 `ALTER TABLE ... ADD CONSTRAINT ... FOREIGN KEY` 审计。

## 已支持形态

- 命名 FK：`ALTER TABLE orders ADD CONSTRAINT fk_orders_user FOREIGN KEY (user_id) REFERENCES users(id)`
- 复合 FK 事实（已测试形态）
- Schema-qualified 引用保留 `referenced_schema`

## 非目标

- 不做在线 schema FK 存在性验证。
- 不新增 FK rule ID——已有规则通过扩展适用性和 `DDL.Constraints` 投影覆盖 ALTER TABLE FK 添加。
- 不做可延迟约束支持或 MATCH FULL 策略扩展。
- 不声称完整约束/索引对等。
- 无 MySQL/TiDB 行为变更。

## 升级说明

无需配置或策略变更。已为 `CREATE TABLE` FK 声明激活的 FK 规则现在也适用于 `ALTER TABLE ... ADD CONSTRAINT ... FOREIGN KEY` 语句（当设置 `--dialect postgresql` 时）。

## 安装

```bash
curl -fsSL https://raw.githubusercontent.com/Fanduzi/DeltaScope/v0.40.0/install.sh | \
  DELTASCOPE_VERSION=v0.40.0 sh
```
