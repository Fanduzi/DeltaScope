# 丰富 Alter 语义任务提示

> 用于 `Rich Alter Semantics` 里程碑的逐任务实施和审查。
> 每个提示假设工作在 `/Users/fan/GolangProjects/deltascope` 内进行。

## 全局规则

- 遵循 `docs/plans/2026-03-20-rich-alter-semantics-design.md` 中的设计。
- 遵循 `docs/plans/2026-03-20-rich-alter-semantics-implementation.md` 中的实施顺序。
- 保持 DDD 倾向的依赖方向：
  - `interfaces -> application -> domain <- infrastructure`
- 不要将 TiDB 解析器 AST 类型暴露到 `internal/application/audit` 之外。
- 通过仅消费领域 `spec.Statement` 保持所有 alter 规则解析器中立。
- 继续仅在离线模式下支持 MySQL 和 TiDB。
- 保持 `three-level-doc` 作为硬关卡：
  - 更新 L2 模块 README 文件
  - 保持 L3 文件头正确
  - 在声称完成前运行 `check_three_level_doc.sh`
- 每个任务的审查者面向完成必须包括：
  - 更改的文件
  - 运行的测试
  - 状态
  - git 提交哈希

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

将领域 alter 模型扩展到 `Action + Name` 之外。

### 范围

- 添加更丰富的规范化 alter 结构
- 保持模型最小化和领域自有

### 文件

- 修改 `internal/domain/spec/ddl.go`
- 修改 `internal/domain/spec/README.md`

### 验收标准

- `spec.Alter` 可以表示更丰富的 alter 语义
- 新的 alter 相关领域结构/类型是解析器中立的
- 领域 README 反映新形状

### 必需验证

- 运行 `go test ./internal/application/audit -run TestExtractMapsAlterTable -v`

### 审查者返回

- 提交哈希
- 提交消息
- 说明领域模型是否保持精简

## 任务 2 提示

### 目标

在应用层丰富 alter 提取。

### 范围

- 将 alter AST 映射到更丰富的规范化领域 alter 记录
- 覆盖列、索引、rename 和选项变更案例

### 文件

- 修改 `internal/application/audit/extract.go`
- 修改 `internal/application/audit/extract_test.go`
- 修改 `internal/application/audit/README.md`

### 验收标准

- 提取支持代表性的 `ALTER TABLE` 形状
- 没有 TiDB AST 泄漏逃逸出应用层
- 测试覆盖新的 alter 详细信息

### 必需验证

- 运行 `go test ./internal/application/audit -run TestExtractMapsAlterTable -v`

### 审查者返回

- 提交哈希
- 提交消息
- 关于提取边界质量的简短说明

## 任务 3 提示

### 目标

为更丰富的 alter 语义准备 DDL 规则层。

### 范围

- 添加 alter 规则 ID
- 添加共享 alter 辅助函数
- 记录新规则表面

### 文件

- 修改 `internal/domain/rule/ddl/common.go`
- 修改 `internal/domain/rule/ddl/config.go`
- 修改 `internal/domain/rule/ddl/README.md`

### 验收标准

- alter 规则 ID 稳定且命名良好
- 辅助函数支持更丰富的 alter 匹配，无需 AST 访问
- DDL 模块文档描述新的 alter 规则表面

### 必需验证

- 运行 `go test ./internal/domain/rule/ddl -run 'TestRegister.*Alter.*' -v`

### 审查者返回

- 提交哈希
- 提交消息
- 说明辅助函数边界是否看起来可复用

## 任务 4 提示

### 目标

实施第一批语义 alter 规则。

### 范围

- 类型变更规则
- rename 相关规则
- 策略默认值和配置模板更新

### 文件

- 创建 `internal/domain/rule/ddl/alter_semantic_rules.go`
- 创建 `internal/domain/rule/ddl/alter_semantic_rules_test.go`
- 修改 `internal/domain/rule/ddl/register.go`
- 修改 `internal/domain/policy/defaults.go`
- 修改 `internal/domain/policy/README.md`
- 修改 `configs/deltascope.example.yaml`

### 验收标准

- 语义 alter 规则具有稳定的 `rule_id`
- 默认值和示例配置与 `config init` 对齐
- 针对性测试覆盖不兼容 vs 允许的 alter 案例

### 必需验证

- 运行 `go test ./internal/domain/rule/ddl -run 'TestAlter.*|TestRegister.*Alter.*' -v`
- 运行 `diff -u configs/deltascope.example.yaml <(go run ./cmd/deltascope config init)`

### 审查者返回

- 提交哈希
- 提交消息
- 已实现的 alter 规则 ID 列表

## 任务 5 提示

### 目标

扩展 alter 规则以覆盖 alter 添加的索引。

### 范围

- 在可能时复用现有索引治理逻辑
- 审计 alter 添加的辅助/唯一/全文索引

### 文件

- 修改 `internal/domain/rule/ddl/alter_semantic_rules.go`
- 修改 `internal/domain/rule/ddl/alter_semantic_rules_test.go`

### 验收标准

- alter 添加的索引继承相关前缀/宽度检查
- 没有不必要地复制重复的 create-table 规则逻辑
- 针对性测试覆盖添加的行为

### 必需验证

- 运行 `go test ./internal/domain/rule/ddl -run 'TestAlter.*Index.*' -v`

### 审查者返回

- 提交哈希
- 提交消息
- 说明逻辑复用是否保持干净

## 任务 6 提示

### 目标

用完整验证和文档关闭里程碑。

### 范围

- 运行完整验证
- 更新顶级项目文档和交接文档

### 文件

- 修改 `README.md`
- 修改 `docs/plans/2026-03-20-deltascope-v1-decisions.md`
- 修改 `docs/plans/2026-03-20-overnight-handoff.md`
- 修改 `docs/plans/2026-03-20-autonomous-progress.md`

### 验收标准

- 完整测试套件通过
- 配置示例与 `config init` 匹配
- 三级文档检查通过
- 文档反映更丰富的 alter 语义和剩余差距

### 必需验证

- 运行 `go test ./...`
- 运行 `diff -u configs/deltascope.example.yaml <(go run ./cmd/deltascope config init)`
- 运行 `/Users/fan/.codex/skills/check-three-level-doc/scripts/check_three_level_doc.sh`

### 审查者返回

- 提交哈希
- 提交消息
- 说明里程碑是否文档完整
