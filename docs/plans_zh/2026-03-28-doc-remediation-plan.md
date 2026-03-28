# 文档修复计划 Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 修复 DeltaScope 文档里影响最大的事实错误，并补齐 CLI、MCP、skill 三个入口的首次成功验证信息。

**Architecture:** 这是一次纯文档修复。先处理被代码直接否定的错误，再统一版本与 release 相关表述，然后修补架构文档里的 MCP 缺口，最后给三个入口补上最小可验证闭环。所有修改都应当小而准，只改确实有问题的文档位置。

**Tech Stack:** Markdown、Go 源码引用、npm package 元数据、shell 安装脚本

---

### Task 1: 修复根 README 的事实错误

**Files:**
- Modify: `README.md:100-139`
- Verify against: `pkg/deltascope/audit.go:114-125`
- Verify against: `internal/interfaces/cli/rules.go:17-61`

**Step 1: 修正 JSON 示例字段名**

把 `README.md` 中 finding 示例里的：

```json
{
  "level": "warning",
  "rule": "ddl.alter.drop.column",
  "message": "dropping column `age` is destructive and cannot be undone"
}
```

改成：

```json
{
  "level": "warning",
  "rule_id": "ddl.alter.drop.column",
  "message": "dropping column `age` is destructive and cannot be undone"
}
```

**Step 2: 修正规则列表命令**

把 `README.md` 中查看规则的示例从：

```bash
deltascope rules
```

改成：

```bash
deltascope rules list
```

**Step 3: 回读修改后的片段**

重新读取 `README.md:100-139`，确认 JSON 示例和命令示例已经与实际代码一致。

**Step 4: 对照源码复核**

重新读取：
- `pkg/deltascope/audit.go:114-125`
- `internal/interfaces/cli/rules.go:17-61`

预期：字段仍为 `rule_id`；命令结构仍为 `rules list`。

**Step 5: Commit**

```bash
git add README.md
git commit -m "docs: fix root readme audit examples"
```

---

### Task 2: 对齐根文档中的版本与 release 表述

**Files:**
- Modify: `README.md:12,25-44`
- Modify: `docs/releases/README.md`
- Verify against: `pkg/deltascope/version.go:8-19`
- Verify against: `packages/deltascope-mcp/package.json:1-34`
- Verify against: `docs/releases/release-notes-v0.9.0.md`
- Verify against: `docs/releases/release-notes-v0.9.1.md`
- Verify against: `docs/releases/release-notes-v0.9.2.md`

**Step 1: 先确定文档里的版本口径**

编辑前，先选择并坚持一种规则：

- **Option A：** 视 `v0.9.2` 为当前版本，并把 README / release 索引 / 模块 README 统一到这个口径
- **Option B：** 尽可能改成版本无关表述，避免在代码元数据尚未完全统一前做过强承诺

不要发明新策略，只能基于当前仓库状态做能自洽的修正。

**Step 2: 更新根 README 的 release-notes 链接**

如果根 README 应指向仓库中最新可见的 release notes，就把它从过时的 `v0.9.1` 链接更新掉。

**Step 3: 处理 release contract 的版本承诺**

如果 `README.md` 声称 npm 包版本与 Go release tag 严格一致，就必须对照：
- `pkg/deltascope/version.go`
- `packages/deltascope-mcp/package.json`

然后二选一：
- 把文案修成严格真实
- 或降低承诺强度，避免当前仓库状态下的过度表述

**Step 4: 更新 release 索引页**

在 `docs/releases/README.md` 中至少补齐：
- `v0.9.0`
- `v0.9.0 中文版`
- `v0.9.1`
- `v0.9.1 中文版`
- `v0.9.2`
- `v0.9.2 中文版`

保留原有更早版本条目。

**Step 5: 回读修改后的文件**

读取：
- `README.md`
- `docs/releases/README.md`

预期：这两个入口文档里不再残留明显过时的“当前版本”说法。

**Step 6: Commit**

```bash
git add README.md docs/releases/README.md
git commit -m "docs: align release references"
```

---

### Task 3: 修复模块 README 中过时的版本说明

**Files:**
- Modify: `cmd/deltascope-server/README.md:11-16`
- Modify: `cmd/deltascope-mcp/README.md:13-20`
- Modify: `pkg/deltascope/README.md:41-47`
- Verify against: `pkg/deltascope/version.go:8-19`

**Step 1: 替换掉过时的 `v0.7.0` 说明**

更新这些模块 README 中仍写着 source build 默认 `v0.7.0` 的地方。

统一采用以下两种方式之一：
- 直接替换成 `pkg/deltascope/version.go` 中的当前默认版本
- 或改写成“默认使用仓库当前 `DefaultVersion`”，降低未来漂移概率

**Step 2: 保持改动范围克制**

不要顺手重写无关段落，只修正已经不准确的版本说明。

**Step 3: 回读三个模块 README**

读取：
- `cmd/deltascope-server/README.md`
- `cmd/deltascope-mcp/README.md`
- `pkg/deltascope/README.md`

预期：除非明确标注为历史背景，否则不应再出现 `v0.7.0` 的当前态描述。

**Step 4: Commit**

```bash
git add cmd/deltascope-server/README.md cmd/deltascope-mcp/README.md pkg/deltascope/README.md
git commit -m "docs: refresh module version notes"
```

---

### Task 4: 在架构文档中把 MCP 补齐

**Files:**
- Modify: `docs/concept/architecture.md:18-65,114-123`
- Modify: `docs/dev/architecture.md:7-47`
- Verify against: `cmd/deltascope-mcp/main.go`
- Verify against: `internal/interfaces/mcp/server.go`

**Step 1: 更新共享审计流程图**

在 `docs/concept/architecture.md` 里，把：

```text
| CLI / HTTP / Library |
```

改成显式包含 MCP 的表达。

同时检查输出/result 部分，确保 MCP 不再被隐式省略。

**Step 2: 更新层级边界说明**

在 `docs/concept/architecture.md` 的 layer boundary 区块中补上：
- `cmd/deltascope-mcp`
- `internal/interfaces/mcp`

**Step 3: 更新实现架构图**

在 `docs/dev/architecture.md` 的 ASCII 图里补上：
- `cmd/deltascope-mcp`
- `internal/interfaces/mcp`

同时把 practical boundaries 中“`internal/interfaces` 只负责 CLI 和 HTTP”之类的说法修正掉。

**Step 4: 回读两份架构文档**

读取：
- `docs/concept/architecture.md`
- `docs/dev/architecture.md`

预期：两份文档都把 MCP 当作当前实现的一部分，而不是遗漏或默认读者自己脑补。

**Step 5: Commit**

```bash
git add docs/concept/architecture.md docs/dev/architecture.md
git commit -m "docs: include mcp in architecture docs"
```

---

### Task 5: 给 CLI 文档补最小成功验证

**Files:**
- Modify: `README.md:17-45`
- Optional verify against: `internal/interfaces/cli/version.go`
- Optional verify against: `cmd/deltascope/main.go`

**Step 1: 在安装段后增加最小验证命令**

在根 README 的安装区增加一个极简验证步骤：

```bash
deltascope version
```

**Step 2: 写明预期结果**

只用一句话说明成功时会发生什么，例如：
- 输出 DeltaScope 语义化版本号并正常退出

不要扩展成复杂 troubleshooting。

**Step 3: 回读安装区**

重新读取 install / quick start 相关段落，确认验证步骤出现在长示例前。

**Step 4: Commit**

```bash
git add README.md
git commit -m "docs: add cli install verification"
```

---

### Task 6: 给 MCP 文档补最小成功验证

**Files:**
- Modify: `README.md:153-169`
- Modify: `docs/recipe/use-deltascope-mcp.md:22-70`
- Verify against: `internal/interfaces/mcp/server.go`

**Step 1: 在根 README 中增加 MCP 成功检查**

在 MCP 注册示例后加一小段说明，告诉用户接入成功后应该看到什么。

内容保持非常具体，例如客户端应显示 DeltaScope server，并暴露这四个工具：
- `audit_sql`
- `describe_rule`
- `list_rules`
- `get_capabilities`

**Step 2: 在 recipe 文档中加入同样的验证段**

在 `docs/recipe/use-deltascope-mcp.md` 中，紧接 launcher/native setup 之后补一个简短的 “Verify setup” 小节。

这个小节只负责告诉用户注册后如何确认 wiring 生效，不讲策略、不讲价值。

**Step 3: 回读 MCP 文档**

读取：
- `README.md`
- `docs/recipe/use-deltascope-mcp.md`

预期：新用户能判断 MCP server 是否真的接好了。

**Step 4: Commit**

```bash
git add README.md docs/recipe/use-deltascope-mcp.md
git commit -m "docs: add mcp setup verification"
```

---

### Task 7: 给 skill 文档补最小成功验证

**Files:**
- Modify: `README.md:171-201`
- Modify: `skills/README.md:37-69`
- Verify against: `skills/deltascope-review/SKILL.md`

**Step 1: 在根 README 中补 skill 成功说明**

在 `/deltascope-review` 示例后加一句最短说明，告诉用户成功时的预期行为，例如：skill 可以接受 SQL 或文件路径并返回 DeltaScope finding。

**Step 2: 在 `skills/README.md` 中增加最小验证步骤**

在 requirement 安装段后加一个短验证，例如：

```bash
deltascope version
```

然后保留现有 `/deltascope-review` 作为第一条端到端成功动作。

**Step 3: 检查平台表述是否一致**

对照：
- `install.sh`
- `packages/deltascope-mcp/lib/releases.js`
- `README.md`

如果根安装/runtime 文档并未正式支持 Windows，就需要把 `skills/README.md` 的 Windows 表述降到不误导的程度，避免造成“平台完全对等支持”的错觉。

**Step 4: 回读 skill 文档**

读取：
- `README.md`
- `skills/README.md`

预期：skill 入口现在有最小成功验证，且平台说明不夸大。

**Step 5: Commit**

```bash
git add README.md skills/README.md
git commit -m "docs: add skill verification guidance"
```

---

### Task 8: 跑一轮文档验证闭环

**Files:**
- Review: 所有在 Task 1-7 中修改过的文件

**Step 1: 全量回读修改后的文档**

重新阅读所有改过的文档，只检查这些失败模式：
- 命令名和代码不一致
- JSON 字段名和代码不一致
- 版本号在文档间互相冲突
- MCP 在架构文档中仍被遗漏
- 验证步骤缺失或写得太虚

**Step 2: 跑针对性 grep 检查**

运行搜索，确认：
- 根 README 中不再残留错误的 `"rule":` 示例
- 根 README 中不再残留错误的 `deltascope rules` 示例
- 更新过的模块 README 中不再残留当前态的 `v0.7.0` 说明

**Step 3: 审 diff 控制范围**

确认这次 diff 只包含：
- 文档事实修正
- release 索引更新
- 少量 verification 增补
- 架构文字/图示对齐

不要夹带无关 copy-edit churn。

**Step 4: 必要时提交最终清理**

```bash
git add README.md docs/releases/README.md docs/concept/architecture.md docs/dev/architecture.md docs/recipe/use-deltascope-mcp.md skills/README.md cmd/deltascope-server/README.md cmd/deltascope-mcp/README.md pkg/deltascope/README.md
git commit -m "docs: complete remediation pass"
```
