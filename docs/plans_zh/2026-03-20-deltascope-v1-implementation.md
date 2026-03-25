# DeltaScope v1 实施计划

> **给 Claude 的提示：** 必需子技能：使用 superpowers:executing-plans 逐任务实施此计划。

**目标：** 构建第一个可工作的 `DeltaScope` 版本，作为离线 MySQL/TiDB DDL-DML 审查库，支持 Cobra CLI 和 YAML 策略，以及稳定的 Markdown/JSON 输出。

**架构：** 使用 DDD 倾向的结构，领域拥有 `StatementSpec`、规则、策略、报告聚合和裁决逻辑。基础设施适配 TiDB 解析器、Viper 配置加载和输出渲染；CLI 作为应用用例的薄接口保持轻量。

**技术栈：** Go, Cobra, Viper, TiDB 解析器, fsnotify, Go testing

---

### 任务 1：初始化仓库骨架

**文件：**
- 创建：`go.mod`
- 创建：`cmd/deltascope/main.go`
- 创建：`internal/interfaces/cli/`
- 创建：`internal/application/`
- 创建：`internal/domain/`
- 创建：`internal/infrastructure/`
- 创建：`pkg/deltascope/`

**步骤 1：初始化 Go 模块**

运行：`go mod init github.com/fan/deltascope`
预期：`go.mod` 被创建，包含新的模块路径。

**步骤 2：创建空包骨架**

创建接口、应用、领域、基础设施和公共包入口的核心目录。

**步骤 3：添加最小的 `main.go`**

实现一个小的 CLI 引导程序，委托给 CLI 包。

**步骤 4：验证编译基线**

运行：`go test ./...`
预期：包可以用占位符实现编译。

**步骤 5：提交**

```bash
git add .
git commit -m "chore: initialize deltascope project skeleton"
```

### 任务 2：定义核心领域类型

**文件：**
- 创建：`internal/domain/spec/statement.go`
- 创建：`internal/domain/spec/ddl.go`
- 创建：`internal/domain/spec/dml.go`
- 创建：`internal/domain/rule/rule.go`
- 创建：`internal/domain/report/result.go`
- 创建：`internal/domain/policy/policy.go`
- 测试：`internal/domain/report/result_test.go`

**步骤 1：编写失败的裁决聚合测试**

覆盖来自 notice/warning/blocker 发现结果的 `pass`、`review` 和 `reject` 行为。

**步骤 2：运行针对性测试**

运行：`go test ./internal/domain/report -run TestVerdict -v`
预期：FAIL，因为类型和聚合尚不存在。

**步骤 3：实现最小的领域对象**

定义 `StatementSpec`、发现级别、裁决计算、摘要计数和策略存根。

**步骤 4：重新运行测试**

运行：`go test ./internal/domain/report -run TestVerdict -v`
预期：PASS。

**步骤 5：提交**

```bash
git add .
git commit -m "feat: define core audit domain types"
```

### 任务 3：实现策略默认值和 YAML 加载

**文件：**
- 创建：`internal/infrastructure/config/viper/loader.go`
- 创建：`internal/application/policy/load.go`
- 创建：`internal/domain/policy/defaults.go`
- 创建：`configs/deltascope.example.yaml`
- 测试：`internal/infrastructure/config/viper/loader_test.go`

**步骤 1：编写失败的配置加载器测试**

覆盖内置默认值、YAML 覆盖加载和规则级值/级别解析。

**步骤 2：运行针对性测试**

运行：`go test ./internal/infrastructure/config/viper -run TestLoader -v`
预期：FAIL，因为加载器缺失。

**步骤 3：实现 Viper 支持的加载**

支持没有文件时使用默认策略，有 YAML 路径时使用文件覆盖。

**步骤 4：添加示例配置**

创建与所选规则 ID 方案匹配的人类可编辑示例。

**步骤 5：重新运行测试**

运行：`go test ./internal/infrastructure/config/viper -run TestLoader -v`
预期：PASS。

**步骤 6：提交**

```bash
git add .
git commit -m "feat: add YAML policy loading with defaults"
```

### 任务 4：添加解析器适配器和语句分类

**文件：**
- 创建：`internal/infrastructure/parser/tidb/parser.go`
- 创建：`internal/application/audit/parse.go`
- 创建：`internal/domain/spec/kind.go`
- 测试：`internal/infrastructure/parser/tidb/parser_test.go`

**步骤 1：编写失败的解析器测试**

覆盖多语句 SQL 的成功解析和解析失败行为。

**步骤 2：运行针对性测试**

运行：`go test ./internal/infrastructure/parser/tidb -run TestParser -v`
预期：FAIL，因为解析器适配器缺失。

**步骤 3：实现 TiDB 解析器适配器**

以基础设施中立的形式返回解析的语句和解析器警告。

**步骤 4：重新运行测试**

运行：`go test ./internal/infrastructure/parser/tidb -run TestParser -v`
预期：PASS。

**步骤 5：提交**

```bash
git add .
git commit -m "feat: add TiDB parser adapter"
```

### 任务 5：从 AST 提取到 `StatementSpec`

**文件：**
- 创建：`internal/application/audit/extract.go`
- 创建：`internal/infrastructure/parser/tidb/extractor.go`
- 测试：`internal/infrastructure/parser/tidb/extractor_test.go`

**步骤 1：编写失败的提取器测试**

覆盖代表性语句：
- `CREATE TABLE`
- `ALTER TABLE`
- `INSERT`
- `UPDATE`
- `DELETE`

**步骤 2：运行针对性测试**

运行：`go test ./internal/infrastructure/parser/tidb -run TestExtractor -v`
预期：FAIL，因为提取未实现。

**步骤 3：实现最小的 `StatementSpec` 提取**

填充语句种类、原始 SQL、规范化 SQL 和第一遍 DDL/DML 子结构。

**步骤 4：重新运行测试**

运行：`go test ./internal/infrastructure/parser/tidb -run TestExtractor -v`
预期：PASS。

**步骤 5：提交**

```bash
git add .
git commit -m "feat: map AST statements into StatementSpec"
```

### 任务 6：构建规则引擎核心

**文件：**
- 创建：`internal/domain/rule/registry.go`
- 创建：`internal/application/audit/evaluate.go`
- 测试：`internal/domain/rule/registry_test.go`

**步骤 1：编写失败的规则引擎测试**

覆盖规则注册、适用性过滤和发现收集。

**步骤 2：运行针对性测试**

运行：`go test ./internal/domain/rule -run TestRegistry -v`
预期：FAIL，因为注册表和评估器不存在。

**步骤 3：实现最小引擎**

添加规则注册、语句适用性检查和语句/全局发现收集。

**步骤 4：重新运行测试**

运行：`go test ./internal/domain/rule -run TestRegistry -v`
预期：PASS。

**步骤 5：提交**

```bash
git add .
git commit -m "feat: add core rule engine"
```

### 任务 7：实现 Tier-1 DDL 规则

**文件：**
- 创建：`internal/domain/rule/ddl/...`
- 测试：`internal/domain/rule/ddl/..._test.go`

**步骤 1：将 DDL 规则分组为小批次**

每个关注点一个测试文件，分批实现：
- 表命名和注释
- 主键形状
- 审计列
- 列约束
- 索引约束
- alter 限制

**步骤 2：每批编写失败的测试**

使用小型 SQL fixtures 并断言稳定的 `rule_id`、`level` 和消息行为。

**步骤 3：为该批次实现最小规则**

不要在一个规则文件中混合无关的检查。

**步骤 4：每批后重新运行针对性测试**

运行：`go test ./internal/domain/rule/ddl/... -v`
预期：PASS。

**步骤 5：每批完成后提交**

例如：

```bash
git add .
git commit -m "feat: add DDL primary key and audit column rules"
```

### 任务 8：实现 Tier-1 DML 规则

**文件：**
- 创建：`internal/domain/rule/dml/...`
- 测试：`internal/domain/rule/dml/..._test.go`

**步骤 1：为 DML 形状规则编写失败的测试**

覆盖：
- 缺少 `WHERE`
- 禁止 `LIMIT`
- 禁止 `ORDER BY`
- 禁止子查询
- 连接没有 `ON`
- 插入行数
- replace / insert-select / on-duplicate 限制

**步骤 2：运行针对性测试**

运行：`go test ./internal/domain/rule/dml/... -v`
预期：最初 FAIL。

**步骤 3：分小批次实现 DML 规则**

保持规则 ID 命名和严重性默认值与策略设计一致。

**步骤 4：重新运行针对性测试**

运行：`go test ./internal/domain/rule/dml/... -v`
预期：PASS。

**步骤 5：提交**

```bash
git add .
git commit -m "feat: add Tier-1 DML rules"
```

### 任务 9：组装应用用例和公共 API

**文件：**
- 创建：`internal/application/audit/service.go`
- 创建：`pkg/deltascope/audit.go`
- 测试：`pkg/deltascope/audit_test.go`

**步骤 1：编写失败的端到端库测试**

覆盖：
- 带内联 SQL 的默认策略
- 配置覆盖路径
- 多语句 SQL
- 裁决和语句分组

**步骤 2：运行针对性测试**

运行：`go test ./pkg/deltascope -run TestAudit -v`
预期：FAIL，因为公共 API 不存在。

**步骤 3：实现用例**

将策略加载、解析、提取、评估和报告聚合连接到一个稳定的公共函数中。

**步骤 4：重新运行测试**

运行：`go test ./pkg/deltascope -run TestAudit -v`
预期：PASS。

**步骤 5：提交**

```bash
git add .
git commit -m "feat: expose public audit API"
```

### 任务 10：添加 Markdown 和 JSON 渲染器

**文件：**
- 创建：`internal/infrastructure/output/markdown/render.go`
- 创建：`internal/infrastructure/output/json/render.go`
- 测试：`internal/infrastructure/output/..._test.go`

**步骤 1：编写失败的渲染器测试**

覆盖两种格式的稳定裁决、摘要和语句发现渲染。

**步骤 2：运行针对性测试**

运行：`go test ./internal/infrastructure/output/... -v`
预期：FAIL，因为渲染器不存在。

**步骤 3：实现 Markdown 和 JSON 格式化**

保持 JSON 键稳定，Markdown 部分对人类和 AI 代理都易于扫描。

**步骤 4：重新运行测试**

运行：`go test ./internal/infrastructure/output/... -v`
预期：PASS。

**步骤 5：提交**

```bash
git add .
git commit -m "feat: add markdown and json renderers"
```

### 任务 11：构建 Cobra CLI

**文件：**
- 创建：`internal/interfaces/cli/root.go`
- 创建：`internal/interfaces/cli/audit.go`
- 创建：`internal/interfaces/cli/config_init.go`
- 创建：`internal/interfaces/cli/version.go`
- 修改：`cmd/deltascope/main.go`
- 测试：`internal/interfaces/cli/..._test.go`

**步骤 1：编写失败的 CLI 测试**

覆盖：
- `audit --sql`
- `audit --file`
- stdin 输入
- `--format json`
- `config init`
- 退出码阈值行为

**步骤 2：运行针对性测试**

运行：`go test ./internal/interfaces/cli/... -v`
预期：在命令实现前 FAIL。

**步骤 3：实现 Cobra 命令**

添加 `audit`、`config init` 和 `version` 作为 v1 命令集。

**步骤 4：干净地绑定配置相关标志**

连接 `--config`、`--dialect`、`--format`、`--fail-on` 和 `--quiet`。

**步骤 5：重新运行测试**

运行：`go test ./internal/interfaces/cli/... -v`
预期：PASS。

**步骤 6：提交**

```bash
git add .
git commit -m "feat: add deltascope cobra cli"
```

### 任务 12：最终验证和文档

**文件：**
- 创建：`README.md`
- 修改：`configs/deltascope.example.yaml`
- 修改：`docs/plans/2026-03-20-deltascope-v1-design.md`

**步骤 1：编写 README**

文档：
- `DeltaScope` 是什么
- 支持的方言
- 离线审计模型
- CLI 示例
- 配置示例
- 输出示例

**步骤 2：运行完整验证**

运行：`go test ./...`
预期：整个仓库 PASS。

**步骤 3：手动烟雾测试 CLI**

运行：
- `go run ./cmd/deltascope audit --sql "delete from t"`
- `go run ./cmd/deltascope audit --sql "delete from t" --format json`

预期：
- 第一个命令的 Markdown 输出
- 第二个命令的有效 JSON 输出

**步骤 4：提交**

```bash
git add .
git commit -m "docs: add DeltaScope README and examples"
```
