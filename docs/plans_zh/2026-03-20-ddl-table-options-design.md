# DeltaScope DDL 表选项批次设计

## 目标

通过添加 create-table 选项和对象形状控制规则，将 DeltaScope 的离线 DDL 覆盖范围深入到 `gAudit` 的表级治理表面。

## 背景

当前提取已经保留：

- 表注释
- 引擎
- 字符集
- 非索引约束

剩余的离线安全表级差距：

- 注释长度
- 引擎允许列表
- 字符集允许列表
- 外键限制
- 分区限制
- `CREATE TABLE ... LIKE`
- `CREATE TABLE ... AS SELECT`

## 方法

### 选项 A：保持提取不变，仅使用现有的 `Options`/`Constraints`

优点：
- 最小的代码更改

缺点：
- 无法区分 `CREATE TABLE ... LIKE`、`... AS SELECT` 或分区表
- 留下多个高价值 `gAudit` 风格开关不可用

### 选项 B：最小丰富 create-table 形状并添加连贯的选项批次

添加几个布尔值到领域 DDL 模型：

- `HasReferTable`
- `HasSelect`
- `HasPartition`

然后为注释长度、引擎、字符集、外键、分区、create-like 和 create-as 实施表级规则。

优点：
- 小模型增长，强大的离线覆盖
- 仍然解析器中立
- 干净地映射到策略和未来输出

缺点：
- 比纯选项批次稍广

### 选项 C：等到更丰富的主键/类型建模之后

优点：
- 更少的并发 DDL 方向

缺点：
- 延迟了一大组简单、高价值的离线规则

## 决策

选择 **选项 B**。

此批次在不强加深度新抽象的情况下添加了真正的覆盖。额外的 DDL 布尔值足够稳定，现在就值得添加。

## 模型更改

用以下内容扩展 `spec.DDL`：

- `HasReferTable bool`
- `HasSelect bool`
- `HasPartition bool`

这些保持对 create-table 形状的狭窄和特定。

## 规则批次

计划的规则 ID：

- `ddl.table.comment.max_length`
- `ddl.table.engine.allowlist`
- `ddl.table.charset.allowlist`
- `ddl.table.foreign_key.forbid`
- `ddl.table.partition.forbid`
- `ddl.table.create_like.forbid`
- `ddl.table.create_as.forbid`

## 策略默认值

推荐默认值：

- 表注释最大长度：`128`
- 引擎允许列表：`["InnoDB"]`
- 字符集允许列表：`["utf8", "utf8mb4"]`
- 外键：禁止
- 分区：禁止
- create like：禁止
- create as：禁止

## 推迟的工作

- 标识符/关键字验证
- 列字符集/排序规则验证
- 表字符集推荐 vs 严格允许列表映射
- 自动递增初始值验证
- create view/drop/truncate/drop-table 治理

## 验证

- 首先扩展提取测试
- 每个选项/对象关注点添加针对性规则测试
- 重新运行：
  - `go test ./internal/application/audit -run TestExtract -v`
  - `go test ./internal/domain/rule/ddl/... -v`
  - `go test ./...`
  - `diff -u configs/deltascope.example.yaml <(go run ./cmd/deltascope config init)`
  - `check_three_level_doc.sh`
