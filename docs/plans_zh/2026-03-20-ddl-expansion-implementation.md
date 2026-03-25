# DDL 扩展实施计划

> **给 Claude 的提示：** 必需子技能：使用 superpowers:executing-plans 逐任务实施此计划。

**目标：** 在保持当前离线、解析器中立规则架构的同时，添加列和审计列治理的第二批 DDL 规则。

**架构：** 仅用新规则所需的语义扩展 `spec.Column`，在 `internal/application/audit` 中丰富提取，然后添加仅在 `spec.Statement` 上操作的独立 DDL 规则。

**技术栈：** Go, TiDB 解析器 AST 提取, Go testing

---

### 任务 1：扩展列提取语义

**文件：**
- 修改：`internal/domain/spec/ddl.go`
- 修改：`internal/domain/spec/README.md`
- 修改：`internal/application/audit/extract.go`
- 修改：`internal/application/audit/extract_test.go`
- 修改：`internal/application/audit/README.md`

**步骤 1：编写失败的提取器断言**

添加覆盖：
- varchar 长度提取
- not-null 提取
- 默认存在/值提取
- current-timestamp 默认提取
- on-update current-timestamp 提取

**步骤 2：运行针对性测试**

运行：`go test ./internal/application/audit -run TestExtract -v`
预期：FAIL。

**步骤 3：实现最小提取**

从 TiDB 列选项填充新的 `spec.Column` 字段，不将 AST 泄漏到领域模型。

**步骤 4：重新运行测试**

运行：`go test ./internal/application/audit -run TestExtract -v`
预期：PASS。

**步骤 5：提交**

```bash
git add .
git commit -m "feat: enrich ddl column extraction"
```

### 任务 2：添加列和审计列 DDL 规则

**文件：**
- 修改：`internal/domain/policy/defaults.go`
- 修改：`configs/deltascope.example.yaml`
- 修改：`internal/domain/policy/README.md`
- 修改：`internal/domain/rule/ddl/common.go`
- 修改：`internal/domain/rule/ddl/register.go`
- 修改：`internal/domain/rule/ddl/README.md`
- 创建：`internal/domain/rule/ddl/column_rules.go`
- 创建：`internal/domain/rule/ddl/column_rules_test.go`
- 创建：`internal/domain/rule/ddl/audit_column_rules.go`
- 创建：`internal/domain/rule/ddl/audit_column_rules_test.go`
- 修改：`internal/domain/rule/ddl/register_test.go`

**步骤 1：编写失败的规则测试**

覆盖：
- 空列列表
- 缺少审计时间戳对
- 缺少列注释
- 列名过长
- varchar 长度过大
- 缺少默认值
- 可空但不允许的列
- float/double 列

**步骤 2：运行针对性测试**

运行：`go test ./internal/domain/rule/ddl/... -v`
预期：FAIL。

**步骤 3：实施规则批次**

每个关注点组一个文件，保持稳定的 `rule_id`。

**步骤 4：连接策略默认值和示例配置**

为新规则添加默认级别和参数。

**步骤 5：重新运行测试**

运行：`go test ./internal/domain/rule/ddl/... -v`
预期：PASS。

**步骤 6：提交**

```bash
git add .
git commit -m "feat: add ddl column governance rules"
```

### 任务 3：验证并记录新 DDL 批次

**文件：**
- 修改：`docs/plans/2026-03-20-deltascope-v1-decisions.md`
- 修改：`docs/plans/2026-03-20-overnight-handoff.md`

**步骤 1：运行完整验证**

运行：`go test ./...`
预期：PASS。

**步骤 2：更新决策和交接文档**

记录：
- 新 DDL 批次覆盖了什么
- 什么被有意推迟

**步骤 3：提交**

```bash
git add .
git commit -m "docs: record second ddl batch progress"
```
