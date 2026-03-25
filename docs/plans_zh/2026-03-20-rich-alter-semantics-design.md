# DeltaScope 丰富 Alter 语义设计

## 目标

使 `ALTER TABLE` 成为一等、解析器中立的审计目标，而不是将其视为平面操作列表。结果应解锁更广泛的离线规则表面，同时保持现有的 DDD 倾向架构。

## 当前状态

`DeltaScope` 已经通过规范的 `spec.Alter{Action, Name}` 记录支持 action 级 alter 限制。这足以进行粗粒度禁止，例如：

- `drop_column`
- `drop_primary_key`
- `drop_index`
- `rename_table`
- `rename_column`
- `change_column`
- `modify_column`

这对于下一个高价值 DDL 差距仍然太薄：

- 列类型变更兼容性
- 超越粗粒度禁止的 rename 语义
- add/drop 索引细节
- alter 表选项变更
- 未来 merge-alter 限制

## 推荐方向

### 选项 A：保持 `spec.Alter` 平面并继续添加 action 特定规则

优点：
- 短期最快
- 无需新的提取器工作

缺点：
- 将 `ALTER TABLE` 变成另一个字符串开关子系统
- 为未来 HTTP/MCP 重用奠定弱基础
- 难以干净地表达更丰富的语义

### 选项 B：用类型化详细信息结构丰富 `spec.Alter`，保持单一规范化形状

将每个 alter 操作表示为带可选类型化详细信息的规范化记录：

- 列 add/drop/modify/change/rename
- 索引 add/drop/rename
- 表选项变更

优点：
- 最强的长期结构
- 保持规则解析器中立
- 保留一个统一的 `DDL.Alter` 入口

缺点：
- 需要仔细的 YAGNI 纪律

### 选项 C：将 alter 语义拆分为 `spec.DDL` 上的许多单独顶级切片

优点：
- 对某些直接消费者简单

缺点：
- 模型碎片化
- 更难保持 alter 排序连贯
- 将 `spec.DDL` 扩展为grab bag

## 决策

选择 **选项 B**。

保持 `DDL.Alter` 作为单一 alter 流，但将其扩展为具有可选类型化详细信息的更丰富的规范化模型。这保留了干净的领域边界，并为未来规则提供稳定的基础。

## 提议的领域形状

保持：

- `DDL.Alter []Alter`

将 `Alter` 大致演进为：

- `Action AlterAction`
- `Name string`
- `Column *AlterColumn`
- `Index *AlterIndex`
- `Options map[string]string`

其中：

- `AlterColumn` 携带旧/新名称、规范化的类型、长度、unsigned、not-null、auto-increment、默认标志和注释
- `AlterIndex` 携带种类、名称、旧/新名称和列

模型应仅包含即将到来的规则实际需要的字段。

## 第一个丰富 Alter 规则批次

使用更丰富的模型实施离线安全规则用于：

- `change column` 禁止
- `modify column` 禁止
- 兼容 vs 不兼容类型变更
- 带显式旧/新名称的 `rename column` 禁止
- `rename index` 禁止
- 带更清晰语义的 `drop index` / `drop column` 限制
- alter 添加的索引上的添加索引前缀和宽度规则

此批次仍应避免实时元数据假设，例如：

- drop 的列是否已存在
- 索引 rename 是否在线冲突

## 提取边界

所有 TiDB AST 检查保持在 `internal/application/audit` 内。

应用提取器应：

1. 解析原始 alter specs
2. 将它们规范化为更丰富的领域 `Alter` 记录
3. 永远不要将 TiDB AST 暴露给领域规则层

## 测试策略

- 首先用代表性 alter 案例扩展提取测试
- 每个 alter 关注点添加针对性领域规则测试
- 保持注册表排序覆盖明确
- 每个 alter 批次后验证完整套件兼容性

## 此里程碑范围外

- 在线存在性检查
- schema 快照比较
- 需要更深入方言/版本语义的 merge-alter 行为
- 每个剩余的 create-table 差距

## 预期结果

此里程碑后，`ALTER TABLE` 不应再是薄特殊情况。它应具有支持有意义的离线治理的稳定领域模型，并为 `DeltaScope` 提供更强的路径来最终在整体 DDL 语义上超越 `gAudit`。
