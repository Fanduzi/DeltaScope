# 发布就绪任务提示词

## 全局执行规则

- 除非创建了专用执行 worktree，否则在 `main` 上工作。
- 将 three-level-doc 视为硬要求。
- 优先维护唯一可信发布路径；不要创建并行打包逻辑。
- 保持 workflow、install script、README 与 checksums 之间的产物名称一致。
- 除非后续有更晚且明确的版本决策，否则把 `v0.6.0` 视为目标版本。

## Reviewer 返回模板

每个任务级 reviewer 应报告：

- 通过或失败
- 精确的 blocker 或剩余风险
- 检查过的文件
- 检查过的测试/命令
- 被评审的 commit hash

## 任务 1 提示词

在 `docs/admin`、`docs/concept`、`docs/dev`、`docs/recipe` 和 `docs/reference` 下建立初始产品文档骨架。只对 `README.md` 与 `README_ZH.md` 做最小修改，使其能链接到新目录。此时不要重写整个落地页。

验收：
- 五个文档区块都存在
- 两个 README 都有链接
- three-level-doc 仍然成立

## 任务 2 提示词

将审计能力矩阵迁移到 `docs/reference/audit-capability-matrix.md`，并更新链接，使产品文档不再把它视为暂时性计划工件。

验收：
- 矩阵位于 `docs/reference`
- README 指向新位置

## 任务 3 提示词

增加概念与架构文档，包括产品层与实现层两个 ASCII 图。

验收：
- `docs/concept/architecture.md` 存在，并含有产品级 ASCII 图
- `docs/dev/architecture.md` 存在，并含有实现级 ASCII 图
- 补充概念文档存在且已被链接

## 任务 4 提示词

增加首批 recipe 与 reference 文档。Recipe 必须是任务导向并适合 DBA / 开发者工作流；reference 文档必须是稳定的查询资料。

验收：
- recipe 文档覆盖离线审计、元数据感知审计、DDL 迁移前审查、CI 中 DML 防护、AI agent 用法、规则/配置查看
- reference 文档覆盖 CLI、配置、规则与 HTTP API

## 任务 5 提示词

将 `README.md` 重写为产品落地页。保留 L1 模块/架构图，但将其放到靠后位置。将首次接触路径从 `go run` 改为安装优先。

验收：
- 上半部分像产品首页
- 安装先于开发导向命令
- L1 / 模块图内容仍保留在后部

## 任务 6 提示词

重写 `README_ZH.md`，使其与英文落地页在结构与质量上保持镜像。

验收：
- 中文落地页与英文信息架构一致
- 示例与链接保持同步

## 任务 7 提示词

在 `.github/workflows/release.yml` 下增加唯一可信发布工作流，将版本元数据对齐为 `v0.6.0`，并让 CHANGELOG / SECURITY 与新目标版本同步。

验收：
- workflow 由 `v*` tag 驱动
- 测试继续通过
- 默认版本变为 `v0.6.0`

## 任务 8 提示词

增加 `install.sh`，并让它与产物名称及 README 安装文档完全对齐。

验收：
- `install.sh` 通过 `sh -n`
- workflow、install script、README 与 changelog 之间的产物命名一致

## 任务 9 提示词

将 `Makefile` 扩展为一个小而稳定的操作入口面，不要把它变成第二套构建系统。

验收：
- 存在 `test`、`build`、`build-cli`、`build-server` 和 CLI E2E 目标
- README / dev 文档对它们做了简洁说明

## 任务 10 提示词

运行最终验证，更新 handoff / progress / decisions，并完成里程碑收口。

验收：
- 验证命令被记录
- handoff / progress / decision 文档反映 release-ready 状态
- 对唯一可信发布路径不再存在歧义