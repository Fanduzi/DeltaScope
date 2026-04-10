# DeltaScope v0.20.0 发行说明

发布日期：2026-04-10

## 概述

DeltaScope `v0.20.0` 新增 **PostgreSQL 信任与误配防护**——一组增量行为，帮助识别 SQL 可能使用了与当前审计方言不同的方言，同时让迁移安全规则的建议更具可操作性。不新增规则；不改变已有规则 ID、级别或触发条件。

## 变更内容

### PostgreSQL 语法启发式通知

在 MySQL/TiDB 路径审计时，DeltaScope 会检测常见的 PostgreSQL 专属语法标记，发出建议 `--dialect postgresql` 的 advisory notice。这有助于在 CI 管道中尽早发现方言误配——尤其是方言标记配错时容易忽略的场景。

检测标记：

| 标记 | 示例 |
|------|------|
| `RETURNING` | `INSERT INTO users(…) VALUES (…) RETURNING id` |
| `ON CONFLICT` | `INSERT INTO users(…) VALUES (…) ON CONFLICT DO NOTHING` |
| `::` 类型转换 | `SELECT id::bigint FROM users` |
| `ALTER COLUMN TYPE USING` | `ALTER TABLE users ALTER COLUMN score TYPE bigint USING abs(score)` |
| `GENERATED AS IDENTITY` | `id bigint GENERATED ALWAYS AS IDENTITY` |

关键行为：
- DeltaScope **不会自动切换方言**——通知仅作为建议。
- 告警包含明确的风险说明和可操作的下一步建议。
- 字符串字面量、引号标识符和注释中的标记被排除，避免误报。

```bash
deltascope audit --sql "insert into users(id) values (1) returning id;" --format markdown
```

示例输出：

```text
## Audit Context
- Mode: `offline`
- Dialect: `mysql` (default)
- Trust Note: Dialect remains `mysql` (default). DeltaScope did not auto-switch dialect.

## Global Findings

- [notice] `dialect.postgresql.syntax.detected.notice`: SQL looks like PostgreSQL because it uses "RETURNING" syntax; if you are auditing PostgreSQL, pass --dialect postgresql
  Risk: DeltaScope does not auto-switch dialect. Auditing PostgreSQL SQL with the MySQL/TiDB parser can produce misleading parse errors or incomplete findings.
  Suggestion: If this SQL targets PostgreSQL, re-run with --dialect postgresql to get accurate findings. If not, you can safely ignore this notice.
```

### 显式 PostgreSQL 能力边界错误

当 PG-capable 构建版本遇到尚未支持的 PostgreSQL 专属功能（如完整的 PostgreSQL DDL 解析）时，现在返回类型化的 `PostgreSQLCapabilityBoundaryError`，取代之前的启发式字符串匹配。CI 管道和工具集成可以更直接地区分真正的解析失败和已知的能力限制。

### CLI 输出信任信号

CLI 的信任导向输出格式现在都包含信任上下文：

| 格式 | 新增内容 |
|------|---------|
| **Markdown** | `## Audit Context` 区段，包含模式、方言来源，以及 PG 语法告警时的信任提示 |
| **JSON** | 顶层 `context` 对象，包含 `mode`、`dialect`、`dialect_source` |
| **Quiet** | `[context]` 行显示模式/方言/来源，`[summary]` 行显示加载/适用/跳过规则数 |

规则摘要（已加载、适用、跳过）现在在 CLI 的 `json`、`markdown` 和 `quiet` 格式中可见，方便确认当前方言下哪些规则运行了、哪些被跳过。`github-actions` 和 `sarif` 格式只输出告警结果，不包含规则摘要元数据。

### PG 迁移安全规则建议质量提升

v0.19.0 引入的四条 PostgreSQL 迁移安全规则现在提供分步迁移指导：

| 规则 | 改进后的建议 |
|------|------------|
| `ddl.pg.create_index.concurrently.require` | 提示 `CONCURRENTLY` 不能在事务内运行 |
| `ddl.pg.alter.add_column.non_null_default.rewrite.warn` | 四步安全路径：nullable → backfill → default → not null |
| `ddl.pg.alter.add_check.not_valid.require` | 两步 `NOT VALID` → `VALIDATE CONSTRAINT`，附带锁级别说明 |
| `ddl.pg.alter.set_data_type.rewrite.warn` | 分阶段影子列迁移策略 |

### 误报修复

PostgreSQL 语法启发式不再对以下场景中的标记触发：
- 字符串字面量（`'returning'`）
- 双引号标识符（`"returning"`）
- 反引号标识符（`` `returning` ``）
- 行注释（`-- returning`）
- 块注释（`/* returning */`）

### 元数据请求兼容性修复

在公共 `pkg/deltascope` API 中混合使用顶层 `Schema`/`MetadataProvider` 字段与旧版 `Metadata` 结构体时，不再在审计准备阶段丢失 schema 或 provider 上下文。

## 安装 / 升级

```bash
curl -fsSL https://raw.githubusercontent.com/Fanduzi/DeltaScope/v0.20.0/install.sh | \
  DELTASCOPE_VERSION=v0.20.0 sh
```

macOS 用户可以通过 Homebrew 安装：

```bash
brew tap Fanduzi/deltascope
brew install --cask deltascope
```

## 兼容性

无破坏性变更。`v0.20.0` 以增量方式扩展了现有审计契约：

- 所有现有的 MySQL/TiDB/PostgreSQL 离线和元数据感知行为保持不变
- PostgreSQL 语法通知是新的 `notice` 级别全局告警——除非设置 `--fail-on notice`，否则不改变退出码
- 规则 ID、严重级别和触发条件不变
- 类型化的能力边界错误是新的错误类型；使用 `errors.Is` 或 `errors.As` 的现有错误路径消费者继续正常工作
- `rule_summary` 和 `context` 是 JSON 输出中的增量字段；忽略未知字段的现有消费者不受影响
