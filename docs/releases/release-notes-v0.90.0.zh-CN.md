# DeltaScope v0.90.0 发行说明

## 概述

v0.90.0 新增 PostgreSQL 元数据感知对象验证。当通过 PostgreSQL 元数据连接审核 DDL 时，DeltaScope 现在会解析选定的非表对象（类型、域、扩展、序列、物化视图、模式、外部服务器、用户映射、发布、注释）的元数据，并将对象存在性和安全属性信息注入生命周期规则发现。本里程碑不添加新规则，不改变规则行为。

## 元数据感知对象验证

当配置了 PostgreSQL 元数据连接时，DeltaScope 通过 `pg_catalog` 查询解析对象元数据，并将安全属性注入生命周期规则发现：

```bash
deltascope audit \
  --dialect postgresql \
  --sql "DROP DOMAIN app.email_address" \
  --host 127.0.0.1 --port 5432 --user root --ask-password --schema app
```

发现输出在元数据可用时包含对象元数据字段：

```json
{
  "rule_id": "ddl.pg.drop_domain.advisory",
  "metadata": {
    "metadata_status": "confirmed",
    "metadata_object_type": "domain",
    "metadata_object_name": "email_address",
    "metadata_exists": true,
    "metadata_has_check": "true"
  }
}
```

当对象不存在时：

```json
{
  "rule_id": "ddl.pg.drop_schema.advisory",
  "metadata": {
    "metadata_status": "not_found",
    "metadata_object_type": "schema",
    "metadata_object_name": "old_schema",
    "metadata_exists": false
  }
}
```

### 支持的对象类型

| 对象类型 | 示例 SQL | 投射属性 |
|---------|---------|---------|
| `schema` | `DROP SCHEMA old_schema` | （仅有 status/name/exists） |
| `type` | `DROP TYPE app.color` | `type_kind` |
| `domain` | `DROP DOMAIN app.email_address` | `has_check` |
| `extension` | `DROP EXTENSION pgcrypto` | `extension_version`, `enabled` |
| `sequence` | `DROP SEQUENCE ticket_seq` | （仅有 status/name/exists） |
| `materialized_view` | `DROP MATERIALIZED VIEW user_summary` | （仅有 status/name/exists） |
| `publication` | `DROP PUBLICATION pub_users` | （仅有 status/name/exists） |
| `foreign_server` | `DROP SERVER fs_test` | `foreign_data_wrapper`, `has_options` |
| `user_mapping` | `DROP USER MAPPING FOR current_user SERVER fs_test` | `server` |
| `comment` | `COMMENT ON TABLE users IS '...'` | `target_type` |

### 安全属性投射

仅 8 个安全属性键被投射到发现中。所有敏感值均通过双重黑名单/白名单过滤：

**可投射键**（白名单）：`type_kind`、`extension_version`、`enabled`、`server`、`foreign_data_wrapper`、`target_type`、`has_options`、`table`。

**被屏蔽键**（黑名单）：password、secret、token、api_key、connection、dsn、connstr、body、definition、comment、label、query、action_sql、options。

## 修复

- 带模式限定的名称解析：`DROP DOMAIN app.email_address` 和 `COMMENT ON TABLE app.users IS '...'` 现在正确提取对象名（`email_address`、`users`），而非模式前缀（`app`）。

## 非目标

- 不添加新规则 ID。v0.90.0 用元数据增强现有规则发现。
- 不声称完整 PostgreSQL DDL 支持。
- 不执行实时权限/角色验证。
- 不执行 DCL 或运行时数据库防火墙行为。
- DeltaScope 不执行迁移。
- MySQL/TiDB 对象元数据解析返回 `unavailable` —— 无行为变更。

## 安装

**macOS（推荐）：**

```bash
brew tap fanduzi/deltascope
brew install --cask deltascope
```

**通用安装脚本：**

```bash
curl -fsSL https://raw.githubusercontent.com/Fanduzi/DeltaScope/v0.90.0/install.sh | \
  DELTASCOPE_VERSION=v0.90.0 sh
```

## 升级

从 v0.80.0 升级：

```bash
# Homebrew
brew upgrade --cask deltascope

# 通用安装脚本（使用新版本重新运行）
curl -fsSL https://raw.githubusercontent.com/Fanduzi/DeltaScope/v0.90.0/install.sh | \
  DELTASCOPE_VERSION=v0.90.0 sh
```

## 验证

```bash
deltascope --version
# 应输出 v0.90.0

# 离线审核（无元数据 — 发现中不包含 metadata_status）
deltascope audit --dialect postgresql --sql "DROP SCHEMA old_schema"

# 元数据感知审核（需要 PostgreSQL 连接）
deltascope audit --dialect postgresql --sql "DROP SCHEMA old_schema" \
  --host 127.0.0.1 --port 5432 --user root --ask-password --schema public \
  --format json
# 发现应包含 metadata_status: "not_found" 或 "confirmed"
```
