# DeltaScope v0.260.0 发行说明

## 概要 — Unsupported Diagnostics Guidance Codes

v0.260.0 为 `parser_error` 诊断新增 `guidance_code` 和 `evidence_ref` 字段，让用户能直接看到未审计语句的不支持边界分类，并点击链接查看详细证据文档。本版本**不**新增 parser 支持、不改变 parser 行为、不新增 SQL 审计规则、不实现 fallback parser、不减少 `parser_error` 计数、不改变审计判定或 finding 语义、不改变任何 SQL 审核行为。

## 诊断引导码

当 DeltaScope 遇到无法解析的 SQL 时，诊断输出现在包含：

- **`guidance_code`** — 稳定的机器可读字符串，标识不支持边界类别（如 `parser_upgrade_candidate`）。
- **`evidence_ref`** — GitHub 文档 URL，指向相关证据章节。

`parser_error` 诊断输出示例：

```json
{
  "classification": "parser_error",
  "guidance_code": "parser_upgrade_candidate",
  "evidence_ref": "https://github.com/Fanduzi/DeltaScope/blob/main/docs/reference/cli.md#parser-upgrade-candidate-evidence-v02500",
  "audited": false
}
```

## 首批已验证 Parser-Upgrade Candidates

| 方言 | DDL 形式 | guidance_code |
|------|----------|---------------|
| MySQL | `ALTER VIEW` | `parser_upgrade_candidate` |
| PostgreSQL | `DROP SUBSCRIPTION ... WITH (drop_slot = true)` | `parser_upgrade_candidate` |

## 公开表面一致

`guidance_code` 和 `evidence_ref` 在所有公开表面一致暴露：

- **SDK** — `spec.Diagnostic` JSON struct tags
- **CLI JSON** — 通过 struct 序列化自动暴露
- **CLI text (markdown)** — 追加到诊断块
- **CLI text (quiet)** — 追加到诊断行
- **HTTP** — 通过 struct 序列化自动暴露
- **MCP** — 通过 struct 序列化自动暴露

## No-Leak 保证

`guidance_code` 是来自受控词汇表的固定字符串。`evidence_ref` 是固定的 GitHub 文档 URL。两个字段均不包含 raw SQL、parser near-text、对象名、函数体、默认表达式或任何用户 payload。

## 非目标

- 不新增 parser 支持。
- 不实现 fallback parser。
- 不新增 SQL 审计规则。
- 不减少 parser_error 计数。
- 不改变 DDL 普查。
- 不改变审计判定或 finding 语义。
- 不改变 SQL 审核行为。
- 不改变 npm/Homebrew 行为。

## Parser-Error 计数（未变）

| 方言 | Parser Error |
|------|:------------:|
| MySQL | 15 |
| TiDB | 9 |
| PostgreSQL | 5 |
| **合计** | **29** |

## DDL 覆盖范围普查（未变）

| 方言 | Total | Finding | Silent | Unsupported | Parser Error |
|------|------:|--------:|-------:|:-----------:|:------------:|
| MySQL | 61 | 46 | 0 | 0 | 15 |
| TiDB | 54 | 45 | 0 | 0 | 9 |
| PostgreSQL (consolidated tracked-case) | 285 | 274 | 6 | 0 | 5 |

## 未变指标

- SQL 语料库: **582/582**, **100.0%**, **245 YAML** fixture 文件。
- PostgreSQL ALTER TABLE 规则计数: **32**（未变）。
- PostgreSQL 合并 DDL 普查: **285/274/6/0/5/0**（未变）。
- MySQL/TiDB DDL Notice 段: **27**（未变）。
- TiDB 专用子段: **7**（未变）。

## 决策记录

`docs/decisions/2026-06-04-v0.260.0-unsupported-diagnostics-guidance-codes.md`
