# 排查元数据感知审计问题

当 `deltascope audit` 离线模式正常工作、但元数据感知模式失败或行为异常时，请使用本指南。只要提供任意连接参数（`--host`、`--port`、`--user`、`--password`、`--ask-password`、`--schema`、`--socket`），元数据感知模式即自动激活。

## 所需 MySQL 权限

DeltaScope 仅读取元数据，不会向数据库写入任何内容。

| 权限 | 用途 |
|------|------|
| `SELECT ON information_schema.*` | 表存在性、列列表、索引列表、表选项 |
| `SELECT ON performance_schema.global_variables` | 实例信息：版本、关键配置变量（如 `innodb_adaptive_hash_index`） |

授权语句：

```sql
GRANT SELECT ON information_schema.* TO 'deltascope'@'%';
GRANT SELECT ON performance_schema.global_variables TO 'deltascope'@'%';
FLUSH PRIVILEGES;
```

**TiDB：** 同样适用 `information_schema` 授权。`performance_schema.global_variables` 为可选项——当该视图不可用时，DeltaScope 自动降级处理。无需额外的按库或按表授权。

## 常见症状与解决方法

### "schema inference is ambiguous; pass --schema"

完整错误信息格式：

```text
Error: schema inference is ambiguous; table `users` found in [app legacy]; pass --schema to disambiguate
exit code: 2
```

**原因：** 目标表名在连接用户可见的多个 Schema 中均存在。DeltaScope 拒绝猜测您指向哪个 Schema。

**解决步骤（按优先级）：**

1. **在 SQL 中限定 Schema**——最明确，始终优先：
   ```sql
   ALTER TABLE app.users ADD COLUMN email VARCHAR(255) NOT NULL DEFAULT '' COMMENT 'email address';
   ```
2. **传入 `--schema` 参数**——对批次中所有未限定的表引用生效：
   ```bash
   deltascope audit \
     --sql "ALTER TABLE users ADD COLUMN email VARCHAR(255) NOT NULL DEFAULT '' COMMENT 'email address'" \
     --host 127.0.0.1 --port 3306 --user deltascope --ask-password \
     --schema app
   ```
3. **减少 Schema 可见性**——使用只能看到目标 Schema 的用户进行连接。
4. **检查 `--schema` 拼写**——拼写错误会导致 DeltaScope 退回歧义解析。

### "table does not exist in schema"

完整错误信息格式：

```text
Error: table `orders` does not exist in schema `app`
exit code: 2
```

**原因：** 在已解析的 Schema 中未找到该表。常见原因：

- 连接到了错误的数据库实例（测试环境 vs 生产环境）。
- Schema 名称有误——检查 `--schema` 或 SQL 中的限定词。
- 表尚未创建（创建该表的迁移在同一批次中，但在快照拍取前尚未执行）。
- 用户缺少 `SELECT ON information_schema.*` 权限——表因 DeltaScope 无法读取其元数据而"不存在"。

**解决方法：** 在目标实例上验证 Schema 名称和表名：

```bash
mysql -h 127.0.0.1 -u deltascope -p -e "SHOW TABLES IN app LIKE 'orders';"
```

### 无法连接

排查清单：

- `--host` 和 `--port` 指向正确的实例。
- `--user` 拼写正确（某些系统区分大小写）。
- 密码方式：交互使用时用 `--ask-password`；脚本使用时用 `--password "$VAR"`。永远不要在命令行中硬编码密码。
- MySQL/TiDB 端口（默认 `3306`）可被运行 DeltaScope 的机器访问，且防火墙未拦截。
- 用户账号未限定特定主机（检查 `mysql.user.Host`）。

#### Socket vs TCP

| 方式 | 适用场景 | 示例 |
|------|---------|------|
| **Unix socket** | 数据库在同一台机器上，socket 路径已知 | `--socket /var/run/mysqld/mysqld.sock` |
| **TCP** | 数据库在其他主机，或在容器/虚拟机中 | `--host 127.0.0.1 --port 3306` |

Unix socket 示例：

```bash
deltascope audit \
  --sql "ALTER TABLE users ADD COLUMN email VARCHAR(255) NOT NULL DEFAULT '' COMMENT 'email address'" \
  --socket /var/run/mysqld/mysqld.sock \
  --user deltascope \
  --ask-password \
  --schema app
```

TCP 示例：

```bash
deltascope audit \
  --sql "ALTER TABLE users ADD COLUMN email VARCHAR(255) NOT NULL DEFAULT '' COMMENT 'email address'" \
  --host 127.0.0.1 \
  --port 3306 \
  --user deltascope \
  --ask-password \
  --schema app
```

若同时提供 `--socket` 和 `--host`，`--socket` 优先生效。

### 方言冲突

```text
Error: dialect conflict: --dialect mysql specified but connected instance is TiDB
exit code: 2
```

**解决方法：** 省略 `--dialect`，让自动检测处理。DeltaScope 在连接时查询 `tidb_version()`——若函数返回结果，方言设置为 `tidb`；否则假定为 MySQL。解析后的方言在 JSON 输出中以 `context.dialect_source: "detected"` 记录。

仅在离线模式下、需要不通过连接覆盖默认方言时，才传入 `--dialect`。

### 输出与离线模式不同

这是预期行为。元数据感知模式激活了需要实时 Schema 上下文的额外规则。例如，列存在性检查只有在 DeltaScope 能读取当前列列表时才会触发。

通过检查 JSON 输出中的 `context` 字段，确认元数据感知模式已激活：

```json
{
  "context": {
    "mode": "metadata-aware",
    "dialect": "mysql",
    "dialect_source": "detected",
    "schema": "app",
    "schema_source": "inferred"
  }
}
```

若 `context.mode` 不是 `"metadata-aware"`，说明连接参数未提供，或连接失败后 DeltaScope 退回了离线模式。请检查参数和凭据。

### 元数据规则未触发

依赖元数据的规则（在 `deltascope rules show` 输出中 `Metadata: true` 的规则）在离线模式下无效。若期望某个元数据规则触发但未触发，请检查：

1. 确认审计在元数据感知模式下运行（见上方——在 JSON 输出中查找 `"mode": "metadata-aware"`）。
2. 确认规则在策略中已启用（`deltascope config show-default` 或您的 `deltascope.yaml`）。
3. 确认连接用户拥有 `SELECT ON information_schema.*` 权限。
4. 确认目标表在已解析的 Schema 中存在——若 DeltaScope 找不到该表，元数据规则无法对其求值。

## TiDB 特别说明

- **`innodb_adaptive_hash_index`**：对 TiDB 目标始终视为非活跃状态。依赖该变量的规则行为与其为关闭状态时一致。
- **`ddl.alter.merge.tidb.require`**：预置策略中默认禁用（`required: false`）。若您的 TiDB 版本支持在线 DDL 合并，可在配置中启用。
- **`tidb_version()` 检测**：连接时自动运行。结果以 `context.dialect_source: "detected"` 记录。无需手动传入 `--dialect tidb`。
- **`performance_schema.global_variables`**：TiDB 上该视图不可用时，DeltaScope 自动降级处理。依赖它的实例信息可能缺失，但审计正常进行。

## 验证元数据模式已激活

检查 JSON 输出中的 `context` 字段：

```bash
deltascope audit \
  --sql "SELECT 1" \
  --host 127.0.0.1 --port 3306 --user deltascope --ask-password --schema app \
  --format json --quiet \
  | jq '.context'
```

元数据模式激活时的预期输出：

```json
{
  "mode": "metadata-aware",
  "dialect": "mysql",
  "dialect_source": "detected",
  "schema": "app",
  "schema_source": "flag"
}
```

若 `context` 字段缺失，或 `mode` 不是 `"metadata-aware"`，说明连接参数未提供，或连接失败后 DeltaScope 退回了离线模式。请检查参数和凭据。

关于元数据感知模式的概念模型，请参阅 [../concept/metadata-aware-mode.zh-CN.md](../concept/metadata-aware-mode.zh-CN.md)。
