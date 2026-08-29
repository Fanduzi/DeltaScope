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

# 查看某条规则在当前配置下的有效状态
deltascope config status dml.where.require
deltascope --config ./deltascope.yaml config status dml.where.require --format json

# 查看内置默认配置
deltascope config show-default
```

`config init` 和 `config show-default` 会把空字符串参数写成 `""`。单独的 `suffix:` 在 YAML 里是 null，无法通过 `config lint`。

**`config lint` 干净通过：**
```text
Config OK
```

当 mention 的规则省略了字段时，`config lint` 也会给出警告，因为规则级替换会把省略的 `enabled` 变成 `false`（见 [Rule-Level Replacement Semantics](#rule-level-replacement-semantics)）。默认情况下警告仅供参考（退出码 0）；加 `--strict` 则以退出码 2 失败。对于下文那种部分覆盖（`level: warning`，省略了 `enabled` 和 `params`），`config lint` 会按省略的字段各打印一条警告，每条都把后续动作交给 `config status`：

```text
Config OK with warnings

Warnings:
- dml.where.require is OFF because "enabled" is omitted.
  This config replaces the whole rule policy; it does not merge with defaults.
  Inspect effective rule status:
    deltascope config status dml.where.require --config ./deltascope.yaml
- dml.where.require removes default params because "params" is omitted.
  This config replaces the whole rule policy; it does not merge with defaults.
  Inspect effective rule status:
    deltascope config status dml.where.require --config ./deltascope.yaml
```

**`config lint` 错误（未知 rule ID）——错误优先于警告：**
```text
unknown rule "ddl.table.comments.require"
```

### config status

`deltascope config status <rule-id>` 展示某条规则在当前配置下是 ON 还是 OFF、触发时会使用哪个 `level`，以及你的配置相对于默认值改动了 `enabled`、`level` 或哪些 params。配置文件通过全局 `--config` 标志选择。

```bash
deltascope config status dml.where.require
deltascope --config ./deltascope.yaml config status dml.where.require
deltascope --config ./deltascope.yaml config status dml.where.require --format json
```

它回答的问题与其它规则命令不同：

- `rules explain <rule-id>` 解释规则本身的含义（它忽略你的配置）。
- `config status <rule-id>` 展示你的配置让这条规则做什么。
- `config lint --file` 校验配置文件的结构与取值。

`config status` 不运行 audit、不解析 SQL、不连接数据库、不改变 audit 行为或规则行为、不改变 finding JSON 结构，也不新增 `severity` 字段。完整的文本与 JSON 输出契约见 [cli.zh-CN.md](cli.zh-CN.md#config-status)。

`config status` 报告的有效策略与 audit 路径实际应用的一致。正因如此，在编辑部分规则之前，有一项配置文件行为必须先理解：规则级替换语义。

### 文件来源

- 通过 `deltascope config init` 生成
- 随代码库提交的示例文件：`configs/deltascope.example.yaml`
- 运行时通过 `--config` 指定加载

### 使用建议

- 生成一次后，将策略文件提交至 CI 中统一使用
- 上线前先用 `config lint` 校验变更
- 将 `configs/deltascope.example.yaml` 作为文档参考，而非唯一的配置来源

---

## Rule-Level Replacement Semantics

规则级替换语义（rule-level replacement semantics）：当你在 YAML 中 **mention** 一条规则时，加载器会替换该规则的整条 policy——它**不会**把你写下的字段局部合并（partial merge）到默认值上。被省略的字段会变成其零值：

| 字段 | 省略后的有效取值 |
|---|---|
| `enabled` | `false` |
| `level` | `""`（空） |
| `params` | 空 |

YAML 中**未 mention** 的规则保持其默认 policy 不变。

这与 audit 路径实际应用的行为完全一致，因此 `config status` 如实报告，而不是隐藏它。最常见的陷阱是只写下你想改的字段：

```yaml
rules:
  dml.where.require:
    level: warning
```

这看起来像是“把 level 从 `blocker` 放宽到 `warning`”。并非如此。因为该规则现在被 mention，它的整条 policy 被替换，`enabled` 被省略因而变为 `false`，规则最终是 **OFF**——它根本不会产生 finding。`config status` 会明确指出：

```text
Current status:
  OFF
  This rule will not produce findings.

Config effect:
  Your config mentions this rule, so it replaces the default rule policy.
  `enabled` is omitted, so the effective value is false.
  `level` changes from blocker to warning.
  `params.required` is removed.
  This rule is OFF.
```

`config lint` 会在部署前提示这个陷阱。mention 一条规则但没写全字段时，`config lint` 会对每个省略的字段各打印一条警告；带 `--strict` 时命令会直接失败。用 `config status <rule-id>` 查看最终的有效状态。完整的警告列表与退出码契约见 [cli.zh-CN.md](cli.zh-CN.md#config-lint)。

若只想改 `level` 又保持规则开启，请写明所有字段，使替换后其余字段保持不变：

```yaml
rules:
  dml.where.require:
    enabled: true
    level: warning
    params:
      required: true
```

完整字段集不必死记。`rules explain <rule-id>` 会打印一段 `Safe override example:`，它取自规则的真实默认值——直接照抄，再调整 level 即可。编辑规则覆盖时推荐的闭环：

```bash
deltascope config lint --file deltascope.yaml                         # 捕获替换风险
deltascope rules explain dml.where.require                            # 复制一份安全的完整覆盖
deltascope config status dml.where.require --config deltascope.yaml   # 确认最终有效状态
```

加载器是否应改为采用局部合并语义（partial merge）是一个更大、独立的决策，不在本版本范围内。在此之前，请把被 mention 的规则视为一次完整替换（rule-level replacement，不是局部合并）。

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

禁止 `MODIFY COLUMN` 中相对于已确认的当前列定义显式修改可空性（nullability），防止意外引入或去除 NULL。若显式重复的 `NULL` 或 `NOT NULL` 与实时列状态一致，则允许通过；前置状态未知时改由 `ddl.alter.modify_column.explicit_nullability_change.unknown_prior_state.advisory` 发出提示。

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

### `ddl.alter.modify_column.explicit_nullability_change.unknown_prior_state.advisory`

> **离线和 metadata-aware 回退规则。** 此规则不会阻断审计。

当 `MODIFY COLUMN` 显式声明 `NULL` 或 `NOT NULL`、但无法确认修改前的列可空性时发出 notice。它不会声称已经发生了状态转换。若实时元数据确认了当前列状态，则该提示被抑制，已有的 `...explicit_nullability_change.forbid` 规则只检查真实转换。

- **默认**：已启用，级别 `notice`
- **参数**：无

**触发示例：**
```sql
ALTER TABLE users MODIFY COLUMN email VARCHAR(320) NOT NULL;
```

**YAML 配置示例：**
```yaml
rules:
  ddl.alter.modify_column.explicit_nullability_change.unknown_prior_state.advisory:
    enabled: true
    level: notice
    params:
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

## DDL：PostgreSQL 迁移安全规则（PostgreSQL Migration Safety Rules）

> 以下规则仅对 PostgreSQL 方言生效，用于捕获可能导致锁表或全表重写的 DDL 操作。

### `ddl.pg.create_index.concurrently.require`

标记 PostgreSQL 上不带 `CONCURRENTLY` 的 `CREATE INDEX`。非并发的 `CREATE INDEX` 会对表持有排他锁，阻塞读写直到索引构建完成。

- **默认**：已启用，级别 `warning`
- **参数**：无

**触发示例：**
```sql
CREATE INDEX idx_name ON users (email);
```

**通过示例：**
```sql
CREATE INDEX CONCURRENTLY idx_name ON users (email);
```

**YAML 配置示例：**
```yaml
rules:
  ddl.pg.create_index.concurrently.require:
    enabled: true
    level: warning
```

---

### `ddl.pg.alter.add_column.non_null_default.rewrite.warn`

警告 `ALTER TABLE … ADD COLUMN … NOT NULL DEFAULT …` 可能触发 PostgreSQL 全表重写。添加带有 volatile 默认值（如 `gen_random_uuid()`）的非空列需要 PostgreSQL 重写每一行。

- **默认**：已启用，级别 `warning`
- **参数**：无

**触发示例：**
```sql
ALTER TABLE users ADD COLUMN uuid UUID NOT NULL DEFAULT gen_random_uuid();
```

**通过示例：**
先添加可为空的列，回填数据，然后在单独的迁移中添加 `NOT NULL` 约束。

**YAML 配置示例：**
```yaml
rules:
  ddl.pg.alter.add_column.non_null_default.rewrite.warn:
    enabled: true
    level: warning
```

---

### `ddl.pg.alter.add_check.not_valid.require`

标记 PostgreSQL 上不带 `NOT VALID` 的 `ALTER TABLE … ADD CHECK (…)`。不使用 `NOT VALID` 添加 `CHECK` 约束需要全表扫描来验证已有行，这会持有 `ACCESS EXCLUSIVE` 锁。

- **默认**：已启用，级别 `warning`
- **参数**：无

**触发示例：**
```sql
ALTER TABLE orders ADD CHECK (total >= 0);
```

**通过示例：**
```sql
ALTER TABLE orders ADD CHECK (total >= 0) NOT VALID;
```

**YAML 配置示例：**
```yaml
rules:
  ddl.pg.alter.add_check.not_valid.require:
    enabled: true
    level: warning
```

---

### `ddl.pg.alter.set_data_type.rewrite.warn`

警告 `ALTER TABLE … ALTER COLUMN … TYPE …` 可能需要 PostgreSQL 全表重写。某些类型变更（如 `varchar` 到 `integer`）需要 PostgreSQL 重写每一行。

- **默认**：已启用，级别 `warning`
- **参数**：无

**触发示例：**
```sql
ALTER TABLE users ALTER COLUMN age TYPE bigint;
```

**通过示例：**
使用三步安全迁移：添加新列 → 回填 → 删除旧列。

**YAML 配置示例：**
```yaml
rules:
  ddl.pg.alter.set_data_type.rewrite.warn:
    enabled: true
    level: warning
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

---

### `dml.table.exists.require`

当实时元数据明确报告 DML 目标表不存在时，阻止 `INSERT`、`UPDATE` 和 `DELETE`。该元数据感知规则仅适用于 MySQL 和 TiDB；没有实时表快照时跳过，不检查 `INSERT ... SELECT` 中的源表。

- **默认**：已启用，级别 `blocker`
- **参数**：无

只有目标关系查询返回 `exists: false` 的表快照时才会触发。缺少快照不声明表存在或不存在；查询失败仍沿用元数据/连接错误路径。

**YAML 配置示例：**
```yaml
rules:
  dml.table.exists.require:
    enabled: true
    level: blocker
```

---

## 信任与误配防护

v0.20.0 引入了 PostgreSQL 信任与误配防护作为增量引擎行为。这些**不能通过策略 YAML 配置**，在 `rules:` 块中没有条目：

- **PostgreSQL 语法启发式通知**（`dialect.postgresql.syntax.detected.notice`）：在 MySQL/TiDB 审计路径检测到 PG 专属语法标记时作为全局建议性告警发出。此行为始终启用，不能被禁用或调整级别。
- **PostgreSQL 能力边界错误**：未支持的 PG 功能面返回类型化的 `PostgreSQLCapabilityBoundaryError`。这是引擎行为，不是规则。
- **启发式误报排除**：PG 语法启发式自动忽略字符串字面量、引号标识符和注释中的标记。无需配置。
- **信任上下文和规则摘要可见性**：CLI 输出格式（json、markdown、quiet）包含审计上下文和规则计数。这些是输出层行为，不是规则参数。

完整能力表请参见[能力矩阵](audit-capability-matrix.zh-CN.md)。

---

## PostgreSQL DDL 覆盖范围（v0.21.0 / v0.23.0 / v0.24.0）

`v0.21.0` 扩展了 PostgreSQL DDL 标准化范围，覆盖常见迁移后续语句。`v0.23.0` 进一步扩展了常见 PostgreSQL `CREATE TABLE` 约束形态的覆盖范围。`v0.24.0` 深化了 `v0.23.0` 已纳入共享审核管线的 PostgreSQL `CREATE TABLE` 形态的语义信息。这些都是覆盖和语义能力改进，复用已有的共享规则和 metadata-aware 语义——**不引入新的规则配置项**。

以下 PostgreSQL `ALTER TABLE` 形式现在通过共享审核管线进行标准化处理，不再返回能力边界错误：

- `ALTER COLUMN ... SET DEFAULT`（动作：`set_default`）
- `ALTER COLUMN ... DROP DEFAULT`（动作：`drop_default`）
- `ALTER COLUMN ... SET NOT NULL`（动作：`set_not_null`）
- `ALTER COLUMN ... DROP NOT NULL`（动作：`drop_not_null`）
- `VALIDATE CONSTRAINT`（动作：`validate_constraint`）——supported 且 auditable，无专用规则
- `DROP CONSTRAINT`（动作：`drop_constraint`）——主键映射在 metadata 可用时通过 `ddl.alter.drop_primary_key` 适用

`v0.23.0` 新增支持的 PostgreSQL `CREATE TABLE` 形态，但不新增配置键：

- 表级命名 `CHECK`
- 列级内联 `CHECK`
- 表级命名 `UNIQUE`
- 列级内联 `UNIQUE`
- 表级命名 `FOREIGN KEY`
- 列级内联 `REFERENCES`

`v0.24.0` 深化了这些建表形态的外键语义：

- 具名 `FOREIGN KEY` 和内联 `REFERENCES` 现在保留解析器拥有的 `ReferencedTable` 和 `ReferencedColumns` 作为共享契约事实。
- 这些是解析器拥有的结构事实，不是实时元数据真相——它们代表 SQL 语句所声明的内容，而非数据库 schema 的当前状态。

配置层面的含义：

- 当标准化后的 PostgreSQL 建表事实与现有规则族匹配时，`ddl.constraint.check.*`、`ddl.constraint.unique_key.*`、`ddl.constraint.foreign_key.*` 等既有结构化命名治理会被复用。
- 现有共享索引规则可以消费内联 `UNIQUE` 产出的索引事实。
- 已有的 `ddl.table.foreign_key.forbid` 继续对所有外键形式生效，包括携带更丰富语义的内联 `REFERENCES`。
- `ReferencedTable` 和 `ReferencedColumns` 是增量字段，使用 `omitempty` JSON 编码——无需新增 policy block。

这些版本都无需修改 `configs/deltascope.example.yaml`。
