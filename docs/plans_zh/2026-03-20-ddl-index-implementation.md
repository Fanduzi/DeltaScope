# DDL 索引批次实施计划

**目标：** 通过丰富索引元数据和实施索引治理规则，添加下一个离线安全的 `CREATE TABLE` DDL 规则切片。

**架构：** 将领域索引模型扩展到足以分类索引种类的程度，丰富应用提取，然后将所有规则逻辑保持在 `internal/domain/rule/ddl` 内。

**技术栈：** Go, TiDB 解析器 AST 提取, Go testing

---

### 任务 1：丰富提取的索引元数据

**文件：**
- 修改：`internal/domain/spec/ddl.go`
- 修改：`internal/domain/spec/README.md`
- 修改：`internal/application/audit/extract.go`
- 修改：`internal/application/audit/extract_test.go`
- 修改：`internal/application/audit/README.md`

**验收：**
- `spec.Index` 携带类型化 `Kind`
- 提取将 create-table 约束映射到稳定的索引种类
- 提取测试覆盖辅助、唯一和全文索引形状

### 任务 2：添加索引治理规则

**文件：**
- 修改：`internal/domain/policy/defaults.go`
- 修改：`configs/deltascope.example.yaml`
- 修改：`internal/domain/policy/README.md`
- 修改：`internal/domain/rule/ddl/common.go`
- 修改：`internal/domain/rule/ddl/register.go`
- 修改：`internal/domain/rule/ddl/register_test.go`
- 修改：`internal/domain/rule/ddl/README.md`
- 创建：`internal/domain/rule/ddl/index_rules.go`
- 创建：`internal/domain/rule/ddl/index_rules_test.go`

**验收：**
- 添加规则 ID：
  - `ddl.index.total.max_count`
  - `ddl.index.columns.max_count`
  - `ddl.index.unique.prefix.require`
  - `ddl.index.secondary.prefix.require`
  - `ddl.index.fulltext.prefix.require`
  - `ddl.index.duplicate.forbid`
- 默认值和示例配置与 `config init` 保持一致
- 确定性注册测试包含新的索引规则

### 任务 3：验证并记录批次进度

**文件：**
- 修改：`docs/plans/2026-03-20-deltascope-v1-decisions.md`
- 修改：`docs/plans/2026-03-20-overnight-handoff.md`

**验收：**
- 完整测试套件通过
- 三级文档检查通过
- 决策日志记录为什么 alter 限制保持推迟
- 交接文档反映已完成的索引批次和剩余的 DDL 差距
