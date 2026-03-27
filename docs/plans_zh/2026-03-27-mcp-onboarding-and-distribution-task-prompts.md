# MCP Onboarding 与 Distribution 任务提示词

> 用于 `MCP Onboarding And Distribution` 里程碑的逐任务实现与评审。
> 默认工作目录为 `/Users/fan/GolangProjects/DeltaScope`。

## Global Rules

- `deltascope-mcp` 必须继续作为规范 MCP server 实现。
- npm 包不得 fork 或重写 MCP tool contract。
- npm 包只承担 launcher/bootstrap 职责。
- 必须保留原生二进制路径，供 CI、自动化和非 Node 用户使用。
- 所有 onboarding 示例都必须能直接复制，并与真实命令和 flag 一致。
- Claude Code 与 Codex 是优先示例，但不是全部用户。
- 始终提供通用手动 stdio 配置示例。
- 始终文档化 direct connection 与 `connection_ref` 两条路径。
- `connection_ref` 文档必须给出真实的 `connections.yaml` 结构。
- 保持中英文文档对齐。
- 所有 launcher 的版本解析、缓存和打包行为都必须有测试或 dry-run 验证。
- 每个任务都返回修改文件、运行测试、状态与 commit hash。

## Milestone Focus

- npm launcher 包 `@fanduzi/deltascope-mcp`
- 基于 release binary 的下载与 cache bootstrap
- Claude Code onboarding
- Codex onboarding
- 通用 MCP client stdio 配置
- 原生 binary fallback
- 专门的 MCP quick-start 与使用文档
- 围绕 launcher、binary 和连接配置形成一个统一产品故事

## Task Intent

### Task 1: 规划文档

- 以中英双语保存已确认的 design、implementation 和 task prompts。
- 让里程碑聚焦 onboarding 与 distribution UX，而不是继续扩展 server 内核。

### Task 2: Launcher Scaffold

- 接入发布 bootstrap 所需的最小 npm 包结构。
- 让 launcher 足够薄，便于推理和维护。

### Task 3: Release 解析与 Cache

- 为当前平台下载正确的 DeltaScope binary。
- 以版本和平台为粒度做 cache，保证重复运行足够快。

### Task 4: Stdio 执行

- 启动真正的 `deltascope-mcp` 并干净转发 stdio。
- 保留原生 MCP 行为，不增加 launcher 私有语义。

### Task 5: Publish Contract

- 明确 npm 包版本与 DeltaScope release 的关系。
- 诚实说明该 package 只是 GitHub Release binaries 的启动器。

### Task 6: Dedicated MCP Guide

- 写出一份新用户真的能照着接上的 guide。
- 覆盖 Claude Code、Codex、通用 stdio config、direct connection 与 `connection_ref`。

### Task 7: README 重组

- 保持 README 快速、可复制。
- 通过专门 guide 承接细节，而不是把所有内容继续堆在一起。

### Task 8: 验证

- 用测试和 package dry-run 证明 launcher 行为正确。
- 保持原生 Go 测试套件为绿。

### Task 9: Release-Readiness Handoff

- 让里程碑进入可发布状态。
- 留下一份清晰的 launcher 发布与 MCP 文档更新清单。
