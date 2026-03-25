# DeltaScope DDL 扩展设计

## 目标

将 `DeltaScope` 从第一个 `CREATE TABLE` 批次扩展，使 DDL 规则目录更接近长期的 `gAudit` 超集目标，而不破坏当前的纯离线架构。

## 范围

此扩展分为三个批次：

1. 列和审计列规则
2. 索引规则
3. alter 限制

第一个实施浪潮将从批次 1 开始，因为当前提取的 `spec.DDL` 形状已经包含足够的稳定列信息来支持它。

## 推荐方法

### 选项 A：保持规则浅层，避免模型更改

仅添加可从现有 `spec.Column{Name, Type, Comment}` 推断的规则。

优点：
- 最快
- 低风险

缺点：
- 审计列检查仍然太弱
- 列默认值和可空性规则仍然不可能
- 会很快迫使第二次提取器重写

### 选项 B：一次扩展列模型，然后添加连贯的列规则批次

用列规则所需的最低额外语义扩展 `spec.Column` 和提取器：

- 长度
- not-null 标志
- 默认存在/值
- `CURRENT_TIMESTAMP` 默认值
- `ON UPDATE CURRENT_TIMESTAMP`

然后添加针对性的规则批次：

- 表必须包含列
- 审计时间戳列
- 列注释要求
- 列名最大长度
- varchar 最大长度
- 默认值要求
- not-null 要求与类型允许列表
- float/double 指导

优点：
- 每个提取器更改最高杠杆
- 与 `gAudit` 列规则一致
- 避免解析器 AST 泄漏到规则

缺点：
- 第一个批次稍大

### 选项 C：直接跳到索引 + alter 限制

优点：
- 更早攻击更大的剩余 DDL 差距

缺点：
- 当前 `spec.Index` 和 `spec.Alter` 形状仍然太薄
- 可能产生脆弱的规则或另一次仓促重构

## 决策

选择 **选项 B**。

`DeltaScope` 应该首先在 create-table 列语义上变得强大，然后再进入索引命名/形状和 alter-action 治理。这使下一批次有价值、可测试且结构干净。

## 模型更改

`spec.Column` 应该仅在即将到来的规则需要真实语义的地方增长：

- `Length int`
- `NotNull bool`
- `HasDefault bool`
- `DefaultValue string`
- `DefaultIsNull bool`
- `DefaultIsCurrentTimestamp bool`
- `OnUpdateCurrentTimestamp bool`

规则仍然消费 `spec.Statement`；解析器细节保持在应用提取器中隐藏。

## 规则批次 2

计划的规则 ID：

- `ddl.table.columns.min_count`
- `ddl.table.audit_columns.require`
- `ddl.column.comment.require`
- `ddl.column.name.max_length`
- `ddl.column.varchar.max_length`
- `ddl.column.default.require`
- `ddl.column.not_null.require`
- `ddl.column.float_double.forbid`

注意：

- 审计列验证保持名称无关，与 `gAudit` 的文档行为匹配
- `not_null.require` 应允许通过策略允许可空的 blob/text/json 列，以及可选的时间类列
- 默认值检查保持结构性，不了解数据库运行时

## 推迟的批次

### 索引批次

在索引名称/前缀要求等规则可以干净实现之前，需要更丰富的 `spec.Index` 元数据，如唯一性/全文分类。

### Alter 批次

在 drop/rename/type-change 限制可以安全建模之前，需要比当前 `Action + Name` 对更丰富的 `spec.Alter` 细节。

## 测试策略

- 首先扩展提取测试
- 每个关注点添加针对性规则测试
- 保持注册表集成覆盖确定性
- 重新运行 `go test ./internal/application/audit/...`、`go test ./internal/domain/rule/ddl/...`，然后 `go test ./...`

## 文档

更新：

- 决策日志
- `internal/domain/spec/README.md`
- `internal/application/audit/README.md`
- `internal/domain/rule/ddl/README.md`
- 仅在模块边界更改时更新根 `README.md`
