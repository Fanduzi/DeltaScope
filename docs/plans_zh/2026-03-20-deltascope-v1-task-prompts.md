# DeltaScope v1 任务提示

> 用于逐任务实施和审查。
> 每个任务提示都设计为在 `/Users/fan/GolangProjects/deltascope` 内工作的执行代理。

## 全局规则

- 遵循 `docs/plans/2026-03-20-deltascope-v1-design.md` 中的设计。
- 遵循 `docs/plans/2026-03-20-deltascope-v1-implementation.md` 中的实施顺序。
- 不要镜像 `gAudit` 包布局或检查器风格。
- 保持 DDD 倾向的架构：
  - `interfaces -> application -> domain <- infrastructure`
- 领域层不得依赖 Cobra、Viper 或 TiDB 解析器 AST 类型。
- `DeltaScope` v1 仅为离线模式，不需要活动数据库连接。
- 支持的方言为 MySQL 和 TiDB。
- CLI 要求：
  - Cobra
  - Viper
  - YAML 配置
  - 默认 Markdown 输出
  - 必需的 `--format json`
- 发现级别：
  - `blocker`
  - `warning`
  - `notice`
- 每个任务的审查者面向结果必须包括：
  - 更改的文件摘要
  - 运行的测试
  - 当前状态
  - 提交后的 git 提交哈希

## 审查者响应模板

报告任务完成时使用此确切结构：

```md
任务：<任务名称>

更改内容：
- ...

验证：
- 运行：`<命令>`
- 结果：PASS/FAIL

提交：
- 哈希：`<提交哈希>`
- 消息：`<提交消息>`

开放问题：
- 无
```

## 任务 1 提示

### 目标

初始化 `DeltaScope` 的仓库骨架。

### 范围

- 创建 Go 模块
- 创建包骨架
- 添加最小的 CLI 引导入口

### 文件

- 创建 `go.mod`
- 创建 `cmd/deltascope/main.go`
- 创建 `internal/interfaces/cli/`
- 创建 `internal/application/`
- 创建 `internal/domain/`
- 创建 `internal/infrastructure/`
- 创建 `pkg/deltascope/`

### 验收标准

- `go.mod` 存在
- 包布局与实施计划匹配
- `cmd/deltascope/main.go` 存在并委托给 CLI 代码
- `go test ./...` 用占位符成功完成

### 必需验证

- 运行 `go test ./...`

### 审查者返回

- 提交哈希
- 提交消息
- 说明骨架是否编译干净

## 任务 2 提示

### 目标

为语句、策略、发现、裁决和结果聚合定义第一个核心领域类型。

### 范围

- 添加领域模型骨架
- 添加裁决聚合行为
- 为 `pass`、`review` 和 `reject` 添加测试

### 文件

- 创建 `internal/domain/spec/statement.go`
- 创建 `internal/domain/spec/ddl.go`
- 创建 `internal/domain/spec/dml.go`
- 创建 `internal/domain/rule/rule.go`
- 创建 `internal/domain/report/result.go`
- 创建 `internal/domain/policy/policy.go`
- 创建 `internal/domain/report/result_test.go`

### 验收标准

- 领域类型存在，归属清晰
- 发现级别为 `blocker`、`warning`、`notice`
- 裁决逻辑有测试覆盖
- 报告聚合测试通过

### 必需验证

- 运行 `go test ./internal/domain/report -run TestVerdict -v`

### 审查者返回

- 提交哈希
- 提交消息
- 关于领域边界质量的简短说明

## 任务 3 提示

### 目标

用 Viper 实现默认策略和 YAML 加载。

### 范围

- 内置默认策略
- YAML 文件覆盖支持
- 示例配置文件

### 文件

- 创建 `internal/infrastructure/config/viper/loader.go`
- 创建 `internal/application/policy/load.go`
- 创建 `internal/domain/policy/defaults.go`
- 创建 `configs/deltascope.example.yaml`
- 创建 `internal/infrastructure/config/viper/loader_test.go`

### 验收标准

- 未提供配置文件时加载默认值
- YAML 覆盖正确反序列化
- 规则 ID 干净地映射到策略条目
- 示例配置与预期的规则命名风格匹配

### 必需验证

- 运行 `go test ./internal/infrastructure/config/viper -run TestLoader -v`

### 审查者返回

- 提交哈希
- 提交消息
- 说明配置格式是否对用户足够稳定

## 任务 4 提示

### 目标

添加 TiDB 解析器适配器和语句分类支持。

### 范围

- 解析多语句 SQL
- 干净地暴露解析器警告和失败
- 避免 AST 类型泄漏到领域

### 文件

- 创建 `internal/infrastructure/parser/tidb/parser.go`
- 创建 `internal/application/audit/parse.go`
- 创建 `internal/domain/spec/kind.go`
- 创建 `internal/infrastructure/parser/tidb/parser_test.go`

### 验收标准

- 解析器适配器可以解析有效的多语句 SQL
- 解析失败有测试覆盖
- AST 包含在基础设施/应用边界内

### 必需验证

- 运行 `go test ./internal/infrastructure/parser/tidb -run TestParser -v`

### 审查者返回

- 提交哈希
- 提交消息
- 说明 AST 泄漏是否被避免

## 任务 5 提示

### 目标

将解析的 AST 转换为统一的 `StatementSpec` 模型。

### 范围

- 为代表性 DDL 和 DML 语句实现第一遍提取
- 填充核心 `StatementSpec` 字段

### 文件

- 创建 `internal/application/audit/extract.go`
- 创建 `internal/infrastructure/parser/tidb/extractor.go`
- 创建 `internal/infrastructure/parser/tidb/extractor_test.go`

### 验收标准

- `CREATE TABLE`、`ALTER TABLE`、`INSERT`、`UPDATE` 和 `DELETE` 被映射
- `StatementSpec` 包含种类、原始 SQL、规范化 SQL 和第一遍子结构
- 测试覆盖提取行为

### 必需验证

- 运行 `go test ./internal/infrastructure/parser/tidb -run TestExtractor -v`

### 审查者返回

- 提交哈希
- 提交消息
- 说明支持哪些语句形状

## 任务 6 提示

### 目标

构建核心规则引擎和注册表。

### 范围

- 规则注册
- 适用性过滤
- 发现收集

### 文件

- 创建 `internal/domain/rule/registry.go`
- 创建 `internal/application/audit/evaluate.go`
- 创建 `internal/domain/rule/registry_test.go`

### 验收标准

- 规则可以确定性注册和评估
- 语句级适用性工作
- 发现被收集到报告流程中

### 必需验证

- 运行 `go test ./internal/domain/rule -run TestRegistry -v`

### 审查者返回

- 提交哈希
- 提交消息
- 说明引擎是否对 Tier-1 规则足够可扩展

## 任务 7 提示

### 目标

实现 Tier-1 DDL 规则。

### 范围

- 表命名和注释
- 主键形状
- 审计列
- 列约束
- 索引约束
- alter 限制

### 文件

- 在 `internal/domain/rule/ddl/` 下创建规则文件
- 在 `internal/domain/rule/ddl/` 下创建测试

### 验收标准

- DDL 规则具有稳定的 `rule_id`
- 规则消息和级别是确定性的
- 每个关注点都有针对性测试覆盖
- 规则与 v1 设计一致并改进了 `gAudit` 结构

### 必需验证

- 运行 `go test ./internal/domain/rule/ddl/... -v`

### 审查者返回

- 提交哈希
- 提交消息
- 已实现规则 ID 列表

## 任务 8 提示

### 目标

实现 Tier-1 DML 规则。

### 范围

- 要求 `WHERE`
- 禁止 `LIMIT`
- 禁止 `ORDER BY`
- 禁止子查询
- 要求 `JOIN ... ON`
- 限制插入行数
- 禁止 `REPLACE`
- 禁止 `INSERT ... SELECT`
- 禁止 `ON DUPLICATE KEY`

### 文件

- 在 `internal/domain/rule/dml/` 下创建规则文件
- 在 `internal/domain/rule/dml/` 下创建测试

### 验收标准

- DML 规则可独立测试
- 规则 ID 和严重性与策略设计一致
- 代表性 SQL fixtures 被覆盖

### 必需验证

- 运行 `go test ./internal/domain/rule/dml/... -v`

### 审查者返回

- 提交哈希
- 提交消息
- 已实现规则 ID 列表

## 任务 9 提示

### 目标

组装应用审计用例和稳定的公共库 API。

### 范围

- 应用服务编排策略、解析、提取、规则和报告
- 稳定的 `pkg/deltascope` 入口

### 文件

- 创建 `internal/application/audit/service.go`
- 创建 `pkg/deltascope/audit.go`
- 创建 `pkg/deltascope/audit_test.go`

### 验收标准

- 公共 API 可以用默认策略审计内联 SQL
- 配置覆盖路径有效
- 多语句 SQL 返回分组的语句结果
- 结果包含裁决和语句发现

### 必需验证

- 运行 `go test ./pkg/deltascope -run TestAudit -v`

### 审查者返回

- 提交哈希
- 提交消息
- 关于公共 API 稳定性的简短说明

## 任务 10 提示

### 目标

为审计结果添加 Markdown 和 JSON 渲染器。

### 范围

- 默认 Markdown 渲染器
- 用于机器消费的稳定 JSON 渲染器

### 文件

- 创建 `internal/infrastructure/output/markdown/render.go`
- 创建 `internal/infrastructure/output/json/render.go`
- 创建渲染器测试

### 验收标准

- Markdown 对人类和 AI 代理都易于扫描
- JSON 键稳定且面向机器
- 渲染器通过针对性测试

### 必需验证

- 运行 `go test ./internal/infrastructure/output/... -v`

### 审查者返回

- 提交哈希
- 提交消息
- 说明 JSON 是否对技能集成足够稳定

## 任务 11 提示

### 目标

为 `DeltaScope` 构建 Cobra CLI。

### 范围

- `audit`
- `config init`
- `version`
- 标志连接和退出码行为

### 文件

- 创建 `internal/interfaces/cli/root.go`
- 创建 `internal/interfaces/cli/audit.go`
- 创建 `internal/interfaces/cli/config_init.go`
- 创建 `internal/interfaces/cli/version.go`
- 修改 `cmd/deltascope/main.go`
- 创建 CLI 测试

### 验收标准

- 支持 `--sql`、`--file` 和 stdin
- 支持 `--format json`
- 支持 `--config`、`--dialect`、`--format`、`--fail-on`、`--quiet`
- `config init` 生成可用的 YAML 模板
- 退出码遵循约定的契约

### 必需验证

- 运行 `go test ./internal/interfaces/cli/... -v`

### 审查者返回

- 提交哈希
- 提交消息
- 用于手动完整性检查的示例 CLI 调用

## 任务 12 提示

### 目标

最终确定文档并端到端验证仓库。

### 范围

- 编写 README
- 验证示例
- 运行完整测试套件

### 文件

- 创建 `README.md`
- 修改 `configs/deltascope.example.yaml`
- 如果实现有合理偏离则更新计划/设计文档

### 验收标准

- README 解释 `DeltaScope` 是什么以及如何使用
- 示例配置与实现一致
- 整个仓库测试通过
- 手动 CLI 烟雾测试在 Markdown 和 JSON 模式下都有效

### 必需验证

- 运行 `go test ./...`
- 运行 `go run ./cmd/deltascope audit --sql "delete from t"`
- 运行 `go run ./cmd/deltascope audit --sql "delete from t" --format json`

### 审查者返回

- 提交哈希
- 提交消息
- 关于 HTTP API 工作前剩余差距的说明
