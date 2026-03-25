# 发布就绪实施计划

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**目标：** 重塑 DeltaScope 的文档与发布界面，使项目能够发布一个成熟的 `v0.6.0` 版本。

**架构：** 保持唯一可信的发布路径与唯一面向产品的文档层次。先修正文档结构与落地页，再围绕同一套产物契约接入发布 workflow、安装脚本与构建入口。

**技术栈：** Go、Cobra CLI、GitHub Actions、shell 安装脚本、Markdown 文档、Makefile

---

### 任务 1：创建产品文档骨架

**Files:**
- Create: `docs/admin/README.md`
- Create: `docs/concept/README.md`
- Create: `docs/dev/README.md`
- Create: `docs/recipe/README.md`
- Create: `docs/reference/README.md`
- Modify: `README.md`
- Modify: `README_ZH.md`

**Step 1: 在脑中写出失败检查**

当前失败点：
- 文档没有按面向产品的区块分组
- README 没有稳定链接到 `admin/concept/dev/recipe/reference`

**Step 2: 增加目录骨架**

为每个新文档区块创建简短 README / index 文件，使树结构可以立刻导航。

**Step 3: 更新顶层 README**

增加文档导航区块，链接到新目录，但此时还不重写整个首页。

**Step 4: Verify**

Run:

```bash
rg -n "docs/(admin|concept|dev|recipe|reference)" README.md README_ZH.md
```

Expected:
- 两个落地页中都存在对应链接

**Step 5: Commit**

```bash
git add README.md README_ZH.md docs/admin/README.md docs/concept/README.md docs/dev/README.md docs/recipe/README.md docs/reference/README.md
git commit -m "docs: add product docs structure"
```

### 任务 2：将能力矩阵迁入参考文档

**Files:**
- Create: `docs/reference/audit-capability-matrix.md`
- Modify: `docs/reference/README.md`
- Modify: `README.md`
- Modify: `README_ZH.md`

**Step 1: 复制规范能力矩阵**

将稳定内容从 `docs/plans/2026-03-21-audit-capability-matrix.md` 移到 `docs/reference/audit-capability-matrix.md`。

**Step 2: 重写链接**

让产品文档指向新的参考位置。

**Step 3: Verify**

Run:

```bash
rg -n "audit-capability-matrix" README.md README_ZH.md docs/reference
```

Expected:
- 产品文档指向 `docs/reference/audit-capability-matrix.md`

**Step 4: Commit**

```bash
git add docs/reference/audit-capability-matrix.md docs/reference/README.md README.md README_ZH.md
git commit -m "docs: move capability matrix into reference docs"
```

### 任务 3：增加架构与概念文档

**Files:**
- Create: `docs/concept/architecture.md`
- Create: `docs/concept/core-concepts.md`
- Create: `docs/concept/metadata-aware-mode.md`
- Create: `docs/dev/architecture.md`
- Modify: `docs/concept/README.md`
- Modify: `docs/dev/README.md`

**Step 1: 编写产品架构文档**

包含一个高层 ASCII 流程图，覆盖：
- SQL 输入
- policy/config
- parser/extractor
- 可选元数据增强
- rule evaluation
- CLI / HTTP / library outputs

**Step 2: 编写实现架构文档**

包含一个 ASCII 分层图，覆盖：
- cmd
- interfaces
- application
- domain
- infrastructure
- pkg

**Step 3: 增加补充概念文档**

保持简洁、面向用户。

**Step 4: Verify**

Run:

```bash
rg -n "ASCII|metadata-aware|rule|verdict" docs/concept docs/dev
```

Expected:
- 文档存在，并包含预期概念锚点

**Step 5: Commit**

```bash
git add docs/concept docs/dev
git commit -m "docs: add concept and architecture references"
```

### 任务 4：增加 Recipe 与 Reference 文档

**Files:**
- Create: `docs/recipe/audit-sql-offline.md`
- Create: `docs/recipe/audit-sql-with-metadata.md`
- Create: `docs/recipe/review-ddl-before-migration.md`
- Create: `docs/recipe/guard-dml-in-ci.md`
- Create: `docs/recipe/use-with-ai-agents.md`
- Create: `docs/recipe/inspect-rules-and-config.md`
- Create: `docs/reference/cli.md`
- Create: `docs/reference/config.md`
- Create: `docs/reference/rules.md`
- Create: `docs/reference/http-api.md`
- Modify: `docs/recipe/README.md`
- Modify: `docs/reference/README.md`

**Step 1: 增加 recipe 集合**

每篇 recipe 都应简短、任务导向、以命令为中心。

**Step 2: 增加 reference 集合**

保持为查询型资料，而不是教程风格。

**Step 3: Verify**

Run:

```bash
find docs/recipe docs/reference -maxdepth 1 -type f | sort
```

Expected:
- 计划中的 recipe 与 reference 文件均存在

**Step 4: Commit**

```bash
git add docs/recipe docs/reference
git commit -m "docs: add recipes and product references"
```

### 任务 5：将 README.md 重写为产品落地页

**Files:**
- Modify: `README.md`

**Step 1: 重写上半部分**

采用以下顺序：
- hero + shields
- 简短定位
- install
- quick start
- why DeltaScope
- key features
- recipes
- documentation links

**Step 2: 将 L1 信息保留在后部**

保留 three-level-doc 要求的架构/模块图，但移到文件下半部分。

**Step 3: 替换 `go run` 的首次接触路径**

优先讲安装与已发布二进制，开发导向说明移到 dev 文档。

**Step 4: Verify**

Run:

```bash
sed -n '1,220p' README.md
```

Expected:
- 上半部分以产品为先
- 模块图保留在后部

**Step 5: Commit**

```bash
git add README.md
git commit -m "docs: rewrite english landing page"
```

### 任务 6：将 README_ZH.md 重写为产品落地页

**Files:**
- Modify: `README_ZH.md`

**Step 1: 镜像英文结构**

不要让中文落地页落后于英文。

**Step 2: Verify**

Run:

```bash
sed -n '1,220p' README_ZH.md
```

Expected:
- 中文落地页与英文保持同样的产品优先结构

**Step 3: Commit**

```bash
git add README_ZH.md
git commit -m "docs: rewrite chinese landing page"
```

### 任务 7：增加发布工作流

**Files:**
- Create: `.github/workflows/release.yml`
- Optional Create: `.goreleaser.yml`
- Modify: `CHANGELOG.md`
- Modify: `SECURITY.md`
- Modify: `pkg/deltascope/version.go`

**Step 1: 选择唯一可信路径**

采用基于 tag 的 GitHub Actions workflow，形态参考已被验证的 `BinlogVisualizer` 发布路径。

**Step 2: 实现发布 job**

它应：
- 在 `v*` tag 上触发
- 运行 `go test ./...`
- 如果使用打包配置，则校验它
- 构建 darwin/linux 的 amd64/arm64 压缩包
- 生成 checksums
- 发布 GitHub Release 资产

**Step 3: 设置下一发布版本**

将默认版本更新为 `v0.6.0`。

**Step 4: Verify**

Run:

```bash
sed -n '1,260p' .github/workflows/release.yml
go test ./...
```

Expected:
- workflow 已存在
- 测试仍通过

**Step 5: Commit**

```bash
git add .github/workflows/release.yml .goreleaser.yml CHANGELOG.md SECURITY.md pkg/deltascope/version.go
git commit -m "build: add release workflow"
```

### 任务 8：增加 install.sh 与产物契约文档

**Files:**
- Create: `install.sh`
- Modify: `README.md`
- Modify: `README_ZH.md`
- Modify: `CHANGELOG.md`

**Step 1: 实现安装脚本**

支持：
- OS / arch 检测
- GitHub release 下载
- 压缩包解压
- 安装目标目录选择

**Step 2: 对齐 README 安装说明**

README 与安装脚本必须在以下方面保持一致：
- archive 名称
- 安装的二进制
- 目标目录假设

**Step 3: Verify**

Run:

```bash
sh -n install.sh
rg -n "install.sh|curl|tar.gz|v0.6.0" README.md README_ZH.md CHANGELOG.md install.sh
```

Expected:
- shell 语法检查通过
- 命名一致

**Step 4: Commit**

```bash
git add install.sh README.md README_ZH.md CHANGELOG.md
git commit -m "build: add install script"
```

### 任务 9：扩展 Makefile 以支撑本地操作工作流

**Files:**
- Modify: `Makefile`
- Modify: `docs/dev/testing.md`
- Modify: `README.md`
- Modify: `README_ZH.md`

**Step 1: 增加稳定目标**

至少包括：
- `test`
- `build`
- `build-cli`
- `build-server`
- `test-e2e-cli`
- `test-e2e-cli-mysql`
- `test-e2e-cli-tidb`

**Step 2: 编写文档**

保持目标集合小而清晰。

**Step 3: Verify**

Run:

```bash
make -n test
make -n build
make -n build-cli
make -n build-server
```

Expected:
- 所有目标都能被正确解析

**Step 4: Commit**

```bash
git add Makefile README.md README_ZH.md docs/dev/testing.md
git commit -m "build: expand make targets"
```

### 任务 10：最终验证与里程碑收口

**Files:**
- Modify: `docs/plans/2026-03-20-overnight-handoff.md`
- Modify: `docs/plans/2026-03-20-autonomous-progress.md`
- Modify: `docs/plans/2026-03-20-deltascope-v1-decisions.md`

**Step 1: 运行验证**

Run:

```bash
go test ./...
make -n test
make -n build
make -n test-e2e-cli
sh -n install.sh
/Users/fan/.codex/skills/check-three-level-doc/scripts/check_three_level_doc.sh
```

**Step 2: 更新 handoff 与 decisions**

记录：
- 新文档布局
- 发布路径
- 安装路径
- 目标版本 `v0.6.0`

**Step 3: Commit**

```bash
git add docs/plans/2026-03-20-overnight-handoff.md docs/plans/2026-03-20-autonomous-progress.md docs/plans/2026-03-20-deltascope-v1-decisions.md
git commit -m "docs: close release readiness milestone"
```