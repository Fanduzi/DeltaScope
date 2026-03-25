# DDL 表选项批次实施计划

**目标：** 为引擎、字符集、注释长度和对象形状限制添加下一个 create-table DDL 切片。

**架构：** 用几个解析器中立的布尔值丰富 create-table 领域形状，然后将规则逻辑保持在 `internal/domain/rule/ddl` 内。

**技术栈：** Go, TiDB 解析器 AST 提取, Go testing

---

### 任务 1：丰富 create-table 形状提取

**文件：**
- 修改：`internal/domain/spec/ddl.go`
- 修改：`internal/domain/spec/README.md`
- 修改：`internal/application/audit/extract.go`
- 修改：`internal/application/audit/extract_test.go`
- 修改：`internal/application/audit/README.md`

**验收：**
- create-table 提取捕获：
  - refer-table / create-like
  - select-backed create-table / create-as
  - 分区存在
- 测试覆盖新的布尔值

### 任务 2：添加表选项/对象形状规则

**文件：**
- 修改：`internal/domain/policy/defaults.go`
- 修改：`configs/deltascope.example.yaml`
- 修改：`internal/domain/policy/README.md`
- 修改：`internal/domain/rule/ddl/common.go`
- 修改：`internal/domain/rule/ddl/register.go`
- 修改：`internal/domain/rule/ddl/register_test.go`
- 修改：`internal/domain/rule/ddl/README.md`
- 创建：`internal/domain/rule/ddl/table_option_rules.go`
- 创建：`internal/domain/rule/ddl/table_option_rules_test.go`

**验收：**
- 添加规则 ID：
  - `ddl.table.comment.max_length`
  - `ddl.table.engine.allowlist`
  - `ddl.table.charset.allowlist`
  - `ddl.table.foreign_key.forbid`
  - `ddl.table.partition.forbid`
  - `ddl.table.create_like.forbid`
  - `ddl.table.create_as.forbid`
- 默认值和示例配置与 `config init` 保持一致
- 注册表集成覆盖新规则

### 任务 3：验证并记录批次进度

**文件：**
- 修改：`README.md`
- 修改：`docs/plans/2026-03-20-deltascope-v1-decisions.md`
- 修改：`docs/plans/2026-03-20-overnight-handoff.md`

**验收：**
- 完整测试套件通过
- 三级文档检查通过
- README 和交接反映新的 create-table 选项覆盖
