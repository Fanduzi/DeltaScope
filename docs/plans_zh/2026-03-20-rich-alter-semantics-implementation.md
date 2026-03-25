# 丰富 Alter 语义实施计划

> **给 Claude 的提示：** 必需子技能：使用 superpowers:executing-plans 逐任务实施此计划。

**目标：** 将 `ALTER TABLE` 从粗粒度仅 action 审计升级到更丰富的规范化领域模型加上一批语义 alter 规则。

**架构：** 保持 `internal/application/audit` 作为唯一的 AST 感知层，用类型化详细信息记录丰富 `internal/domain/spec.Alter`，并将所有 alter 审计保持在 `internal/domain/rule/ddl` 内，针对解析器中立领域结构。

**技术栈：** Go, TiDB 解析器 AST, Cobra/Viper 配置流程, Go testing

---

### 任务 1：扩展领域 alter 模型

**文件：**
- 修改：`internal/domain/spec/ddl.go`
- 修改：`internal/domain/spec/README.md`

**步骤 1：在下游测试中添加失败的期望**

更新即将到来的提取器/规则测试，以便它们需要更丰富的 alter 详细信息，例如：
- 旧/新列名
- 规范化目标类型
- 索引种类和列列表
- 更改的表选项

**步骤 2：运行针对性测试**

运行：`go test ./internal/application/audit -run TestExtractMapsAlterTable -v`
预期：一旦测试期望新详细信息，FAIL。

**步骤 3：添加最小的领域结构**

添加：
- `AlterAction` 类型
- `AlterColumn`
- `AlterIndex`
- 更丰富的 `Alter`

仅保留第一批规则需要的字段。

**步骤 4：重新运行针对性测试**

运行：`go test ./internal/application/audit -run TestExtractMapsAlterTable -v`
预期：直到提取实施，仍 FAIL。

**步骤 5：提交**

```bash
git add internal/domain/spec/ddl.go internal/domain/spec/README.md
git commit -m "refactor: expand alter domain model"
```

### 任务 2：丰富 alter 提取

**文件：**
- 修改：`internal/application/audit/extract.go`
- 修改：`internal/application/audit/extract_test.go`
- 修改：`internal/application/audit/README.md`

**步骤 1：编写失败的提取器案例**

覆盖：
- `ALTER TABLE ... MODIFY COLUMN`
- `ALTER TABLE ... CHANGE COLUMN`
- `ALTER TABLE ... RENAME COLUMN`
- `ALTER TABLE ... ADD INDEX`
- `ALTER TABLE ... DROP INDEX`
- `ALTER TABLE ... RENAME INDEX`
- `ALTER TABLE ... ENGINE=...`

**步骤 2：运行针对性提取器测试**

运行：`go test ./internal/application/audit -run TestExtractMapsAlterTable -v`
预期：FAIL。

**步骤 3：实施最小提取**

填充更丰富的 `Alter` 详细信息，不将 TiDB AST 暴露到应用层之外。

**步骤 4：重新运行提取器测试**

运行：`go test ./internal/application/audit -run TestExtractMapsAlterTable -v`
预期：PASS。

**步骤 5：提交**

```bash
git add internal/application/audit/extract.go internal/application/audit/extract_test.go internal/application/audit/README.md
git commit -m "feat: enrich alter extraction"
```

### 任务 3：添加 alter 语义规则构造函数和共享辅助函数

**文件：**
- 修改：`internal/domain/rule/ddl/common.go`
- 修改：`internal/domain/rule/ddl/config.go`
- 修改：`internal/domain/rule/ddl/README.md`

**步骤 1：定义新规则 ID**

为第一批更丰富的 alter 添加 ID，例如：
- `ddl.alter.modify_column.compatible.require`
- `ddl.alter.rename_index.forbid`
- `ddl.alter.add_index.secondary.prefix.require`

**步骤 2：添加共享辅助函数**

辅助函数应覆盖：
- alter 适用性检查
- 定位 rename/类型变更详细信息
- 兼容性分类

**步骤 3：添加/更新模块文档**

在模块 README 中记录新的 alter 规则表面。

**步骤 4：提交**

```bash
git add internal/domain/rule/ddl/common.go internal/domain/rule/ddl/config.go internal/domain/rule/ddl/README.md
git commit -m "refactor: prepare alter semantic rules"
```

### 任务 4：实施类型变更和 rename alter 规则

**文件：**
- 创建：`internal/domain/rule/ddl/alter_semantic_rules.go`
- 创建：`internal/domain/rule/ddl/alter_semantic_rules_test.go`
- 修改：`internal/domain/rule/ddl/register.go`
- 修改：`internal/domain/policy/defaults.go`
- 修改：`internal/domain/policy/README.md`
- 修改：`configs/deltascope.example.yaml`

**步骤 1：编写失败的领域规则测试**

覆盖：
- 不兼容的 modify/change 类型
- 兼容类型变更允许路径
- rename column 禁止
- rename index 禁止

**步骤 2：运行针对性 DDL 测试**

运行：`go test ./internal/domain/rule/ddl -run 'TestAlter.*|TestRegister.*Alter.*' -v`
预期：FAIL。

**步骤 3：实施最小规则**

保持规则聚焦和解析器中立。

**步骤 4：连接默认值和配置模板**

添加新默认值并保持 `configs/deltascope.example.yaml` 与 `config init` 对齐。

**步骤 5：重新运行针对性 DDL 测试**

运行：`go test ./internal/domain/rule/ddl -run 'TestAlter.*|TestRegister.*Alter.*' -v`
预期：PASS。

**步骤 6：提交**

```bash
git add internal/domain/rule/ddl/alter_semantic_rules.go internal/domain/rule/ddl/alter_semantic_rules_test.go internal/domain/rule/ddl/register.go internal/domain/policy/defaults.go internal/domain/policy/README.md configs/deltascope.example.yaml
git commit -m "feat: add alter semantic rules"
```

### 任务 5：在可能的情况下复用 create-table 索引规则用于 alter 添加的索引

**文件：**
- 修改：`internal/domain/rule/ddl/alter_semantic_rules.go`
- 修改：`internal/domain/rule/ddl/alter_semantic_rules_test.go`

**步骤 1：为 alter 添加的索引添加失败的测试**

覆盖：
- 辅助索引前缀
- 索引宽度/键部分数量

**步骤 2：运行针对性测试**

运行：`go test ./internal/domain/rule/ddl -run 'TestAlter.*Index.*' -v`
预期：FAIL。

**步骤 3：实施最小逻辑**

在可能时复用现有索引治理辅助函数，而不是重复规则逻辑。

**步骤 4：重新运行针对性测试**

运行：`go test ./internal/domain/rule/ddl -run 'TestAlter.*Index.*' -v`
预期：PASS。

**步骤 5：提交**

```bash
git add internal/domain/rule/ddl/alter_semantic_rules.go internal/domain/rule/ddl/alter_semantic_rules_test.go
git commit -m "feat: extend alter rules for added indexes"
```

### 任务 6：最终验证和文档

**文件：**
- 修改：`README.md`
- 修改：`docs/plans/2026-03-20-deltascope-v1-decisions.md`
- 修改：`docs/plans/2026-03-20-overnight-handoff.md`
- 修改：`docs/plans/2026-03-20-autonomous-progress.md`

**步骤 1：运行完整验证套件**

运行：
- `go test ./...`
- `diff -u configs/deltascope.example.yaml <(go run ./cmd/deltascope config init)`
- `/Users/fan/.codex/skills/check-three-level-doc/scripts/check_three_level_doc.sh`

预期：PASS。

**步骤 2：更新顶级文档**

记录：
- 更丰富的 alter 语义现在覆盖什么
- 什么被推迟
- 哪些新提交落地

**步骤 3：提交**

```bash
git add README.md docs/plans/2026-03-20-deltascope-v1-decisions.md docs/plans/2026-03-20-overnight-handoff.md docs/plans/2026-03-20-autonomous-progress.md
git commit -m "docs: record rich alter semantics milestone"
```
