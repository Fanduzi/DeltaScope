# DeltaScope v0.51.0 发行说明

## 概要

v0.51.0 扩展了 PostgreSQL ALTER TABLE 审核覆盖，新增三条补位规则。DeltaScope 现在会对 `ALTER TABLE ... DROP COLUMN`、`ALTER TABLE ... VALIDATE CONSTRAINT` 以及不带默认值的 nullable `ALTER TABLE ... ADD COLUMN` 发出警告/提示——在既有 migration-safety 规则之外，覆盖最常见的 ALTER TABLE 安全盲区。

## 新增

- 三条新 PostgreSQL-only 规则：
  - `ddl.pg.alter.drop_column.advisory` — `ALTER TABLE ... DROP COLUMN` 移除列时发出警告 (warning)
  - `ddl.pg.alter.validate_constraint.advisory` — `ALTER TABLE ... VALIDATE CONSTRAINT` 执行验证扫描时发出提示 (notice)
  - `ddl.pg.alter.add_column.nullable.notice` — `ALTER TABLE ... ADD COLUMN` 添加不带默认值的可空列时发出提示 (notice)
- 覆盖每条规则触发形式的语料库 fixture。
- 全部三条规则的服务层测试（通过 `AuditSQL`）。
- 四个公共表面的测试覆盖：`pkg/deltascope` Audit、CLI Execute、HTTP handler 和 MCP audit_sql 工具。

## 非目标

- 这不是完整的 PostgreSQL ALTER TABLE 覆盖。剩余 ALTER TABLE 子命令（如 `ALTER COLUMN TYPE`、`ADD CONSTRAINT ... NOT VALID`、`DISABLE TRIGGER` 等）仍为显式边界。
- 没有 MySQL/TiDB 行为变更。
- 除三条新 PostgreSQL-only 规则条目外，没有默认策略变更。

## 安装

**macOS（推荐）：**

```bash
brew tap fanduzi/deltascope
brew install --cask deltascope
```

**通用安装脚本：**

```bash
curl -fsSL https://raw.githubusercontent.com/Fanduzi/DeltaScope/v0.51.0/install.sh | \
  DELTASCOPE_VERSION=v0.51.0 sh
```
