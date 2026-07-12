# DeltaScope v0.380.0 发行说明

## 概要 — Query Access Analysis 基础能力

v0.380.0 将查询访问分析（Query Access Analysis）作为独立公共能力发布，与现有 audit 路径并列。它检查 SQL，并报告读取分类、准入结论、需要权限的表/列、输出血缘与结构化权限要求。它不做调用方身份认证、不评估授权、不为鉴权去连接数据库、不改写 SQL，也不充当策略引擎。

公开准入值为 `admissible`、`rejected`、`indeterminate`。鉴权层对 `indeterminate` 应按 fail-closed 处理（默认拒绝）。不存在 `severity` 字段；查询访问结果不是 audit finding。结果不包含原始 SQL、字面量、凭据、连接串或 parser 片段。

SDK、CLI、HTTP 已提供该能力。MCP 工具集合仍为 `audit_sql`、`describe_rule`、`list_rules`、`get_capabilities` 四项；本版本不提供 query-access MCP 工具。

## 改了什么

- 新增查询访问分析：读取分类为 `read_only`、`not_read_only`、`indeterminate`。
- 准入由分类推导：`read_only` → `admissible`，`not_read_only` → `rejected`，其余为 `indeterminate`。
- 模式：
  - `strict`（默认）：投影、过滤、连接、分组、having、排序、窗口等全部已解析源列都产生 `read_column` 要求。
  - `projection_only`：仅输出（投影）源列需要 `read_column`。非投影列仍可能被用于推断；适用时结果会返回 `projection_only_inference_risk` 警告。
- 权限对象始终是基表与视图。CTE 与派生表不直接要求权限，必须回溯到物理来源。
- 元数据不完整、引用歧义、通配符无法完整展开、函数/运算符效果未知时 fail-closed。
- PostgreSQL 保持保守边界：不确定表达式（例如运算符、函数调用、副作用未知的类型转换）保持不可准入（分类/准入为 `indeterminate`）。
- 对外接入面：
  - SDK：`deltascope.AnalyzeQueryAccess`
  - CLI：`deltascope query-access analyze`
  - HTTP：`POST /v1/query-access/analyze`
  - MCP：不提供 query-access 工具（明确延后）
- 查询访问语料：**44** 个用例（MySQL/TiDB 路径 **22** + PostgreSQL **22**），**88** 个夹具文件；`make query-access-corpus-gates` 通过。
- 决策记录：`docs/decisions/2026-07-11-query-access-analysis-foundation.md`、`docs/decisions/2026-07-11-cte-derived-table-lineage-resolution.md`。

## 哪些没变

- 现有 audit 行为、默认策略与已注册规则目录不变。
- MCP 工具集合不变（仍为四个工具；无 query-access 工具）。
- audit finding 仍使用 `level` 作为公开优先级字段。不引入 `severity` 字段。
- 查询访问结果不使用 `severity`（不引入 `severity` 字段），也不是 audit finding。
- 隐私/不泄漏：结构化结果契约不包含原始 SQL、字面量、凭据、连接串或 parser 片段。

## 非目标

- 不做运行时授权评估、调用方认证或数据库会话鉴权。
- 不为调用方鉴权去连接数据库。
- 不做策略引擎、自动授权或 SQL 改写服务。
- 不做行级安全评估或列脱敏。
- 不做 MCP query-access 工具。
- 不声称覆盖全部 SQL 语法，也不声称各方言表达式形态完全对等。
- 不引入 `severity` 字段（no `severity` field）。

## 规则目录事实

已注册 audit 规则目录与 v0.370.0 相同。本版本新增独立的查询访问能力，不是已注册规则的变更。

| 指标 | 数量 |
|------|----:|
| 规则总数 | **371** |
| blocker | 72 |
| warning | 142 |
| notice | 157 |

| 方言范围 | 规则 |
|----------|----:|
| common | 177 |
| postgresql | 191 |
| mysql | 1 |
| tidb | 2 |

| 语句类型 | 规则 |
|----------|----:|
| ddl | 361 |
| dml | 10 |

## 未变指标

- SQL 语料：**582/582**，**100.0%**，**247** YAML 夹具文件（未变）。
- PostgreSQL ALTER TABLE 配置条目：**53**（未变）。
- DDL 覆盖目录：**400** 条目（未变；mysql 61、tidb 54、postgresql 285、parser_upgrade_candidate 18）。

## 决策记录

- `docs/decisions/2026-07-11-query-access-analysis-foundation.md`
- `docs/decisions/2026-07-11-cte-derived-table-lineage-resolution.md`
