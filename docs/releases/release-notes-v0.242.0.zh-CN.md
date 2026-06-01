# DeltaScope v0.242.0 发行说明

## 概要 — Release Recovery CI Polish

v0.242.0 正式包含 v0.241.0 之后的 `GH_TOKEN` preflight 修复，并增加防回归门禁，确保 release recovery 工作流的 preflight 认证 wiring 不会被静默删除。`make release-recovery-contract-test` 目标现在会静态检查 `.github/workflows/release-recover.yml` 的 preflight 步骤包含 `GH_TOKEN: ${{ secrets.GITHUB_TOKEN }}`。本版本不改变 SQL 审核行为、parser 支持、审计规则、fallback parser 实现、parser_error 计数或任何面向产品的审核输出。

## Release Recovery Preflight 认证修复

- **Preflight GH_TOKEN wiring**（`.github/workflows/release-recover.yml`）：恢复工作流的 preflight 步骤现在包含 `GH_TOKEN: ${{ secrets.GITHUB_TOKEN }}`，使 `gh release download` 和 `gh release view` 命令正常执行。此修复在 v0.241.0 发布后以 `0e3f31c` 提交；v0.242.0 是第一个包含该修复的 tagged release。
- **防回归门禁**（`Makefile`）：`make release-recovery-contract-test` 现在静态检查工作流文件中 `GH_TOKEN` 的 wiring 是否存在。如果未来的编辑删除了该 wiring，契约测试将会失败。

## 非目标

- 不新增 parser 支持。
- 不新增 SQL 审计规则。
- 不实现 fallback parser。
- 不减少 parser_error 计数。
- 不实现完整 MySQL/TiDB/PostgreSQL DDL 支持。
- 不实现方言对等。
- 不实现运行时/catalog 校验。
- 不改变任何 SQL 审核行为。
- 不改变 DDL 普查数据。
- 不改变 PG ALTER TABLE 规则计数。

## Parser-Error 计数（未变）

| 方言 | Parser Error |
|------|:------------:|
| MySQL | 15 |
| TiDB | 9 |
| PostgreSQL | 5 |
| **合计** | **29** |

## Parser-Error 可行性分类（未变）

| 分类 | MySQL | TiDB | PostgreSQL | 合计 |
|------|-------|------|------------|------|
| `parser_upgrade_candidate` | 5 | 0 | 5 | 10 |
| `bounded_fallback_candidate` | 1 | 3 | 0 | 4 |
| `product_unsupported_or_inapplicable` | 0 | 6 | 0 | 6 |
| `unsafe_fallback_defer` | 9 | 0 | 0 | 9 |
| `needs_research` | 0 | 0 | 0 | 0 |

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

`docs/decisions/2026-06-01-v0.242.0-release-recovery-ci-polish.md`
