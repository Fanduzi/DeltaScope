# DeltaScope 源感知 Alter 检查设计

## 目标

将 `ALTER TABLE` 审计从粗粒度离线形状检查深化到源感知、关系感知的 alter 判断。此里程碑应保持当前解析器中立的模型，但添加足够的语义结构来决定变更是否可能安全、有风险或明显不兼容，而无需活动数据库访问。

## 当前状态

里程碑 2 已交付：

- 更丰富的解析器中立 `spec.Alter`
- 更丰富的列/索引/rename/选项变更的 alter 提取
- rename-index 禁止
- `MODIFY COLUMN` 和 `CHANGE COLUMN` 的目标类型族允许列表
- alter 添加的索引前缀检查

这很有用，但在最高价值的地方仍然浅薄：

- 没有列类型变更的源到目标比较
- 没有可空性/默认值/unsigned 转换的语义处理
- 除了纯前缀命名外没有更强的 alter-index 生命周期治理

## 推荐方向

### 选项 A：继续针对当前 payload 添加孤立的 alter 规则

优点：
- 最短期的进展最快

缺点：
- 将日益复杂的逻辑推入临时规则代码
- 源感知检查变得字符串繁重且脆弱

### 选项 B：用显式源/目标变更事实丰富 alter 提取

优点：
- 将解析器知识保持在应用层
- 给规则一个稳定的语义基础
- 让以后的在线/schema 感知检查复用相同模型

缺点：
- 需要提取器和领域演进

### 选项 C：推迟源感知检查直到实时 schema 支持存在

优点：
- 避免部分离线语义

缺点：
- 将最大 DDL 差距开放太久
- 阻止 alter 治理的有意义进展

## 决策

选择 **选项 B**。

在现有 alter 模型上添加第二个语义层：不是实时 schema 真相，而是规范化变更事实，比较语句表示要更改的内容。规则层应能够推理：

- 源列标识
- 目标列标识
- 源和目标类型族
- 源和目标可空性/默认值/unsigned/autoincrement 标志
- 索引添加/删除/重命名生命周期细节

## 提议的模型演进

保持 `DDL.Alter []Alter` 作为单一 alter 流，但用关系感知细节丰富类型化 alter payload：

- `AlterColumn`
  - 源标识
  - 目标定义
  - 语句形状中静态存在时的源定义子集
  - 规范化变更标志如 rename / type change / nullability change / default change
- `AlterIndex`
  - 生命周期操作种类
  - 旧/新名称
  - 添加的目标定义

提取器不应发明实时 schema 事实。它应仅保留可从语句本身推断的内容。

## 此里程碑的规则表面

### 列变更规则

- 源到目标类型兼容性策略
- 可空性收紧/放松策略
- 默认变更策略
- unsigned 和 auto-increment 转换策略
- rename-plus-type-change 复合限制

### Alter-index 规则

- 添加索引宽度/列数检查
- alter 添加的索引的重复索引检查
- 超越纯禁止开关的 drop/rename 索引治理

## 非目标

- 在线对象存在性检查
- 实际行数/影响估计
- 完整 create-table 超集工作
- HTTP API 或 MCP 服务工作

## 预期结果

此里程碑后，`ALTER TABLE` 应从"更好的规范化"转变为"有意义的判断"。剩余的最大 DDL 差距应然后转回 `CREATE TABLE` 广度，使下一个里程碑成为干净的 create-table 超集推进。
