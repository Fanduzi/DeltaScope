# DeltaScope v0.230.0 发行说明

## 概要 — Unsupported Diagnostics Evidence

v0.230.0 引入结构化诊断证据，覆盖 parser-error 和 unsupported-statement 两种未审计结果，在四个公开产品接口（SDK、CLI、HTTP、MCP）统一输出。每条 DeltaScope 无法完整审计的语句现在附带一个 `spec.Diagnostic`，包含 `classification`、`reason`、`action_hint`、`audited` 和 `dialect` 字段。本版本**不**新增 parser 支持、不新增 SQL 审核规则、不实现 fallback parser、不降低 parser_error 数量。

## 新增结果契约

- `spec.Diagnostic` — 每语句结构化诊断。
- `report.Result.Diagnostics` — 公开结果上的诊断数组。

### 诊断字段

| 字段 | 用途 |
|------|------|
| `classification` | `parser_error` 或 `unsupported_statement` |
| `reason` | 人类可读的未审计原因说明 |
| `action_hint` | 用户下一步操作指引 |
| `audited` | `false` — 语句未被完整审计 |
| `dialect` | 本次审核请求所选方言 |

## 公开接口覆盖

| 接口 | 诊断已输出 | 操作提示已包含 |
|------|:--------:|:-----------:|
| SDK (`pkg/deltascope`) | 是 | 是 |
| CLI (`deltascope`) | 是 | 是 |
| HTTP (`deltascope-server`) | 是 | 是 |
| MCP (`deltascope-mcp`) | 是 | 是 |

## 诊断示例

### Parser-Error 诊断

| 字段 | 值 |
|------|------|
| `classification` | `parser_error` |
| `reason` | `statement was not audited because the selected dialect parser could not parse it; no audit findings were inferred` |
| `action_hint` | 包含 `verify the selected dialect` |
| `audited` | `false` |

### Unsupported-Statement 诊断

| 字段 | 值 |
|------|------|
| `classification` | `unsupported_statement` |
| `reason` | `DeltaScope recognized this statement or feature but does not audit it yet` |
| `action_hint` | 包含人工复核 / 未来支持指引 |
| `audited` | `false` |

诊断中不包含原始 SQL 或 payload。

## Parser-Error 数量（不变）

| 方言 | Parser 错误 |
|------|:----------:|
| MySQL | 15 |
| TiDB | 9 |
| PostgreSQL | 5 |
| **合计** | **29** |

## Parser-Error 可行性桶（不变）

| 桶 | MySQL | TiDB | PostgreSQL | 合计 |
|----|-------|------|------------|------|
| `parser_upgrade_candidate` | 5 | 0 | 5 | 10 |
| `bounded_fallback_candidate` | 1 | 3 | 0 | 4 |
| `product_unsupported_or_inapplicable` | 0 | 6 | 0 | 6 |
| `unsafe_fallback_defer` | 9 | 0 | 0 | 9 |
| `needs_research` | 0 | 0 | 0 | 0 |

## DDL 覆盖范围普查（不变）

| 方言 | 总数 | Finding | 静默 | 不支持 | Parser 错误 |
|------|-----:|-------:|-----:|:------:|:----------:|
| MySQL | 61 | 46 | 0 | 0 | 15 |
| TiDB | 54 | 45 | 0 | 0 | 9 |
| PostgreSQL（consolidated tracked-case） | 285 | 274 | 6 | 0 | 5 |

## 不变指标

- SQL corpus：**582/582**，**100.0%**，**245 YAML** fixture 文件。
- PostgreSQL ALTER TABLE 规则数：**32**（不变）。
- PostgreSQL consolidated DDL census：**285/274/6/0/5/0**（不变）。
- MySQL/TiDB DDL Notice 节：**27**（不变）。
- TiDB 专有子节：**7**（不变）。

## 非目标

- 不新增 parser 支持。
- 不新增 SQL 审核规则。
- 不实现 fallback parser。
- 不降低 parser_error 数量。
- 不声称 full MySQL/TiDB/PostgreSQL DDL support。
- 不声称 dialect parity。
- 不声称 runtime/catalog validation。
- 诊断中不复制原始 SQL 或 payload。

## 决策记录

`docs/decisions/2026-05-28-v0.230.0-unsupported-diagnostics-evidence.md`
