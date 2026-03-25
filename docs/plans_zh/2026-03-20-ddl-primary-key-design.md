# DeltaScope DDL 主键语义设计

## 目标

通过为常见单列情况强制更强的主键语义，关闭另一个高价值 `gAudit` 差距。

## 背景

DeltaScope 已经检查：

- 主键存在性
- 主键列数限制

它还没有强制 `gAudit` 对许多团队期望的更强形状约定：

- bigint 主键
- unsigned 主键
- auto-increment 主键
- 显式 not-null 主键

## 方法

### 选项 A：保持主键规则仅为结构性的

优点：
- 无提取器更改

缺点：
- 错过一些最有意见和最有用的治理检查

### 选项 B：稍微丰富列元数据并添加语义 PK 规则

添加：

- `Column.Unsigned`
- `Column.AutoIncrement`

然后根据这些字段评估声明的主键列。

优点：
- 小的模型更改
- 对常见单列 PK 模式高价值
- 在规则层仍然完全离线和解析器中立

缺点：
- 复合主键仍然需要单独处理

### 选项 C：先等待标识符和类型族工作

优点：
- 更少的并发 DDL 方向

缺点：
- 延迟另一个清晰的 `gAudit` 对等胜利

## 决策

选择 **选项 B**。

此批次小、自包含，捕获许多团队在审查中实际关心的规则。

## 模型更改

用以下内容扩展 `spec.Column`：

- `Unsigned bool`
- `AutoIncrement bool`

规则将通过名称从规范化列列表中查找主键列。

## 规则批次

计划的规则 ID：

- `ddl.table.primary_key.bigint.require`
- `ddl.table.primary_key.unsigned.require`
- `ddl.table.primary_key.auto_increment.require`
- `ddl.table.primary_key.not_null.require`

## 策略默认值

推荐默认值：

- bigint required: true
- unsigned required: true
- auto increment required: true
- not null required: true

这些默认值与现有项目方向暗示的更严格约定一致。

## 推迟的工作

- 复合 PK 语义的更智能处理
- 键名标识符检查
- 自动递增初始值检查

## 验证

- 扩展 unsigned/auto_increment 元数据的提取测试
- 添加针对性 PK-语义规则测试
- 重新运行：
  - `go test ./internal/application/audit -run TestExtract -v`
  - `go test ./internal/domain/rule/ddl/... -v`
  - `go test ./...`
  - `diff -u configs/deltascope.example.yaml <(go run ./cmd/deltascope config init)`
  - `check_three_level_doc.sh`
