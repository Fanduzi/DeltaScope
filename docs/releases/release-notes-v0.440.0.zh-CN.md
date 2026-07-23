# DeltaScope v0.440.0 发行说明

## 概要 - CLI TLS E2E 作为 CI 与发布门禁

v0.440.0 将 CLI TLS 端到端套件（`make test-e2e-cli-tls`）提升为强制门禁。它现在通过专用的 GitHub Actions 工作流在每次 pull request 和推送到 `main` 时运行，并被组合进 `make release-test-gates`，因此在它通过之前无法发布版本。该套件使用 Compose 分配的动态主机端口和唯一项目名，可安全并行运行，采用 fail-closed 的 Docker 策略，并由资源清理回归夹具保障。本版本仅涉及测试与发布基础设施。

本版本不更改任何产品、审计或 Query Access 行为。默认离线 SDK、CLI 和 HTTP 行为以及 MCP Query Access 可用性自 v0.430.0 起保持不变。

## 变更内容

- CLI TLS E2E 套件（`make test-e2e-cli-tls`）作为必需的 GitHub Actions 门禁在 pull request 和推送到 `main` 时运行，并被组合进发布工作流调用的 `make release-test-gates`。
- TLS 夹具使用 Compose 分配的动态主机端口，配合唯一的 Compose 项目名和容器名覆盖，因此运行不会与其他服务或并行运行发生冲突。
- 强制 fail-closed 的 Docker 策略：CI 中若 Docker 不可用则套件失败，且在 CI 或设置了必需标志时拒绝 `--docker-optional`。
- 夹具生命周期被加固：Compose 拆除加上对残留容器/网络/卷的显式检查以及临时工作区清理，并由专用清理回归夹具（`make test-e2e-cli-tls-regression`）验证。
- MySQL TLS 夹具使用 TCP + TLS 就绪 healthcheck 和可读的服务器密钥，使门禁在 Linux CI 上反映真实的 `TCP+TLS` 连通性，而非 Unix socket 的假阳性。
- 决策记录：`docs/decisions/2026-07-22-query-access-cli-tls-and-credentials.md`（已接受；更新了 v0.440.0 的 CI/发布门禁证据）。

## 保持不变

- 默认离线 SDK、CLI 和 HTTP 审计行为不变。没有默认路径会自动启用 TLS 或更改凭证处理方式。
- CLI TLS 模式、凭证源和 PostgreSQL `--database` 选择自 v0.430.0 起不变。Query Access 提交的 SQL 仍仅作分析，不会被执行。
- Query Access 仅发出静态要求。它不认证调用方、不评估授权、不强制 RLS、不脱敏列、不自动授予权限、不重写 SQL、不保证后续执行快照。
- MCP 工具仍仅为 `audit_sql`、`describe_rule`、`list_rules` 和 `get_capabilities`。不添加 Query Access 工具。
- 审计规则目录和默认审计行为不变。`level` 仍是公开审计优先级字段；不引入 severity 字段。
- Query Access 结果不包含原始 SQL、字面量、函数名、DSN、凭证、驱动错误、会话数据、端点地址或密钥。

## 非目标

- 不是新产品功能。本版本仅更改 CI、发布门禁与测试基础设施。
- 不是对 Query Access 语义、可证明纯函数范围或字面量操作数可接纳性的更改。
- 不是 SQL 执行或数据返回 API。
- 不是数据库 grant、role、RLS 或会话授权评估。不是脱敏、重写或执行快照保证。
- 不是 MCP Query Access 工具。
- 不引入 severity 字段，注册的审计规则目录不变。

## 规则目录事实

注册的审计规则目录自 v0.430.0 起不变。本版本仅更改 CI 与发布门禁。

| 指标 | 数量 |
|------|------:|
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

## 不变指标

- SQL 语料库：**582/582**，**100.0%**，**247** 个 YAML 测试夹具文件。
- PostgreSQL ALTER TABLE 配置条目：**53**。
- DDL 覆盖目录：**400** 条（mysql 61，tidb 54，postgresql 285，parser_upgrade_candidate 18）。

## 决策记录

- `docs/decisions/2026-07-22-query-access-cli-tls-and-credentials.md`（本版本；更新了 CI/发布门禁证据）
- `docs/decisions/2026-07-20-query-access-online-connection-registry.md`（v0.420.0）
- MySQL/TiDB 内建语义清单（v0.410.0）：`docs/decisions/2026-07-18-query-access-mysql-tidb-builtin-semantic-manifests.md`
- 通用纯效果 Query Access（v0.400.0）：`docs/decisions/2026-07-16-query-access-common-pure-effects.md`
- 受信 PostgreSQL SDK（v0.390.0）：`docs/decisions/2026-07-12-query-access-pure-read-admissibility.md`
- Query Access 基础（v0.380.0）：`docs/decisions/2026-07-11-query-access-analysis-foundation.md`
