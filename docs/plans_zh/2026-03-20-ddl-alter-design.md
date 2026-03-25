# DeltaScope DDL Alter 批次设计

## 目标

通过添加安全的 `ALTER TABLE` 限制规则来关闭下一个主要离线 DDL 差距，这些规则在规范化 alter 操作上运行，无需活动数据库元数据。

## 背景

当前提取器已经将 alter 操作规范化为简单的领域值：

- `add_columns`
- `drop_column`
- `modify_column`
- `change_column`
- `rename_column`
- `rename_table`
- `drop_primary_key`
- `drop_index`
- `add_constraint`
- `table_option`

这对于完整的 alter 分析还不够，但足以进行操作级禁止规则。

## 方法

### 选项 A：等待更丰富的 alter 模型

优点：
- 避免未来规则输入迁移

缺点：
- 留下大面积 DDL 风险表面未覆盖
- 减慢已经有部分离线规则安全的领域的进度

### 选项 B：现在添加操作级禁止规则，保持粗粒度

为已经规范化的 alter 操作实施策略驱动的限制：

- drop column
- drop primary key
- drop index
- rename table
- rename column
- change column
- modify column

优点：
- 立即高价值
- 没有解析器泄漏到规则
- 匹配多个 `gAudit` 治理开关

缺点：
- 对于类型兼容性或存在性检查仍然太粗粒度

### 选项 C：尝试在一个批次中深度解决 alter 语义

优点：
- 更强的长期模型

缺点：
- 更大的建模工作
- 在仓库仍在快速成长时可能产生仓促的抽象

## 决策

选择 **选项 B**。

DeltaScope 现在应该捕获高信号离线 alter 限制，同时为以后更丰富的 alter 模型保留空间。

## 规则批次

计划的规则 ID：

- `ddl.alter.drop_column.forbid`
- `ddl.alter.drop_primary_key.forbid`
- `ddl.alter.drop_index.forbid`
- `ddl.alter.rename_table.forbid`
- `ddl.alter.rename_column.forbid`
- `ddl.alter.change_column.forbid`
- `ddl.alter.modify_column.forbid`

## 策略默认值

推荐默认值：

- drop column：默认允许
- drop primary key：默认禁止
- drop index：默认允许
- rename table：默认禁止
- rename column：默认禁止
- change column：默认禁止
- modify column：默认允许

这反映了当前产品立场：默认阻止最危险的结构重写，但避免全局禁止每个 alter 操作。

## 推迟的工作

- 列类型兼容性分析
- drop/rename 对象的存在性检查
- 索引 rename 限制
- MySQL/TiDB 的 merge-alter-table 限制
- 超越 create-table 的对象/表选项验证

## 验证

- 仅在需要时扩展 alter 提取断言
- 为每个禁止操作添加针对性规则测试
- 保持注册表排序覆盖确定性
- 重新运行：
  - `go test ./internal/application/audit -run TestExtract -v`
  - `go test ./internal/domain/rule/ddl/... -v`
  - `go test ./...`
  - `diff -u configs/deltascope.example.yaml <(go run ./cmd/deltascope config init)`
  - `check_three_level_doc.sh`
