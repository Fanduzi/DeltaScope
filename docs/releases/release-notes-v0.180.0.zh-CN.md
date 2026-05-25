# DeltaScope v0.180.0 发行说明

## 摘要 — 发布面一致性门禁

v0.180.0 是一个发布工程里程碑，新增了发布面一致性检查器到发布门禁流水线。本版本**不新增** SQL parser 支持、不新增审计规则、不改变产品行为。

## 变更内容

- 新增发布面一致性检查器（`scripts/verify_release_consistency.py`），验证发布域事实在所有发布面之间保持一致：着陆页、发行说明 EN/ZH、变更记录、路线图、规则参考和能力矩阵。
- `make release-version-surface-gates` 现在同时运行现有的 shell 版本面检查器和新的 Python 发布语义一致性检查器。
- `make release-contract-gates` 通过依赖继承新检查器。
- 新增 `make release-consistency-test` 目标运行一致性检查器测试套件。
- 决策记录：`docs/decisions/2026-05-25-v0.180.0-release-surface-consistency-gates.md`。

## 保护面

一致性检查器强制执行：

- 着陆页最近发布卡片不会漂移到旧版本序列。
- 残留普查数字（如 `finding_covered`）与版本特定事实匹配。
- 过时措辞模式（如 "unsupported_boundary N→N"）被捕获。
- PG ALTER TABLE 规则数量不会在版本之间漂移。
- EN/ZH 规则 ID 和数字奇偶性保持一致。
- 发布过度声明模式在非 no-leak/非目标上下文中被标记。
- 禁止的 payload 术语只出现在 no-leak 上下文内。

## 负面测试覆盖

测试套件（`scripts/test_verify_release_consistency.py`）覆盖：

- 着陆页中过时的最近发布卡片。
- 错误的 `finding_covered` 计数（如 64 而非 60）。
- 过时的 "unsupported_boundary N→N" 措辞。
- 发行说明中缺少 ZH 规则 ID。
- 发行说明中的正面过度声明。
- No-leak 上下文之外的禁止 payload。

## 不变指标

v0.180.0 不改变 SQL parser、规则或产品行为：

- PostgreSQL ALTER TABLE 规则数：**32**（不变）。
- 残留普查：**66/60/2/0/4/0**（不变）。
- SQL 语料库：**535/535**，**100.0%**，**243 YAML 文件**（不变）。

## 非目标

- 非新 SQL 规则或 parser 特性发布。
- 非完整 PostgreSQL ALTER TABLE 支持。
- 非 PostgreSQL 18 parser 支持。
- 非运行时/在线验证。
- 非改写时长估算。
- 非 v1.0/稳定 API 合同声明。
