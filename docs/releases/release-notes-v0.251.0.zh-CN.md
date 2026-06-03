# DeltaScope v0.251.0 发行说明

## 概要 — Release Tag Annotation Guard

v0.251.0 新增 release tag 标注守卫，防止 lightweight git tag 通过发布流水线。本版本**不**新增 parser 支持、不改变 parser 行为、不新增 SQL 审计规则、不实现 fallback parser、不减少 `parser_error` 计数、不改变公开输出格式、不改变任何 SQL 审核行为。

## Tag 标注守卫

- **`scripts/verify_release_tag_annotation.py`** — 验证本地 tag 为 annotated tag（`git cat-file -t` 返回 `tag`），而非 lightweight tag。
- **`make release-tag-annotation-test`** — 运行离线单元测试（无需 tag 存在）。
- **`make release-tag-annotation-gate VERSION=vX.Y.Z`** — 在 tag 创建后验证（仅 post-tag 阶段）。
- **`.github/workflows/release.yml`** — 早期守卫步骤，在任何 artifact 发布前拒绝 lightweight tag。

## 非目标

- 不新增 parser 支持。
- 不实现 fallback parser。
- 不新增 SQL 审计规则。
- 不减少 parser_error 计数。
- 不改变 DDL 普查。
- 不改变公开输出格式。
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

`docs/decisions/2026-06-03-v0.251.0-release-tag-annotation-guard.md`
