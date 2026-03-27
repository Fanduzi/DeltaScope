# MCP Onboarding 与 Distribution 实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 交付完整的 DeltaScope MCP onboarding 与 distribution 体验，包括 npm launcher、主流客户端接入文档，以及让 `deltascope-mcp` 更像一等 MCP 产品的手动配置说明。

**Architecture:** 保持 `deltascope-mcp` 作为规范 Go stdio server，新增加一层极薄的 npm launcher，用于下载并缓存 release 二进制；同时重组文档，让 README 只保留 quick start，并由一份专门的 MCP onboarding guide 负责主流客户端与手动配置路径。

**Tech Stack:** Go release artifacts, GoReleaser, GitHub Releases, npm package tooling, Node.js launcher code, Markdown docs, shell verification, MCP client configuration examples

---

### Task 1: 保存规划文档

**Files:**
- Create: `docs/plans/2026-03-27-mcp-onboarding-and-distribution-design.md`
- Create: `docs/plans/2026-03-27-mcp-onboarding-and-distribution-implementation.md`
- Create: `docs/plans/2026-03-27-mcp-onboarding-and-distribution-task-prompts.md`
- Create: `docs/plans_zh/2026-03-27-mcp-onboarding-and-distribution-design.md`
- Create: `docs/plans_zh/2026-03-27-mcp-onboarding-and-distribution-implementation.md`
- Create: `docs/plans_zh/2026-03-27-mcp-onboarding-and-distribution-task-prompts.md`

- [ ] **Step 1: 保存确认后的中英双语规划集**

将 6 份文档以统一命名和统一范围落盘。

- [ ] **Step 2: 复查规划集是否漂移**

确认所有文档都围绕同一里程碑：npm launcher、主流 MCP onboarding、手动配置支持。

- [ ] **Step 3: 提交规划文档**

Run:

```bash
git add docs/plans/2026-03-27-mcp-onboarding-and-distribution-*.md docs/plans_zh/2026-03-27-mcp-onboarding-and-distribution-*.md
git commit -m "docs: plan MCP onboarding distribution milestone"
```

Expected: 只包含这 6 份新规划文档的一个 commit。

### Task 2: 创建 npm launcher 包骨架

**Files:**
- Create: `packages/deltascope-mcp/package.json`
- Create: `packages/deltascope-mcp/README.md`
- Create: `packages/deltascope-mcp/bin/deltascope-mcp.js`
- Create: `packages/deltascope-mcp/lib/...`
- Modify: 受新 package 树影响的根文档或模块 README
- Test: launcher smoke tests

- [ ] **Step 1: 先写失败的 launcher smoke test**

定义一个测试，证明 launcher 可以解析 release asset URL、下载二进制到 cache，并通过 stdio 启动它。

- [ ] **Step 2: 建立 package scaffold**

创建名为 `@fanduzi/deltascope-mcp` 的包，并暴露一个可执行入口。

- [ ] **Step 3: 实现最小命令透传**

让 launcher 只承担 bootstrap、cache 与 process execution，不承担 MCP 语义。

- [ ] **Step 4: 运行聚焦测试**

Run:

```bash
npm test --prefix packages/deltascope-mcp
```

Expected: launcher bootstrap 测试通过。

- [ ] **Step 5: 提交**

```bash
git add packages/deltascope-mcp
git commit -m "feat: add npm MCP launcher scaffold"
```

### Task 3: 实现 release 解析、下载与缓存

**Files:**
- Modify: `packages/deltascope-mcp/bin/deltascope-mcp.js`
- Create/Modify: `packages/deltascope-mcp/lib/platform.js`
- Create/Modify: `packages/deltascope-mcp/lib/releases.js`
- Create/Modify: `packages/deltascope-mcp/lib/cache.js`
- Test: launcher resolution tests

- [ ] **Step 1: 先写失败的版本与平台解析测试**

覆盖：

- 默认 latest release 解析
- 显式版本覆盖
- darwin/linux 与 amd64/arm64 的 asset naming
- cache hit 与 cache miss

- [ ] **Step 2: 实现平台标准化**

将 Node 侧宿主机信息映射到 DeltaScope release asset 命名合同。

- [ ] **Step 3: 实现 release 下载与解压**

从 GitHub Releases 下载匹配 archive，并把其中的 `deltascope-mcp` 解压到确定性的 cache 路径。

- [ ] **Step 4: 实现 cache 复用**

确保重复执行时不会在版本未变的情况下重复下载。

- [ ] **Step 5: 运行聚焦测试**

Run:

```bash
npm test --prefix packages/deltascope-mcp
```

Expected: release 解析与 cache 测试通过。

- [ ] **Step 6: 提交**

```bash
git add packages/deltascope-mcp
git commit -m "feat: implement MCP launcher download cache"
```

### Task 4: 增加 launcher 执行与 stdio bridging 验证

**Files:**
- Modify: `packages/deltascope-mcp/bin/deltascope-mcp.js`
- Modify/Create: launcher test files
- Test: launcher end-to-end execution test

- [ ] **Step 1: 先写失败的 launcher execution test**

通过 mock 或 fixture 的方式验证 launcher 能透明转发 stdio 与 exit status。

- [ ] **Step 2: 实现进程启动**

以保留 MCP stdio 行为的方式启动 cache 中的原生二进制。

- [ ] **Step 3: 增加参数透传**

确保 launcher 可以把 `-connections-path` 等 flag 继续传给原生二进制。

- [ ] **Step 4: 运行聚焦测试**

Run:

```bash
npm test --prefix packages/deltascope-mcp
```

Expected: stdio bridging 测试通过。

- [ ] **Step 5: 提交**

```bash
git add packages/deltascope-mcp
git commit -m "feat: bridge stdio through npm MCP launcher"
```

### Task 5: 接入 package publish 与 release 文档

**Files:**
- Modify: release docs
- Modify/Create: package publish workflow/config if needed
- Modify: root README 的 release/install 段落
- Modify: package README
- Test: publish-config sanity checks

- [ ] **Step 1: 定义 package publish contract**

写清 package 名称、与 DeltaScope release tag 的版本关系，以及是否一一镜像。

- [ ] **Step 2: 增加 publish/config 文件**

只增加发布 launcher 所需的最小 package metadata 与 workflow wiring。

- [ ] **Step 3: 明确 release contract**

文档里必须说明 npm 包只是 GitHub Release binaries 的 bootstrap 层，不是第二套 MCP 实现。

- [ ] **Step 4: 运行 sanity check**

Run:

```bash
npm pack --dry-run --prefix packages/deltascope-mcp
```

Expected: package 只打入预期的 launcher 文件。

- [ ] **Step 5: 提交**

```bash
git add packages/deltascope-mcp README.md docs
git commit -m "build: define MCP launcher publish contract"
```

### Task 6: 新增专门的 MCP onboarding guide

**Files:**
- Create: `docs/recipe/use-deltascope-mcp.md`
- Create: `docs/recipe/use-deltascope-mcp.zh-CN.md`
- Modify: `docs/recipe/README.md` if needed
- Modify: 现有 MCP 相关文档中的 cross-link

- [ ] **Step 1: 起草英文 guide**

覆盖：

- DeltaScope MCP 是什么
- 推荐 `npx` 路径
- 原生 binary fallback
- Claude Code
- Codex
- 通用 stdio TOML/JSON config
- direct connection
- `connection_ref`
- 最小 `connections.yaml`
- 常见失败

- [ ] **Step 2: 起草中文 guide**

保持与英文 guide 相同的结构和示例。

- [ ] **Step 3: 增加可直接复制的接入片段**

至少包括：

```bash
claude mcp add --scope user deltascope -- npx -y @fanduzi/deltascope-mcp
codex mcp add deltascope -- npx -y @fanduzi/deltascope-mcp
```

以及通用 stdio 配置示例。

- [ ] **Step 4: 做内容一致性复查**

确认 guide 反映的是真实 MCP contract 与 `connection_ref` 行为。

- [ ] **Step 5: 提交**

```bash
git add docs/recipe/use-deltascope-mcp.md docs/recipe/use-deltascope-mcp.zh-CN.md docs/recipe
git commit -m "docs: add dedicated MCP onboarding guide"
```

### Task 7: 重组 README 与现有 MCP 引用

**Files:**
- Modify: `README.md`
- Modify: `README_ZH.md`
- Modify: `docs/recipe/use-with-ai-agents.md`
- Modify: `docs/recipe/use-with-ai-agents.zh-CN.md`
- Modify: `cmd/deltascope-mcp/README.md`

- [ ] **Step 1: 先写失败的 doc checklist**

列出 README 必须满足的结果：

- README 含有简洁的 MCP quick start
- README 链接到专门的 MCP guide
- agent recipe 通过互链减负，而不是继续堆 onboarding 内容
- `cmd/deltascope-mcp/README.md` 继续聚焦命令本身

- [ ] **Step 2: 在根 README 中加入 MCP quick start**

保持足够短、足够可复制。

- [ ] **Step 3: 精简并重链接现有 MCP 文本**

让 `use-with-ai-agents` 聚焦 agent workflow 语义，而不是继续复制整套 onboarding。

- [ ] **Step 4: 运行文档 sanity check**

对照阅读中英文 README 的 MCP 段落，确认入口命令一致。

- [ ] **Step 5: 提交**

```bash
git add README.md README_ZH.md docs/recipe/use-with-ai-agents.md docs/recipe/use-with-ai-agents.zh-CN.md cmd/deltascope-mcp/README.md
git commit -m "docs: streamline MCP quick start entrypoints"
```

### Task 8: 增加 launcher 与 onboarding contract 的验证

**Files:**
- Modify/Create: launcher tests
- Modify/Create: docs validation notes
- Possibly Modify: CI workflows if launcher verification should run automatically

- [ ] **Step 1: 定义验证矩阵**

覆盖：

- launcher 单元测试
- package dry run
- 原生 Go 测试套件仍为绿
- 文档中的命令和 flag 与真实实现一致

- [ ] **Step 2: 补齐缺失的自动化检查**

只加维持 launcher 发布路径可信所需的最小 CI 或本地验证命令。

- [ ] **Step 3: 跑组合验证**

Run:

```bash
npm test --prefix packages/deltascope-mcp
npm pack --dry-run --prefix packages/deltascope-mcp
go test ./...
```

Expected: 都通过，且不改变既有 MCP server contract。

- [ ] **Step 4: 提交**

```bash
git add packages/deltascope-mcp .github docs
git commit -m "test: verify MCP launcher onboarding contract"
```

### Task 9: 最终复查与 release-readiness handoff

**Files:**
- Modify: release notes 或后续里程碑说明（如有必要）
- Modify: 受影响的 docs index / README
- Modify: 剩余 MCP onboarding 引用

- [ ] **Step 1: 按 design 重新对照结果**

确认交付已覆盖：

- npm launcher
- Claude Code
- Codex
- 通用 MCP clients
- 手动 native binary setup
- direct connection
- `connection_ref`

- [ ] **Step 2: 运行最终验证**

Run:

```bash
npm test --prefix packages/deltascope-mcp
npm pack --dry-run --prefix packages/deltascope-mcp
go test ./...
```

Expected: launcher 与原生 server 最终均为绿色状态。

- [ ] **Step 3: 准备 release follow-up**

记录下一个 release 必须包含的内容：launcher publish、更新后的 README、专门的 MCP guide、以及已验证的 onboarding snippets。

- [ ] **Step 4: 提交**

```bash
git add .
git commit -m "docs: close MCP onboarding distribution milestone"
```
