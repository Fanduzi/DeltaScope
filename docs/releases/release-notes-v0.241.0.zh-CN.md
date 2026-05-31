# DeltaScope v0.241.0 发行说明

## 概要 — Release Recovery Dry-Run Patch

v0.241.0 为 `release-recover.yml` 工作流增加默认安全的 `dry_run` 输入（默认 `true`）。在 dry-run 模式下，工作流会走完完整的恢复预检流水线 —— release asset/checksum 校验、Homebrew cask 渲染/验证（含 tap clone/diff）以及 npm 包状态检查 —— 但不执行任何破坏性或发布类副作用操作。真正的恢复仍需显式设置 `dry_run=false`。本补丁不改变 SQL 审核行为、parser 支持、审计规则、fallback parser 实现、parser_error 计数或任何面向产品的审核输出。

## Release Recovery Dry-Run

- **默认安全的 `dry_run` 输入**（`.github/workflows/release-recover.yml`）：恢复工作流新增必需的 `dry_run` 布尔输入，默认 `true`。操作人员必须显式设置 `dry_run=false` 才能执行真正的恢复操作。
- **Dry-run 验证范围**（dry-run 模式下会执行的部分）：
  - Release asset 和 checksum 预检。
  - Homebrew cask 渲染/验证：clone tap，渲染 cask，显示与当前 cask 文件的 `git diff --stat`。
  - npm 包状态检查：验证目标版本是否已存在于 registry。
- **Dry-run 不执行**（dry-run 模式下被抑制的部分）：
  - Homebrew tap push。
  - Homebrew install 验证。
  - npm publish。
  - GitHub release 上传或删除。
  - Git tag 操作。
- **真正的恢复**：操作人员必须显式设置 `dry_run=false` 才能执行实际的 Homebrew tap push、npm publish 或 Homebrew install 验证。

## 更新的 Make Targets

| Target | 用途 |
|--------|------|
| `make release-recovery-contract-test` | 验证 dry-run 默认值和恢复工作流的契约门禁 |

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

`docs/decisions/2026-05-31-v0.241.0-release-recovery-dry-run-patch.md`
