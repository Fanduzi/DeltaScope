# DeltaScope v0.60.0 发行说明

## 概要

v0.60.0 新增 PostgreSQL 表级权限 DCL 窄支持。DeltaScope 现在可以规范化 `GRANT ... ON TABLE` 和 `REVOKE ... ON TABLE`，并配有四条 PostgreSQL-only 新发现用于离线迁移审查。支持多个 privileges（如 `SELECT, INSERT`）、多个 grantees、schema-qualified 表名（如 `public.users`）、`GRANT ALL PRIVILEGES` 和 `REVOKE ... CASCADE`。DeltaScope 不做任何形式的实时校验——不验证 grantee/role 是否存在、不验证 table/object 是否存在、不验证当前用户是否有授权权限、不计算 effective privileges、不解析 role inheritance、不验证 ownership、不评估 RLS/policies。这是窄表级权限 DCL 支持，不是广泛的治理或 admin DCL 支持。

## 规范化形式

| SQL | 规范化操作 |
|-----|-----------|
| `GRANT SELECT ON users TO reader` | `grant_table_privilege` |
| `GRANT SELECT, INSERT ON users TO reader, writer` | `grant_table_privilege`（privileges=[SELECT, INSERT], grantees=[reader, writer]） |
| `GRANT ALL PRIVILEGES ON users TO admin` | `grant_table_privilege`（all_privileges=true） |
| `GRANT SELECT ON public.users TO reader` | `grant_table_privilege`（schema=public） |
| `REVOKE SELECT ON users FROM reader` | `revoke_table_privilege` |
| `REVOKE INSERT, UPDATE ON users FROM writer, editor` | `revoke_table_privilege`（privileges=[INSERT, UPDATE], grantees=[writer, editor]） |
| `REVOKE ALL PRIVILEGES ON users FROM admin` | `revoke_table_privilege`（all_privileges=true） |
| `REVOKE SELECT ON users FROM reader CASCADE` | `revoke_table_privilege`（cascade=true） |

## 新增 PostgreSQL-only 规则

| 规则 ID | 触发条件 | 默认级别 |
|---------|---------|---------|
| `ddl.pg.grant.table_privilege.notice` | 任意表级 `GRANT` | notice |
| `ddl.pg.grant.table_privilege.all.warn` | `GRANT ALL PRIVILEGES ON TABLE` | warning |
| `ddl.pg.revoke.table_privilege.notice` | 任意表级 `REVOKE` | notice |
| `ddl.pg.revoke.table_privilege.cascade.warn` | `REVOKE ... ON TABLE ... CASCADE` | warning |

## 重复发现

`GRANT ALL PRIVILEGES ON TABLE` 会同时触发 `ddl.pg.grant.table_privilege.notice` 和 `ddl.pg.grant.table_privilege.all.warn`。`REVOKE ... ON TABLE ... CASCADE` 会同时触发 `ddl.pg.revoke.table_privilege.notice` 和 `ddl.pg.revoke.table_privilege.cascade.warn`。这些重复发现是刻意为之——每条规则关注不同层面（操作本身 vs. 过度授权 / CASCADE 副作用风险）。

## 明确不支持/延迟的操作

| SQL | 状态 |
|-----|------|
| `GRANT ... ON ALL TABLES IN SCHEMA` | 不支持 |
| Sequence privileges（`GRANT ... ON SEQUENCE`） | 不支持 |
| Role membership（`GRANT role TO role`） | 不支持 |
| `ALTER DEFAULT PRIVILEGES` | 不支持 |

## 实时校验边界

DeltaScope 不对表级权限做任何形式的实时校验：
- 不验证 grantee/role 是否存在
- 不验证 table/object 是否存在
- 不验证当前用户是否有授权权限
- 不计算 effective privileges
- 不解析 role inheritance
- 不验证 ownership
- 不评估 RLS/policies

## 测试覆盖

- AST 普查测试，记录所有表级权限 DCL 形式的稳定 parser 事实。
- Parser/extractor 规范化测试，覆盖所有支持的 GRANT/REVOKE 变体。
- Corpus fixtures 覆盖四条新规则的触发形式。
- 服务层测试，通过 `AuditSQL` 覆盖代表性表级权限 DCL 变体。
- 公共接口测试，覆盖全部四个面：`pkg/deltascope` Audit、CLI Execute、HTTP handler 和 MCP audit_sql tool。

## Non-Goals

- DeltaScope 不对表级权限做任何形式的实时校验。
- `ALL TABLES IN SCHEMA`、sequence privileges、role membership 和 `ALTER DEFAULT PRIVILEGES` 不支持。
- 这是窄表级权限 DCL 支持，不是广泛的治理或 admin DCL 支持。
- 无 MySQL/TiDB 行为变更。
- 除四条新 PostgreSQL-only 规则外，无默认策略变更。

## 安装

**macOS（推荐）：**

```bash
brew tap fanduzi/deltascope
brew install --cask deltascope
```

**通用安装器：**

```bash
curl -fsSL https://raw.githubusercontent.com/Fanduzi/DeltaScope/v0.60.0/install.sh | \
  DELTASCOPE_VERSION=v0.60.0 sh
```
