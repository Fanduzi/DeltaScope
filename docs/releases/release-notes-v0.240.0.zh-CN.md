# DeltaScope v0.240.0 发行说明

## 概要 — Release Workflow Idempotency & Recovery Hardening

v0.240.0 加固发布流水线，防止下游部分失败和误触发重跑。新增 release asset 与 npm 包预检验证工具、手动 `release-recover.yml` 恢复工作流，以及全量发布流程的幂等守卫（已有 asset 时阻止 GoReleaser 重跑）。本版本**不**改变 SQL 审核行为、不新增 parser 支持、不新增审核规则、不实现 fallback parser、不降低 parser_error 数量。

## 发布安全改进

- **发布恢复预检工具**（`scripts/verify_release_assets.py`、`scripts/verify_npm_package_state.sh`）：只读验证指定版本的 release asset、checksum 和 npm 包状态。全量发布和恢复工作流共用。
- **手动发布恢复工作流**（`.github/workflows/release-recover.yml`）：运维触发，重跑指定的下游发布任务（npm publish、Homebrew、GitHub release body）。不会重跑 GoReleaser 或覆盖已有 asset。
- **全量发布幂等守卫**（`.github/workflows/release.yml`）：全量发布工作流在运行 GoReleaser 之前检查已有 release asset。如果 asset 已存在，工作流早失败并提示使用恢复工作流。
- **运维文档**（`docs/dev/release-recovery.md`）：发布恢复操作流程和预检工具使用说明。

## 新增 Make 目标

| 目标 | 用途 |
|------|------|
| `make release-recovery-preflight VERSION=vX.Y.Z` | 验证已发布版本的 release asset 和 npm 状态 |

## 非目标

- 不新增 parser 支持。
- 不新增 SQL 审核规则。
- 不实现 fallback parser。
- 不降低 parser_error 数量。
- 不声称 full MySQL/TiDB/PostgreSQL DDL support。
- 不声称 dialect parity。
- 不声称 runtime/catalog validation。
- 不改变任何 SQL 审核行为。

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

## 决策记录

`docs/decisions/2026-05-30-v0.240.0-release-workflow-idempotency-recovery-hardening.md`
