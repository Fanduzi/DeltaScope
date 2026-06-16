# 查看规则与配置

在运行大批量审计前，使用内置的发现命令了解 DeltaScope 将执行哪些规则。这些命令无需数据库连接，完全基于编译好的规则注册表和策略文件运行。

## 发现规则

### 列出所有规则

```bash
deltascope rules list
```

输出示例（节选——实际构建版本可能包含更多规则）：

```
RULE ID                                    LEVEL    DIALECT  KIND  CATEGORY
dml.where.require                          blocker  common   dml   dml_safety
dml.limit.forbid                           warning  common   dml   dml_safety
dml.subquery.forbid                        blocker  common   dml   dml_safety
dml.join.on.require                        blocker  common   dml   dml_safety
dml.insert.rows.max_count                  warning  common   dml   dml_safety
dml.order_by.forbid                        warning  common   dml   dml_safety
ddl.table.comment.require                  warning  common   ddl   table
ddl.table.name.max_length                  blocker  common   ddl   table
ddl.column.comment.require                 warning  common   ddl   column
ddl.alter.drop_column.forbid               warning  common   ddl   alter_table
...
```

`DIALECT` 为 `common`（适用于 MySQL、TiDB、PostgreSQL）或具体方言，例如 `mysql`、`postgresql`。部分规则需要实时数据库连接才能评估，详见 [使用元数据审核 SQL](audit-sql-with-metadata.zh-CN.md)，这类规则在离线时为 no-op。

### 按类型或级别过滤

```bash
# 仅显示 DML 规则
deltascope rules list --kind dml

# 仅显示 DDL 中 blocker 级别的规则
deltascope rules list --kind ddl --level blocker

# 所有 warning 级别的规则（不限类型）
deltascope rules list --level warning

# 限定到某一方言的规则
deltascope rules list --dialect postgresql

# 按 rule ID 或元数据关键词搜索
deltascope rules list --search drop
```

### 查看规则详情

```bash
deltascope rules explain dml.where.require
```

输出：

```
Rule ID:    dml.where.require
Level:      blocker
Enabled:    true
Dialects:   common
Kind:       dml
Category:   dml_safety
Config Key: dml.where.require

Summary:
  Require DML where require

Why:
  The statement is missing a clause, option, or object that the shipped policy requires.

Risk:
  Ignoring this rule can allow high-impact data changes to proceed with less safety review.

Suggestion:
  Add the required clause, option, or object explicitly so the rule no longer has to infer intent.

Tags: dml, common, dml_safety, require
Trigger Example:
  DELETE FROM users;
Valid Example:
  DELETE FROM users WHERE id = 1;

Default Params:
  required: true

Config Example:
  rules:
    dml.where.require:
      enabled: true
      level: blocker
      params:
        required: true
```

带数值参数的 DDL 规则示例：

```bash
deltascope rules explain ddl.table.name.max_length
```

输出（末尾）：

```
Default Params:
  limit: 64

Config Example:
  rules:
    ddl.table.name.max_length:
      enabled: true
      level: blocker
      params:
        limit: 64
```

`rules explain` 只读取 shipped catalog，不看你的配置。要查看配置让规则做什么，用 `config status <rule-id>`（见下文 [校验配置文件](#校验配置文件)）。

### 按关键词搜索规则

```bash
# 按 rule ID 或元数据关键词搜索
deltascope rules list --search drop
```

输出（节选）：

```
RULE ID                                  LEVEL    DIALECT  KIND  CATEGORY
ddl.alter.drop_column.exists.require     blocker  common   ddl   alter_table
ddl.alter.drop_column.forbid             warning  common   ddl   alter_table
ddl.alter.drop_index.forbid              warning  common   ddl   alter_table
ddl.table.drop.forbid                    blocker  common   ddl   table
...
```

## 管理配置

### 生成默认配置

将完整的默认策略导出为 YAML 文件并提交到代码仓库：

```bash
deltascope config init > deltascope.yaml
```

生成的 YAML 文件包含每条规则的默认 `enabled` 状态、`level` 以及所有 `params`。以此作为团队策略定制的起点。

### 校验配置文件

修改配置后，在部署前进行校验，捕获规则 ID 拼写错误、无效参数类型，以及规则级替换风险：

```bash
deltascope config lint --file ./deltascope.yaml
```

干净的文件输出 `Config OK`，退出码 0：

```
Config OK
```

合法但 mention 规则时未写全字段的文件，会对每个省略字段各打印一条警告，退出码仍为 0。这与下文 [常见配置任务](#禁用某条规则) 中的替换风险是同一回事：mention 一条规则会替换整条 policy，省略 `enabled` 会把规则关成 OFF。

```yaml
rules:
  dml.where.require:
    level: warning
```

```
Config OK with warnings

Warnings:
- rule "dml.where.require" is mentioned without "enabled"; the rule policy is replaced, not partially merged, so omitted "enabled" becomes false and the rule is OFF
- rule "dml.where.require" is mentioned without "params"; the rule policy is replaced, not partially merged, so omitted "params" become empty, removing the default params
```

加 `--strict` 后，出现警告即以退出码 2 失败，这正是 CI 中想要的行为：

```bash
deltascope config lint --file ./deltascope.yaml --strict
```

校验错误会打印到 stderr 并以退出码 2 退出，且优先于警告：

```
unknown rule "ddl.table.comments.require"
invalid level "critical" for rule "ddl.column.comment.require"
invalid type for dml.insert.rows.max_count.limit: got string, want int
```

`config lint` 没有 JSON 输出。要确认被警告的规则最终落在什么有效状态，接着用 `config status`：

```bash
deltascope --config ./deltascope.yaml config status dml.where.require
```

```text
Current status:
  OFF
  This rule will not produce findings.

Config effect:
  Your config mentions this rule, so it replaces the default rule policy.
  `enabled` is omitted, so the effective value is false.
  `level` changes from blocker to warning.
  This rule is OFF.
```

完整的 `config lint` 警告列表与退出码契约见 [cli.zh-CN.md](../reference/cli.zh-CN.md#config-lint)。

### 查看内置默认值

不生成文件，直接查看编译内置的默认配置：

```bash
deltascope config show-default
```

与 `config init` 输出内容相同，但仅打印到标准输出——可用于与已提交配置文件进行对比：

```bash
diff <(deltascope config show-default) ./deltascope.yaml
```

## 常见配置任务

### 禁用某条规则

将 `enabled` 设置为 `false` 完全关闭该规则：

```yaml
rules:
  ddl.alter.drop_column.forbid:
    enabled: false
```

### 降低规则级别

修改 `level` 将发现从 `warning` 降级为 `notice`（或其他合法级别）：

```yaml
rules:
  ddl.table.comment.require:
    enabled: true
    level: notice    # 默认为 warning——为本团队降级
```

### 提升规则级别

```yaml
rules:
  ddl.column.comment.require:
    enabled: true
    level: blocker   # 默认为 warning——为本团队升级
```

### 调整规则参数

覆盖数值或布尔参数以适应您的环境：

```yaml
rules:
  dml.insert.rows.max_count:
    enabled: true
    level: warning
    params:
      limit: 500    # 默认为 100——为批量导入工作流放宽限制

  ddl.table.name.max_length:
    enabled: true
    level: blocker
    params:
      limit: 32    # 比默认的 64 更严格
```

### 完整配置示例片段

```yaml
# deltascope.yaml
rules:
  # DML 规则
  dml.where.require:
    enabled: true
    level: blocker

  dml.limit.forbid:
    enabled: true
    level: warning

  dml.insert.rows.max_count:
    enabled: true
    level: warning
    params:
      limit: 500

  # DDL 规则
  ddl.table.comment.require:
    enabled: true
    level: notice           # 降级——团队正在逐步推广注释规范

  ddl.column.comment.require:
    enabled: true
    level: warning

  ddl.alter.drop_column.forbid:
    enabled: false          # 禁用——运维团队手动处理列删除

  ddl.table.name.max_length:
    enabled: true
    level: blocker
    params:
      limit: 48
```

编辑完成后，在使用前务必校验：

```bash
deltascope config lint --file ./deltascope.yaml
```
