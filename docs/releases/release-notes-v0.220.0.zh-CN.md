# DeltaScope v0.220.0 发行说明

## 概要 — Parser-Error Unsupported Contract 加固

v0.220.0 对四个公开产品接口（SDK、CLI、HTTP、MCP）的 unsupported-contract 诊断信息进行统一加固。Parser-error 语句现在统一输出标准诊断——`statement was not audited because the selected dialect parser could not parse it; no audit findings were inferred`——替代了原始 parser `near ...` 片段和被追踪的 forbidden payload。本版本**不**新增 parser 支持、不新增 SQL 审核规则、不实现 fallback parser、不降低 parser_error 数量。

## 标准诊断信息

```
statement was not audited because the selected dialect parser could not parse it; no audit findings were inferred
```

当遇到 parser-error 语句时，SDK、CLI、HTTP 和 MCP 四个接口统一输出此诊断。原始 parser `near ...` 片段和被追踪的 forbidden payload 不再出现在公开输出中。

## 公开接口覆盖

| 接口 | 诊断已标准化 | 原始片段已移除 |
|------|:----------:|:-----------:|
| SDK (`pkg/deltascope`) | 是 | 是 |
| CLI (`deltascope`) | 是 | 是 |
| HTTP (`deltascope-server`) | 是 | 是 |
| MCP (`deltascope-mcp`) | 是 | 是 |

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
|------|-----:|--------:|-----:|:------:|:----------:|
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
- PostgreSQL capability boundary error 保持不变。

## 决策记录

`docs/decisions/2026-05-28-v0.220.0-parser-error-unsupported-contract-hardening.md`
