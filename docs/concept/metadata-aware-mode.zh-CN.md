# 元数据感知模式

## 概述

元数据感知模式（Metadata-Aware Mode）在离线模式所使用的同一审计流程之上，叠加实时的实例和 Schema 信息。它**不**取代离线评估——而是对其进行补充。所有支持离线运行的规则继续正常执行。需要实时信息的规则在相关信息存在时自动激活，在信息缺失时优雅地空操作（no-op）。

---

## 激活元数据感知模式

向 `deltascope audit` 提供任意数据库连接标志即可激活元数据感知模式。无需传递专用的模式标志——任何连接参数的存在即足以激活该模式。

可激活该模式的连接标志：

| 标志 | 用途 |
|---|---|
| `--host` | MySQL/TiDB 主机地址 |
| `--port` | 端口号（默认：3306） |
| `--user` | 数据库用户名 |
| `--password` | 密码（通过命令行传入） |
| `--ask-password` | 以交互方式提示输入密码 |
| `--socket` | Unix socket 路径 |
| `--schema` | 非限定表名的默认 Schema |

任意单个连接标志即可激活该模式。例如：

```bash
# 最简方式：主机 + 用户（密码以交互方式输入）
deltascope audit --host db.example.com --user readonly --ask-password migration.sql

# 指定 schema 和 socket
deltascope audit --socket /var/run/mysqld/mysqld.sock --user auditor --schema app migration.sql
```

---

## 方言自动检测

当连接到实时实例时，DeltaScope 通过查询 `tidb_version()` 系统变量自动检测方言：

- **成功**（变量存在且返回值）：方言检测为 **TiDB**
- **失败**（变量不存在或返回错误）：方言检测为 **MySQL**

如果你显式传入 `--dialect` 且与自动检测的方言冲突，审计将以退出码 2 失败。为避免冲突，连接实时实例时请省略 `--dialect`，让自动检测处理方言识别。

---

## Schema 推断

当 SQL 语句引用不带 Schema 限定符的表时（例如 `ALTER TABLE users ...`），DeltaScope 使用以下四步逻辑解析目标 Schema。第一个匹配的步骤即为最终结果：

1. **SQL 中的限定名**：若 SQL 已包含表的 Schema 限定（例如 `ALTER TABLE app.users ...`），则直接使用该 Schema。
2. **`--schema` 标志**：若命令行提供了 `--schema`，则使用该值。
3. **唯一匹配**：若该表在已连接用户可见的所有 Schema 中恰好存在于一个 Schema，则自动推断该 Schema。
4. **模糊匹配**：若该表存在于多个 Schema，则审计失败，并输出：
   ```
   schema inference for table "users" is ambiguous; pass --schema to specify
   ```

若目标表在任何 Schema 中均未找到（且规则要求表已存在），审计同样失败，并提示你传入 `--schema` 或确认表名。

---

## 加载的信息内容

### 实例信息（Instance Facts）

实例信息描述 MySQL 或 TiDB 实例的配置，每次审计会话加载一次，并附加到批次中的每条语句。

| 信息项 | 说明 |
|---|---|
| 版本字符串 | 实例报告的 MySQL 或 TiDB 版本 |
| `character_set_database` | 已连接数据库的默认字符集 |
| `innodb_large_prefix` | 是否启用大索引前缀（`ON`/`OFF`） |
| `innodb_default_row_format` | 默认 InnoDB 行格式（`DYNAMIC`、`COMPACT` 等） |
| `innodb_adaptive_hash_index` | 是否启用自适应哈希索引（`ON`/`OFF`） |

### 表快照（Table Snapshot）

表快照是目标表的当前定义，从 `information_schema` 中加载，附加到引用特定表的语句上。

快照包含：

- **列定义**：名称、数据类型、可空性、默认值、注释
- **索引定义**：索引名称、类型（BTREE/HASH/FULLTEXT）、唯一性、索引列
- **主键状态**：是否存在主键及其覆盖的列
- **表选项**：存储引擎、字符集、行格式、表注释及其他 CREATE TABLE 选项

---

## 元数据感知模式启用的检查

以下检查仅在加载了相关信息时才会激活：

| 检查项 | 所需信息 |
|---|---|
| 列/索引/表存在性检查（例如待添加的列不得已存在） | 表快照 |
| ALTER TABLE 类型兼容性检查（新类型必须与现有列类型兼容） | 表快照 |
| 行大小估算（预计行大小不得超过 InnoDB 行大小限制） | 表快照 + 实例信息 |
| 索引键长度估算（索引键必须在实例定义的限制范围内） | 表快照 + 实例信息 |
| DROP/TRUNCATE 行数提示（当 `information_schema` 中 `table_rows` 较大时发出警告） | 表快照 |
| 自适应哈希索引警告 | `innodb_adaptive_hash_index` 实例信息 |
| 表选项兼容性检查（例如字符集变更与当前 Schema 字符集的兼容性） | 表快照 + 实例信息 |

---

## 不做的事情

- **不取代离线规则**：元数据存在时，所有支持离线运行的规则继续执行。元数据感知模式严格为叠加模式。
- **Schema 模糊时不猜测**：若表名存在于多个 Schema，审计将以清晰的错误信息失败。DeltaScope 永远不会静默地优先选择某个 Schema。
- **不静默跳过元数据需求**：若特定表的元数据无法加载，该表的元数据依赖检查将被优雅地跳过；离线检查仍会继续运行，不会因元数据缺失而产生虚假错误。

---

## 所需 MySQL 权限

通过 `--user` 提供的数据库用户需要以下最低权限：

```sql
-- Read schema metadata
GRANT SELECT ON information_schema.* TO 'auditor'@'%';

-- Read InnoDB instance configuration variables
GRANT SELECT ON performance_schema.global_variables TO 'auditor'@'%';
```

无需写权限。DeltaScope 永远不会修改目标数据库。

---

## TiDB 说明

- **自动检测**：TiDB 通过 `tidb_version()` 系统变量识别，无需手动指定标志。
- **自适应哈希索引**：在 TiDB 上，`innodb_adaptive_hash_index` 始终被视为未启用；对应的警告规则被抑制。
- **合并 ALTER 建议**：`ddl.alter.merge.mysql.require` 规则针对 MySQL DDL 惯例设计。在 TiDB 上，由于 TiDB 对并发 DDL 的处理方式不同，该规则在随产品附带的策略中默认禁用。
- **权限**：相同的 `information_schema` 授权同样适用于 TiDB。TiDB 不以相同方式暴露 `performance_schema.global_variables`；当这些变量不可用时，DeltaScope 会优雅地降级处理。

---

## 输出上下文

元数据感知模式激活时，DeltaScope 会在输出中附加描述连接和 Schema 解析方式的上下文信息。

**JSON 输出**在顶层包含一个 `context` 对象：

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

字段含义：

| 字段 | 可选值 | 说明 |
|---|---|---|
| `mode` | `offline`、`metadata-aware` | 元数据补充是否处于激活状态 |
| `dialect` | `mysql`、`tidb` | 评估所使用的方言 |
| `dialect_source` | `detected`、`explicit` | 方言来源：自动检测还是 `--dialect` 标志 |
| `schema` | Schema 名称 | 解析后的默认 Schema |
| `schema_source` | `flag`、`inferred`、`qualified` | Schema 的确定方式 |

**Markdown 输出**在发现结果表格之前，前置一个 `## Audit Context` 章节，以人类可读的形式呈现相同信息。
