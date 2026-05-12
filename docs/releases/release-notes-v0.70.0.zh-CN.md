# DeltaScope v0.70.0 发行说明

## 概述

v0.70.0 扩展了 PostgreSQL 选定非权限 DDL 生命周期覆盖范围：RLS/行级安全策略、触发器、函数/存储过程、高级视图，以及选定的 ALTER 对象生命周期（schema、index、materialized view）。新增 28 条 PostgreSQL 审核规则，横跨 5 个生命周期族；SQL 语料库扩展到 433/433 目标（100% 覆盖率）；SDK、CLI、HTTP、MCP 四层公开面全覆盖。本里程碑覆盖 31 个选定的 PostgreSQL 非权限 DDL 形式——不声称完整 PostgreSQL DDL 语法覆盖。

## 选定 PostgreSQL DDL 生命周期覆盖

v0.70.0 系统性地将 DeltaScope 的 PostgreSQL 离线审核扩展到选定的非权限 DDL 生命周期族。PostgreSQL DDL 完成度普查现已覆盖 31 个被分类为 `finding_covered` 的形式，涵盖 RLS/策略、触发器、函数/存储过程、视图，以及选定的 ALTER 对象生命周期操作。许多 PG DDL 族仍然是有意延后的（见"不包含"部分）。

## 新增规则

| 规则 ID | 方言 | 级别 | 触发语句 |
|---------|------|:----:|----------|
| `ddl.pg.create_policy.notice` | PostgreSQL | notice | `CREATE POLICY` |
| `ddl.pg.alter_policy.notice` | PostgreSQL | notice | `ALTER POLICY` |
| `ddl.pg.drop_policy.warn` | PostgreSQL | warning | `DROP POLICY` |
| `ddl.pg.alter.enable_rls.notice` | PostgreSQL | notice | `ALTER TABLE ... ENABLE ROW LEVEL SECURITY` |
| `ddl.pg.alter.disable_rls.warn` | PostgreSQL | warning | `ALTER TABLE ... DISABLE ROW LEVEL SECURITY` |
| `ddl.pg.alter.force_rls.notice` | PostgreSQL | notice | `ALTER TABLE ... FORCE ROW LEVEL SECURITY` |
| `ddl.pg.alter.no_force_rls.notice` | PostgreSQL | notice | `ALTER TABLE ... NO FORCE ROW LEVEL SECURITY` |
| `ddl.pg.create_trigger.notice` | PostgreSQL | notice | `CREATE TRIGGER` |
| `ddl.pg.create_constraint_trigger.warn` | PostgreSQL | warning | `CREATE CONSTRAINT TRIGGER` |
| `ddl.pg.drop_trigger.advisory` | PostgreSQL | notice | `DROP TRIGGER` |
| `ddl.pg.create_function.notice` | PostgreSQL | notice | `CREATE FUNCTION` |
| `ddl.pg.create_function.security_definer.warn` | PostgreSQL | warning | `CREATE FUNCTION ... SECURITY DEFINER` |
| `ddl.pg.create_or_replace_function.advisory` | PostgreSQL | notice | `CREATE OR REPLACE FUNCTION` |
| `ddl.pg.drop_function.advisory` | PostgreSQL | notice | `DROP FUNCTION` |
| `ddl.pg.create_procedure.notice` | PostgreSQL | notice | `CREATE PROCEDURE` |
| `ddl.pg.drop_procedure.advisory` | PostgreSQL | notice | `DROP PROCEDURE` |
| `ddl.pg.create_or_replace_view.advisory` | PostgreSQL | notice | `CREATE OR REPLACE VIEW` |
| `ddl.pg.create_temp_view.notice` | PostgreSQL | notice | `CREATE TEMP VIEW` / `CREATE TEMPORARY VIEW` |
| `ddl.pg.create_view.check_option.notice` | PostgreSQL | notice | `CREATE VIEW ... WITH CHECK OPTION` |
| `ddl.pg.drop_view.cascade.warn` | PostgreSQL | warning | `DROP VIEW ... CASCADE` |
| `ddl.pg.alter_view.rename.notice` | PostgreSQL | notice | `ALTER VIEW ... RENAME TO` |
| `ddl.pg.alter_view.set_schema.notice` | PostgreSQL | notice | `ALTER VIEW ... SET SCHEMA` |
| `ddl.pg.alter_schema.rename.notice` | PostgreSQL | notice | `ALTER SCHEMA ... RENAME TO` |
| `ddl.pg.alter_schema.owner.notice` | PostgreSQL | notice | `ALTER SCHEMA ... OWNER TO` |
| `ddl.pg.alter_index.rename.notice` | PostgreSQL | notice | `ALTER INDEX ... RENAME TO` |
| `ddl.pg.alter_index.set_tablespace.notice` | PostgreSQL | notice | `ALTER INDEX ... SET TABLESPACE` |
| `ddl.pg.alter_materialized_view.rename.notice` | PostgreSQL | notice | `ALTER MATERIALIZED VIEW ... RENAME TO` |
| `ddl.pg.alter_materialized_view.set_schema.notice` | PostgreSQL | notice | `ALTER MATERIALIZED VIEW ... SET SCHEMA` |

### RLS / 行级安全策略生命周期

```bash
deltascope audit --dialect postgresql --sql "CREATE POLICY users_select ON users FOR SELECT USING (true)"
# → [notice] ddl.pg.create_policy.notice

deltascope audit --dialect postgresql --sql "ALTER TABLE users ENABLE ROW LEVEL SECURITY"
# → [notice] ddl.pg.alter.enable_rls.notice

deltascope audit --dialect postgresql --sql "DROP POLICY users_select ON users"
# → [warning] ddl.pg.drop_policy.warn
```

### 触发器生命周期

```bash
deltascope audit --dialect postgresql --sql "CREATE TRIGGER trg_audit AFTER INSERT ON users FOR EACH ROW EXECUTE FUNCTION log_change()"
# → [notice] ddl.pg.create_trigger.notice

deltascope audit --dialect postgresql --sql "DROP TRIGGER trg_audit ON users"
# → [notice] ddl.pg.drop_trigger.advisory
```

### 函数 / 存储过程生命周期

```bash
deltascope audit --dialect postgresql --sql "CREATE FUNCTION add(a int, b int) RETURNS int LANGUAGE sql AS \$\$ SELECT a + b \$\$"
# → [notice] ddl.pg.create_function.notice

deltascope audit --dialect postgresql --sql "DROP FUNCTION add(int, int)"
# → [notice] ddl.pg.drop_function.advisory
```

### 高级视图生命周期

```bash
deltascope audit --dialect postgresql --sql "CREATE OR REPLACE VIEW v_users AS SELECT id FROM users"
# → [notice] ddl.pg.create_or_replace_view.advisory

deltascope audit --dialect postgresql --sql "DROP VIEW v_users CASCADE"
# → [warning] ddl.pg.drop_view.cascade.warn
```

### 选定 ALTER 对象生命周期

```bash
deltascope audit --dialect postgresql --sql "ALTER SCHEMA app RENAME TO app_new"
# → [notice] ddl.pg.alter_schema.rename.notice

deltascope audit --dialect postgresql --sql "ALTER INDEX idx_old RENAME TO idx_new"
# → [notice] ddl.pg.alter_index.rename.notice

deltascope audit --dialect postgresql --sql "ALTER MATERIALIZED VIEW mv_stats SET SCHEMA archive"
# → [notice] ddl.pg.alter_materialized_view.set_schema.notice
```

## 归一化

| 语句 | 归一化操作 | 对象类型 |
|------|-----------|----------|
| `ALTER SCHEMA app RENAME TO app_new` | `alter_schema` | schema |
| `ALTER SCHEMA app OWNER TO owner` | `alter_schema` | schema |
| `ALTER INDEX idx RENAME TO idx_new` | `alter_index` | index |
| `ALTER INDEX idx SET TABLESPACE ts` | `alter_index` | index |
| `ALTER MATERIALIZED VIEW mv RENAME TO mv_new` | `alter_materialized_view` | materialized_view |
| `ALTER MATERIALIZED VIEW mv SET SCHEMA s` | `alter_materialized_view` | materialized_view |

## 质量

- SQL 语料库：242 条策略规则，433/433 受支持目标，100% 覆盖率
- PostgreSQL DDL 完成度普查：31/31 选定非权限 DDL 形式 `finding_covered`
- SDK、CLI、HTTP、MCP 四层公开面全覆盖验证
- 所有新生命周期形式的 AST 特征化测试
- 默认策略方言隔离保持不变

## 不包含

- 不声称完整 PostgreSQL DDL 支持。仅覆盖选定的非权限 DDL 生命周期族；许多族仍延后。
- 不做实时权限/角色验证。GRANT/REVOKE 规则仅输出信息性通知。
- 不执行 DCL 或运行时数据库防火墙行为。
- DeltaScope 不执行迁移。
- 延后的 PG DDL 族：`ALTER TYPE` 深度变更、`ALTER DOMAIN`（除重命名外）、`ALTER EXTENSION` 扩展生命周期、`ALTER FOREIGN TABLE`/`SERVER`/`USER MAPPING`、`ALTER AGGREGATE`/`OPERATOR`/`CONVERSION`、`ALTER PUBLICATION`/`SUBSCRIPTION`、`ALTER STATISTICS`/`ALTER RULE`、`COMMENT ON`、`SECURITY LABEL`。
- 权限/DCL 在运行时验证方面不在范围内。

## 安装

**macOS（推荐）：**

```bash
brew tap fanduzi/deltascope
brew install --cask deltascope
```

**通用安装器：**

```bash
curl -fsSL https://raw.githubusercontent.com/Fanduzi/DeltaScope/v0.70.0/install.sh | \
  DELTASCOPE_VERSION=v0.70.0 sh
```

## 升级

如果之前安装的是 v0.64.0：

```bash
# Homebrew
brew upgrade --cask deltascope

# 通用安装器（用新版本重新运行）
curl -fsSL https://raw.githubusercontent.com/Fanduzi/DeltaScope/v0.70.0/install.sh | \
  DELTASCOPE_VERSION=v0.70.0 sh
```

## 验证

```bash
deltascope --version
# 应输出 v0.70.0

deltascope audit --dialect postgresql --sql "CREATE POLICY users_select ON users FOR SELECT USING (true)"
# 应产生 ddl.pg.create_policy.notice 发现

deltascope audit --dialect postgresql --sql "ALTER SCHEMA app RENAME TO app_new"
# 应产生 ddl.pg.alter_schema.rename.notice 发现
```
