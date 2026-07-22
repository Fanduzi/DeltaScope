# DeltaScope v0.420.0 发行说明

## 概要 - Online Query Access Connection Registry

v0.420.0 将内联 HTTP `connection` 对象替换为通过 `connection_id` 引用的运营方管理的命名连接。HTTP 在线审计和 Query Access 现在从服务端连接注册表解析数据库目标，而非在每个请求中接受 host/port/user/password 字段。TLS 模式、API 密钥允许列表和能力推导均附加在命名连接条目上。

一个有界的标量函数子集（LOWER、UPPER、LENGTH、CHAR_LENGTH、ABS、CEIL、FLOOR、COALESCE、NULLIF、IFNULL）在连接的服务器身份匹配受支持的系列（MySQL 5.7.x、MySQL 8.0.x、MySQL 8.4.x、TiDB 8.5.x、PostgreSQL 17.x）时，为在线 Query Access 启用。该子集经过刻意收窄。每个标量函数的每个操作数都必须是直接物理基表列；嵌套表达式、字面量和函数调用会被拒绝。

默认离线 SDK、CLI 和 HTTP 保持不变。CLI 保留直接连接标志并新增 `--database` 用于 PostgreSQL 目标选择。MCP 仍然没有 Query Access 工具。

## 变更内容

- HTTP 在线审计和 Query Access 通过 `connection_id` 从运营方管理的命名连接解析目标。内联 HTTP `connection` 对象（host/port/user/password_env/schema）已移除，不提供兼容性开关。
- TLS 模式按连接声明：`disabled`（默认）或 `enabled`。启用时，证书链和主机名验证不可禁用。`tls_ca_file` 是可选的；未指定时使用系统信任根。`tls_mode: disabled` 故意不使用 TLS。HTTP 请求体不接受任何 TLS 配置。
- 为在线 Query Access 启用的有界标量函数子集：LOWER、UPPER、LENGTH、CHAR_LENGTH、ABS、CEIL、FLOOR、COALESCE、NULLIF、IFNULL。每个操作数都必须是直接物理基表列。IFNULL 仅限 MySQL/TiDB（PostgreSQL 上为 N/A）。
- 在线会话从连接的服务器身份推导能力：MySQL 5.7.x、MySQL 8.0.x、MySQL 8.4.x、TiDB 8.5.x、PostgreSQL 17.x。不接受调用方提供的清单或配置覆盖。
- HTTP 认证使用每个连接条目的 API 密钥允许列表。不接受每请求凭证。
- CLI 保留直接连接标志（`--host`、`--port`、`--user`、`--ask-password`、`--schema`）。CLI 新增 `--database` 标志用于 PostgreSQL 目标选择。
- 决策记录：`docs/decisions/2026-07-20-query-access-online-connection-registry.md`（已接受；关联里程碑/版本：v0.420.0）。

## 保持不变

- 不带 `connection_id` 的默认 `AnalyzeQueryAccess`、CLI `query-access analyze` 和 HTTP `POST /v1/query-access/analyze` 对携带函数的查询仍保持失败关闭。默认 SDK/CLI/HTTP 路径不会自动提升所有函数查询。
- Query Access 仅发出静态要求。它不认证调用方、不评估授权、不强制 RLS、不脱敏列、不自动授予权限、不重写 SQL、不保证后续执行快照。
- MCP 工具仍仅为 `audit_sql`、`describe_rule`、`list_rules` 和 `get_capabilities`。不添加 Query Access 工具。
- 审计规则目录和默认审计行为不变。`level` 仍是公开审计优先级字段；不引入 severity 字段。
- Query Access 结果不包含原始 SQL、字面量、函数名、DSN、凭证、驱动错误、会话数据、端点地址或密钥。不引入 `severity` 字段。
- PostgreSQL 的受控会话目录/OID 证明路径不变。MySQL/TiDB 连接注册表条目不会影响 PostgreSQL，反之亦然。

## 非目标

- 不是 SQL 执行或数据返回 API。
- 不是任意 host/password HTTP 请求。请求体不能提交数据库凭证、主机、DSN 或 CA 路径。
- 不是 UDF/存储函数、类型转换、字面量、嵌套表达式或宽泛的函数名允许列表。
- 不是数据库 grant、role、RLS 或会话授权评估。不是脱敏、重写或执行快照保证。
- 不是 MySQL/TiDB 配置作为 SQL 模式证明。
- 不是 MCP Query Access 工具。
- 不引入 severity 字段，注册的审计规则目录不变。

## 支持矩阵

| 函数 | MySQL 5.7 | MySQL 8.0 | MySQL 8.4 | TiDB 8.5 | PostgreSQL 17 |
|---|---|---|---|---|---|
| LOWER, UPPER, LENGTH, CHAR_LENGTH | ✓ | ✓ | ✓ | ✓ | ✓ |
| ABS, CEIL, FLOOR | ✓ | ✓ | ✓ | ✓ | ✓ |
| COALESCE, NULLIF | ✓（直接列） | ✓（直接列） | ✓（直接列） | ✓（直接列） | ✓（直接列） |
| IFNULL | ✓（直接列） | ✓（直接列） | ✓（直接列） | ✓（直接列） | N/A |

每个标量函数的每个操作数都必须是直接物理基表列。不支持这些运算符内的嵌套表达式、字面量和函数调用。IFNULL 是 MySQL/TiDB 内建函数，PostgreSQL 无等效函数。

## 规则目录事实

注册的审计规则目录自 v0.410.0 起不变。本版本仅更改 Query Access 连接模型和在线标量函数子集。

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

- `docs/decisions/2026-07-20-query-access-online-connection-registry.md`（本版本）
- MySQL/TiDB 内建语义清单（v0.410.0）：`docs/decisions/2026-07-18-query-access-mysql-tidb-builtin-semantic-manifests.md`
- 通用纯效果 Query Access（v0.400.0）：`docs/decisions/2026-07-16-query-access-common-pure-effects.md`
- 受信 PostgreSQL SDK（v0.390.0）：`docs/decisions/2026-07-12-query-access-pure-read-admissibility.md`
- Query Access 基础（v0.380.0）：`docs/decisions/2026-07-11-query-access-analysis-foundation.md`
