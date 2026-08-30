# DeltaScope v0.510.1 发行说明

## 概要 - Linux 测试契约热修与 #54–#57 再发布

v0.510.1 再发布已经过审查的 #54–#57 源码工作，并加上一处确定性的 Linux 测试契约修正。官方 v0.510.0 GitHub Actions 运行 33302045413 通过 provenance 后，在 `TestAuditCommandLoadsTLSCAFile` 失败，且尚未产出任何资产：空 Linux loopback 返回有界 `connection refused`，而该测试 allowlist 只接受 TLS hostname/certificate、`connection failed` 或 `connection timed out`。v0.510.0 不是一次成功的已发布 GitHub Release。本次热修只扩展该有界运行时断言；生产侧分类不变。

在 provenance 与平台构建成功的前提下，即使 Homebrew cask 发布或安装验证失败，tag 触发的工作流仍会发布 `@fanduzi/deltascope-mcp`。默认 CLI JSON 把方言上大量的 skip 列表压缩为 `{reason, count}` 聚合；`--include-skipped-rules` 恢复逐规则列表。混合迁移在存在未审计 `parser_error` 诊断时，即使 findings 只有 notice，也不会再给出 `pass`，而是在 SDK、CLI、HTTP、MCP 上落到 `review`。元数据感知 CLI 的 TCP 拒绝是有界的 `connection refused`，退出码 3。

这仍是静态分析。DeltaScope 不执行提交的 SQL、不取回查询结果，也不做授权、授权清单、RLS 或脱敏判定。MCP 仍然没有 Query Access 工具。已注册审核规则目录仍为 373 条。支持的 rule-and-dialect fixture coverage 仍为 586/586、100.0%、286 个 YAML 文件；这不是 SQL 语法或 grammar coverage。已恢复的 `@fanduzi/deltascope-mcp@0.500.0` 是独立的历史发布，v0.510.1 不会改动它。

## 变更内容

### Linux TLS CA 测试契约

- `TestAuditCommandLoadsTLSCAFile` 在有效 CA 解析之后，除 TLS hostname/certificate、`connection failed`、`connection timed out` 外，现在也接受有界 `connection refused`。
- 不改变生产侧分类器、文案或退出码。带类型的 `syscall.ECONNREFUSED` 仍按 #57 映射为 `connection refused`，退出码 3。

### 发布通道解耦（#54）

- `publish-mcp-launcher-package` 只等待四个平台构建 job，并传递依赖 provenance。
- Homebrew 发布与 Homebrew 安装验证仍会运行，但不再门禁 npm。
- `make release-workflow-hygiene-gates` 拒绝把 npm 重新接到 Homebrew，或丢掉平台构建 / provenance 前置条件的工作流图。

### 压缩 CLI JSON skipped rules（#55）

- 默认 CLI JSON 把 `rule_summary.skipped` 写成按 reason 排序的 `{reason, count}` 数组；没有 skip 时为 `[]`，并省略 `rule_summary.skipped_rules`。
- 仅审计的 `--include-skipped-rules` 增加 `rule_summary.skipped_rules`，对象形状为 `{rule_id, reason}`，同时保留聚合 `skipped` 字段。
- `--quiet --format json` 与相同其他 flag 的普通 JSON 字节级一致。
- SDK、HTTP、MCP、Markdown、GitHub Actions、SARIF 和 GitLab Code Quality 输出不变。

### 解析 review 下限（#56）

- 在共享 application 结果缝上，只要部分解析存在 `audited=false` 的 `parser_error` 诊断，且 finding 聚合结果是 `pass`，就把 verdict 提升为 `review`。
- 既有 `review` 与 `reject` 不会降级。完全无法解析的输入仍遵循 #24/#43 行为。
- SDK、CLI、HTTP、MCP 序列化同一份下限后的 verdict。application 与 SDK 仍返回非空 parser 错误；CLI 仍退出 2；HTTP 与 MCP 仍标记错误。

### CLI 连接拒绝（#57）

- 元数据感知 CLI 把带类型的 `syscall.ECONNREFUSED` 分类为 `connection refused`，退出码 3。
- 其他非 TLS 拨号失败仍为 `connection failed`，退出码 3。鉴权、超时、TLS、密码源和 PostgreSQL 端口映射不变。
- 可移植 CLI 输出不含 host、port、user、database、schema、DSN、密码、原始驱动文本、文件系统路径或版本。

## 保持不变

- MCP 工具仍仅为 `audit_sql`、`describe_rule`、`list_rules`、`get_capabilities`。
- v0.500.0 的 Query Access 契约不变：空 `--sql`、仅审计 flag、MySQL/TiDB schema 绑定、PG17 版本边界，以及默认/离线 fail-closed 路径。
- `level` 仍是公开优先级字段；不引入 severity 字段。
- 已注册审核规则目录、SQL corpus fixture coverage 和 DDL 覆盖目录计数相对 v0.500.0 不变。
- 既有 release tag、GitHub Release、npm 包和 Homebrew cask 在本 tag 发布前保持不动。
- 保留 v0.510.0 历史说明；不把 v0.510.0 宣传为成功的已发布 GitHub Release，也不为它增加 landing 历史卡片。

## 非目标

- 不是把 v0.510.0 当作一次成功的已发布 GitHub Release，也不重打或改写既有 tag。
- 不是 MCP Query Access 工具。
- 不是授权、授权清单、角色、RLS、脱敏、改写、SQL 执行或返回数据的 API。
- 不是新的 verdict 枚举、回退语法，也不是对解析失败语句的语义猜测。
- 不是非 TLS 协议/握手分类，也不泄漏连接内部信息。
- 不改变 SDK、HTTP 或 MCP 的 skipped-rule JSON 形状。
- 不改变已注册规则目录，也不是 SQL syntax 或 grammar coverage。
- 不是 severity 字段；不改变任何既有已发布产物或已有 tag。
- 不是把 `connection refused` 在生产侧改映射成 `connection failed`。

## 规则目录事实

已注册审核规则目录相对 v0.500.0 不变。

| 指标 | 数量 |
|------|------:|
| 规则总数 | **373** |
| blocker | 73 |
| warning | 142 |
| notice | 158 |

| 方言范围 | 规则数 |
|----------|------:|
| common | 177 |
| postgresql | 191 |
| mysql | 1 |
| tidb | 2 |
| mysql and tidb | 2 |

| 语句种类 | 规则数 |
|----------|------:|
| ddl | 362 |
| dml | 11 |

## 语料与目录事实

- 支持的 rule-and-dialect fixture coverage：**586/586**，**100.0%**，**286** 个 YAML fixture；这不是 SQL syntax 或 grammar coverage。
- PostgreSQL ALTER TABLE 配置项：**53**。
- DDL 覆盖目录：**407** 条（mysql 62，tidb 55，postgresql 290，parser_upgrade_candidate 18）。

## 决策记录

- `docs/decisions/2026-08-30-v0.510.1-linux-tls-ca-test-contract.md`（本版本；Linux 测试契约）
- `docs/decisions/2026-08-30-release-channel-npm-homebrew-parity.md`（#54）
- `docs/decisions/2026-08-30-cli-json-skipped-rule-compaction.md`（#55）
- `docs/decisions/2026-08-30-partial-parser-error-verdict-review-floor.md`（#56）
- `docs/decisions/2026-08-30-cli-connection-error-categories.md`（#57）
- `docs/decisions/2026-08-30-partial-parser-error-recovery.md`（v0.500.0；#43）
- `docs/decisions/2026-08-17-cli-metadata-connection-exit-mapping.md`（v0.490.0；#23）
