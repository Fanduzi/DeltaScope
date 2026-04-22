# DeltaScope v0.42.0 发行说明

## 概览

DeltaScope v0.42.0 新增 PostgreSQL NOT VALID 约束校验配对能力。当命名的 `ALTER TABLE ... ADD CONSTRAINT ... NOT VALID` CHECK 或 FOREIGN KEY 约束在同一次审计 SQL 批次中没有被后续匹配的 `ALTER TABLE ... VALIDATE CONSTRAINT ...` 语句跟随时，DeltaScope 会发出 warning。

## 新增内容

- 新的 PostgreSQL-only GlobalRule：`ddl.pg.alter.not_valid_constraint.validate.require`
- 默认级别：`warning`
- 适用范围：命名的 CHECK / FOREIGN KEY `NOT VALID` 约束添加
- 匹配键：相同 schema + table + constraint name
- 行为：若后续存在匹配的 `VALIDATE CONSTRAINT`，则 suppress warning
- 产品面：CLI、HTTP、MCP 和 `pkg/deltascope` 都会暴露这个 global finding
- 置信度：SQL corpus 覆盖和 Docker-backed PostgreSQL e2e 已锁定该契约

## 示例 SQL

存在问题的批次：

```sql
ALTER TABLE orders ADD CONSTRAINT chk_orders_amount CHECK (amount >= 0) NOT VALID;
```

成对且干净的批次：

```sql
ALTER TABLE orders ADD CONSTRAINT chk_orders_amount CHECK (amount >= 0) NOT VALID;
ALTER TABLE orders VALIDATE CONSTRAINT chk_orders_amount;
```

## CLI 示例

```bash
deltascope audit \
  --dialect postgresql \
  --format json \
  --sql "ALTER TABLE orders ADD CONSTRAINT chk_orders_amount CHECK (amount >= 0) NOT VALID;"
```

JSON 输出摘录：

```json
{
  "global_findings": [
    {
      "rule_id": "ddl.pg.alter.not_valid_constraint.validate.require",
      "level": "warning",
      "message": "NOT VALID constraint \"chk_orders_amount\" on table \"orders\" should be followed by VALIDATE CONSTRAINT in the audited migration batch"
    }
  ]
}
```

## 规则契约

| 字段 | 值 |
|------|----|
| Rule ID | `ddl.pg.alter.not_valid_constraint.validate.require` |
| 类型 | PostgreSQL-only GlobalRule |
| 默认级别 | `warning` |
| 适用对象 | 命名的 CHECK / FOREIGN KEY `NOT VALID` 约束 |
| 抑制条件 | 同一次审计 SQL 批次中后续存在匹配的 `VALIDATE CONSTRAINT` |

## 非目标

- 首次支持 `VALIDATE CONSTRAINT`
- 查询 live database validation state
- 跨文件或跨部署窗口追踪校验
- 匹配未命名约束
- 校验 CHECK 表达式正确性
- 校验 FK referenced table 正确性
- 改变 MySQL/TiDB 行为
- 新增 public API contract

## 安装

```bash
curl -fsSL https://raw.githubusercontent.com/Fanduzi/DeltaScope/v0.42.0/install.sh | \
  DELTASCOPE_VERSION=v0.42.0 sh
```
