# DeltaScope v1 设计文档

## 目标

构建 `DeltaScope` 作为 MySQL 和 TiDB 的新型 SQL 审查引擎。它必须保留 `gAudit` 的核心 DDL/DML 审计价值，但采用更清晰的架构、更强的规则模型，并具备通向 HTTP API 和 MCP 交付的路径。

## 产品范围

### 版本路线图

- **v1:** Go 库 + CLI
- **v2:** HTTP API 服务
- **v3:** MCP 服务器

### v1 边界

- 仅离线静态审计
- 不需要活动数据库连接
- 输入是 SQL 文本加上可选的 YAML 配置
- 未提供配置文件时使用内置默认配置
- 支持的方言为 **MySQL + TiDB**

### v1 非目标

- 不进行在线元数据查询
- 不针对真实数据库检查表/列是否存在
- 不进行基于 `EXPLAIN` 的影响行估计
- 不支持 schema 快照输入模型

## 关键产品原则

- `DeltaScope` 必须随时间成为 `gAudit` 审计能力的**严格超集**
- v1 应该已经覆盖 `gAudit` 的主要离线静态规则集
- 架构质量比克隆旧接口更重要
- 规则是一等公民产品特性，而非分散的条件判断
- 结果必须对 AI 代理、CI 和未来服务保持稳定

## 架构

`DeltaScope` 采用 DDD 倾向的架构，具有统一的领域审查模型。

### 分层

- `interfaces` 用于 CLI，未来支持 HTTP/MCP
- `application` 用于用例编排
- `domain` 用于核心审计概念和逻辑
- `infrastructure` 用于解析器/配置/输出适配器

### 依赖方向

`interfaces -> application -> domain <- infrastructure`

领域层不得依赖 Cobra、Viper 或 TiDB 解析器 AST 类型。

## 领域模型

核心领域对象是统一的 `StatementSpec`。

### `StatementSpec`

每个解析后的 SQL 语句在规则评估前都会被转换为规范化领域模型。规则从不直接在解析器 AST 节点上操作。

预期字段：

- 语句种类
- 方言
- 原始 SQL
- 规范化 SQL
- 源位置（如果有）
- 解析器警告（如果有）

按需附加的可选子结构：

- `TableSpec`
- `ColumnSpec[]`
- `IndexSpec[]`
- `ConstraintSpec[]`
- `AlterAction[]`
- `DMLSpec`
- `ObjectRefs`

## 审计流程

v1 流程为：

1. 加载内置默认值
2. 可选加载 YAML 配置覆盖
3. 将 SQL 解析为 AST
4. 将 AST 转换为 `StatementSpec`
5. 选择适用的规则
6. 评估规则
7. 聚合发现结果和裁决
8. 渲染 Markdown 或 JSON 输出

## 规则与策略

### 规则引擎

每个规则是一个独立单元，具有：

- 稳定的 `rule_id`
- 语句适用性
- 策略驱动的配置
- 发现结果输出

### 规则 ID 方案

规则 ID 使用点分命名，例如：

- `ddl.table.comment.require`
- `ddl.table.primary_key.require`
- `ddl.column.varchar.max_length`
- `ddl.index.secondary.prefix.require`
- `dml.where.require`
- `dml.limit.forbid`

### 策略格式

使用一个 YAML 配置文件，分组的顶级节：

- `app`
- `parser`
- `rules`
- `output`

按规则 ID 配置规则，例如：

```yaml
rules:
  ddl.table.comment.require:
    enabled: true
    level: warning

  ddl.table.name.max_length:
    enabled: true
    value: 64
    level: blocker

  dml.where.require:
    enabled: true
    level: blocker
```

## 结果模型

### 发现级别

- `blocker`
- `warning`
- `notice`

### 裁决

- 如果有任何 `blocker` 则为 `reject`
- 如果没有 blocker 但至少有一个 `warning` 则为 `review`
- 否则为 `pass`

### 发现形状

每个发现应支持：

- `rule_id`
- `level`
- `message`
- `statement_index`
- `statement_kind`
- `location`
- `suggestion`
- `metadata`

### 结果形状

顶级结果包含：

- 语句结果
- 全局发现
- 摘要计数
- 最终裁决

每个语句结果包含其自己的发现。

## CLI 设计

### 主命令

- `deltascope audit`

### 输入模式

- `--sql`
- `--file`
- `stdin`

### 关键标志

- `--config`
- `--dialect`
- `--format`
- `--fail-on`
- `--quiet`

### 输出

- 默认格式：`markdown`
- 必需的可选格式：`json`

### 退出码

- `0`：成功且低于失败阈值
- `1`：审计完成但超过阈值
- `2`：用户/输入/配置错误
- `3`：内部/运行时错误

## 配置和工具选择

- CLI 框架：`cobra`
- 配置系统：`viper`
- 配置格式：`YAML`
- 配置热重载：从 v1 开始设计，对未来长期运行模式最有用

对于 v1 CLI，配置监视器不应扭曲一次性命令模型。架构应支持以后添加配置监视，而无需现在强制不必要的运行时复杂性。

## v1 规则范围

### Tier 1：v1 必需

从 `gAudit` 派生并改进结构的主要离线静态规则：

- DDL 表命名、注释、字符集、引擎
- 主键存在性和形状
- 审计列
- 列命名、注释、类型和默认约束
- 类型允许/禁止规则
- 索引命名、数量、冗余和重复检查
- alter/drop/rename 限制
- create table as/like、外键、分区、视图切换
- DML where/limit/order by/subquery/join-on 规则
- 插入行数规则
- replace/on-duplicate/insert-select 限制

### Tier 2：后续离线增强

- 更丰富的类型兼容性检查
- 更深入的 alter-action 建模
- 更强的索引长度/前缀推断
- 更好的位置跨度和修复建议
- 更好的方言细节

### Tier 3：未来在线能力

- 表/列/索引的存在性检查
- 基于 explain 的影响行估计
- drop/truncate 的实时行数约束
- 实时版本感知规则决策

## 仓库结构方向

计划结构：

- `cmd/deltascope`
- `internal/interfaces`
- `internal/application`
- `internal/domain`
- `internal/infrastructure`
- `pkg/deltascope`

在 `internal/domain` 内，预期重点领域：

- `spec`
- `rule`
- `policy`
- `report`

在 `internal/infrastructure` 内，预期适配器：

- `parser/tidb`
- `config/viper`
- `output/markdown`
- `output/json`

## 质量标准

- 重写不得镜像 `gAudit` 包布局或检查器风格
- 领域规则必须与解析器 AST 解耦
- 输出契约必须对 AI 代理友好
- v1 应为 CLI、HTTP API 和 MCP 的重用做好准备，无需重新架构核心审计逻辑
