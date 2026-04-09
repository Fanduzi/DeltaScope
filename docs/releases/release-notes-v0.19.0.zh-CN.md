# DeltaScope v0.19.0 发行说明

发布日期：2026-04-09

## 概述

DeltaScope `v0.19.0` 新增 **PostgreSQL 迁移安全规则包**和两个新的 CI 原生输出格式（`github-actions` 和 `sarif`）。迁移安全规则用于防范常见的 PostgreSQL DDL 模式，避免引发全表重写、长时间持锁或生产事故——无需数据库连接。

## 变更内容

### PostgreSQL 迁移安全规则（4 条）

四条新的离线安全规则，用于标记高风险的 PostgreSQL DDL 模式：

| 规则 ID | 捕获的问题 | 默认级别 |
|---------|-----------|:--------:|
| `ddl.pg.create_index.concurrently.require` | 不带 `CONCURRENTLY` 的 `CREATE INDEX` 持有排他锁，阻塞读写 | warning |
| `ddl.pg.alter.add_column.non_null_default.rewrite.warn` | 添加带有 volatile 默认值（如 `gen_random_uuid()`）的 `NOT NULL` 列会触发全表重写 | warning |
| `ddl.pg.alter.add_check.not_valid.require` | 不带 `NOT VALID` 的 `ADD CHECK` 约束需要持 `ACCESS EXCLUSIVE` 锁的全表扫描 | warning |
| `ddl.pg.alter.set_data_type.rewrite.warn` | 更改列类型（如 `varchar` 到 `integer`）可能需要全表重写 | warning |

这些规则仅在设置 `--dialect postgresql` 时生效，MySQL/TiDB 方言下自动跳过。

### GitHub Actions 输出格式

使用 `--format github-actions` 生成 CI 内联注解（`::error`、`::warning`、`::notice`），渲染在 GitHub Actions 工作流日志中。标题和消息中的特殊字符按照 GitHub 工作流命令规范进行转义。

```bash
deltascope audit --dialect postgresql --file ./migrations/20260409_add_index.sql --format github-actions
```

### SARIF 输出格式

使用 `--format sarif` 生成标准 SARIF 2.1.0 JSON，用于 GitHub Code Scanning、Azure DevOps 和其他 SARIF 消费方。规则元数据（来自 explanation suggestion 的帮助文本）放在 `tool.driver.rules` 下，严重级别映射到 SARIF 级别。

```bash
deltascope audit --file ./migrations.sql --dialect postgresql --format sarif > deltascope.sarif
```

### 输出中的规则摘要

JSON、markdown 和 quiet 输出现在包含规则摘要，显示已加载、适用和跳过的规则数量。在 JSON 中以 `rule_summary` 字段出现；在 markdown 中渲染为 `## Rule Summary` 和 `## Skipped Rules` 区段。GitHub Actions 和 SARIF 输出不包含规则摘要。

## 安装 / 升级

```bash
curl -fsSL https://raw.githubusercontent.com/Fanduzi/DeltaScope/v0.19.0/install.sh | \
  DELTASCOPE_VERSION=v0.19.0 sh
```

macOS 用户可以通过 Homebrew 安装：

```bash
brew tap Fanduzi/deltascope
brew install --cask deltascope
```

## 兼容性

无破坏性变更。`v0.19.0` 以增量方式扩展了现有审计契约：

- 所有现有的 MySQL/TiDB/PostgreSQL 离线和元数据感知行为保持不变
- 新的 PostgreSQL 迁移安全规则通过 `--dialect postgresql` 生效（其他方言自动跳过）
- 新的 `github-actions` 和 `sarif` 输出格式通过 `--format` 选择，不影响现有的 `markdown` 或 `json` 输出
- `rule_summary` 是 JSON 输出中的增量字段；忽略未知字段的现有消费者不受影响
