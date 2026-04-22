# DeltaScope v0.41.0 发行说明

## 概述

DeltaScope v0.41.0 为已批准的 PostgreSQL `ALTER TABLE ... ADD CONSTRAINT CHECK` 形态保留语句级 CHECK 约束事实，使已有的 CHECK 命名规则和 PostgreSQL `NOT VALID` 建议规则可以在 CLI、HTTP、MCP 和 `pkg/deltascope` 四条产品面上产生 findings。

## 变更内容

### PostgreSQL ALTER TABLE CHECK 约束事实支持

PostgreSQL `ALTER TABLE ... ADD CONSTRAINT ... CHECK` 形态现在通过 PostgreSQL 提取器保留语句级 CHECK 约束元数据。`DDL.Constraints` 投影使已有的 CHECK 命名规则和 PostgreSQL `NOT VALID` 建议规则可以触发 ALTER TABLE CHECK 添加。

| 保留事实 | 说明 |
|---------|------|
| 约束名称 | 显式命名的 CHECK 约束标识符 |
| CHECK 表达式 | 定义约束的布尔表达式 |

### 规则覆盖解锁

| Rule ID | 触发条件 |
|---------|---------|
| `ddl.pg.alter.add_check.not_valid.require` | `ADD CHECK` 约束应使用 `NOT VALID` 以避免持 `ACCESS EXCLUSIVE` 锁的全表扫描 |
| `ddl.constraint.check.name.prefix.require` | 显式命名的 CHECK 约束未以要求的 structured naming 前缀开头（配置后生效） |
| `ddl.constraint.check.name.suffix.require` | 显式命名的 CHECK 约束未以要求的 structured naming 后缀结尾（配置后生效） |
| `ddl.constraint.check.name.contains.require` | 显式命名的 CHECK 约束未包含任一已配置的 structured naming token（配置后生效） |

### 公共接口

四条产品面均产生显式 `rule_id` 发现：

| 接口 | 行为 |
|------|------|
| CLI | 正常审核输出，包含 `rule_id` findings |
| HTTP（`POST /v1/audit`） | 正常审核响应，findings 含明确 `rule_id` |
| MCP（`audit_sql`） | 正常 tool result，findings 含明确 `rule_id` |
| `pkg/deltascope` | `Audit()` 返回带 findings 的 result，含明确 `rule_id` |

### Docker-backed E2E 覆盖

PostgreSQL CLI e2e 通过 Docker-backed 测试路径覆盖了 `ddl.pg.alter.add_check.not_valid.require` 和 `ddl.constraint.check.name.prefix.require` 的语句局部 `ALTER TABLE ... ADD CONSTRAINT ... CHECK` 审计。

## 已支持形态

- 命名 CHECK：`ALTER TABLE orders ADD CONSTRAINT amount_positive CHECK (amount >= 0)`
- 配置驱动的命名治理：`ddl.constraint.check.name.prefix.require`，前缀 `ck_`

## 非目标

- 不做在线 schema CHECK 存在性验证。
- 不新增规则 ID——`ddl.pg.alter.add_check.not_valid.require` 已注册；CHECK 命名规则通过扩展适用性覆盖 ALTER CHECK 路径。
- 不做 `NOT VALID` 校验强制或可延迟约束支持。
- 无 MySQL/TiDB 行为变更。

## 升级说明

无需配置或策略变更。`ddl.pg.alter.add_check.not_valid.require` 规则在设置 `--dialect postgresql` 时对 `ALTER TABLE ... ADD CONSTRAINT ... CHECK` 语句默认触发。CHECK 命名规则需要显式配置（`prefix`、`suffix` 或 `contains` 参数）才会产生 findings。

## 安装

```bash
curl -fsSL https://raw.githubusercontent.com/Fanduzi/DeltaScope/v0.41.0/install.sh | \
  DELTASCOPE_VERSION=v0.41.0 sh
```
