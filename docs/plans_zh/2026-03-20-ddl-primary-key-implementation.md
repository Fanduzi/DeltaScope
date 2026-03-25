# DDL 主键语义实施计划

**目标：** 添加 bigint、unsigned、auto-increment 和 not-null 要求的主键语义规则。

**架构：** 稍微丰富规范化列元数据，然后将语义规则评估保持在 `internal/domain/rule/ddl` 内。

**技术栈：** Go, TiDB 解析器 AST 提取, Go testing

---

### 任务 1：丰富提取的列元数据

**文件：**
- 修改：`internal/domain/spec/ddl.go`
- 修改：`internal/domain/spec/README.md`
- 修改：`internal/application/audit/extract.go`
- 修改：`internal/application/audit/extract_test.go`
- 修改：`internal/application/audit/README.md`

**验收：**
- 列提取捕获 `Unsigned` 和 `AutoIncrement`
- 提取测试覆盖两个标志

### 任务 2：添加主键语义规则

**文件：**
- 修改：`internal/domain/policy/defaults.go`
- 修改：`configs/deltascope.example.yaml`
- 修改：`internal/domain/policy/README.md`
- 修改：`internal/domain/rule/ddl/common.go`
- 修改：`internal/domain/rule/ddl/register.go`
- 修改：`internal/domain/rule/ddl/register_test.go`
- 修改：`internal/domain/rule/ddl/README.md`
- 创建：`internal/domain/rule/ddl/primary_key_semantic_rules.go`
- 创建：`internal/domain/rule/ddl/primary_key_semantic_rules_test.go`

**验收：**
- 添加规则 ID：
  - `ddl.table.primary_key.bigint.require`
  - `ddl.table.primary_key.unsigned.require`
  - `ddl.table.primary_key.auto_increment.require`
  - `ddl.table.primary_key.not_null.require`
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
- README 和交接反映更强的 PK 语义
