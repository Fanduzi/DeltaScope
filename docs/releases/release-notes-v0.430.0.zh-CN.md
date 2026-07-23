# DeltaScope v0.430.0 发行说明

## 概要 - 安全直接 CLI TLS 与凭证清理

v0.430.0 对 CLI 直接连接的 `audit` 和 `query-access analyze` 强制启用 TLS。启用 TLS 时，CLI 会验证完整证书链和精确主机名，这些检查不可禁用。明文 `--password` 和 `-p` 标志已移除。支持的密码来源仅有 `--password-env`、`--password-file` 和 `--ask-password`。CLI 新增 `--database` 标志，用于 `audit` 和 `query-access analyze` 的 PostgreSQL 目标选择。Query Access 提交的 SQL 不会被执行。

默认离线 SDK、CLI 和 HTTP 行为以及 MCP Query Access 可用性保持不变。

## 变更内容

- CLI `audit` 和 `query-access analyze` 直接连接支持 `--tls-mode enabled`，强制进行证书链和主机名验证。`--tls-ca-file` 是可选的；未指定时使用系统信任根。`--tls-mode disabled`（默认）故意不使用 TLS。
- 明文 `--password` 和 `-p` 标志已移除，不提供兼容性开关。支持的密码来源为 `--password-env`（环境变量名）、`--password-file`（文件路径）和 `--ask-password`（交互式提示）。
- CLI 新增 `--database` 标志，用于 `audit` 和 `query-access analyze` 的 PostgreSQL 目标选择。
- Query Access 提交的 SQL 仅作分析，不会被执行。不引入 SQL 执行路径。
- 决策记录：`docs/decisions/2026-07-22-query-access-cli-tls-and-credentials.md`（已接受；关联里程碑/版本：v0.430.0）。

## 保持不变

- 默认离线 SDK、CLI 和 HTTP 审计行为不变。没有默认路径会自动启用 TLS 或更改凭证处理方式。
- Query Access 仅发出静态要求。它不认证调用方、不评估授权、不强制 RLS、不脱敏列、不自动授予权限、不重写 SQL、不保证后续执行快照。
- MCP 工具仍仅为 `audit_sql`、`describe_rule`、`list_rules` 和 `get_capabilities`。不添加 Query Access 工具。
- 审计规则目录和默认审计行为不变。`level` 仍是公开审计优先级字段；不引入 severity 字段。
- Query Access 结果不包含原始 SQL、字面量、函数名、DSN、凭证、驱动错误、会话数据、端点地址或密钥。
- HTTP 连接模型和 API 密钥允许列表自 v0.420.0 起不变。

## 非目标

- 不是 SQL 执行或数据返回 API。
- 不是任意密码提交。仅接受 `--password-env`、`--password-file` 和 `--ask-password`。
- 不是数据库 grant、role、RLS 或会话授权评估。不是脱敏、重写或执行快照保证。
- 不是 SQL 模式证明、任意函数、UDF 或宽泛的函数名允许列表。
- 不是 MCP Query Access 工具。
- 不引入 severity 字段，注册的审计规则目录不变。

## 规则目录事实

注册的审计规则目录自 v0.420.0 起不变。本版本仅更改 CLI TLS 和凭证模型。

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

- `docs/decisions/2026-07-22-query-access-cli-tls-and-credentials.md`（本版本）
- `docs/decisions/2026-07-20-query-access-online-connection-registry.md`（v0.420.0）
- MySQL/TiDB 内建语义清单（v0.410.0）：`docs/decisions/2026-07-18-query-access-mysql-tidb-builtin-semantic-manifests.md`
- 通用纯效果 Query Access（v0.400.0）：`docs/decisions/2026-07-16-query-access-common-pure-effects.md`
- 受信 PostgreSQL SDK（v0.390.0）：`docs/decisions/2026-07-12-query-access-pure-read-admissibility.md`
- Query Access 基础（v0.380.0）：`docs/decisions/2026-07-11-query-access-analysis-foundation.md`
