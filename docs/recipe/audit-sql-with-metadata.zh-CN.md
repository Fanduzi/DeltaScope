# 携带元数据审计 SQL

当当前 Schema 状态或实例配置信息对审计结果有影响时，请使用元数据感知模式。只要提供任意连接参数（`--host`、`--port`、`--user`、`--password`、`--ask-password`、`--schema` 或 `--socket`），元数据感知模式即自动激活。

在此模式下，DeltaScope 在执行规则评估前先连接目标数据库，为每条语句附加 `TableSnapshot`（当前列列表、索引、行数估算）和 `InstanceFacts`（版本号、关键配置变量），再运行完整规则集——包括需要实时 Schema 上下文的规则。

## 最小权限要求

DeltaScope 仅读取元数据，不会向数据库写入任何内容。至少授予以下权限：

```sql
-- 最低要求：读取 information_schema 和 performance_schema
GRANT SELECT ON information_schema.* TO 'deltascope'@'%';
GRANT SELECT ON performance_schema.global_variables TO 'deltascope'@'%';

-- 说明：information_schema 的授权已足够读取表存在性、列列表和索引列表，
-- 无需额外的按表或按库授权。
```

> **注意：** DeltaScope 不需要 DDL/DML 权限，永远不会向目标数据库写入数据。

TiDB 使用相同的 `information_schema` 结构。`performance_schema.global_variables` 授权对 TiDB 为可选项——当该视图不可用时，DeltaScope 会自动降级处理。

## TCP 连接

```bash
deltascope audit \
  --sql "ALTER TABLE users ADD COLUMN email VARCHAR(255) NOT NULL DEFAULT '' COMMENT 'email address'" \
  --host 127.0.0.1 \
  --port 3306 \
  --user deltascope \
  --ask-password \
  --schema app
```

`--ask-password` 以交互方式提示输入密码，避免密码出现在 shell 历史或进程列表中。对于脚本环境，可通过环境变量注入密码：

```bash
deltascope audit \
  --sql "ALTER TABLE users ADD COLUMN email VARCHAR(255) NOT NULL DEFAULT '' COMMENT 'email address'" \
  --host 127.0.0.1 \
  --port 3306 \
  --user deltascope \
  --password "$DELTASCOPE_PASSWORD" \
  --schema app
```

## Unix Socket 连接

在本机运行 MySQL 或 TiDB 时，Unix socket 连接可避免 TCP 开销和防火墙规则：

```bash
deltascope audit \
  --sql "ALTER TABLE users ADD COLUMN email VARCHAR(255) NOT NULL DEFAULT '' COMMENT 'email address'" \
  --socket /var/run/mysqld/mysqld.sock \
  --user deltascope \
  --ask-password \
  --schema app
```

数据库在其他主机时使用 TCP；在本地开发或 CI 容器中已知 socket 路径时使用 Unix socket。

## 完整 JSON 输出

JSON 输出包含 `context` 字段，记录方言（dialect）和 Schema 的解析方式：

```bash
deltascope audit \
  --sql "ALTER TABLE users ADD COLUMN email VARCHAR(255) NOT NULL DEFAULT '' COMMENT 'email address'" \
  --host 127.0.0.1 \
  --port 3306 \
  --user deltascope \
  --ask-password \
  --schema app \
  --format json
```

```json
{
  "verdict": "pass",
  "summary": {
    "statements": 1,
    "blockers": 0,
    "warnings": 0,
    "notices": 0
  },
  "context": {
    "mode": "metadata-aware",
    "dialect": "mysql",
    "dialect_source": "detected",
    "schema": "app",
    "schema_source": "flag"
  },
  "statements": [
    {
      "index": 1,
      "kind": "ALTER TABLE",
      "raw_sql": "ALTER TABLE users ADD COLUMN email VARCHAR(255) NOT NULL DEFAULT '' COMMENT 'email address'",
      "findings": []
    }
  ],
  "global_findings": []
}
```

`dialect_source: "detected"` 表示 DeltaScope 查询了 `tidb_version()`（未找到 → 确认为 MySQL）。`schema_source: "flag"` 表示 Schema 来自 `--schema` 参数。

## Schema 解析

DeltaScope 按以下优先级顺序为每条语句解析目标 Schema：

| 情形 | 工作方式 | 示例 |
|------|---------|------|
| **SQL 中已限定 Schema** | SQL 显式指定了 Schema，DeltaScope 直接使用。 | `ALTER TABLE app.users ADD COLUMN ...` |
| **使用 `--schema` 参数** | SQL 未指定 Schema，但设置了 `--schema`，DeltaScope 使用该值。 | `--schema app` 配合 `ALTER TABLE users ...` |
| **自动推断** | SQL 未指定 Schema，也未设置 `--schema`。表名在连接用户可见的所有 Schema 中唯一存在，DeltaScope 自动使用该 Schema。 | 所有可见 Schema 中只有一个 `users` 表 |
| **歧义错误** | SQL 未指定 Schema，也未设置 `--schema`。表名在多个可见 Schema 中均存在，DeltaScope 以退出码 `2` 报错并要求消除歧义。 | `users` 同时存在于 `app` 和 `legacy` Schema |

### 歧义 Schema 示例

```bash
# users 表同时存在于 `app` 和 `legacy` Schema——DeltaScope 拒绝猜测
deltascope audit \
  --sql "ALTER TABLE users ADD COLUMN email VARCHAR(255)" \
  --host 127.0.0.1 \
  --port 3306 \
  --user deltascope \
  --ask-password
```

```text
Error: schema inference is ambiguous; table `users` found in [app legacy]; pass --schema to disambiguate
exit code: 2
```

解决方法：添加 `--schema app`，或在 SQL 中限定表名：`ALTER TABLE app.users ADD COLUMN email VARCHAR(255)`。

## TiDB 注意事项

- **自动检测**：DeltaScope 在连接时查询 `tidb_version()`。若该函数返回结果，方言设置为 `tidb`；否则假定为 MySQL。`dialect_source` 在 JSON 输出中记录为 `"detected"`。
- **不要传递 `--dialect`**：连接 TiDB 时省略 `--dialect`，交由自动检测处理。若目标为 TiDB 却传入 `--dialect mysql`（或反之），将导致退出码 `2`。
- **`innodb_adaptive_hash_index`**：对 TiDB 目标始终视为非活跃状态。依赖该变量的规则行为与其为关闭状态时一致。
- **Merge-alter 规则**：预置策略中 `ddl.alter.merge.mysql.require` 已启用；`ddl.alter.merge.tidb.require` 默认禁用（`required: false`）。如需启用，请在配置文件中调整。
- **`performance_schema.global_variables`**：TiDB 上该视图不可用时，DeltaScope 自动降级处理。依赖它的实例信息可能缺失，但审计可正常进行。

## 元数据感知模式与离线模式的输出差异

同一条 SQL，是否携带元数据可能产生不同的审查结果。例如，为已存在的列添加 `ALTER TABLE` 操作，只有在元数据感知模式下才能检测到：

**离线模式**（无连接）：

```bash
deltascope audit --sql "ALTER TABLE users ADD COLUMN email VARCHAR(255) NOT NULL DEFAULT '' COMMENT 'email'" --format json
```

```json
{
  "verdict": "pass",
  "summary": { "statements": 1, "blockers": 0, "warnings": 0, "notices": 0 },
  "statements": [
    { "index": 1, "kind": "ALTER TABLE", "raw_sql": "...", "findings": [] }
  ],
  "global_findings": []
}
```

**元数据感知模式**（`app.users` 中 `email` 列已存在）：

```bash
deltascope audit \
  --sql "ALTER TABLE users ADD COLUMN email VARCHAR(255) NOT NULL DEFAULT '' COMMENT 'email'" \
  --host 127.0.0.1 --port 3306 --user deltascope --ask-password --schema app \
  --format json
```

```json
{
  "verdict": "reject",
  "summary": { "statements": 1, "blockers": 1, "warnings": 0, "notices": 0 },
  "context": {
    "mode": "metadata-aware",
    "dialect": "mysql",
    "dialect_source": "detected",
    "schema": "app",
    "schema_source": "flag"
  },
  "statements": [
    {
      "index": 1,
      "kind": "ALTER TABLE",
      "raw_sql": "ALTER TABLE users ADD COLUMN email VARCHAR(255) NOT NULL DEFAULT '' COMMENT 'email'",
      "findings": [
        {
          "rule_id": "ddl.alter.column.exists",
          "level": "blocker",
          "message": "column `email` already exists in table `users`",
          "suggestion": "Remove the ADD COLUMN clause or check the column name",
          "location": { "line": 1, "column": 1 }
        }
      ]
    }
  ],
  "global_findings": []
}
```

这正是元数据感知模式在 `ALTER TABLE` 预检方面的价值所在。
