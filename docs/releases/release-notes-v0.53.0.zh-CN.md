# DeltaScope v0.53.0 发行说明

## 概要

v0.53.0 新增 PostgreSQL `REFRESH MATERIALIZED VIEW` 离线审核覆盖。DeltaScope 现在将所有四种刷新变体通过审核管线进行标准化处理，并新增两条 PostgreSQL-only 规则：对非并发刷新给出警告，对 `WITH NO DATA` 刷新给出提示——后者可能导致下游读取方看到空结果。

## 标准化形式

| SQL | 标准化操作 |
|-----|-----------|
| `REFRESH MATERIALIZED VIEW mv` | `refresh_materialized_view` |
| `REFRESH MATERIALIZED VIEW CONCURRENTLY mv` | `refresh_materialized_view` |
| `REFRESH MATERIALIZED VIEW mv WITH DATA` | `refresh_materialized_view` |
| `REFRESH MATERIALIZED VIEW mv WITH NO DATA` | `refresh_materialized_view` |

## 新增 PostgreSQL-only 规则

| 规则 ID | 触发条件 | 默认级别 |
|---------|---------|---------|
| `ddl.pg.refresh_materialized_view.concurrently.warn` | 非并发刷新（默认或显式 `WITH DATA`） | warning |
| `ddl.pg.refresh_materialized_view.no_data.notice` | `REFRESH MATERIALIZED VIEW ... WITH NO DATA` | notice |

- `CONCURRENTLY` 刷新通过两条规则均不产生 finding。
- `WITH NO DATA` 同时触发两条规则，因为它也是非并发的。

## 测试覆盖

- AST 普查测试记录所有四种刷新变体的稳定解析器事实。
- 解析器/提取器标准化测试。
- 两条规则的语料库 fixture。
- 通过 `AuditSQL` 对所有四种变体进行服务级测试。
- 四个公共面测试：`pkg/deltascope` Audit、CLI Execute、HTTP handler 和 MCP audit_sql tool。

## 非目标

- 不提供 `CONCURRENTLY` 所需的唯一索引在线验证。DeltaScope 不会检查物化视图上是否存在唯一索引。
- 不对底层视图查询进行查询分析、成本分析或依赖分析。
- 不影响 MySQL/TiDB 行为。
- 除新增两条 PostgreSQL-only 规则外，不改变默认策略。

## 安装

**macOS（推荐）：**

```bash
brew tap fanduzi/deltascope
brew install --cask deltascope
```

**通用安装脚本：**

```bash
curl -fsSL https://raw.githubusercontent.com/Fanduzi/DeltaScope/v0.53.0/install.sh | \
  DELTASCOPE_VERSION=v0.53.0 sh
```
