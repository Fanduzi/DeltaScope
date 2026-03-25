# DDL Alter 批次实施计划

**目标：** 通过强制可配置的 action 级限制，添加下一个离线安全的 `ALTER TABLE` 规则切片。

**架构：** 重用现有的规范化 `spec.Alter` 操作，将规则保持在 `internal/domain/rule/ddl` 内，避免新的解析器泄漏或在线元数据依赖。

**技术栈：** Go, TiDB 解析器 AST 提取, Go testing

---

### 任务 1：添加 alter 限制规则

**文件：**
- 修改：`internal/domain/policy/defaults.go`
- 修改：`configs/deltascope.example.yaml`
- 修改：`internal/domain/policy/README.md`
- 修改：`internal/domain/rule/ddl/common.go`
- 修改：`internal/domain/rule/ddl/register.go`
- 修改：`internal/domain/rule/ddl/register_test.go`
- 修改：`internal/domain/rule/ddl/README.md`
- 创建：`internal/domain/rule/ddl/alter_rules.go`
- 创建：`internal/domain/rule/ddl/alter_rules_test.go`

**验收：**
- 添加规则 ID：
  - `ddl.alter.drop_column.forbid`
  - `ddl.alter.drop_primary_key.forbid`
  - `ddl.alter.drop_index.forbid`
  - `ddl.alter.rename_table.forbid`
  - `ddl.alter.rename_column.forbid`
  - `ddl.alter.change_column.forbid`
  - `ddl.alter.modify_column.forbid`
- 默认值和示例配置与 `config init` 保持一致
- 确定性注册测试包含 alter 规则

### 任务 2：验证并记录批次进度

**文件：**
- 修改：`README.md`
- 修改：`docs/plans/2026-03-20-deltascope-v1-decisions.md`
- 修改：`docs/plans/2026-03-20-overnight-handoff.md`

**验收：**
- 完整测试套件通过
- 三级文档检查通过
- 根 README 提到新的 alter 覆盖
- 决策日志记录为什么 action 级 alter 规则在更丰富的 alter 建模之前落地
