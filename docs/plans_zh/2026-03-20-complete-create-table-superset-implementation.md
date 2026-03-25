# 完成 Create-Table 超集实施计划

> **给 Claude 的提示：** 必需子技能：使用 superpowers:executing-plans 逐任务实施此计划。

**目标：** 完成离线 `CREATE TABLE` 规则表面，使其在 create-table 覆盖上明显超过 `gAudit`。

**架构：** 保持 create-table 提取在 `internal/application/audit`，仅扩展即将到来的规则已经需要的解析器中立事实，并在 `internal/domain/rule/ddl` 内添加剩余的广度聚焦规则族。

**技术栈：** Go, TiDB 解析器 AST, Cobra/Viper 配置流程, Go testing

---

### 任务 1：审计剩余 create-table 差距并确定确切规则 ID

**文件：**
- 修改：`internal/domain/rule/ddl/common.go`
- 修改：`internal/domain/rule/ddl/README.md`
- 修改：`docs/plans/2026-03-20-deltascope-v1-decisions.md`

**步骤 1：添加剩余 create-table 规则 ID**

为选择的差距封闭规则确定稳定 ID。

**步骤 2：添加/更新文档**

记录哪些 create-table 族在此里程碑范围内。

**步骤 3：提交**

```bash
git add internal/domain/rule/ddl/common.go internal/domain/rule/ddl/README.md docs/plans/2026-03-20-deltascope-v1-decisions.md
git commit -m "docs: pin create-table superset rule surface"
```

### 任务 2：添加标识符和关键字治理

**文件：**
- 修改：`internal/application/audit/extract.go`
- 修改：`internal/application/audit/extract_test.go`
- 创建：`internal/domain/rule/ddl/identifier_rules.go`
- 创建：`internal/domain/rule/ddl/identifier_rules_test.go`
- 修改：`internal/domain/rule/ddl/register.go`

**步骤 1：编写失败的测试**

覆盖：
- 无效标识符字符
- 保留关键字名称
- 表/列/索引命名边缘案例

**步骤 2：运行针对性测试**

运行：`go test ./internal/domain/rule/ddl -run 'Test.*Identifier.*|Test.*Keyword.*' -v`
预期：FAIL。

**步骤 3：实施最小提取/规则**

仅保留标识符检查需要的事实。

**步骤 4：重新运行测试**

预期：PASS。

**步骤 5：提交**

```bash
git add internal/application/audit/extract.go internal/application/audit/extract_test.go internal/domain/rule/ddl/identifier_rules.go internal/domain/rule/ddl/identifier_rules_test.go internal/domain/rule/ddl/register.go
git commit -m "feat: add create-table identifier governance"
```

### 任务 3：添加更广泛的类型族和字符集/排序规则

**文件：**
- 修改：`internal/application/audit/extract.go`
- 修改：`internal/application/audit/extract_test.go`
- 创建：`internal/domain/rule/ddl/type_family_rules.go`
- 创建：`internal/domain/rule/ddl/type_family_rules_test.go`
- 修改：`internal/domain/rule/ddl/register.go`
- 修改：`internal/domain/policy/defaults.go`
- 修改：`internal/domain/policy/README.md`
- 修改：`configs/deltascope.example.yaml`

**步骤 1：编写失败的测试**

覆盖：
- blob/json/bit/timestamp 族策略
- char-vs-varchar 指导
- 字符集/排序规则限制（可用时）

**步骤 2：运行针对性测试**

运行：`go test ./internal/domain/rule/ddl -run 'Test.*(Type|Charset|Collation).*' -v`
预期：FAIL。

**步骤 3：实施最小规则**

保持语义诚实和离线安全。

**步骤 4：对齐策略/配置**

更新默认值和示例配置。

**步骤 5：重新运行测试**

预期：PASS。

**步骤 6：提交**

```bash
git add internal/application/audit/extract.go internal/application/audit/extract_test.go internal/domain/rule/ddl/type_family_rules.go internal/domain/rule/ddl/type_family_rules_test.go internal/domain/rule/ddl/register.go internal/domain/policy/defaults.go internal/domain/policy/README.md configs/deltascope.example.yaml
git commit -m "feat: add create-table type-family governance"
```

### 任务 4：深化冗余索引分析

**文件：**
- 修改：`internal/domain/rule/ddl/index_rules.go`
- 修改：`internal/domain/rule/ddl/index_rules_test.go`

**步骤 1：编写失败的测试**

覆盖：
- 精确重复索引
- 左前缀冗余索引
- 值得标记为离线的唯一与辅助重叠案例

**步骤 2：运行针对性测试**

运行：`go test ./internal/domain/rule/ddl -run 'Test.*Index.*' -v`
预期：FAIL。

**步骤 3：实施最小逻辑**

扩展现有索引治理而不是替换它。

**步骤 4：重新运行测试**

预期：PASS。

**步骤 5：提交**

```bash
git add internal/domain/rule/ddl/index_rules.go internal/domain/rule/ddl/index_rules_test.go
git commit -m "feat: deepen create-table redundant index checks"
```

### 任务 5：封闭剩余 create-table 对象形状差距

**文件：**
- 修改：`internal/domain/rule/ddl/table_option_rules.go`
- 修改：`internal/domain/rule/ddl/table_option_rules_test.go`
- 修改：`internal/domain/policy/defaults.go`
- 修改：`configs/deltascope.example.yaml`

**步骤 1：编写失败的测试**

覆盖选择的剩余对象形状/表选项规则。

**步骤 2：运行针对性测试**

运行：`go test ./internal/domain/rule/ddl -run 'Test.*(TableOption|ObjectShape).*' -v`
预期：FAIL。

**步骤 3：实施最小逻辑**

保持规则集中在离线安全检查上。

**步骤 4：重新运行测试**

预期：PASS。

**步骤 5：提交**

```bash
git add internal/domain/rule/ddl/table_option_rules.go internal/domain/rule/ddl/table_option_rules_test.go internal/domain/policy/defaults.go configs/deltascope.example.yaml
git commit -m "feat: close create-table object-shape gaps"
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

记录 create-table 覆盖现在相对于 `gAudit` 是 create-table 超集线，以及任何仍然开放的 DDL 差距。

**步骤 3：提交**

```bash
git add README.md docs/plans/2026-03-20-deltascope-v1-decisions.md docs/plans/2026-03-20-overnight-handoff.md docs/plans/2026-03-20-autonomous-progress.md
git commit -m "docs: close create-table superset milestone"
```
