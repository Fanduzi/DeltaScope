# DeltaScope v0.470.0 发行说明

## 概要 - 发布恢复来源证明强制

v0.470.0 加固手动触发的发布恢复（release-recovery）工作流，使其执行与常规 tag 触发发布工作流相同的 release-candidate 来源证明契约。常规恢复现在只接受未来的、release-candidate 来源证明有效、且 peeled tag 目标 commit 可从受信任的 `origin/main` 引用到达的 annotated tag。恢复 dispatch 只在 `refs/heads/main` 上运行，发布 job 只检出 preflight 已验证的 peeled tag 目标 SHA，恢复契约门禁为封闭式（hermetic），历史标签 `v0.240.0` 和 `v0.460.0` 是其文档化的 fail-closed 负例。

这仅是发布工程变更。它不改变 SQL 审核行为、Query Access、SDK、CLI、HTTP 或 MCP 接入面，也不改动任何既有 release tag、GitHub Release、npm 包或 Homebrew cask。

## 变更内容

### 恢复来源证明准入

恢复 preflight 在提取校验和、执行任何 Homebrew 或 npm 变更之前，先通过既有的 post-tag candidate 门禁验证请求的输入 tag。工作流以完整历史检出请求的 tag，获取 `origin/main`，并把 `refs/remotes/origin/main` 作为验证器显式的受信任 main 引用。因此常规恢复只接受未来的、来源证明有效的 annotated tag；不含 `.release-candidate` 来源证明的历史标签（包括 `v0.460.0`）fail-closed。没有任何工作流输入可以绕过来源证明，`dry_run` 同样要求来源证明有效。

### 仅限 main 的 Dispatch-Ref 守卫

恢复 `preflight` 的第一步是一个独立、fail-closed 的守卫，要求 `github.ref` 精确等于 `refs/heads/main`，并在检出、校验和提取或任何外部工作之前失败。branch-ref 或 tag-ref 的 dispatch 会在守卫处失败，因此常规恢复始终运行 `main` 上已审阅的工作流定义。结构检查器要求守卫为规范形状，并拒绝嵌套死代码 exit 和 else 分支反转守卫。

### 已验证 Peeled SHA 发布固定

来源证明门禁通过后，`preflight` 将已验证的 tag 解析为其 peeled commit SHA，并作为 `tag_target_sha` job 输出导出。`publish-homebrew-cask` 和 `publish-mcp-launcher-package` 只检出 `needs.preflight.outputs.tag_target_sha`——绝不检出工作流默认分支、输入 tag 名称或任何在运行过程中可能移动的引用。这封闭了 preflight 验证与发布检出之间的 verify-then-publish 缺口。

### 封闭式恢复契约门禁

`release-recovery-contract-test` 现在是静态、离线、确定性的门禁。其正向路径是在临时 Git fixture 中用本地 stub 构建的未来有效 release-candidate 链；其负向路径证明历史非来源证明标签 `v0.240.0` 和 `v0.460.0` 在任何发布 stub 之前失败。该门禁不需要网络、GitHub Release 或 npm registry 访问，`v0.240.0` 不再作为在线正向门禁输入。

### 结构检查器与卫生门禁接线

恢复来源证明结构检查器由 `scripts/verify_release_workflow_hygiene.sh` 调用，因此被 `make release-workflow-hygiene-gates` 和 `make release-contract-gates` 继承。该检查器还断言恢复 `preflight` 显式声明 job 级 `permissions: contents: read`，不持有任何发布或外部变更权限。

## 保持不变

- SQL 审核行为、已注册审计规则目录和默认审计输出不变。`level` 仍是公开审计优先级字段；不引入 severity 字段。
- 所有接入面上的 Query Access 行为不变：默认离线 SDK、CLI 和 HTTP 保持 fail-closed，MCP 仍无 Query Access 工具。
- MCP 工具仍仅为 `audit_sql`、`describe_rule`、`list_rules` 和 `get_capabilities`。
- 既有 release tag、GitHub Release、npm 包和 Homebrew cask 不受影响。已发布的 `v0.460.0` 产物仍然有效；只是该标签不满足之后引入的策略。
- 常规 tag 触发发布工作流保留其既有的 release-candidate 来源证明强制。

## 非目标

- 不是历史恢复自动化。恢复不含 `.release-candidate` 来源证明的历史标签属于此常规工作流之外的事故决策，需要单独评审。
- 不是通用紧急绕过机制、基于版本号的覆盖开关或允许列表。
- 不是加密签名或外部审批存储。
- 不是自动化的 tag、release、包或 cask 回退与删除。
- 不是产品行为变更：无 SQL 审核、parser、规则、Query Access、SDK、CLI、HTTP 或 MCP 语义变更。
- 不改变任何已发布产物或既有标签。

## 规则目录事实

已注册审计规则目录自 v0.460.0 起不变。本版本仅改变发布恢复工作流的强制约束。

| 指标 | 数量 |
|------|-----:|
| 规则总数 | **371** |
| blocker | 72 |
| warning | 142 |
| notice | 157 |

| 方言范围 | 规则数 |
|----------|-------:|
| common | 177 |
| postgresql | 191 |
| mysql | 1 |
| tidb | 2 |

| 语句类型 | 规则数 |
|----------|-------:|
| ddl | 361 |
| dml | 10 |

## 未变化指标

- SQL 语料库：**582/582**，**100.0%**，**247** 个 YAML fixture 文件。
- PostgreSQL ALTER TABLE 配置条目：**53**。
- DDL 覆盖范围目录：**400** 条（mysql 61、tidb 54、postgresql 285、parser_upgrade_candidate 18）。

## 决策记录

- `docs/decisions/2026-07-30-release-recovery-provenance-enforcement.md`（本版本）
- `docs/decisions/2026-07-29-release-candidate-provenance-enforcement.md`（相关：常规发布工作流来源证明）
- `docs/decisions/2026-07-28-query-access-relationless-literal-selects.md`（v0.460.0）
