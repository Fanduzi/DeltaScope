# DeltaScope v0.370.0 发行说明

## 概要 — TiDB RETURNING 方言边界

v0.370.0 升级了 TiDB parser，并为 DML `RETURNING` 划出清晰的方言边界。TiDB 方言现在接受 parser 可识别的 `RETURNING`（`INSERT ... RETURNING`、`UPDATE ... RETURNING` 以及单表 `DELETE ... RETURNING`），把它当作合法的 TiDB 语法。MySQL Server 不支持 DML `RETURNING`，因此在 MySQL 方言下，被解析出的 `RETURNING` 子句会发出一条专用的全局通知，而不是静默地通过审计。

`RETURNING` 不再被当作 PostgreSQL 专属语法标记。在这个版本之前，它会走到 PostgreSQL 误配提示；parser 升级后它能解析成功，于是对合法的 TiDB SQL 来说 PostgreSQL 提示就是错的。`RETURNING` 已从 PostgreSQL 误配标记列表中移除，`ON CONFLICT`、`::`、`ALTER COLUMN TYPE USING`、`GENERATED AS IDENTITY` 保持原有的 PostgreSQL 误配行为。

这是一次方言契约的修正，不是文案打磨。不存在 `severity` 字段，DeltaScope 继续使用 `level`。新 finding 只携带有限的元数据（当前方言、建议方言、标记名），不携带原始 SQL、RETURNING 列名、表达式、parser 片段、连接信息或凭证。

## 改了什么

- 升级 TiDB parser，使 DML `RETURNING` 在 MySQL/TiDB parser 路径上能为 `INSERT`、`UPDATE` 和单表 `DELETE` 解析成功。
- `spec.DML` 新增布尔 JSON 字段 `has_returning`，仅当解析出的语句确实带有 `RETURNING` 子句时才置位。该字段来自 parser，DeltaScope 不输出 RETURNING 列名、表达式、别名或 parser 子树。名为 `returning` 的标识符或别名不会置位。
- `RETURNING` 从 PostgreSQL 误配标记启发式中移除。`ON CONFLICT`、`::`、`ALTER COLUMN TYPE USING`、`GENERATED AS IDENTITY` 保持原有 PostgreSQL 误配通知。
- 新增不可配置的全局 finding `dialect.mysql.returning.unsupported.notice`（级别 `notice`），只在当前方言为 `mysql` 且解析出的 DML 带 `RETURNING` 子句时从成功路径发出。TiDB 不会触发（TiDB 支持 `RETURNING`），PostgreSQL 也不会触发。消息只面向 MySQL Server，并在 SQL 目标是 TiDB 时建议用 `--dialect tidb` 重跑。它不从原始 SQL 推断。
- SDK、CLI、HTTP、MCP 通过共享的审计结果一致地展示该行为。没有任何接入面新增专用的 RETURNING 接口。

## 哪些没变

- `level` 仍是公开的优先级字段。不引入 `severity` 字段。
- 规则目录不变（371 条规则）。这个版本调整 parser 方言行为并新增一条不可配置的全局 finding，不是已注册规则的变更。
- `ON CONFLICT`、`::`、`ALTER COLUMN TYPE USING`、`GENERATED AS IDENTITY` 的 PostgreSQL 误配行为不变。
- `REPLACE ... RETURNING` 本版本不支持，保持当前的 parser-error/unsupported 路径。目标 parser 不把 `RETURNING` 挂到 `REPLACE` 上。
- DeltaScope 不会自动切换方言。
- finding 与诊断不携带原始 SQL、parser 片段、RETURNING 列名、表达式或连接信息。

## 非目标

- 不做 MariaDB 方言。DeltaScope 不声称 MySQL 方言支持 MariaDB `RETURNING`。
- 不输出 `ReturningColumns`、`ReturningExpressions` 或 parser 子树。
- 不支持 `REPLACE ... RETURNING`，也不支持 parser 不支持的 multi-table `DELETE ... RETURNING` 变体。
- 不做允许或禁止 `RETURNING` 的可配置策略规则。
- 不做 fallback parser，也不从 raw parse-error SQL 推断。
- 不为 SDK、HTTP、MCP 新增专用接口；接入面一致性来自共享的审计结果。
- 不引入 `severity` 字段。

## 规则目录事实

规则目录与 v0.360.0 相同。本次修正 parser 方言行为，不是已注册规则的变更。

| 指标 | 数量 |
|------|----:|
| 规则总数 | **371** |

| 方言范围 | 规则 |
|----------|----:|
| common | 177 |
| postgresql | 191 |
| mysql | 1 |
| tidb | 2 |

| 语句类型 | 规则 |
|----------|----:|
| ddl | 361 |
| dml | 10 |

## 未变指标

- SQL 语料：**582/582**，**100.0%**，**247** YAML 夹具文件（较 245 增加；新增两个 `RETURNING` 方言边界语料用例）。
- PostgreSQL ALTER TABLE 配置条目：**53**（未变）。
- DDL 覆盖目录：**400** 条目（未变）。

## 决策记录

`docs/decisions/2026-06-23-v0.370.0-tidb-returning-dialect-boundary.md`
