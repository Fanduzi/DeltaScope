# 可解释审计结果实施计划

> **给 Claude：** 必须使用 `superpowers:executing-plans` 子技能逐任务执行该计划。

**目标：** 为 DeltaScope 审计 finding 增加一层共享解释层，使 CLI、HTTP 与库使用方在不改变 verdict 语义的前提下，得到更可执行、可自解释的结果。

**架构：** 保持现有离线优先审计引擎与 verdict 流程不变。增加一套通过 `rule_id` 关联的稳定解释模型，将规则目录扩展为解释事实来源，在规则求值之后对 finding 进行增强，并通过 CLI、HTTP 与 `pkg/deltascope` 暴露相同结构化数据。

**技术栈：** Go、现有 application/domain/infrastructure 分层、Cobra CLI、HTTP adapter、`pkg/deltascope`、Markdown 文档、Go testing

---

### 任务 1：保存里程碑规划工件

**文件：**
- 创建： `docs/plans/2026-03-24-explainable-audit-results-design.md`
- 创建： `docs/plans/2026-03-24-explainable-audit-results-implementation.md`
- 创建： `docs/plans/2026-03-24-explainable-audit-results-task-prompts.md`

**步骤1:** 保存已批准的设计文档
**步骤2:** 保存实施计划与任务提示词
**步骤3:** 复查三份文档的命名与范围一致性
**步骤4:** 提交

### 任务 2：将解释模型加入共享结果路径

**文件：**
- 修改： `internal/domain/rule/...`
- 修改： `internal/domain/report/...`
- 修改： `pkg/deltascope/...`
- 修改： 受影响模块 `README.md` 文件
- 测试： 受影响包下聚焦 result/finding 的测试

**步骤1:** 编写失败测试，验证在不改变 verdict 行为的前提下为 finding 附带解释数据
**步骤2:** 增加包含已约定字段的稳定解释模型
**步骤3:** 将该模型接入审计管线与公共包使用的共享结果类型
**步骤4:** 当解释字段缺失时，保持现有结果语义不变
**步骤5:** 运行聚焦测试
**步骤6:** 提交

### 任务 3：将规则目录扩展为解释事实来源

**文件：**
- 修改： `internal/domain/rule/catalog/...`
- 修改： 规则目录相关 README 文件
- 测试： `internal/domain/rule/catalog/...`

**步骤1:** 为解释元数据完整性、查找稳定性与优雅降级编写失败测试
**步骤2:** 扩展目录条目，增加 summary、risk、remediation、config hints 与 metadata-aware notes
**步骤3:** 只在确实提升修复清晰度时加入最有价值的示例
**步骤4:** 保持执行逻辑与目录元数据分离，只通过 `rule_id` 关联
**步骤5:** 运行聚焦测试
**步骤6:** 提交

### 任务 4：增加求值后的解释增强

**文件：**
- 修改： `internal/application/audit/...`
- 修改： finding 增强所需的共享 helper 文件
- 修改： 受影响模块 `README.md` 文件
- 测试： 聚焦 audit service 的测试

**步骤1:** 为求值后增强编写失败测试
**步骤2:** 实现共享的解释增强阶段，通过 `rule_id` 查找目录数据
**步骤3:** 使用 finding 数据、statement kind 与元数据可用性上下文填充解释字段
**步骤4:** 确保缺失的目录数据会退化为最小解释，而不是使审计失败
**步骤5:** 验证 verdict 与 finding 数量保持不变
**步骤6:** 运行聚焦测试
**步骤7:** 提交

### 任务 5：通过 `pkg/deltascope` 暴露结构化解释

**文件：**
- 修改： `pkg/deltascope/audit.go`
- 修改： `pkg/deltascope/README.md`
- 测试： `pkg/deltascope/...`

**步骤1:** 为返回结果中的解释字段编写公共包失败测试
**步骤2:** 通过稳定公共结果类型暴露解释结构
**步骤3:** 在 package README 中用简短示例记录新字段
**步骤4:** 运行聚焦测试
**步骤5:** 提交

### 任务 6：在不破坏紧凑默认输出的前提下增加 CLI 解释输出

**文件：**
- 修改： `internal/interfaces/cli/...`
- 修改： `cmd/deltascope/README.md`
- 修改： `internal/interfaces/cli/README.md`
- 测试： `internal/interfaces/cli/cli_test.go`

**步骤1:** 为详细解释输出与 JSON 暴露编写失败 CLI 测试
**步骤2:** 在保持当前紧凑默认输出的前提下，增加详细/解释导向输出模式
**步骤3:** 清晰渲染 `why`、`risk`、`suggestion` 与 config hints，方便人类阅读
**步骤4:** 在非详细模式下保持 quiet / shell-friendly 行为可预测
**步骤5:** 运行聚焦测试
**步骤6:** 提交

### 任务 7：通过 HTTP 界面暴露结构化解释

**文件：**
- 修改： `internal/interfaces/http/...`
- 修改： `cmd/deltascope-server/README.md`
- 修改： 受影响 HTTP README 文件
- 测试： HTTP handler / response 测试

**步骤1:** 为 `POST /v1/audit` 响应中的解释字段编写失败 HTTP 测试
**步骤2:** 更新响应 shaping，将结构化解释数据包含进去
**步骤3:** 保持现有 verdict、summary 与 statement 语义稳定
**步骤4:** 在相关 HTTP 文档中记录增强后的响应结构
**步骤5:** 运行聚焦测试
**步骤6:** 提交

### 任务 8：为解释增加元数据感知透明性

**文件：**
- 修改： 解释增强代码与相关元数据共享类型
- 修改： 仅在更清晰传递上下文时，调整 `internal/infrastructure/metadata/...` 或共享元数据 helper
- 测试： 元数据感知路径与离线路径测试

**步骤1:** 为离线运行与元数据感知运行中的 `metadata_note` 行为编写失败测试
**步骤2:** 为元数据增强与元数据受限场景增加明确的解释说明
**步骤3:** 当实例事实或表快照不可用时，保持离线路径说明诚实
**步骤4:** 运行聚焦测试
**步骤5:** 提交

### 任务 9：更新面向产品的可解释结果文档

**文件：**
- 修改： `docs/reference/...`
- 修改： `docs/recipe/...`
- 修改： `README.md`
- 修改： `README_ZH.md`
- 修改： 受影响模块 `README.md` 文件

**步骤1:** 增加或更新文档，解释如何理解增强 finding
**步骤2:** 增加 CLI、HTTP 与库的示例，展示解释字段在实践中的样子
**步骤3:** 在适用处保持中英文文档对齐
**步骤4:** 当导出类型或依赖变更时，同步更新模块 README
**步骤5:** 运行链接与内容的 sanity 检查
**步骤6:** 提交

### 任务 10：最终验证与里程碑收口

**文件：**
- 修改： `docs/plans/2026-03-20-overnight-handoff.md`
- 修改： `docs/plans/2026-03-20-autonomous-progress.md`
- 修改： `docs/plans/2026-03-20-deltascope-v1-decisions.md`
- 修改： 所有变更过的模块 `README.md` 文件

**步骤1:** 运行完整验证，包括聚焦包测试、CLI 测试、HTTP 测试，以及更广泛的 `go test ./...` 覆盖
**步骤2:** 验证增强解释不会改变现有用例中的 verdict 结果
**步骤3:** 若当前仓库工作流要求，则运行 three-level-doc 校验
**步骤4:** 用解释里程碑结果更新 handoff/progress/decision 文档
**步骤5:** 提交
**步骤6:** 推送