# 源感知 Alter 检查实施计划

> **给 Claude 的提示：** 必需子技能：使用 superpowers:executing-plans 逐任务实施此计划。

**目标：** 用源感知列变更语义和更强的 alter-index 生命周期检查深化离线 `ALTER TABLE` 审计。

**架构：** 将 TiDB AST 保持在 `internal/application/audit`，在 `internal/domain/spec` 中丰富解析器中立 alter 事实，并通过仅领域辅助函数和规则在 `internal/domain/rule/ddl` 中实施所有判断。

**技术栈：** Go, TiDB 解析器 AST, Cobra/Viper 配置流程, Go testing

---

### 任务 1：为源感知变更判断丰富 alter 领域事实

**文件：**
- 修改：`internal/domain/spec/ddl.go`
- 修改：`internal/domain/spec/README.md`

**步骤 1：在下游提取器/规则测试中添加失败的期望**

期望 alter payload 暴露足够细节用于：
- 源 vs 目标类型族
- 可空性/默认值/unsigned 变更
- rename-plus-change 复合案例

**步骤 2：运行针对性测试**

运行：`go test ./internal/application/audit -run TestExtractMapsAlterTable -v`
预期：FAIL。

**步骤 3：添加最小的领域形状**

仅添加第一个源感知规则批次所需的关系感知 alter 字段。

**步骤 4：重新运行针对性测试**

运行：`go test ./internal/application/audit -run TestExtractMapsAlterTable -v`
预期：直到提取更新，仍 FAIL。

**步骤 5：提交**

```bash
git add internal/domain/spec/ddl.go internal/domain/spec/README.md
git commit -m "refactor: enrich alter change facts"
```

### 任务 2：扩展 alter 提取以获取源感知事实

**文件：**
- 修改：`internal/application/audit/extract.go`
- 修改：`internal/application/audit/extract_test.go`
- 修改：`internal/application/audit/README.md`

**步骤 1：编写失败的提取器案例**

覆盖：
- 带显式目标定义的 modify/change
- rename 加 type/default/nullability 变更
- alter 添加索引宽度案例

**步骤 2：运行针对性提取器测试**

运行：`go test ./internal/application/audit -run TestExtractMapsAlterTable -v`
预期：FAIL。

**步骤 3：实施最小提取**

仅从语句本地数据填充解析器中立变更事实。

**步骤 4：重新运行提取器测试**

运行：`go test ./internal/application/audit -run TestExtractMapsAlterTable -v`
预期：PASS。

**步骤 5：提交**

```bash
git add internal/application/audit/extract.go internal/application/audit/extract_test.go internal/application/audit/README.md
git commit -m "feat: extract source-aware alter facts"
```

### 任务 3：准备源感知 alter 策略的规则辅助函数

**文件：**
- 修改：`internal/domain/rule/ddl/common.go`
- 修改：`internal/domain/rule/ddl/config.go`
- 修改：`internal/domain/rule/ddl/README.md`

**步骤 1：添加新规则 ID**

为以下内容引入稳定 ID：
- 源到目标兼容性
- 可空性/默认值转换检查
- alter 添加索引宽度和重复检查

**步骤 2：添加共享辅助函数**

辅助函数应覆盖：
- 比较源/目标列事实
- 分类转换种类
- 将 alter 添加的索引投影到可复用的 create-table 索引规则输入

**步骤 3：添加/更新文档**

记录新规则表面和辅助函数意图。

**步骤 4：提交**

```bash
git add internal/domain/rule/ddl/common.go internal/domain/rule/ddl/config.go internal/domain/rule/ddl/README.md
git commit -m "refactor: prepare source-aware alter rules"
```

### 任务 4：实施源感知列 alter 规则

**文件：**
- 修改：`internal/domain/rule/ddl/alter_semantic_rules.go`
- 修改：`internal/domain/rule/ddl/alter_semantic_rules_test.go`
- 修改：`internal/domain/rule/ddl/register.go`
- 修改：`internal/domain/policy/defaults.go`
- 修改：`internal/domain/policy/README.md`
- 修改：`configs/deltascope.example.yaml`

**步骤 1：编写失败的规则测试**

覆盖：
- 明显不兼容的源到目标变更
- 安全的同族窄案例
- 可空性/默认值/unsigned 转换

**步骤 2：运行针对性 DDL 测试**

运行：`go test ./internal/domain/rule/ddl -run 'TestAlter.*(Column|Transition|Register).*' -v`
预期：FAIL。

**步骤 3：实施最小规则**

保持离线语义保守且明确。

**步骤 4：对齐默认值和配置模板**

更新默认策略加 `configs/deltascope.example.yaml`。

**步骤 5：重新运行针对性 DDL 测试**

运行：`go test ./internal/domain/rule/ddl -run 'TestAlter.*(Column|Transition|Register).*' -v`
预期：PASS。

**步骤 6：提交**

```bash
git add internal/domain/rule/ddl/alter_semantic_rules.go internal/domain/rule/ddl/alter_semantic_rules_test.go internal/domain/rule/ddl/register.go internal/domain/policy/defaults.go internal/domain/policy/README.md configs/deltascope.example.yaml
git commit -m "feat: add source-aware alter column rules"
```

### 任务 5：扩展 alter-index 生命周期治理

**文件：**
- 修改：`internal/domain/rule/ddl/alter_semantic_rules.go`
- 修改：`internal/domain/rule/ddl/alter_semantic_rules_test.go`
- 修改：`internal/domain/rule/ddl/register.go`

**步骤 1：编写失败的测试**

覆盖：
- alter 添加索引宽度检查
- alter 添加重复索引检查
- 如果现有事实支持则加强 rename/drop index 案例

**步骤 2：运行针对性测试**

运行：`go test ./internal/domain/rule/ddl -run 'TestAlter.*Index.*' -v`
预期：FAIL。

**步骤 3：实施最小逻辑**

在可能时复用现有 create-table 索引规则体。

**步骤 4：重新运行针对性测试**

运行：`go test ./internal/domain/rule/ddl -run 'TestAlter.*Index.*' -v`
预期：PASS。

**步骤 5：提交**

```bash
git add internal/domain/rule/ddl/alter_semantic_rules.go internal/domain/rule/ddl/alter_semantic_rules_test.go internal/domain/rule/ddl/register.go
git commit -m "feat: deepen alter index lifecycle checks"
```

### 任务 6：验证并关闭里程碑

**文件：**
- 修改：`README.md`
- 修改：`docs/plans/2026-03-20-deltascope-v1-decisions.md`
- 修改：`docs/plans/2026-03-20-overnight-handoff.md`
- 修改：`docs/plans/2026-03-20-autonomous-progress.md`

**步骤 1：运行完整验证**

运行：
- `go test ./...`
- `diff -u configs/deltascope.example.yaml <(go run ./cmd/deltascope config init)`
- `/Users/fan/.codex/skills/check-three-level-doc/scripts/check_three_level_doc.sh`

预期：PASS。

**步骤 2：更新文档**

记录：
- 现在存在哪些源感知 alter 检查
- 哪些差距移到了 create-table 超集里程碑
- 关键提交和权衡

**步骤 3：提交**

```bash
git add README.md docs/plans/2026-03-20-deltascope-v1-decisions.md docs/plans/2026-03-20-overnight-handoff.md docs/plans/2026-03-20-autonomous-progress.md
git commit -m "docs: close source-aware alter checks milestone"
```
