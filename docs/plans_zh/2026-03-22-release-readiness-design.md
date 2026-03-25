# 发布就绪设计

## 目标

在当前 `v0.5.0` 基线之后，为 DeltaScope 的第一次成熟公开发布做好准备，具体包括：

- 将文档树重构为面向产品与面向维护者的两类区块
- 将 `README.md` 与 `README_ZH.md` 重写为落地页，而不是内部开发说明
- 增加一条可信且唯一的 GitHub Actions 发布路径
- 增加与已发布产物命名一致的 `install.sh`
- 将 `Makefile` 扩展为一个小而稳定的操作入口面

目标版本为 `v0.6.0`。仓库当前已经发布过 `v0.5.0`，因此版本号不能回退。

## 背景

DeltaScope 目前已经具备：

- 完整的 CLI 界面
- 针对 MySQL 与 TiDB 的元数据感知 live smoke 覆盖
- 一个轻量 HTTP 服务
- 一个稳定的公开 Go 包

但它仍缺少真正达到产品级的发布与文档界面。当前缺口包括：

- `README.md` 与 `README_ZH.md` 仍然更像实施笔记
- 审计能力矩阵还放在 `docs/plans/` 下，这不是长期归宿
- `.github/workflows/` 下尚无发布工作流
- 还没有 `install.sh`
- `Makefile` 目前只暴露元数据 E2E 目标

## 非目标

这个里程碑不包括：

- 增加新的审计规则
- 修改 HTTP 服务行为
- 增加 MCP 支持
- 增加 Homebrew 之类的包管理器分发
- 替换现有 CLI 或 HTTP 契约

## 考虑过的方案

### 方案 A：先发布，再补文档

先立即交付 workflow、压缩包与安装脚本，之后再清理文档。

优点：

- 最快能打 tag

缺点：

- 发布文档与产物命名容易漂移
- 首页仍然低估产品成熟度
- 发布后的支持成本会上升

### 方案 B：文档与发布一起做

先重构文档、重写首页，再基于同一套产物契约接入发布流水线与安装路径。

优点：

- 对外形成一个一致的产品界面
- README / install / release 资产始终一致
- 降低发布后的混乱成本

缺点：

- 里程碑范围略大一些

### 方案 C：一口气做完整打包生态

一次性完成 workflow、安装脚本、包管理器支持、发布说明、文档重组与 CI 扩展。

优点：

- 启动面最强

缺点：

- 单个里程碑范围过大
- 发布路径更容易反复变更

## 推荐

选择方案 B。

它能把里程碑聚焦在一件事上：让 DeltaScope 看起来并表现得像一个可发布产品。同时，它也符合当前需要统一文档、产物命名、安装说明与 GitHub Release 输出的实际需求。

## 设计

### 1. 文档信息架构

在面向产品的文档与内部计划文档之间建立清晰分层。

目标结构：

- `docs/admin/`
- `docs/concept/`
- `docs/dev/`
- `docs/recipe/`
- `docs/reference/`

职责划分：

- `docs/admin/`：发布、路线图、支持、安全扩展
- `docs/concept/`：产品级架构、核心概念、元数据感知模式
- `docs/dev/`：开发流程、测试、实现架构
- `docs/recipe/`：面向 DBA 与开发者工作流的任务型指南
- `docs/reference/`：CLI、HTTP API、配置、规则、能力矩阵等稳定查询文档

`docs/plans/` 继续仅用于内部设计、实施计划、交接与里程碑记录。

### 2. README 策略

`README.md` 与 `README_ZH.md` 应变成产品落地页。

前半部分内容：

- 项目标识与 shields
- 简短定位说明
- 安装
- 快速开始
- 为什么使用 DeltaScope
- 关键特性
- recipes
- 文档索引

后半部分内容：

- 满足 L1 three-level-doc 契约要求的架构图与模块图
- contributing、status、license 链接

这样既保留 L1 必需信息，又不会让它主导用户与项目的第一次接触。

### 3. 架构图

在两个位置使用 ASCII 图：

- `docs/concept/architecture.md`：高层产品工作流
- `docs/dev/architecture.md`：实现分层与依赖方向

README 应链接到这些文档，而不是内联大段图示。

### 4. 能力矩阵迁移

将审计能力矩阵从 `docs/plans/2026-03-21-audit-capability-matrix.md` 迁移到稳定引用位置，理想目标为：

- `docs/reference/audit-capability-matrix.md`

能力矩阵应成为产品/参考工件，而不是一份暂时性的计划附件。

### 5. 可信发布路径

采用唯一发布路径：

`tag -> GitHub Actions release workflow -> tested archives -> checksums -> GitHub Release assets`

以现有 `BinlogVisualizer` workflow 形态为参考：

- 运行 `go test ./...`
- 校验打包配置
- 构建 darwin/linux 的 amd64/arm64 压缩包
- 汇总 checksums
- 在 `v*` tag 上发布 GitHub Release 资产

不要维护多条半重叠的发布路径。

### 6. 安装路径

增加 `install.sh`，它应：

- 解析 OS 与架构
- 从 GitHub Releases 下载匹配的发布包
- 校验并解压该包
- 将 `deltascope`，以及可选的 `deltascope-server`，安装到目标目录

以下几处的产物命名必须完全一致：

- workflow
- install script
- README 安装示例
- checksums

### 7. Makefile 扩展

保持 `Makefile` 小而稳定，暴露常见操作，例如：

- `test`
- `test-e2e-cli`
- `build`
- `build-cli`
- `build-server`
- `install-local`

不要把它变成第二套构建系统。

### 8. 发布版本

下一发布目标版本使用 `v0.6.0`。

理由：

- 仓库历史与公开版本串中已经存在 `v0.5.0`
- 这个里程碑是一次功能/产品界面升级，而不仅仅是 patch
- 升到 `v0.6.0` 可以保持语义顺序，并避免改写历史

## 验收标准

当满足以下条件时，该里程碑完成：

1. 文档树包含新的面向产品目录，并且能力矩阵已从 `docs/plans/` 中迁出。
2. `README.md` 与 `README_ZH.md` 已像产品落地页，同时在后部保留 L1 模块映射要求。
3. `.github/workflows/release.yml` 已存在并定义唯一可信发布路径。
4. `install.sh` 与 workflow 产物名、README 安装说明保持一致。
5. `Makefile` 暴露了约定的常用目标。
6. 发布文档与脚本都一致地以 `v0.6.0` 为目标。
7. 仓库已准备好进行真实的基于 tag 的发布路径验证。