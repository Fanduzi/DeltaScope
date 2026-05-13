# DeltaScope v0.80.0 发行说明

## 概述

v0.80.0 扩展了选定的 PostgreSQL 非权限 DDL 深度覆盖范围，新增 36 条仅限 PostgreSQL 的规则，横跨 6 个族：复合类型属性、扩展成员、发布/订阅、外部对象、注解生命周期，以及事件触发器/重写规则。SQL 语料库扩展到 469/469 目标（100% 覆盖率）。SDK、CLI、HTTP、MCP 四层公开面全覆盖。PostgreSQL DDL 完成度普查现已覆盖 38 个被分类为 `finding_covered` 的选定非权限 DDL 形式。本里程碑不声称完整 PostgreSQL DDL 语法覆盖。

## 选定 PostgreSQL DDL 深度覆盖

v0.80.0 系统性地将 DeltaScope 的 PostgreSQL 离线审核扩展到 6 个额外的非权限 DDL 族。PostgreSQL DDL 完成度普查现已覆盖 38 个被分类为 `finding_covered` 的形式，涵盖复合类型属性、扩展成员、发布/订阅、外部对象、注解生命周期，以及事件触发器/重写规则操作。许多 PG DDL 族仍然是有意延后的（见"不包含"部分）。

## 新增规则

| 规则 ID | 方言 | 级别 | 触发语句 |
|---------|------|:----:|----------|
| `ddl.pg.alter_type.add_attribute.notice` | PostgreSQL | notice | `ALTER TYPE ... ADD ATTRIBUTE` |
| `ddl.pg.alter_type.drop_attribute.warn` | PostgreSQL | warning | `ALTER TYPE ... DROP ATTRIBUTE` |
| `ddl.pg.alter_type.alter_attribute_type.warn` | PostgreSQL | warning | `ALTER TYPE ... ALTER ATTRIBUTE ... TYPE` |
| `ddl.pg.alter_type.rename_attribute.notice` | PostgreSQL | notice | `ALTER TYPE ... RENAME ATTRIBUTE` |
| `ddl.pg.alter_extension.add_member.notice` | PostgreSQL | notice | `ALTER EXTENSION ... ADD TABLE` |
| `ddl.pg.alter_extension.drop_member.warn` | PostgreSQL | warning | `ALTER EXTENSION ... DROP TABLE` |
| `ddl.pg.create_publication.notice` | PostgreSQL | notice | `CREATE PUBLICATION` |
| `ddl.pg.alter_publication.notice` | PostgreSQL | notice | `ALTER PUBLICATION` |
| `ddl.pg.drop_publication.warn` | PostgreSQL | warning | `DROP PUBLICATION` |
| `ddl.pg.create_subscription.notice` | PostgreSQL | notice | `CREATE SUBSCRIPTION` |
| `ddl.pg.alter_subscription.notice` | PostgreSQL | notice | `ALTER SUBSCRIPTION` |
| `ddl.pg.alter_subscription.disable.warn` | PostgreSQL | warning | `ALTER SUBSCRIPTION ... DISABLE` |
| `ddl.pg.drop_subscription.warn` | PostgreSQL | warning | `DROP SUBSCRIPTION` |
| `ddl.pg.create_foreign_table.notice` | PostgreSQL | notice | `CREATE FOREIGN TABLE` |
| `ddl.pg.alter_foreign_table.notice` | PostgreSQL | notice | `ALTER FOREIGN TABLE` |
| `ddl.pg.drop_foreign_table.warn` | PostgreSQL | warning | `DROP FOREIGN TABLE` |
| `ddl.pg.create_foreign_server.notice` | PostgreSQL | notice | `CREATE SERVER` |
| `ddl.pg.alter_foreign_server.notice` | PostgreSQL | notice | `ALTER SERVER` |
| `ddl.pg.drop_foreign_server.warn` | PostgreSQL | warning | `DROP SERVER` |
| `ddl.pg.create_user_mapping.notice` | PostgreSQL | notice | `CREATE USER MAPPING` |
| `ddl.pg.alter_user_mapping.notice` | PostgreSQL | notice | `ALTER USER MAPPING` |
| `ddl.pg.drop_user_mapping.warn` | PostgreSQL | warning | `DROP USER MAPPING` |
| `ddl.pg.create_foreign_data_wrapper.notice` | PostgreSQL | notice | `CREATE FOREIGN DATA WRAPPER` |
| `ddl.pg.alter_foreign_data_wrapper.notice` | PostgreSQL | notice | `ALTER FOREIGN DATA WRAPPER` |
| `ddl.pg.drop_foreign_data_wrapper.warn` | PostgreSQL | warning | `DROP FOREIGN DATA WRAPPER` |
| `ddl.pg.comment_on.notice` | PostgreSQL | notice | `COMMENT ON` |
| `ddl.pg.comment_on.remove.notice` | PostgreSQL | notice | `COMMENT ON ... IS NULL` |
| `ddl.pg.security_label.notice` | PostgreSQL | notice | `SECURITY LABEL` |
| `ddl.pg.security_label.remove.notice` | PostgreSQL | notice | `SECURITY LABEL ... IS NULL` |
| `ddl.pg.create_event_trigger.notice` | PostgreSQL | notice | `CREATE EVENT TRIGGER` |
| `ddl.pg.alter_event_trigger.notice` | PostgreSQL | notice | `ALTER EVENT TRIGGER` |
| `ddl.pg.alter_event_trigger.disable.warn` | PostgreSQL | warning | `ALTER EVENT TRIGGER ... DISABLE` |
| `ddl.pg.drop_event_trigger.warn` | PostgreSQL | warning | `DROP EVENT TRIGGER` |
| `ddl.pg.create_rule.notice` | PostgreSQL | notice | `CREATE RULE` |
| `ddl.pg.alter_rule.notice` | PostgreSQL | notice | `ALTER RULE` |
| `ddl.pg.drop_rule.warn` | PostgreSQL | warning | `DROP RULE` |

### 复合类型属性生命周期

```bash
deltascope audit --dialect postgresql --sql "ALTER TYPE address ADD ATTRIBUTE zip_code text"
# → [notice] ddl.pg.alter_type.add_attribute.notice

deltascope audit --dialect postgresql --sql "ALTER TYPE address DROP ATTRIBUTE zip_code"
# → [warning] ddl.pg.alter_type.drop_attribute.warn

deltascope audit --dialect postgresql --sql "ALTER TYPE address ALTER ATTRIBUTE zip_code TYPE varchar(10)"
# → [warning] ddl.pg.alter_type.alter_attribute_type.warn

deltascope audit --dialect postgresql --sql "ALTER TYPE address RENAME ATTRIBUTE zip_code TO postal_code"
# → [notice] ddl.pg.alter_type.rename_attribute.notice
```

### 扩展成员生命周期

```bash
deltascope audit --dialect postgresql --sql "ALTER EXTENSION pg_trgm ADD TABLE trgm_test"
# → [notice] ddl.pg.alter_extension.add_member.notice

deltascope audit --dialect postgresql --sql "ALTER EXTENSION pg_trgm DROP TABLE trgm_test"
# → [warning] ddl.pg.alter_extension.drop_member.warn
```

### 发布 / 订阅生命周期

```bash
deltascope audit --dialect postgresql --sql "CREATE PUBLICATION pub_users FOR TABLE users"
# → [notice] ddl.pg.create_publication.notice

deltascope audit --dialect postgresql --sql "ALTER PUBLICATION pub_users ADD TABLE orders"
# → [notice] ddl.pg.alter_publication.notice

deltascope audit --dialect postgresql --sql "DROP PUBLICATION pub_users"
# → [warning] ddl.pg.drop_publication.warn

deltascope audit --dialect postgresql --sql "CREATE SUBSCRIPTION sub_users CONNECTION 'host=pg port=5432 dbname=mydb' PUBLICATION pub_users"
# → [notice] ddl.pg.create_subscription.notice

deltascope audit --dialect postgresql --sql "ALTER SUBSCRIPTION sub_users DISABLE"
# → [warning] ddl.pg.alter_subscription.disable.warn

deltascope audit --dialect postgresql --sql "DROP SUBSCRIPTION sub_users"
# → [warning] ddl.pg.drop_subscription.warn
```

### 外部对象生命周期

```bash
deltascope audit --dialect postgresql --sql "CREATE FOREIGN TABLE remote_users (id int, name text) SERVER pg_remote"
# → [notice] ddl.pg.create_foreign_table.notice

deltascope audit --dialect postgresql --sql "DROP FOREIGN TABLE remote_users"
# → [warning] ddl.pg.drop_foreign_table.warn

deltascope audit --dialect postgresql --sql "CREATE SERVER pg_remote FOREIGN DATA WRAPPER postgres_fdw OPTIONS (host 'remote-host')"
# → [notice] ddl.pg.create_foreign_server.notice

deltascope audit --dialect postgresql --sql "DROP SERVER pg_remote"
# → [warning] ddl.pg.drop_foreign_server.warn

deltascope audit --dialect postgresql --sql "CREATE USER MAPPING FOR current_user SERVER pg_remote OPTIONS (user 'remote_user')"
# → [notice] ddl.pg.create_user_mapping.notice

deltascope audit --dialect postgresql --sql "DROP USER MAPPING FOR current_user SERVER pg_remote"
# → [warning] ddl.pg.drop_user_mapping.warn

deltascope audit --dialect postgresql --sql "CREATE FOREIGN DATA WRAPPER pg_fdw"
# → [notice] ddl.pg.create_foreign_data_wrapper.notice

deltascope audit --dialect postgresql --sql "DROP FOREIGN DATA WRAPPER pg_fdw"
# → [warning] ddl.pg.drop_foreign_data_wrapper.warn
```

### 注解生命周期

```bash
deltascope audit --dialect postgresql --sql "COMMENT ON TABLE users IS 'User accounts table'"
# → [notice] ddl.pg.comment_on.notice

deltascope audit --dialect postgresql --sql "COMMENT ON TABLE users IS NULL"
# → [notice] ddl.pg.comment_on.remove.notice

deltascope audit --dialect postgresql --sql "SECURITY LABEL FOR selinux ON TABLE users IS 'system_u:object_r:sepgsql_table_t:s0'"
# → [notice] ddl.pg.security_label.notice

deltascope audit --dialect postgresql --sql "SECURITY LABEL FOR selinux ON TABLE users IS NULL"
# → [notice] ddl.pg.security_label.remove.notice
```

### 事件触发器 / 重写规则生命周期

```bash
deltascope audit --dialect postgresql --sql "CREATE EVENT TRIGGER trg_ddl ON ddl_command_start EXECUTE FUNCTION log_ddl()"
# → [notice] ddl.pg.create_event_trigger.notice

deltascope audit --dialect postgresql --sql "ALTER EVENT TRIGGER trg_ddl DISABLE"
# → [warning] ddl.pg.alter_event_trigger.disable.warn

deltascope audit --dialect postgresql --sql "DROP EVENT TRIGGER trg_ddl"
# → [warning] ddl.pg.drop_event_trigger.warn

deltascope audit --dialect postgresql --sql "CREATE RULE notify_insert AS ON INSERT TO users DO ALSO NOTIFY users_changed"
# → [notice] ddl.pg.create_rule.notice

deltascope audit --dialect postgresql --sql "DROP RULE notify_insert ON users"
# → [warning] ddl.pg.drop_rule.warn
```

## 归一化

| 语句 | 归一化操作 | 对象类型 |
|------|-----------|----------|
| `ALTER TYPE address ADD ATTRIBUTE zip_code text` | `alter_type` | composite_type |
| `ALTER TYPE address DROP ATTRIBUTE zip_code` | `alter_type` | composite_type |
| `ALTER TYPE address ALTER ATTRIBUTE zip_code TYPE varchar(10)` | `alter_type` | composite_type |
| `ALTER TYPE address RENAME ATTRIBUTE zip_code TO postal_code` | `alter_type` | composite_type |
| `ALTER EXTENSION pg_trgm ADD TABLE trgm_test` | `alter_extension` | extension |
| `ALTER EXTENSION pg_trgm DROP TABLE trgm_test` | `alter_extension` | extension |
| `CREATE PUBLICATION pub_users FOR TABLE users` | `create_publication` | publication |
| `ALTER PUBLICATION pub_users ADD TABLE orders` | `alter_publication` | publication |
| `DROP PUBLICATION pub_users` | `drop_publication` | publication |
| `CREATE SUBSCRIPTION sub_users CONNECTION ... PUBLICATION pub_users` | `create_subscription` | subscription |
| `ALTER SUBSCRIPTION sub_users DISABLE` | `alter_subscription` | subscription |
| `DROP SUBSCRIPTION sub_users` | `drop_subscription` | subscription |
| `CREATE FOREIGN TABLE remote_users (...) SERVER pg_remote` | `create_foreign_table` | foreign_table |
| `ALTER FOREIGN TABLE remote_users ADD COLUMN email text` | `alter_foreign_table` | foreign_table |
| `DROP FOREIGN TABLE remote_users` | `drop_foreign_table` | foreign_table |
| `CREATE SERVER pg_remote FOREIGN DATA WRAPPER postgres_fdw` | `create_foreign_server` | foreign_server |
| `ALTER SERVER pg_remote OPTIONS (SET host 'new-host')` | `alter_foreign_server` | foreign_server |
| `DROP SERVER pg_remote` | `drop_foreign_server` | foreign_server |
| `CREATE USER MAPPING FOR current_user SERVER pg_remote` | `create_user_mapping` | user_mapping |
| `ALTER USER MAPPING FOR current_user SERVER pg_remote OPTIONS (SET user 'new_user')` | `alter_user_mapping` | user_mapping |
| `DROP USER MAPPING FOR current_user SERVER pg_remote` | `drop_user_mapping` | user_mapping |
| `CREATE FOREIGN DATA WRAPPER pg_fdw` | `create_foreign_data_wrapper` | foreign_data_wrapper |
| `ALTER FOREIGN DATA WRAPPER pg_fdw OPTIONS (ADD debug 'true')` | `alter_foreign_data_wrapper` | foreign_data_wrapper |
| `DROP FOREIGN DATA WRAPPER pg_fdw` | `drop_foreign_data_wrapper` | foreign_data_wrapper |
| `COMMENT ON TABLE users IS 'description'` | `comment_on` | - |
| `COMMENT ON TABLE users IS NULL` | `comment_on` | - |
| `SECURITY LABEL FOR selinux ON TABLE users IS 'label'` | `security_label` | - |
| `SECURITY LABEL FOR selinux ON TABLE users IS NULL` | `security_label` | - |
| `CREATE EVENT TRIGGER trg_ddl ON ddl_command_start EXECUTE FUNCTION log_ddl()` | `create_event_trigger` | event_trigger |
| `ALTER EVENT TRIGGER trg_ddl DISABLE` | `alter_event_trigger` | event_trigger |
| `DROP EVENT TRIGGER trg_ddl` | `drop_event_trigger` | event_trigger |
| `CREATE RULE notify_insert AS ON INSERT TO users DO ALSO NOTIFY users_changed` | `create_rule` | rule |
| `ALTER RULE notify_insert ON users RENAME TO notify_insert_v2` | `alter_rule` | rule |
| `DROP RULE notify_insert ON users` | `drop_rule` | rule |

## 质量

- SQL 语料库：278 条策略规则，469/469 受支持目标，100% 覆盖率
- PostgreSQL DDL 深度覆盖普查：38/38 选定非权限 DDL 形式 `finding_covered`，1 个 `parser_error`
- SDK、CLI、HTTP、MCP 四层公开面全覆盖验证（36 条新规则）
- 默认策略方言隔离保持不变

## 隐私边界

预期语料库输出不包含：
- 订阅连接字符串（`CONNECTION '...'` 值）
- 外部对象选项值（`OPTIONS (...)` 内容）
- 注释文本（`IS '...'` 值）
- 安全标签文本（`IS '...'` 值）
- 事件触发器函数体（`EXECUTE FUNCTION ...` 体）
- 重写规则体（`DO ...` 子句内容）

规则仅输出结构性生命周期事实（操作、对象类型、对象名称），不保留或暴露敏感负载值。

## 不包含

- 不声称完整 PostgreSQL DDL 支持。仅覆盖选定的非权限 DDL 生命周期族；许多族仍延后。
- 不做实时权限/角色验证。GRANT/REVOKE 规则仅输出信息性通知。
- 不执行 DCL 或运行时数据库防火墙行为。
- DeltaScope 不执行迁移。
- 延后项：`DROP SUBSCRIPTION ... WITH (drop_slot = true)` 仍然是真正的解析器错误。
- 延后的 PG DDL 族：更广泛的 PostgreSQL 语法、聚合/操作符/转换/统计信息生命周期，以及其他超出范围的族仍然有意图地未覆盖。
- 权限/DCL 在运行时验证方面不在范围内：GRANT/REVOKE、角色/用户、默认权限（v0.60.0 已有的表级 DCL 支持除外）。

## 安装

**macOS（推荐）：**

```bash
brew tap fanduzi/deltascope
brew install --cask deltascope
```

**通用安装器：**

```bash
curl -fsSL https://raw.githubusercontent.com/Fanduzi/DeltaScope/v0.80.0/install.sh | \
  DELTASCOPE_VERSION=v0.80.0 sh
```

## 升级

如果之前安装的是 v0.70.0：

```bash
# Homebrew
brew upgrade --cask deltascope

# 通用安装器（用新版本重新运行）
curl -fsSL https://raw.githubusercontent.com/Fanduzi/DeltaScope/v0.80.0/install.sh | \
  DELTASCOPE_VERSION=v0.80.0 sh
```

## 验证

```bash
deltascope --version
# 应输出 v0.80.0

deltascope audit --dialect postgresql --sql "CREATE PUBLICATION pub_users FOR TABLE users"
# 应产生 ddl.pg.create_publication.notice 发现

deltascope audit --dialect postgresql --sql "CREATE FOREIGN TABLE remote_users (id int) SERVER pg_remote"
# 应产生 ddl.pg.create_foreign_table.notice 发现

deltascope audit --dialect postgresql --sql "COMMENT ON TABLE users IS 'description'"
# 应产生 ddl.pg.comment_on.notice 发现

deltascope audit --dialect postgresql --sql "CREATE EVENT TRIGGER trg_ddl ON ddl_command_start EXECUTE FUNCTION log_ddl()"
# 应产生 ddl.pg.create_event_trigger.notice 发现
```
