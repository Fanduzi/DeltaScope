# 源感知 Alter 检查任务提示

> 用于 `Source-Aware Alter Checks` 里程碑的逐任务实施和审查。
> 每个提示假设工作在 `/Users/fan/GolangProjects/deltascope` 内进行。

## 全局规则

- 遵循 `docs/plans/2026-03-20-source-aware-alter-checks-design.md` 中的设计。
- 遵循 `docs/plans/2026-03-20-source-aware-alter-checks-implementation.md` 中的实施顺序。
- 保持 DDD 倾向的依赖方向：
  - `interfaces -> application -> domain <- infrastructure`
- 不要将 TiDB 解析器 AST 暴露到 `internal/application/audit` 之外。
- 通过仅消费领域 `spec.Statement` 保持所有 alter 规则解析器中立。
- 继续仅在离线模式下支持 MySQL 和 TiDB。
- 保持 `three-level-doc` 作为硬关卡。
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

为源感知变更判断丰富 alter 领域事实。

### 范围

- 仅添加下游规则需要的最小关系感知字段
- 保持模型解析器中立和领域自有

### 文件

- 修改 `internal/domain/spec/ddl.go`
- 修改 `internal/domain/spec/README.md`

### 验收标准

- alter payload 可以描述源感知列变更事实
- 领域模型保持精简且解析器中立
- 模块文档反映新形状

### 必需验证

- 运行 `go test ./internal/application/audit -run TestExtractMapsAlterTable -v`

## 任务 2 提示

### 目标

在应用层提取源感知 alter 事实。

### 范围

- 扩展 alter 提取而不泄漏 TiDB AST
- 覆盖关系感知列变更和 alter-index 细节

### 文件

- 修改 `internal/application/audit/extract.go`
- 修改 `internal/application/audit/extract_test.go`
- 修改 `internal/application/audit/README.md`

### 验收标准

- 代表性 `ALTER TABLE` 形状产生更丰富的解析器中立事实
- 没有 TiDB AST 逃逸出应用层
- 测试覆盖新的提取细节

### 必需验证

- 运行 `go test ./internal/application/audit -run TestExtractMapsAlterTable -v`

## 任务 3 提示

### 目标

为源感知 alter 策略准备 DDL 规则层。

### 范围

- 添加稳定的源感知 alter 规则 ID
- 添加用于变更比较和 alter-index 投影的共享辅助函数
- 记录新辅助函数/规则表面

### 文件

- 修改 `internal/domain/rule/ddl/common.go`
- 修改 `internal/domain/rule/ddl/config.go`
- 修改 `internal/domain/rule/ddl/README.md`

### 验收标准

- 规则 ID 稳定且诚实
- 辅助函数支持源感知 alter 判断，无需 AST 访问
- DDL 模块文档描述新表面

### 必需验证

- 运行 `go test ./internal/domain/rule/ddl -run 'TestRegister.*Alter.*' -v`

## 任务 4 提示

### 目标

实施源感知列 alter 规则。

### 范围

- 源到目标兼容性策略
- 可空性/默认值/unsigned 转换检查
- 默认值和配置模板更新

### 文件

- 修改 `internal/domain/rule/ddl/alter_semantic_rules.go`
- 修改 `internal/domain/rule/ddl/alter_semantic_rules_test.go`
- 修改 `internal/domain/rule/ddl/register.go`
- 修改 `internal/domain/policy/defaults.go`
- 修改 `internal/domain/policy/README.md`
- 修改 `configs/deltascope.example.yaml`

### 验收标准

- 规则 ID 对实际判断的内容保持诚实
- 默认值和示例配置与 `config init` 对齐
- 针对性测试覆盖被阻止 vs 允许的转换

### 必需验证

- 运行 `go test ./internal/domain/rule/ddl -run 'TestAlter.*(Column|Transition|Register).*' -v`
- 运行 `diff -u configs/deltascope.example.yaml <(go run ./cmd/deltascope config init)`

## 任务 5 提示

### 目标

扩展 alter-index 生命周期治理。

### 范围

- 在可能时复用现有索引规则逻辑
- 覆盖 alter 添加宽度/重复检查和支持的 rename/drop 案例

### 文件

- 修改 `internal/domain/rule/ddl/alter_semantic_rules.go`
- 修改 `internal/domain/rule/ddl/alter_semantic_rules_test.go`
- 修改 `internal/domain/rule/ddl/register.go`

### 验收标准

- alter-index 生命周期检查干净地复用现有逻辑
- 没有不必要地复制重复的 create-table 规则体
- 针对性测试覆盖添加的行为

### 必需验证

- 运行 `go test ./internal/domain/rule/ddl -run 'TestAlter.*Index.*' -v`

## 任务 6 提示

### 目标

用完整验证和文档关闭里程碑。

### 范围

- 运行完整验证
- 更新顶级文档和交接文档

### 文件

- 修改 `README.md`
- 修改 `docs/plans/2026-03-20-deltascope-v1-decisions.md`
- 修改 `docs/plans/2026-03-20-overnight-handoff.md`
- 修改 `docs/plans/2026-03-20-autonomous-progress.md`

### 验收标准

- 完整测试套件通过
- 配置示例与 `config init` 匹配
- 三级文档检查通过
- 文档反映新的源感知 alter 行为和剩余 create-table 差距

### 必需验证

- 运行 `go test ./...`
- 运行 `diff -u configs/deltascope.example.yaml <(go run ./cmd/deltascope config init)`
- 运行 `/Users/fan/.codex/skills/check-three-level-doc/scripts/check_three_level_doc.sh`
