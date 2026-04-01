# 配置参考

DeltaScope 使用 YAML 策略文件（policy file）来调整规则的启用状态与阈值，无需重新编译二进制文件。

---

## 配置文件格式

每条规则在 YAML 文件中的结构如下：

```yaml
rules:
  <rule-id>:
    enabled: true        # bool：是否启用该规则
    level: blocker       # string：blocker | warning | notice
    params:              # map：各规则专属参数（可选）
      <key>: <value>
```

### 级别（Level）含义

| 级别 | 含义 |
|------|------|
| `blocker` | 阻断发布流程，`--fail-on blocker` 时返回非零退出码 |
| `warning` | 提示需关注，`--fail-on warning` 时同样阻断流程 |
| `notice` | 纯提示信息，不影响退出码 |

### `--fail-on` 与级别的交互关系

| `--fail-on` 值 | 阻断于 blocker | 阻断于 warning | 阻断于 notice |
|----------------|:--------------:|:--------------:|:-------------:|
| `blocker`      | ✓              | ✗              | ✗             |
| `warning`      | ✓              | ✓              | ✗             |
| `notice`       | ✓              | ✓              | ✓             |
| `none`         | ✗              | ✗              | ✗             |

---

## 配置命令（Config Commands）

```bash
# 生成默认配置文件
deltascope config init > deltascope.yaml

# 校验配置文件语法与 rule ID
deltascope config lint --file ./deltascope.yaml

# 查看内置默认配置
deltascope config show-default
```

**`config lint` 成功示例：**
```text
Config file ./deltascope.yaml is valid.
```

**`config lint` 失败示例（未知 rule ID）：**
```text
Error: unknown rule ID "ddl.table.comments.require" (did you mean "ddl.table.comment.require"?)
```

### 文件来源

- 通过 `deltascope config init` 生成
- 随代码库提交的示例文件：`configs/deltascope.example.yaml`
- 运行时通过 `--config` 指定加载

### 使用建议

- 生成一次后，将策略文件提交至 CI 中统一使用
- 上线前先用 `config lint` 校验变更
- 将 `configs/deltascope.example.yaml` 作为文档参考，而非唯一的配置来源

---

## 规则配置参考

> **元数据感知模式（Metadata-aware mode）说明**：部分规则需要连接数据库以获取实时 schema 信息。这类规则在纯离线审计时会自动跳过（no-op），不产生任何 finding，保证离线审计的安全性。

### Structured Naming Governance（结构化命名治理）

DeltaScope 在现有基于正则的 `pattern` 规则之外，补充了结构化命名治理模型。

- 当你需要一个统一的正则门槛时，使用 `*.name.pattern.require`，例如 `^[A-Za-z0-9_]+$`。
- 当你需要明确的命名语义时，使用下列 naming governance 规则，例如 `prefix`、`suffix`、`contains`。
- 这两层能力是互补关系。structured naming governance 不是 `pattern` 规则的替代品。
- `contains` 采用 OR 语义。只要命中任意一个已配置 token，就视为通过。
- naming finding 只针对显式命名对象产生；未命名对象和隐式对象会被跳过。

所有 naming governance 规则都遵循同一结构：

```yaml
rules:
  <rule-id>:
    enabled: true
    level: warning
    params:
      prefix: "..."
      suffix: "..."
      contains: ["...", "..."]
```

实际使用时，只配置与该 rule ID 对应的参数即可。空值会让内置规则保持 inert（启用但不生效）。

| 目标对象 | Rule ID |
|----------|---------|
| 表名 | `ddl.table.name.prefix.require`, `ddl.table.name.suffix.require`, `ddl.table.name.contains.require` |
| 列名 | `ddl.column.name.prefix.require`, `ddl.column.name.suffix.require`, `ddl.column.name.contains.require` |
| 唯一索引名 | `ddl.index.unique.prefix.require`, `ddl.index.unique.suffix.require`, `ddl.index.unique.contains.require` |
| 普通二级索引名 | `ddl.index.secondary.prefix.require`, `ddl.index.secondary.suffix.require`, `ddl.index.secondary.contains.require` |
| 全文索引名 | `ddl.index.fulltext.prefix.require`, `ddl.index.fulltext.suffix.require`, `ddl.index.fulltext.contains.require` |
| 主键约束名 | `ddl.constraint.primary_key.name.prefix.require`, `ddl.constraint.primary_key.name.suffix.require`, `ddl.constraint.primary_key.name.contains.require` |
| 唯一键约束名 | `ddl.constraint.unique_key.name.prefix.require`, `ddl.constraint.unique_key.name.suffix.require`, `ddl.constraint.unique_key.name.contains.require` |
| 外键约束名 | `ddl.constraint.foreign_key.name.prefix.require`, `ddl.constraint.foreign_key.name.suffix.require`, `ddl.constraint.foreign_key.name.contains.require` |
| CHECK 约束名 | `ddl.constraint.check.name.prefix.require`, `ddl.constraint.check.name.suffix.require`, `ddl.constraint.check.name.contains.require` |

代表性配置示例：

```yaml
rules:
  ddl.table.name.prefix.require:
    enabled: true
    level: warning
    params:
      prefix: "tbl_"

  ddl.column.name.suffix.require:
    enabled: true
    level: warning
    params:
      suffix: "_id"

  ddl.index.secondary.prefix.require:
    enabled: true
    level: warning
    params:
      prefix: "idx_"

  ddl.constraint.foreign_key.name.contains.require:
    enabled: true
    level: warning
    params:
      contains: ["user", "account"] # OR 语义
```

---

## DDL：建表规则（Create Table Rules）

### `ddl.table.comment.require`

要求 `CREATE TABLE` 语句包含非空 `COMMENT` 子句，以描述表的用途。

- **默认**：已启用，级别 `warning`

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `required` | `bool` | `true` | 设为 `false` 可完全禁用注释检查 |

**触发示例（Triggers）：**
```sql
CREATE TABLE orders (id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY);
```

**通过示例（Passes）：**
```sql
CREATE TABLE orders (id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY)
  COMMENT '订单主表';
```

**YAML 配置示例：**
```yaml
rules:
  ddl.table.comment.require:
    enabled: true
    level: warning
    params:
      required: true
```

---

### `ddl.table.comment.max_length`

限制表注释的最大字符数。

- **默认**：已启用，级别 `warning`

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `limit` | `int` | `128` | 表注释允许的最大字符数 |

**触发示例：**
```sql
CREATE TABLE t (id BIGINT) COMMENT '这是一段超过 128 个字符的表注释，包含了大量冗余描述，实际上并不需要写得如此详细，适当简洁即可……';
```

**通过示例：**
```sql
CREATE TABLE t (id BIGINT) COMMENT '简短的表描述';
```

**YAML 配置示例：**
```yaml
rules:
  ddl.table.comment.max_length:
    enabled: true
    level: warning
    params:
      limit: 128
```

---

### `ddl.table.name.max_length`

限制表名的最大字符长度。

- **默认**：已启用，级别 `blocker`

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `limit` | `int` | `64` | 表名允许的最大字符数（≥ 1） |

**触发示例：**
```sql
CREATE TABLE this_is_a_very_long_table_name_that_exceeds_the_maximum_allowed_length (id BIGINT);
```

**通过示例：**
```sql
CREATE TABLE orders (id BIGINT);
```

**YAML 配置示例：**
```yaml
rules:
  ddl.table.name.max_length:
    enabled: true
    level: blocker
    params:
      limit: 64
```

---

### `ddl.table.name.pattern.require`

要求表名匹配指定的正则表达式模式（pattern）。

- **默认**：已启用，级别 `blocker`

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `required` | `bool` | `true` | 设为 `false` 则跳过模式校验 |
| `pattern` | `string` | `^[A-Za-z0-9_]+$` | 表名必须匹配的正则表达式 |

**触发示例：**
```sql
CREATE TABLE order-items (id BIGINT);
```

**通过示例：**
```sql
CREATE TABLE order_items (id BIGINT);
```

**YAML 配置示例：**
```yaml
rules:
  ddl.table.name.pattern.require:
    enabled: true
    level: blocker
    params:
      required: true
      pattern: "^[A-Za-z0-9_]+$"
```

---

### `ddl.table.name.keyword.forbid`

禁止使用 SQL 保留关键字作为表名。

- **默认**：已启用，级别 `blocker`

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `forbid` | `bool` | `true` | 设为 `false` 则允许保留字表名 |

**触发示例：**
```sql
CREATE TABLE `select` (id BIGINT);
```

**通过示例：**
```sql
CREATE TABLE user_select (id BIGINT);
```

**YAML 配置示例：**
```yaml
rules:
  ddl.table.name.keyword.forbid:
    enabled: true
    level: blocker
    params:
      forbid: true
```

---

### `ddl.table.columns.min_count`

要求 `CREATE TABLE` 至少定义指定数量的列。

- **默认**：已启用，级别 `blocker`

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `limit` | `int` | `1` | 表至少需要定义的列数（≥ 1） |

**触发示例：**
```sql
CREATE TABLE empty_table ();
```

**通过示例：**
```sql
CREATE TABLE minimal (id BIGINT);
```

**YAML 配置示例：**
```yaml
rules:
  ddl.table.columns.min_count:
    enabled: true
    level: blocker
    params:
      limit: 1
```

---

### `ddl.table.engine.allowlist`

要求表使用许可列表（allowlist）中的存储引擎（engine）。

- **默认**：已启用，级别 `blocker`

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `values` | `[]string` | `["InnoDB"]` | 允许使用的存储引擎名称列表（大小写不敏感） |

**触发示例：**
```sql
CREATE TABLE t (id BIGINT) ENGINE=MyISAM;
```

**通过示例：**
```sql
CREATE TABLE t (id BIGINT) ENGINE=InnoDB;
```

**YAML 配置示例：**
```yaml
rules:
  ddl.table.engine.allowlist:
    enabled: true
    level: blocker
    params:
      values: [InnoDB]
```

---

### `ddl.table.charset.allowlist`

要求表级字符集（charset）在许可列表内。

- **默认**：已启用，级别 `blocker`

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `values` | `[]string` | `["utf8", "utf8mb4"]` | 允许使用的字符集列表（大小写不敏感） |

**触发示例：**
```sql
CREATE TABLE t (id BIGINT) CHARSET=latin1;
```

**通过示例：**
```sql
CREATE TABLE t (id BIGINT) CHARSET=utf8mb4;
```

**YAML 配置示例：**
```yaml
rules:
  ddl.table.charset.allowlist:
    enabled: true
    level: blocker
    params:
      values: [utf8, utf8mb4]
```

---

### `ddl.table.row_format.allowlist`

要求表的行格式（row format）在许可列表内。

- **默认**：已启用，级别 `blocker`

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `values` | `[]string` | `["DYNAMIC"]` | 允许使用的行格式列表 |
| `require_explicit` | `bool` | `false` | 设为 `true` 则要求 DDL 中必须显式声明行格式 |

**触发示例：**
```sql
CREATE TABLE t (id BIGINT) ROW_FORMAT=COMPACT;
```

**通过示例：**
```sql
CREATE TABLE t (id BIGINT) ROW_FORMAT=DYNAMIC;
```

**YAML 配置示例：**
```yaml
rules:
  ddl.table.row_format.allowlist:
    enabled: true
    level: blocker
    params:
      values: [DYNAMIC]
      require_explicit: false
```

---

### `ddl.table.auto_increment.init_value.require`

要求 `AUTO_INCREMENT` 初始值等于指定值。

- **默认**：已启用，级别 `blocker`

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `value` | `int` | `1` | `AUTO_INCREMENT` 的要求初始值 |

**触发示例：**
```sql
CREATE TABLE t (id BIGINT AUTO_INCREMENT PRIMARY KEY) AUTO_INCREMENT=1000;
```

**通过示例：**
```sql
CREATE TABLE t (id BIGINT AUTO_INCREMENT PRIMARY KEY) AUTO_INCREMENT=1;
```

**YAML 配置示例：**
```yaml
rules:
  ddl.table.auto_increment.init_value.require:
    enabled: true
    level: blocker
    params:
      value: 1
```

---

### `ddl.table.foreign_key.forbid`

禁止在 `CREATE TABLE` 中定义外键（FOREIGN KEY）。

- **默认**：已启用，级别 `blocker`

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `forbid` | `bool` | `true` | 设为 `false` 则允许外键 |

**触发示例：**
```sql
CREATE TABLE orders (
  id BIGINT PRIMARY KEY,
  user_id BIGINT,
  FOREIGN KEY (user_id) REFERENCES users(id)
);
```

**通过示例：**
```sql
CREATE TABLE orders (id BIGINT PRIMARY KEY, user_id BIGINT);
```

**YAML 配置示例：**
```yaml
rules:
  ddl.table.foreign_key.forbid:
    enabled: true
    level: blocker
    params:
      forbid: true
```

---

### `ddl.table.partition.forbid`

禁止在 `CREATE TABLE` 中使用分区（PARTITION）。

- **默认**：已启用，级别 `blocker`

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `forbid` | `bool` | `true` | 设为 `false` 则允许分区 |

**触发示例：**
```sql
CREATE TABLE t (id BIGINT, dt DATE)
PARTITION BY RANGE (YEAR(dt)) (
  PARTITION p0 VALUES LESS THAN (2020)
);
```

**通过示例：**
```sql
CREATE TABLE t (id BIGINT, dt DATE);
```

**YAML 配置示例：**
```yaml
rules:
  ddl.table.partition.forbid:
    enabled: true
    level: blocker
    params:
      forbid: true
```

---

### `ddl.table.create_like.forbid`

禁止使用 `CREATE TABLE ... LIKE` 语法。

- **默认**：已启用，级别 `blocker`

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `forbid` | `bool` | `true` | 设为 `false` 则允许 LIKE 建表 |

**触发示例：**
```sql
CREATE TABLE t2 LIKE t1;
```

**通过示例：**
```sql
CREATE TABLE t2 (id BIGINT);
```

**YAML 配置示例：**
```yaml
rules:
  ddl.table.create_like.forbid:
    enabled: true
    level: blocker
    params:
      forbid: true
```

---

### `ddl.table.create_as.forbid`

禁止使用 `CREATE TABLE ... AS SELECT` 语法。

- **默认**：已启用，级别 `blocker`

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `forbid` | `bool` | `true` | 设为 `false` 则允许 AS SELECT 建表 |

**触发示例：**
```sql
CREATE TABLE t2 AS SELECT * FROM t1;
```

**通过示例：**
```sql
CREATE TABLE t2 (id BIGINT, name VARCHAR(64));
```

**YAML 配置示例：**
```yaml
rules:
  ddl.table.create_as.forbid:
    enabled: true
    level: blocker
    params:
      forbid: true
```

---

### `ddl.table.audit_columns.require`

要求表包含创建时间和更新时间两个审计时间列（audit timestamp columns）：一个设置了 `DEFAULT CURRENT_TIMESTAMP`，另一个设置了 `DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP`。

- **默认**：已启用，级别 `warning`

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `required` | `bool` | `true` | 设为 `false` 则跳过审计列检查 |

**触发示例：**
```sql
CREATE TABLE orders (id BIGINT PRIMARY KEY, amount DECIMAL(10,2));
```

**通过示例：**
```sql
CREATE TABLE orders (
  id BIGINT PRIMARY KEY,
  amount DECIMAL(10,2),
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
);
```

**YAML 配置示例：**
```yaml
rules:
  ddl.table.audit_columns.require:
    enabled: true
    level: warning
    params:
      required: true
```

---

### `ddl.table.row_size.max_bytes.require`

估算 InnoDB 行大小，超过限制时报告 finding。依赖实例事实（instance facts）中的字符集和行格式信息。

> **需要元数据感知模式。** 离线审计时本规则不生效（no-op）。

- **默认**：已启用，级别 `blocker`

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `required` | `bool` | `true` | 设为 `false` 则禁用行大小估算 |

**触发示例：**
```sql
-- 仅在元数据感知模式下，当估算行大小 > 65535 字节时触发
CREATE TABLE wide (
  c1 VARCHAR(16383),
  c2 VARCHAR(16383),
  c3 VARCHAR(16383),
  c4 VARCHAR(16383)
);
```

**通过示例：**
```sql
CREATE TABLE compact (id BIGINT, name VARCHAR(255));
```

**YAML 配置示例：**
```yaml
rules:
  ddl.table.row_size.max_bytes.require:
    enabled: true
    level: blocker
    params:
      required: true
```

---

### `ddl.table.denylist.forbid`

禁止对黑名单（denylist）中指定的 schema 或表执行 DDL 操作。

- **默认**：已启用，级别 `blocker`

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `schemas` | `[]string` | `[]` | 被禁止操作的 schema 名称列表 |
| `tables` | `[]string` | `[]` | 被禁止操作的表名列表（不含 schema 限定） |
| `qualified_tables` | `[]string` | `[]` | 被禁止操作的全限定表名列表，格式为 `schema.table` |

**触发示例：**
```sql
-- 当 "core" 在 schemas 列表中时触发
CREATE TABLE core.settings (id BIGINT);
```

**YAML 配置示例：**
```yaml
rules:
  ddl.table.denylist.forbid:
    enabled: true
    level: blocker
    params:
      schemas: []
      tables: []
      qualified_tables: []
```

---

### `ddl.table.exists.create.forbid`

当目标表已存在于数据库中时，禁止执行 `CREATE TABLE`。

> **需要元数据感知模式。** 离线审计时本规则不生效（no-op）。

- **默认**：已启用，级别 `blocker`
- **参数**：无

**YAML 配置示例：**
```yaml
rules:
  ddl.table.exists.create.forbid:
    enabled: true
    level: blocker
    params:
```

---

### `ddl.table.primary_key.require`

要求 `CREATE TABLE` 定义主键（PRIMARY KEY）。

- **默认**：已启用，级别 `blocker`

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `required` | `bool` | `true` | 设为 `false` 则允许无主键建表 |

**触发示例：**
```sql
CREATE TABLE t (id BIGINT, name VARCHAR(64));
```

**通过示例：**
```sql
CREATE TABLE t (id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY);
```

**YAML 配置示例：**
```yaml
rules:
  ddl.table.primary_key.require:
    enabled: true
    level: blocker
    params:
      required: true
```

---

### `ddl.table.primary_key.columns.max_count`

限制主键最多包含的列数。

- **默认**：已启用，级别 `warning`

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `limit` | `int` | `1` | 主键允许的最大列数 |

**触发示例：**
```sql
CREATE TABLE t (a BIGINT, b BIGINT, PRIMARY KEY (a, b));
```

**通过示例：**
```sql
CREATE TABLE t (id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY);
```

**YAML 配置示例：**
```yaml
rules:
  ddl.table.primary_key.columns.max_count:
    enabled: true
    level: warning
    params:
      limit: 1
```

---

### `ddl.table.primary_key.bigint.require`

要求单列主键使用 `BIGINT` 类型。

- **默认**：已启用，级别 `blocker`

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `required` | `bool` | `true` | 设为 `false` 则跳过此检查 |

**触发示例：**
```sql
CREATE TABLE t (id INT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY);
```

**通过示例：**
```sql
CREATE TABLE t (id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY);
```

**YAML 配置示例：**
```yaml
rules:
  ddl.table.primary_key.bigint.require:
    enabled: true
    level: blocker
    params:
      required: true
```

---

### `ddl.table.primary_key.unsigned.require`

要求单列主键声明为 `UNSIGNED`。

- **默认**：已启用，级别 `blocker`

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `required` | `bool` | `true` | 设为 `false` 则跳过此检查 |

**触发示例：**
```sql
CREATE TABLE t (id BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY);
```

**通过示例：**
```sql
CREATE TABLE t (id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY);
```

**YAML 配置示例：**
```yaml
rules:
  ddl.table.primary_key.unsigned.require:
    enabled: true
    level: blocker
    params:
      required: true
```

---

### `ddl.table.primary_key.auto_increment.require`

要求单列主键使用 `AUTO_INCREMENT`。

- **默认**：已启用，级别 `blocker`

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `required` | `bool` | `true` | 设为 `false` 则跳过此检查 |

**触发示例：**
```sql
CREATE TABLE t (id BIGINT UNSIGNED NOT NULL PRIMARY KEY);
```

**通过示例：**
```sql
CREATE TABLE t (id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY);
```

**YAML 配置示例：**
```yaml
rules:
  ddl.table.primary_key.auto_increment.require:
    enabled: true
    level: blocker
    params:
      required: true
```

---

### `ddl.table.primary_key.not_null.require`

要求主键列声明 `NOT NULL`。

- **默认**：已启用，级别 `blocker`

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `required` | `bool` | `true` | 设为 `false` 则跳过此检查 |

**触发示例：**
```sql
CREATE TABLE t (id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY);
```

**通过示例：**
```sql
CREATE TABLE t (id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY);
```

**YAML 配置示例：**
```yaml
rules:
  ddl.table.primary_key.not_null.require:
    enabled: true
    level: blocker
    params:
      required: true
```

---

## DDL：列规则（Column Rules）

### `ddl.column.comment.require`

要求每个列都定义 `COMMENT`。

- **默认**：已启用，级别 `warning`

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `required` | `bool` | `true` | 设为 `false` 则跳过列注释检查 |

**触发示例：**
```sql
CREATE TABLE t (id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY);
```

**通过示例：**
```sql
CREATE TABLE t (id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY COMMENT '主键ID');
```

**YAML 配置示例：**
```yaml
rules:
  ddl.column.comment.require:
    enabled: true
    level: warning
    params:
      required: true
```

---

### `ddl.column.name.max_length`

限制列名的最大字符数。

- **默认**：已启用，级别 `blocker`

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `limit` | `int` | `64` | 列名允许的最大字符数（≥ 1） |

**触发示例：**
```sql
CREATE TABLE t (this_column_name_is_way_too_long_to_be_acceptable_in_any_database BIGINT);
```

**通过示例：**
```sql
CREATE TABLE t (user_id BIGINT);
```

**YAML 配置示例：**
```yaml
rules:
  ddl.column.name.max_length:
    enabled: true
    level: blocker
    params:
      limit: 64
```

---

### `ddl.column.name.pattern.require`

要求列名匹配指定的正则表达式模式。

- **默认**：已启用，级别 `blocker`

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `required` | `bool` | `true` | 设为 `false` 则跳过模式校验 |
| `pattern` | `string` | `^[A-Za-z0-9_]+$` | 列名必须匹配的正则表达式 |

**触发示例：**
```sql
CREATE TABLE t (user-name VARCHAR(64));
```

**通过示例：**
```sql
CREATE TABLE t (user_name VARCHAR(64));
```

**YAML 配置示例：**
```yaml
rules:
  ddl.column.name.pattern.require:
    enabled: true
    level: blocker
    params:
      required: true
      pattern: "^[A-Za-z0-9_]+$"
```

---

### `ddl.column.name.keyword.forbid`

禁止使用 SQL 保留关键字作为列名。

- **默认**：已启用，级别 `blocker`

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `forbid` | `bool` | `true` | 设为 `false` 则允许保留字列名 |

**触发示例：**
```sql
CREATE TABLE t (`select` BIGINT, `from` VARCHAR(64));
```

**通过示例：**
```sql
CREATE TABLE t (user_select BIGINT, source_from VARCHAR(64));
```

**YAML 配置示例：**
```yaml
rules:
  ddl.column.name.keyword.forbid:
    enabled: true
    level: blocker
    params:
      forbid: true
```

---

### `ddl.column.default.require`

要求每个列（blob/text 类型除外）都声明默认值（DEFAULT）。

- **默认**：已启用，级别 `warning`

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `required` | `bool` | `true` | 设为 `false` 则跳过默认值检查 |

**触发示例：**
```sql
CREATE TABLE t (name VARCHAR(64) NOT NULL);
```

**通过示例：**
```sql
CREATE TABLE t (name VARCHAR(64) NOT NULL DEFAULT '');
```

**YAML 配置示例：**
```yaml
rules:
  ddl.column.default.require:
    enabled: true
    level: warning
    params:
      required: true
```

---

### `ddl.column.not_null.require`

要求每个列（blob/text 及可选地时间类型列除外）声明 `NOT NULL`。

- **默认**：已启用，级别 `warning`

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `required` | `bool` | `true` | 设为 `false` 则跳过非空检查 |
| `allow_time_null` | `bool` | `true` | 设为 `true` 时，datetime/timestamp/date/time/year 类型允许可空 |

**触发示例：**
```sql
CREATE TABLE t (name VARCHAR(64));
```

**通过示例：**
```sql
CREATE TABLE t (name VARCHAR(64) NOT NULL DEFAULT '');
```

**YAML 配置示例：**
```yaml
rules:
  ddl.column.not_null.require:
    enabled: true
    level: warning
    params:
      required: true
      allow_time_null: true
```

---

### `ddl.column.varchar.max_length`

限制 `VARCHAR` 列的最大字符长度。

- **默认**：已启用，级别 `blocker`

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `limit` | `int` | `16383` | VARCHAR 允许的最大字符数（≥ 1） |

**触发示例：**
```sql
CREATE TABLE t (content VARCHAR(20000));
```

**通过示例：**
```sql
CREATE TABLE t (content VARCHAR(1000));
```

**YAML 配置示例：**
```yaml
rules:
  ddl.column.varchar.max_length:
    enabled: true
    level: blocker
    params:
      limit: 16383
```

---

### `ddl.column.char.max_length`

限制 `CHAR` 列的最大字符长度。

- **默认**：已启用，级别 `warning`

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `limit` | `int` | `64` | CHAR 允许的最大字符数 |

**触发示例：**
```sql
CREATE TABLE t (code CHAR(200));
```

**通过示例：**
```sql
CREATE TABLE t (code CHAR(32));
```

**YAML 配置示例：**
```yaml
rules:
  ddl.column.char.max_length:
    enabled: true
    level: warning
    params:
      limit: 64
```

---

### `ddl.column.float_double.forbid`

禁止使用 `FLOAT` 或 `DOUBLE` 类型（存在精度丢失风险）。

- **默认**：已启用，级别 `warning`

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `forbid` | `bool` | `true` | 设为 `false` 则允许浮点类型 |

**触发示例：**
```sql
CREATE TABLE t (price DOUBLE, ratio FLOAT);
```

**通过示例：**
```sql
CREATE TABLE t (price DECIMAL(10, 2));
```

**YAML 配置示例：**
```yaml
rules:
  ddl.column.float_double.forbid:
    enabled: true
    level: warning
    params:
      forbid: true
```

---

### `ddl.column.blob_text.forbid`

警告或禁止使用 `BLOB`/`TEXT` 系列类型（不含 `JSON`）。

- **默认**：已启用，级别 `warning`，`forbid: false`（仅发出警告，不阻断）

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `forbid` | `bool` | `false` | 设为 `true` 则产生 finding，`false` 时规则不触发 |

**触发示例（当 `forbid: true` 时）：**
```sql
CREATE TABLE t (content TEXT, data LONGBLOB);
```

**通过示例：**
```sql
CREATE TABLE t (content VARCHAR(4096));
```

**YAML 配置示例：**
```yaml
rules:
  ddl.column.blob_text.forbid:
    enabled: true
    level: warning
    params:
      forbid: false
```

---

### `ddl.column.json.forbid`

警告或禁止使用 `JSON` 类型。

- **默认**：已启用，级别 `warning`，`forbid: false`（仅发出警告，不阻断）

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `forbid` | `bool` | `false` | 设为 `true` 则产生 finding |

**触发示例（当 `forbid: true` 时）：**
```sql
CREATE TABLE t (metadata JSON);
```

**通过示例：**
```sql
CREATE TABLE t (metadata VARCHAR(2048));
```

**YAML 配置示例：**
```yaml
rules:
  ddl.column.json.forbid:
    enabled: true
    level: warning
    params:
      forbid: false
```

---

### `ddl.column.bit.forbid`

警告或禁止使用 `BIT` 类型。

- **默认**：已启用，级别 `warning`，`forbid: false`（仅发出警告，不阻断）

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `forbid` | `bool` | `false` | 设为 `true` 则产生 finding |

**触发示例（当 `forbid: true` 时）：**
```sql
CREATE TABLE t (flag BIT(1));
```

**通过示例：**
```sql
CREATE TABLE t (flag TINYINT UNSIGNED NOT NULL DEFAULT 0);
```

**YAML 配置示例：**
```yaml
rules:
  ddl.column.bit.forbid:
    enabled: true
    level: warning
    params:
      forbid: false
```

---

### `ddl.column.timestamp.forbid`

禁止使用 `TIMESTAMP` 类型（建议改用 `DATETIME`）。

- **默认**：已启用，级别 `warning`，`forbid: true`

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `forbid` | `bool` | `true` | 设为 `false` 则允许 TIMESTAMP |

**触发示例：**
```sql
CREATE TABLE t (created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP);
```

**通过示例：**
```sql
CREATE TABLE t (created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP);
```

**YAML 配置示例：**
```yaml
rules:
  ddl.column.timestamp.forbid:
    enabled: true
    level: warning
    params:
      forbid: true
```

---

### `ddl.column.charset.allowlist`

要求列级字符集在许可列表内。

- **默认**：已启用，级别 `blocker`

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `values` | `[]string` | `["utf8", "utf8mb4"]` | 允许使用的字符集列表（大小写不敏感） |

**触发示例：**
```sql
CREATE TABLE t (name VARCHAR(64) CHARSET latin1);
```

**通过示例：**
```sql
CREATE TABLE t (name VARCHAR(64) CHARSET utf8mb4);
```

**YAML 配置示例：**
```yaml
rules:
  ddl.column.charset.allowlist:
    enabled: true
    level: blocker
    params:
      values: [utf8, utf8mb4]
```

---

### `ddl.column.collation.allowlist`

要求列级排序规则（collation）在许可列表内。

- **默认**：已启用，级别 `blocker`

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `values` | `[]string` | `["utf8_general_ci", "utf8mb4_general_ci", "utf8mb4_bin"]` | 允许使用的 collation 列表（大小写不敏感） |

**触发示例：**
```sql
CREATE TABLE t (name VARCHAR(64) COLLATE utf8mb4_unicode_ci);
```

**通过示例：**
```sql
CREATE TABLE t (name VARCHAR(64) COLLATE utf8mb4_general_ci);
```

**YAML 配置示例：**
```yaml
rules:
  ddl.column.collation.allowlist:
    enabled: true
    level: blocker
    params:
      values: [utf8_general_ci, utf8mb4_general_ci, utf8mb4_bin]
```

---

### `ddl.column.charset_collation.match.require`

要求列的字符集与排序规则相匹配（例如 utf8mb4 字符集不能搭配 utf8 的 collation）。

- **默认**：已启用，级别 `blocker`

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `required` | `bool` | `true` | 设为 `false` 则跳过匹配检查 |

**触发示例：**
```sql
CREATE TABLE t (name VARCHAR(64) CHARSET utf8mb4 COLLATE utf8_general_ci);
```

**通过示例：**
```sql
CREATE TABLE t (name VARCHAR(64) CHARSET utf8mb4 COLLATE utf8mb4_general_ci);
```

**YAML 配置示例：**
```yaml
rules:
  ddl.column.charset_collation.match.require:
    enabled: true
    level: blocker
    params:
      required: true
```

---

## DDL：索引规则（Index Rules）

### `ddl.index.total.max_count`

限制单张表的索引总数（含所有类型）。

- **默认**：已启用，级别 `warning`

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `limit` | `int` | `12` | 允许的最大索引数量 |

**触发示例：**
```sql
CREATE TABLE t (
  id BIGINT PRIMARY KEY,
  a INT, b INT, c INT, d INT, e INT, f INT, g INT, h INT, i INT, j INT, k INT, l INT,
  INDEX idx_a(a), INDEX idx_b(b), INDEX idx_c(c), INDEX idx_d(d),
  INDEX idx_e(e), INDEX idx_f(f), INDEX idx_g(g), INDEX idx_h(h),
  INDEX idx_i(i), INDEX idx_j(j), INDEX idx_k(k), INDEX idx_l(l), INDEX idx_a2(a,b)
);
```

**YAML 配置示例：**
```yaml
rules:
  ddl.index.total.max_count:
    enabled: true
    level: warning
    params:
      limit: 12
```

---

### `ddl.index.columns.max_count`

限制单个索引包含的最大列数（复合索引）。

- **默认**：已启用，级别 `warning`

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `limit` | `int` | `8` | 单个索引允许的最大列数 |

**触发示例：**
```sql
CREATE TABLE t (a INT, b INT, c INT, d INT, e INT, f INT, g INT, h INT, i INT,
  INDEX idx_many(a,b,c,d,e,f,g,h,i));
```

**通过示例：**
```sql
CREATE TABLE t (a INT, b INT, c INT, INDEX idx_abc(a,b,c));
```

**YAML 配置示例：**
```yaml
rules:
  ddl.index.columns.max_count:
    enabled: true
    level: warning
    params:
      limit: 8
```

---

### `ddl.index.unique.prefix.require`

要求唯一索引（UNIQUE INDEX）名称以指定前缀开头。

- **默认**：已启用，级别 `warning`

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `required` | `bool` | `true` | 设为 `false` 则跳过前缀检查 |
| `prefix` | `string` | `uniq_` | 唯一索引名称必须以此前缀开头 |

**触发示例：**
```sql
CREATE TABLE t (id BIGINT, email VARCHAR(255), UNIQUE INDEX email_unique(email));
```

**通过示例：**
```sql
CREATE TABLE t (id BIGINT, email VARCHAR(255), UNIQUE INDEX uniq_email(email));
```

**YAML 配置示例：**
```yaml
rules:
  ddl.index.unique.prefix.require:
    enabled: true
    level: warning
    params:
      required: true
      prefix: "uniq_"
```

---

### `ddl.index.secondary.prefix.require`

要求普通二级索引（SECONDARY INDEX）名称以指定前缀开头。

- **默认**：已启用，级别 `warning`

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `required` | `bool` | `true` | 设为 `false` 则跳过前缀检查 |
| `prefix` | `string` | `idx_` | 普通索引名称必须以此前缀开头 |

**触发示例：**
```sql
CREATE TABLE t (id BIGINT, name VARCHAR(64), INDEX name_idx(name));
```

**通过示例：**
```sql
CREATE TABLE t (id BIGINT, name VARCHAR(64), INDEX idx_name(name));
```

**YAML 配置示例：**
```yaml
rules:
  ddl.index.secondary.prefix.require:
    enabled: true
    level: warning
    params:
      required: true
      prefix: "idx_"
```

---

### `ddl.index.fulltext.prefix.require`

要求全文索引（FULLTEXT INDEX）名称以指定前缀开头。

- **默认**：已启用，级别 `warning`

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `required` | `bool` | `true` | 设为 `false` 则跳过前缀检查 |
| `prefix` | `string` | `full_` | 全文索引名称必须以此前缀开头 |

**触发示例：**
```sql
CREATE TABLE t (id BIGINT, content TEXT, FULLTEXT INDEX content_ft(content));
```

**通过示例：**
```sql
CREATE TABLE t (id BIGINT, content TEXT, FULLTEXT INDEX full_content(content));
```

**YAML 配置示例：**
```yaml
rules:
  ddl.index.fulltext.prefix.require:
    enabled: true
    level: warning
    params:
      required: true
      prefix: "full_"
```

---

### `ddl.index.name.pattern.require`

要求索引名称匹配指定的正则表达式模式。

- **默认**：已启用，级别 `blocker`

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `required` | `bool` | `true` | 设为 `false` 则跳过模式校验 |
| `pattern` | `string` | `^[A-Za-z0-9_]+$` | 索引名称必须匹配的正则表达式 |

**触发示例：**
```sql
CREATE TABLE t (id BIGINT, name VARCHAR(64), INDEX idx-name(name));
```

**通过示例：**
```sql
CREATE TABLE t (id BIGINT, name VARCHAR(64), INDEX idx_name(name));
```

**YAML 配置示例：**
```yaml
rules:
  ddl.index.name.pattern.require:
    enabled: true
    level: blocker
    params:
      required: true
      pattern: "^[A-Za-z0-9_]+$"
```

---

### `ddl.index.name.keyword.forbid`

禁止使用 SQL 保留关键字作为索引名称。

- **默认**：已启用，级别 `blocker`

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `forbid` | `bool` | `true` | 设为 `false` 则允许保留字索引名 |

**YAML 配置示例：**
```yaml
rules:
  ddl.index.name.keyword.forbid:
    enabled: true
    level: blocker
    params:
      forbid: true
```

---

### `ddl.index.duplicate.forbid`

禁止在同一张表中定义列完全相同的重复索引。

- **默认**：已启用，级别 `warning`

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `forbid` | `bool` | `true` | 设为 `false` 则允许重复索引 |

**触发示例：**
```sql
CREATE TABLE t (id BIGINT, a INT, INDEX idx_a1(a), INDEX idx_a2(a));
```

**通过示例：**
```sql
CREATE TABLE t (id BIGINT, a INT, b INT, INDEX idx_a(a), INDEX idx_ab(a, b));
```

**YAML 配置示例：**
```yaml
rules:
  ddl.index.duplicate.forbid:
    enabled: true
    level: warning
    params:
      forbid: true
```

---

### `ddl.index.redundant_left_prefix.forbid`

禁止定义左前缀冗余的索引（即某索引的列序列是另一个索引的前缀子集）。

- **默认**：已启用，级别 `warning`

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `forbid` | `bool` | `true` | 设为 `false` 则允许冗余前缀索引 |

**触发示例：**
```sql
CREATE TABLE t (id BIGINT, a INT, b INT, INDEX idx_a(a), INDEX idx_ab(a, b));
-- idx_a 是 idx_ab 的左前缀，属于冗余索引
```

**YAML 配置示例：**
```yaml
rules:
  ddl.index.redundant_left_prefix.forbid:
    enabled: true
    level: warning
    params:
      forbid: true
```

---

### `ddl.index.redundant_unique_overlap.forbid`

禁止唯一索引的列集合与另一个唯一索引重叠（使其中一个变为冗余）。

- **默认**：已启用，级别 `warning`

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `forbid` | `bool` | `true` | 设为 `false` 则允许唯一索引重叠 |

**YAML 配置示例：**
```yaml
rules:
  ddl.index.redundant_unique_overlap.forbid:
    enabled: true
    level: warning
    params:
      forbid: true
```

---

### `ddl.index.key_length.max_bytes.require`

估算索引键长度，超过引擎限制时报告 finding。依赖实例事实（instance facts）中的大前缀（large prefix）和版本信息。

> **需要元数据感知模式。** 离线审计时本规则不生效（no-op）。

- **默认**：已启用，级别 `blocker`

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `required` | `bool` | `true` | 设为 `false` 则禁用键长度估算 |

**YAML 配置示例：**
```yaml
rules:
  ddl.index.key_length.max_bytes.require:
    enabled: true
    level: blocker
    params:
      required: true
```

---

## DDL：视图规则（View Rules）

### `ddl.view.create.forbid`

禁止创建视图（CREATE VIEW）。

- **默认**：已启用，级别 `blocker`

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `forbid` | `bool` | `true` | 设为 `false` 则允许创建视图 |

**触发示例：**
```sql
CREATE VIEW v_users AS SELECT id, name FROM users;
```

**YAML 配置示例：**
```yaml
rules:
  ddl.view.create.forbid:
    enabled: true
    level: blocker
    params:
      forbid: true
```

---

## DDL：ALTER TABLE 规则（Alter Table Rules）

### `ddl.alter.drop_column.forbid`

禁止（或发出警告）执行 `DROP COLUMN` 操作。

- **默认**：已启用，级别 `warning`，`forbid: false`（仅发出警告，不阻断）

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `forbid` | `bool` | `false` | 设为 `true` 则阻断 DROP COLUMN |

**触发示例（当 `forbid: true` 时）：**
```sql
ALTER TABLE users DROP COLUMN middle_name;
```

**YAML 配置示例：**
```yaml
rules:
  ddl.alter.drop_column.forbid:
    enabled: true
    level: warning
    params:
      forbid: false
```

---

### `ddl.alter.drop_primary_key.forbid`

禁止删除主键（DROP PRIMARY KEY）。

- **默认**：已启用，级别 `blocker`，`forbid: true`

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `forbid` | `bool` | `true` | 设为 `false` 则允许删除主键 |

**触发示例：**
```sql
ALTER TABLE t DROP PRIMARY KEY;
```

**YAML 配置示例：**
```yaml
rules:
  ddl.alter.drop_primary_key.forbid:
    enabled: true
    level: blocker
    params:
      forbid: true
```

---

### `ddl.alter.drop_index.forbid`

禁止（或发出警告）执行 `DROP INDEX` 操作。

- **默认**：已启用，级别 `warning`，`forbid: false`

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `forbid` | `bool` | `false` | 设为 `true` 则阻断 DROP INDEX |

**YAML 配置示例：**
```yaml
rules:
  ddl.alter.drop_index.forbid:
    enabled: true
    level: warning
    params:
      forbid: false
```

---

### `ddl.alter.rename_table.forbid`

禁止 ALTER TABLE 中的重命名表操作（RENAME TABLE）。

- **默认**：已启用，级别 `blocker`

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `forbid` | `bool` | `true` | 设为 `false` 则允许重命名表 |

**触发示例：**
```sql
ALTER TABLE users RENAME TO members;
```

**YAML 配置示例：**
```yaml
rules:
  ddl.alter.rename_table.forbid:
    enabled: true
    level: blocker
    params:
      forbid: true
```

---

### `ddl.alter.rename_column.forbid`

禁止重命名列（RENAME COLUMN）。

- **默认**：已启用，级别 `blocker`

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `forbid` | `bool` | `true` | 设为 `false` 则允许重命名列 |

**触发示例：**
```sql
ALTER TABLE users RENAME COLUMN user_name TO username;
```

**YAML 配置示例：**
```yaml
rules:
  ddl.alter.rename_column.forbid:
    enabled: true
    level: blocker
    params:
      forbid: true
```

---

### `ddl.alter.rename_index.forbid`

禁止重命名索引（RENAME INDEX）。

- **默认**：已启用，级别 `blocker`

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `forbid` | `bool` | `true` | 设为 `false` 则允许重命名索引 |

**触发示例：**
```sql
ALTER TABLE t RENAME INDEX idx_old TO idx_new;
```

**YAML 配置示例：**
```yaml
rules:
  ddl.alter.rename_index.forbid:
    enabled: true
    level: blocker
    params:
      forbid: true
```

---

### `ddl.alter.change_column.forbid`

禁止使用 `CHANGE COLUMN` 语法（该语法可同时修改列名和定义）。推荐改用 `MODIFY COLUMN` 或 `RENAME COLUMN`。

- **默认**：已启用，级别 `blocker`

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `forbid` | `bool` | `true` | 设为 `false` 则允许 CHANGE COLUMN |

**触发示例：**
```sql
ALTER TABLE t CHANGE COLUMN old_name new_name VARCHAR(128);
```

**YAML 配置示例：**
```yaml
rules:
  ddl.alter.change_column.forbid:
    enabled: true
    level: blocker
    params:
      forbid: true
```

---

### `ddl.alter.modify_column.forbid`

禁止（或发出警告）使用 `MODIFY COLUMN` 语法。

- **默认**：已启用，级别 `warning`，`forbid: false`

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `forbid` | `bool` | `false` | 设为 `true` 则阻断 MODIFY COLUMN |

**YAML 配置示例：**
```yaml
rules:
  ddl.alter.modify_column.forbid:
    enabled: true
    level: warning
    params:
      forbid: false
```

---

### `ddl.alter.modify_column.target_type_family.allowlist`

`MODIFY COLUMN` 的目标类型族（type family）必须在许可列表内，防止跨系列的不兼容类型变更。

- **默认**：已启用，级别 `blocker`

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `required` | `bool` | `true` | 设为 `false` 则跳过类型族检查 |
| `allowed_type_families` | `[]string` | `["integer", "decimal", "string", "binary", "time"]` | 允许的目标类型族列表 |

可用类型族：`integer`、`decimal`、`string`、`binary`、`time`、`float`、`blob`、`text`、`other`

**触发示例：**
```sql
-- 将整型改为字符串类型，跨类型族变更
ALTER TABLE t MODIFY COLUMN user_id VARCHAR(64);
```

**YAML 配置示例：**
```yaml
rules:
  ddl.alter.modify_column.target_type_family.allowlist:
    enabled: true
    level: blocker
    params:
      required: true
      allowed_type_families: [integer, decimal, string, binary, time]
```

---

### `ddl.alter.change_column.target_type_family.allowlist`

`CHANGE COLUMN` 的目标类型族必须在许可列表内（与 `modify_column` 规则逻辑相同）。

- **默认**：已启用，级别 `blocker`

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `required` | `bool` | `true` | 设为 `false` 则跳过类型族检查 |
| `allowed_type_families` | `[]string` | `["integer", "decimal", "string", "binary", "time"]` | 允许的目标类型族列表 |

**YAML 配置示例：**
```yaml
rules:
  ddl.alter.change_column.target_type_family.allowlist:
    enabled: true
    level: blocker
    params:
      required: true
      allowed_type_families: [integer, decimal, string, binary, time]
```

---

### `ddl.alter.modify_column.compatibility.require`

检查 `MODIFY COLUMN` 的类型变更是否向后兼容（例如不允许缩减整型精度）。

- **默认**：已启用，级别 `blocker`

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `required` | `bool` | `true` | 设为 `false` 则跳过兼容性检查 |

**触发示例：**
```sql
-- 将 BIGINT 降级为 INT，可能导致数据截断
ALTER TABLE t MODIFY COLUMN id INT UNSIGNED NOT NULL;
```

**YAML 配置示例：**
```yaml
rules:
  ddl.alter.modify_column.compatibility.require:
    enabled: true
    level: blocker
    params:
      required: true
```

---

### `ddl.alter.change_column.compatibility.require`

检查 `CHANGE COLUMN` 的类型变更是否向后兼容。

- **默认**：已启用，级别 `blocker`

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `required` | `bool` | `true` | 设为 `false` 则跳过兼容性检查 |

**YAML 配置示例：**
```yaml
rules:
  ddl.alter.change_column.compatibility.require:
    enabled: true
    level: blocker
    params:
      required: true
```

---

### `ddl.alter.table_option.compatibility.require`

检查 ALTER TABLE 中表级选项（engine、charset 等）的变更是否兼容。

- **默认**：已启用，级别 `warning`

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `required` | `bool` | `true` | 设为 `false` 则跳过表选项兼容性检查 |

**YAML 配置示例：**
```yaml
rules:
  ddl.alter.table_option.compatibility.require:
    enabled: true
    level: warning
    params:
      required: true
```

---

### `ddl.alter.modify_column.explicit_nullability_change.forbid`

禁止 `MODIFY COLUMN` 中显式修改列的可空性（nullability），防止意外引入或去除 NULL。

- **默认**：已启用，级别 `blocker`

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `forbid` | `bool` | `true` | 设为 `false` 则允许显式修改可空性 |

**触发示例：**
```sql
-- 若 name 原本是 NOT NULL，现在显式改为 NULL
ALTER TABLE t MODIFY COLUMN name VARCHAR(128) NULL;
```

**YAML 配置示例：**
```yaml
rules:
  ddl.alter.modify_column.explicit_nullability_change.forbid:
    enabled: true
    level: blocker
    params:
      forbid: true
```

---

### `ddl.alter.change_column.explicit_nullability_change.forbid`

与 `modify_column.explicit_nullability_change.forbid` 逻辑相同，作用于 `CHANGE COLUMN`。

- **默认**：已启用，级别 `blocker`

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `forbid` | `bool` | `true` | 设为 `false` 则允许 |

**YAML 配置示例：**
```yaml
rules:
  ddl.alter.change_column.explicit_nullability_change.forbid:
    enabled: true
    level: blocker
    params:
      forbid: true
```

---

### `ddl.alter.modify_column.explicit_default_change.forbid`

禁止 `MODIFY COLUMN` 中显式修改列的默认值（DEFAULT）。

- **默认**：已启用，级别 `blocker`

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `forbid` | `bool` | `true` | 设为 `false` 则允许显式修改默认值 |

**YAML 配置示例：**
```yaml
rules:
  ddl.alter.modify_column.explicit_default_change.forbid:
    enabled: true
    level: blocker
    params:
      forbid: true
```

---

### `ddl.alter.change_column.explicit_default_change.forbid`

与 `modify_column.explicit_default_change.forbid` 逻辑相同，作用于 `CHANGE COLUMN`。

- **默认**：已启用，级别 `blocker`

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `forbid` | `bool` | `true` | 设为 `false` 则允许 |

**YAML 配置示例：**
```yaml
rules:
  ddl.alter.change_column.explicit_default_change.forbid:
    enabled: true
    level: blocker
    params:
      forbid: true
```

---

### `ddl.alter.modify_column.explicit_auto_increment_change.forbid`

禁止 `MODIFY COLUMN` 中显式修改列的 `AUTO_INCREMENT` 属性。

- **默认**：已启用，级别 `blocker`

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `forbid` | `bool` | `true` | 设为 `false` 则允许显式修改 AUTO_INCREMENT |

**YAML 配置示例：**
```yaml
rules:
  ddl.alter.modify_column.explicit_auto_increment_change.forbid:
    enabled: true
    level: blocker
    params:
      forbid: true
```

---

### `ddl.alter.change_column.explicit_auto_increment_change.forbid`

与 `modify_column.explicit_auto_increment_change.forbid` 逻辑相同，作用于 `CHANGE COLUMN`。

- **默认**：已启用，级别 `blocker`

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `forbid` | `bool` | `true` | 设为 `false` 则允许 |

**YAML 配置示例：**
```yaml
rules:
  ddl.alter.change_column.explicit_auto_increment_change.forbid:
    enabled: true
    level: blocker
    params:
      forbid: true
```

---

### `ddl.alter.add_index.columns.max_count`

限制 `ALTER TABLE ... ADD INDEX` 中单个新增索引包含的最大列数。

- **默认**：已启用，级别 `warning`

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `limit` | `int` | `8` | 新增索引允许的最大列数 |

**触发示例：**
```sql
ALTER TABLE t ADD INDEX idx_many(a,b,c,d,e,f,g,h,i);
```

**YAML 配置示例：**
```yaml
rules:
  ddl.alter.add_index.columns.max_count:
    enabled: true
    level: warning
    params:
      limit: 8
```

---

### `ddl.alter.add_index.duplicate.forbid`

禁止通过 `ALTER TABLE` 添加与已有索引列完全相同的重复索引。

- **默认**：已启用，级别 `warning`

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `forbid` | `bool` | `true` | 设为 `false` 则允许重复索引 |

**YAML 配置示例：**
```yaml
rules:
  ddl.alter.add_index.duplicate.forbid:
    enabled: true
    level: warning
    params:
      forbid: true
```

---

### `ddl.alter.add_index.redundant_left_prefix.forbid`

禁止通过 `ALTER TABLE` 添加左前缀冗余的索引。

- **默认**：已启用，级别 `warning`

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `forbid` | `bool` | `true` | 设为 `false` 则允许冗余前缀索引 |

**YAML 配置示例：**
```yaml
rules:
  ddl.alter.add_index.redundant_left_prefix.forbid:
    enabled: true
    level: warning
    params:
      forbid: true
```

---

### `ddl.alter.add_index.redundant_unique_overlap.forbid`

禁止通过 `ALTER TABLE` 添加与已有唯一索引重叠的唯一索引。

- **默认**：已启用，级别 `warning`

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `forbid` | `bool` | `true` | 设为 `false` 则允许唯一索引重叠 |

**YAML 配置示例：**
```yaml
rules:
  ddl.alter.add_index.redundant_unique_overlap.forbid:
    enabled: true
    level: warning
    params:
      forbid: true
```

---

### `ddl.alter.add_index.unique.prefix.require`

要求通过 `ALTER TABLE` 新增的唯一索引名称以指定前缀开头。

- **默认**：已启用，级别 `warning`

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `required` | `bool` | `true` | 设为 `false` 则跳过前缀检查 |
| `prefix` | `string` | `uniq_` | 唯一索引名称必须以此前缀开头 |

**YAML 配置示例：**
```yaml
rules:
  ddl.alter.add_index.unique.prefix.require:
    enabled: true
    level: warning
    params:
      required: true
      prefix: "uniq_"
```

---

### `ddl.alter.add_index.secondary.prefix.require`

要求通过 `ALTER TABLE` 新增的普通二级索引名称以指定前缀开头。

- **默认**：已启用，级别 `warning`

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `required` | `bool` | `true` | 设为 `false` 则跳过前缀检查 |
| `prefix` | `string` | `idx_` | 普通索引名称必须以此前缀开头 |

**YAML 配置示例：**
```yaml
rules:
  ddl.alter.add_index.secondary.prefix.require:
    enabled: true
    level: warning
    params:
      required: true
      prefix: "idx_"
```

---

### `ddl.alter.add_index.fulltext.prefix.require`

要求通过 `ALTER TABLE` 新增的全文索引名称以指定前缀开头。

- **默认**：已启用，级别 `warning`

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `required` | `bool` | `true` | 设为 `false` 则跳过前缀检查 |
| `prefix` | `string` | `full_` | 全文索引名称必须以此前缀开头 |

**YAML 配置示例：**
```yaml
rules:
  ddl.alter.add_index.fulltext.prefix.require:
    enabled: true
    level: warning
    params:
      required: true
      prefix: "full_"
```

---

### `ddl.table.exists.alter.require`

当目标表不存在于数据库中时，禁止执行 `ALTER TABLE`。

> **需要元数据感知模式。** 离线审计时本规则不生效（no-op）。

- **默认**：已启用，级别 `blocker`
- **参数**：无

**YAML 配置示例：**
```yaml
rules:
  ddl.table.exists.alter.require:
    enabled: true
    level: blocker
    params:
```

---

### `ddl.alter.add_column.exists.forbid`

当要添加的列已存在时，阻止 `ALTER TABLE ... ADD COLUMN`。

> **需要元数据感知模式。** 离线审计时本规则不生效（no-op）。

- **默认**：已启用，级别 `blocker`
- **参数**：无

**YAML 配置示例：**
```yaml
rules:
  ddl.alter.add_column.exists.forbid:
    enabled: true
    level: blocker
    params:
```

---

### `ddl.alter.drop_column.exists.require`

当要删除的列不存在时，阻止 `ALTER TABLE ... DROP COLUMN`。

> **需要元数据感知模式。** 离线审计时本规则不生效（no-op）。

- **默认**：已启用，级别 `blocker`
- **参数**：无

**YAML 配置示例：**
```yaml
rules:
  ddl.alter.drop_column.exists.require:
    enabled: true
    level: blocker
    params:
```

---

### `ddl.alter.modify_column.exists.require`

当要修改的列不存在时，阻止 `ALTER TABLE ... MODIFY COLUMN`。

> **需要元数据感知模式。** 离线审计时本规则不生效（no-op）。

- **默认**：已启用，级别 `blocker`
- **参数**：无

**YAML 配置示例：**
```yaml
rules:
  ddl.alter.modify_column.exists.require:
    enabled: true
    level: blocker
    params:
```

---

### `ddl.alter.change_column.exists.require`

当要变更的列不存在时，阻止 `ALTER TABLE ... CHANGE COLUMN`。

> **需要元数据感知模式。** 离线审计时本规则不生效（no-op）。

- **默认**：已启用，级别 `blocker`
- **参数**：无

**YAML 配置示例：**
```yaml
rules:
  ddl.alter.change_column.exists.require:
    enabled: true
    level: blocker
    params:
```

---

### `ddl.alter.rename_column.exists.require`

当要重命名的列不存在时，阻止 `ALTER TABLE ... RENAME COLUMN`。

> **需要元数据感知模式。** 离线审计时本规则不生效（no-op）。

- **默认**：已启用，级别 `blocker`
- **参数**：无

**YAML 配置示例：**
```yaml
rules:
  ddl.alter.rename_column.exists.require:
    enabled: true
    level: blocker
    params:
```

---

### `ddl.alter.add_index.exists.forbid`

当要添加的索引已存在时，阻止 `ALTER TABLE ... ADD INDEX`。

> **需要元数据感知模式。** 离线审计时本规则不生效（no-op）。

- **默认**：已启用，级别 `blocker`
- **参数**：无

**YAML 配置示例：**
```yaml
rules:
  ddl.alter.add_index.exists.forbid:
    enabled: true
    level: blocker
    params:
```

---

### `ddl.alter.drop_index.exists.require`

当要删除的索引不存在时，阻止 `ALTER TABLE ... DROP INDEX`。

> **需要元数据感知模式。** 离线审计时本规则不生效（no-op）。

- **默认**：已启用，级别 `blocker`
- **参数**：无

**YAML 配置示例：**
```yaml
rules:
  ddl.alter.drop_index.exists.require:
    enabled: true
    level: blocker
    params:
```

---

### `ddl.alter.rename_index.exists.require`

当要重命名的索引不存在时，阻止 `ALTER TABLE ... RENAME INDEX`。

> **需要元数据感知模式。** 离线审计时本规则不生效（no-op）。

- **默认**：已启用，级别 `blocker`
- **参数**：无

**YAML 配置示例：**
```yaml
rules:
  ddl.alter.rename_index.exists.require:
    enabled: true
    level: blocker
    params:
```

---

### `ddl.alter.drop_primary_key.exists.require`

当主键不存在时，阻止 `ALTER TABLE ... DROP PRIMARY KEY`。

> **需要元数据感知模式。** 离线审计时本规则不生效（no-op）。

- **默认**：已启用，级别 `blocker`
- **参数**：无

**YAML 配置示例：**
```yaml
rules:
  ddl.alter.drop_primary_key.exists.require:
    enabled: true
    level: blocker
    params:
```

---

## DDL：对象生命周期规则（Object Lifecycle Rules）

### `ddl.table.drop.forbid`

禁止执行 `DROP TABLE`。

- **默认**：已启用，级别 `blocker`

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `forbid` | `bool` | `true` | 设为 `false` 则允许 DROP TABLE |

**触发示例：**
```sql
DROP TABLE users;
```

**YAML 配置示例：**
```yaml
rules:
  ddl.table.drop.forbid:
    enabled: true
    level: blocker
    params:
      forbid: true
```

---

### `ddl.table.drop.exists.require`

当目标表不存在时，禁止执行 `DROP TABLE`（需元数据感知模式）。

> **需要元数据感知模式。** 离线审计时本规则不生效（no-op）。

- **默认**：已启用，级别 `blocker`
- **参数**：无

**YAML 配置示例：**
```yaml
rules:
  ddl.table.drop.exists.require:
    enabled: true
    level: blocker
    params:
```

---

### `ddl.table.drop.adaptive_hash.warn`

当实例开启了 InnoDB 自适应哈希索引（Adaptive Hash Index）时，对 `DROP TABLE` 发出警告，提示可能存在资源竞争。

> **需要元数据感知模式。** 离线审计时本规则不生效（no-op）。

- **默认**：已启用，级别 `warning`
- **参数**：无

**YAML 配置示例：**
```yaml
rules:
  ddl.table.drop.adaptive_hash.warn:
    enabled: true
    level: warning
    params:
```

---

### `ddl.table.drop.rows.max_count`

当表的数据行数超过阈值时，对 `DROP TABLE` 发出警告（需元数据感知模式）。

> **需要元数据感知模式。** 离线审计时本规则不生效（no-op）。

- **默认**：已启用，级别 `warning`

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `limit` | `int` | `100` | 行数超过此值时触发警告（≥ 1） |

**YAML 配置示例：**
```yaml
rules:
  ddl.table.drop.rows.max_count:
    enabled: true
    level: warning
    params:
      limit: 100
```

---

### `ddl.table.truncate.forbid`

禁止执行 `TRUNCATE TABLE`。

- **默认**：已启用，级别 `blocker`

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `forbid` | `bool` | `true` | 设为 `false` 则允许 TRUNCATE TABLE |

**触发示例：**
```sql
TRUNCATE TABLE logs;
```

**YAML 配置示例：**
```yaml
rules:
  ddl.table.truncate.forbid:
    enabled: true
    level: blocker
    params:
      forbid: true
```

---

### `ddl.table.truncate.exists.require`

当目标表不存在时，禁止执行 `TRUNCATE TABLE`（需元数据感知模式）。

> **需要元数据感知模式。** 离线审计时本规则不生效（no-op）。

- **默认**：已启用，级别 `blocker`
- **参数**：无

**YAML 配置示例：**
```yaml
rules:
  ddl.table.truncate.exists.require:
    enabled: true
    level: blocker
    params:
```

---

### `ddl.table.truncate.adaptive_hash.warn`

当实例开启了 InnoDB 自适应哈希索引时，对 `TRUNCATE TABLE` 发出警告。

> **需要元数据感知模式。** 离线审计时本规则不生效（no-op）。

- **默认**：已启用，级别 `warning`
- **参数**：无

**YAML 配置示例：**
```yaml
rules:
  ddl.table.truncate.adaptive_hash.warn:
    enabled: true
    level: warning
    params:
```

---

### `ddl.table.truncate.rows.max_count`

当表的数据行数超过阈值时，对 `TRUNCATE TABLE` 发出警告（需元数据感知模式）。

> **需要元数据感知模式。** 离线审计时本规则不生效（no-op）。

- **默认**：已启用，级别 `warning`

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `limit` | `int` | `100` | 行数超过此值时触发警告（≥ 1） |

**YAML 配置示例：**
```yaml
rules:
  ddl.table.truncate.rows.max_count:
    enabled: true
    level: warning
    params:
      limit: 100
```

---

## DDL：全局规则（Global Rules）

全局规则（global rules）在所有语句评估完成后统一执行，用于跨语句的批量分析。

### `ddl.alter.merge.mysql.require`

当同一批次中针对同一张表存在多条 MySQL 方言的 `ALTER TABLE` 语句时，建议将其合并为单条 `ALTER TABLE`，以减少 Online DDL 的执行次数。

- **默认**：已启用，级别 `warning`

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `required` | `bool` | `true` | 设为 `false` 则禁用合并检查 |

**触发示例：**
```sql
ALTER TABLE users ADD COLUMN age INT;
ALTER TABLE users ADD COLUMN gender VARCHAR(8);
-- 两条语句针对同一张表，建议合并
```

**通过示例：**
```sql
ALTER TABLE users ADD COLUMN age INT, ADD COLUMN gender VARCHAR(8);
```

**YAML 配置示例：**
```yaml
rules:
  ddl.alter.merge.mysql.require:
    enabled: true
    level: warning
    params:
      required: true
```

---

### `ddl.alter.merge.tidb.require`

与 `ddl.alter.merge.mysql.require` 逻辑相同，但作用于 TiDB 方言。默认 `required: false`，因为 TiDB 原生支持并发 DDL，合并需求相对较低。

- **默认**：已启用，级别 `warning`，`required: false`

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `required` | `bool` | `false` | 设为 `true` 则启用 TiDB 方言的合并检查 |

**YAML 配置示例：**
```yaml
rules:
  ddl.alter.merge.tidb.require:
    enabled: true
    level: warning
    params:
      required: false
```

---

## DML 规则（DML Rules）

### `dml.where.require`

要求 `UPDATE` 和 `DELETE` 语句必须包含 `WHERE` 子句，防止全表更新或删除。

- **默认**：已启用，级别 `blocker`

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `required` | `bool` | `true` | 设为 `false` 则允许无 WHERE 的 DML |

**触发示例：**
```sql
DELETE FROM users;
UPDATE orders SET status = 'cancelled';
```

**通过示例：**
```sql
DELETE FROM users WHERE id = 42;
UPDATE orders SET status = 'cancelled' WHERE id = 100;
```

**YAML 配置示例：**
```yaml
rules:
  dml.where.require:
    enabled: true
    level: blocker
    params:
      required: true
```

---

### `dml.limit.forbid`

禁止 `UPDATE`/`DELETE` 语句中使用 `LIMIT` 子句。

- **默认**：已启用，级别 `warning`

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `forbid` | `bool` | `true` | 设为 `false` 则允许 LIMIT |

**触发示例：**
```sql
DELETE FROM logs WHERE created_at < '2020-01-01' LIMIT 1000;
```

**通过示例：**
```sql
DELETE FROM logs WHERE created_at < '2020-01-01';
```

**YAML 配置示例：**
```yaml
rules:
  dml.limit.forbid:
    enabled: true
    level: warning
    params:
      forbid: true
```

---

### `dml.order_by.forbid`

禁止 `UPDATE`/`DELETE` 语句中使用 `ORDER BY` 子句。

- **默认**：已启用，级别 `warning`

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `forbid` | `bool` | `true` | 设为 `false` 则允许 ORDER BY |

**触发示例：**
```sql
DELETE FROM logs ORDER BY created_at LIMIT 100;
```

**通过示例：**
```sql
DELETE FROM logs WHERE id IN (SELECT id FROM logs ORDER BY created_at LIMIT 100);
```

**YAML 配置示例：**
```yaml
rules:
  dml.order_by.forbid:
    enabled: true
    level: warning
    params:
      forbid: true
```

---

### `dml.subquery.forbid`

禁止 `UPDATE`/`DELETE` 语句中使用子查询（subquery）。

- **默认**：已启用，级别 `blocker`

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `forbid` | `bool` | `true` | 设为 `false` 则允许子查询 |

**触发示例：**
```sql
DELETE FROM orders WHERE user_id IN (SELECT id FROM users WHERE status = 'inactive');
```

**通过示例：**
```sql
DELETE FROM orders WHERE user_id = 42;
```

**YAML 配置示例：**
```yaml
rules:
  dml.subquery.forbid:
    enabled: true
    level: blocker
    params:
      forbid: true
```

---

### `dml.join.on.require`

要求 `UPDATE`/`DELETE` 语句中的 `JOIN` 必须带 `ON` 子句，防止笛卡尔积（Cartesian product）。

- **默认**：已启用，级别 `blocker`

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `required` | `bool` | `true` | 设为 `false` 则允许无 ON 的 JOIN |

**触发示例：**
```sql
UPDATE orders o JOIN users u SET o.status = 'cancelled' WHERE o.id = 1;
```

**通过示例：**
```sql
UPDATE orders o JOIN users u ON o.user_id = u.id SET o.status = 'cancelled' WHERE o.id = 1;
```

**YAML 配置示例：**
```yaml
rules:
  dml.join.on.require:
    enabled: true
    level: blocker
    params:
      required: true
```

---

### `dml.insert.rows.max_count`

限制单条 `INSERT` 语句最多插入的行数（VALUES 行数）。

- **默认**：已启用，级别 `warning`

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `limit` | `int` | `100` | 单条 INSERT 允许的最大行数 |

**触发示例：**
```sql
INSERT INTO t (id) VALUES (1),(2), ... (超过 100 行) ...;
```

**通过示例：**
```sql
INSERT INTO t (id) VALUES (1),(2),(3);
```

**YAML 配置示例：**
```yaml
rules:
  dml.insert.rows.max_count:
    enabled: true
    level: warning
    params:
      limit: 100
```

---

### `dml.replace.forbid`

禁止使用 `REPLACE INTO` 语句（该语句会隐式删除再插入，存在数据风险）。

- **默认**：已启用，级别 `blocker`

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `forbid` | `bool` | `true` | 设为 `false` 则允许 REPLACE INTO |

**触发示例：**
```sql
REPLACE INTO users (id, name) VALUES (1, 'Alice');
```

**通过示例：**
```sql
INSERT INTO users (id, name) VALUES (1, 'Alice') ON DUPLICATE KEY UPDATE name = 'Alice';
```

**YAML 配置示例：**
```yaml
rules:
  dml.replace.forbid:
    enabled: true
    level: blocker
    params:
      forbid: true
```

---

### `dml.insert.select.forbid`

禁止使用 `INSERT INTO ... SELECT` 语法，防止大批量数据导入造成锁争用。

- **默认**：已启用，级别 `blocker`

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `forbid` | `bool` | `true` | 设为 `false` 则允许 INSERT SELECT |

**触发示例：**
```sql
INSERT INTO archive_orders SELECT * FROM orders WHERE created_at < '2020-01-01';
```

**YAML 配置示例：**
```yaml
rules:
  dml.insert.select.forbid:
    enabled: true
    level: blocker
    params:
      forbid: true
```

---

### `dml.insert.on_duplicate.forbid`

禁止使用 `INSERT INTO ... ON DUPLICATE KEY UPDATE` 语法。

- **默认**：已启用，级别 `blocker`

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `forbid` | `bool` | `true` | 设为 `false` 则允许 ON DUPLICATE KEY UPDATE |

**触发示例：**
```sql
INSERT INTO users (id, name) VALUES (1, 'Alice') ON DUPLICATE KEY UPDATE name = 'Alice';
```

**YAML 配置示例：**
```yaml
rules:
  dml.insert.on_duplicate.forbid:
    enabled: true
    level: blocker
    params:
      forbid: true
```

---

### `dml.table.denylist.forbid`

禁止对黑名单中指定的 schema 或表执行 DML 操作。

- **默认**：已启用，级别 `blocker`

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `schemas` | `[]string` | `[]` | 被禁止操作的 schema 名称列表 |
| `tables` | `[]string` | `[]` | 被禁止操作的表名列表（不含 schema 限定） |
| `qualified_tables` | `[]string` | `[]` | 被禁止操作的全限定表名列表，格式为 `schema.table` |

**YAML 配置示例：**
```yaml
rules:
  dml.table.denylist.forbid:
    enabled: true
    level: blocker
    params:
      schemas: []
      tables: []
      qualified_tables: []
```
