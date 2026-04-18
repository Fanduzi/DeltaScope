# DeltaScope v0.36.1 发行说明

## 概述

DeltaScope v0.36.1 是 SQL corpus 覆盖率与发布置信度补丁版本。它不新增审计规则，也不扩大 parser 支持范围；本版本把现有 supported-rule corpus 契约做成明确、可见、会阻断发布漂移的验证面。

## 变更内容

### SQL Corpus 覆盖率门禁

`make sql-corpus-gates` 现在验证每个当前支持的 `rule_id × dialect` 表面至少有一个 SQL corpus 用例。

这个契约有意窄于“所有 policy key × 所有方言”。它跟踪当前稳定的 extractor/rule 支持面，避免在 PostgreSQL extractor 事实尚不可用时过度声称覆盖。

### SQL Corpus Inventory Report

`make sql-corpus-report` 会打印当前 supported-rule 覆盖 inventory，包括：

- 已发布 policy rule 数量
- 支持的 `rule_id × dialect` 目标数量
- 已覆盖目标数量
- 覆盖率百分比
- 各方言 corpus fixture 数量
- 已推迟的 rule surface

本次发布时的 inventory：

| 指标 | 值 |
|------|----|
| Policy rule IDs | 156 |
| 支持的 rule/dialect 目标 | 340 |
| 已覆盖 rule/dialect 目标 | 340 |
| 覆盖率 | 100.0% |
| Corpus expected YAML 文件 | 49 |
| MySQL fixtures | 13 |
| TiDB fixtures | 11 |
| PostgreSQL fixtures | 25 |
| Deferred rule IDs | 2 |

### 发布门禁集成

`release-test-gates` 现在会运行 SQL corpus coverage gate。`release-smoke` workflow 也包含显式的 supported-rule corpus coverage 步骤。

## 未变更的部分

- 无新增审计 rule ID。
- 无 parser 支持范围扩展。
- 无 spec 契约扩展。
- 无 MySQL、TiDB 或 PostgreSQL 行为变化。
- 无新增 CLI 标志。

## 安装

```bash
curl -fsSL https://raw.githubusercontent.com/Fanduzi/DeltaScope/v0.36.1/install.sh | \
  DELTASCOPE_VERSION=v0.36.1 sh
```

## 后续跟进

下一步产品能力工作应转向真实 PostgreSQL 支持面缺口，而不是继续增加 corpus plumbing，尤其是 PostgreSQL primary-key facts 与 PostgreSQL `ALTER TABLE ... ADD CONSTRAINT ... UNIQUE` 这类 index-like facts。
