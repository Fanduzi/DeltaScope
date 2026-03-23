# 查看规则与配置

在运行大批量审计前，使用内置的发现命令了解 DeltaScope 将执行哪些规则。这些命令无需数据库连接，完全基于编译好的规则注册表和策略文件运行。

## 发现规则

### 列出所有规则

```bash
deltascope rules list
```

输出示例（节选——实际构建版本可能包含更多规则）：

```
RULE ID                                    KIND  LEVEL    METADATA
dml.where.require                          dml   blocker  false
dml.limit.forbid                           dml   warning  false
dml.subquery.forbid                        dml   blocker  false
dml.join.on.require                        dml   blocker  false
dml.insert.rows.max_count                  dml   warning  false
dml.orderby.require                        dml   warning  false
ddl.table.comment.require                  ddl   warning  false
ddl.table.name.max_length                  ddl   blocker  false
ddl.column.comment.require                 ddl   warning  false
ddl.column.charset.forbid                  ddl   warning  false
ddl.column.nullable.forbid                 ddl   warning  false
ddl.alter.drop_column.forbid               ddl   blocker  false
ddl.alter.merge.mysql.require              ddl   warning  false
ddl.table.row_size.max_bytes.require       ddl   blocker  true
ddl.table.rows.max_count.require           ddl   blocker  true
...
```

`METADATA` 列表示该规则是否需要实时数据库连接（`true`）或可离线运行（`false`）。

### 按类型或级别过滤

```bash
# 仅显示 DML 规则
deltascope rules list --kind dml

# 仅显示 DDL 中 blocker 级别的规则
deltascope rules list --kind ddl --level blocker

# 所有 warning 级别的规则（不限类型）
deltascope rules list --level warning

# 所有需要元数据的规则
deltascope rules list --metadata
```

### 查看规则详情

```bash
deltascope rules show dml.where.require
```

输出：

```
Rule ID:     dml.where.require
Kind:        dml
Level:       blocker
Description: UPDATE or DELETE must include a WHERE clause to prevent full-table modifications
Metadata:    false
Params:
  required (bool, default: true)
```

另一个带数值参数的 DDL 规则示例：

```bash
deltascope rules show ddl.table.name.max_length
```

输出：

```
Rule ID:     ddl.table.name.max_length
Kind:        ddl
Level:       blocker
Description: Table name must not exceed the configured maximum length
Metadata:    false
Params:
  max_length (int, default: 64)
```

需要元数据的规则示例：

```bash
deltascope rules show ddl.table.row_size.max_bytes.require
```

输出：

```
Rule ID:     ddl.table.row_size.max_bytes.require
Kind:        ddl
Level:       blocker
Description: Estimated row size must not exceed the configured limit; requires metadata (table snapshot) to evaluate
Metadata:    true
Params:
  max_bytes (int, default: 65535)
```

### 按关键词搜索规则

```bash
# 查找所有提及 "metadata" 的规则
deltascope rules search metadata

# 查找与表前缀相关的规则
deltascope rules search prefix

# 查找与 DROP 相关的规则
deltascope rules search drop
```

输出示例（`deltascope rules search drop`）：

```
RULE ID                          KIND  LEVEL    METADATA
ddl.alter.drop_column.forbid     ddl   blocker  false
ddl.object.drop_table.forbid     ddl   blocker  false
```

## 管理配置

### 生成默认配置

将完整的默认策略导出为 YAML 文件并提交到代码仓库：

```bash
deltascope config init > deltascope.yaml
```

生成的 YAML 文件包含每条规则的默认 `enabled` 状态、`level` 以及所有 `params`。以此作为团队策略定制的起点。

### 校验配置文件

修改配置后，在部署前进行校验，捕获规则 ID 拼写错误或无效参数值：

```bash
deltascope config lint --file ./deltascope.yaml
```

成功输出：

```
Config file ./deltascope.yaml is valid.
```

失败输出——规则 ID 未知（附带 did-you-mean 建议）：

```
Error: unknown rule ID "ddl.table.comments.require" in ./deltascope.yaml (did you mean "ddl.table.comment.require"?)
```

失败输出——参数类型无效：

```
Error: rule "dml.insert.rows.max_count": param "limit" expects int, got string "five hundred"
```

失败输出——级别值无效：

```
Error: rule "ddl.column.comment.require": unknown level "critical" (expected: blocker, warning, notice)
```

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

### 降低规则严重级别

修改 `level` 将发现从 `warning` 降级为 `notice`（或其他合法级别）：

```yaml
rules:
  ddl.table.comment.require:
    enabled: true
    level: notice    # 默认为 warning——为本团队降级
```

### 提升规则严重级别

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
      max_length: 32    # 比默认的 64 更严格
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
      max_length: 48
```

编辑完成后，在使用前务必校验：

```bash
deltascope config lint --file ./deltascope.yaml
```
