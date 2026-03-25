# DeltaScope DDL 索引批次设计

## 目标

用聚焦的 `CREATE TABLE` 索引治理批次扩展 `DeltaScope` 的离线 DDL 覆盖范围，向 `gAudit` 的索引规则表面迈进，同时不强制过早的 `ALTER TABLE` 建模。

## 背景

第二个 DDL 批次已经丰富了列语义并添加了列/审计列治理。下一个最安全的差距是索引治理，因为提取的 `CREATE TABLE` 形状已经包含索引名称和索引列。唯一缺少的语义是索引种类。

## 方法

### 选项 A：保持 `spec.Index` 浅层，仅添加总数检查

优点：
- 最少更改
- 非常低的实施风险

缺点：
- 无法表达唯一/全文前缀规则
- 留下 `gAudit` 的大部分索引配置未覆盖

### 选项 B：一次添加索引种类并实施连贯的 create-table 索引批次

用类型化的 `Kind` 扩展 `spec.Index` 并将 TiDB 约束种类映射到该模型。然后实施：

- 辅助索引总数
- 每个索引列数限制
- 唯一索引前缀要求
- 辅助索引前缀要求
- 全文索引前缀要求
- 重复索引检测

优点：
- 小模型增长，高价值
- 与 `gAudit` 索引策略表面一致
- 保持解析器中立和离线安全

缺点：
- 比纯计数批次稍广

### 选项 C：现在跳到 alter 限制

优点：
- 更早解决更危险的 DDL 操作

缺点：
- 当前 `spec.Alter` 只有 `Action + Name`
- 高概率产生脆弱规则和另一次强制重构

## 决策

选择 **选项 B**。

`DeltaScope` 应该首先完成强大的 `CREATE TABLE` 治理表面。用 `Kind` 扩展 `spec.Index` 是一个廉价、稳定的改进，可以立即解锁多个有意义的规则，而 alter 限制仍然需要更丰富的提取元数据。

## 模型更改

添加类型化索引种类字段：

- `spec.IndexKind`
- `spec.Index.Kind`

计划值：

- `IndexKindPrimary`
- `IndexKindSecondary`
- `IndexKindUnique`
- `IndexKindFulltext`
- `IndexKindUnknown`

提取规则：

- 主键保持在 `DDL.PrimaryKey` 中，但应携带 `KindPrimary`
- `KEY` / `INDEX` 映射到 `KindSecondary`
- `UNIQUE*` 映射到 `KindUnique`
- `FULLTEXT` 映射到 `KindFulltext`

## 规则批次

计划的规则 ID：

- `ddl.index.total.max_count`
- `ddl.index.columns.max_count`
- `ddl.index.unique.prefix.require`
- `ddl.index.secondary.prefix.require`
- `ddl.index.fulltext.prefix.require`
- `ddl.index.duplicate.forbid`

## 策略默认值

推荐默认值：

- 索引总数限制：`12`
- 每索引列限制：`8`
- 唯一前缀：`uniq_`
- 辅助前缀：`idx_`
- 全文前缀：`full_`
- 重复检测：启用

小写默认值比 `gAudit` 的大写示例更适合 DeltaScope 的命名风格，同时保持语义等价。

## 推迟的工作

- alter-table 索引 rename/drop 限制
- 除完全重复外的冗余前缀检测
- 索引表达式/前缀长度感知的重复分析

## 验证

- 首先扩展提取覆盖
- 每个索引关注点添加针对性规则测试
- 保持注册表排序覆盖确定性
- 重新运行：
  - `go test ./internal/application/audit -run TestExtract -v`
  - `go test ./internal/domain/rule/ddl/... -v`
  - `go test ./...`
  - `diff -u configs/deltascope.example.yaml <(go run ./cmd/deltascope config init)`
  - `check_three_level_doc.sh`
