# DeltaScope v0.510.4 发行说明

发布日期：2026-09-06

## 概要

v0.510.4 把拆开的公开信号命名清楚，避免把 Verdict 当成 Fail Threshold、把 Rule Catalog 当成 Loaded，或在 MCP/CLI/HTTP 上对缺失能力保持沉默。GitHub issues [#65](https://github.com/Fanduzi/DeltaScope/issues/65)–[#76](https://github.com/Fanduzi/DeltaScope/issues/76) 进入官方发布包。

DeltaScope 仍是静态分析：不执行提交的 SQL、不返回查询结果，也不做授权判定。MCP 没有 Query Access 工具。Rule Catalog 为 **376** 条，其中三条默认关闭的 `dml.impact.*` opt-in 规则。Default Policy 不启用这三条。支持的 rule-and-dialect fixture coverage 仍为 586/586（100.0%），共 286 个 YAML fixture；这不是 SQL syntax 或 grammar coverage。

## 修复

- [#65](https://github.com/Fanduzi/DeltaScope/issues/65)：CLI JSON 增加并列字段 `fail_on_triggered`。仅 notice 且 `--fail-on notice` 时 Verdict 仍是 `pass`，进程退出码非 0。SDK / HTTP / MCP Result 不加该字段。
- [#66](https://github.com/Fanduzi/DeltaScope/issues/66)：`install.sh` 在拼 GitHub 下载 URL 前，给裸 `MAJOR.MINOR.PATCH` 补 `v`。
- [#67](https://github.com/Fanduzi/DeltaScope/issues/67)：Catalog、Default Policy、Loaded 与 `fk_forbid` 抑制是不同事实。`config status` 报告外键命名抑制，而不是缺失规则。
- [#68](https://github.com/Fanduzi/DeltaScope/issues/68)：空 `--sql` 在 `audit` 仍为 exit 2，在 `query-access analyze` 仍为 exit 3。错误文案分别为 `audit:` 与 `query-access:`。
- [#69](https://github.com/Fanduzi/DeltaScope/issues/69)：MCP launcher 保持 `engines.node >=24`，低于 Node 24 在下载或启动前 fail-closed。
- [#70](https://github.com/Fanduzi/DeltaScope/issues/70)：未打 tag / `go install @main` 的构建报告 Go module 或 VCS 信息。`DefaultVersion` 只做 fallback。
- [#71](https://github.com/Fanduzi/DeltaScope/issues/71)：PR 与 `main` 运行 `go test ./...` 和 PostgreSQL unit 门禁。重 e2e 仍在发版。
- [#72](https://github.com/Fanduzi/DeltaScope/issues/72)：`get_capabilities` 声明 `query_access: { available: false, surfaces: ["cli", "http"] }`。不增加 Query Access MCP 工具。
- [#73](https://github.com/Fanduzi/DeltaScope/issues/73)：GitHub Action 示例 pin 跟踪当前稳定 tag。
- [#74](https://github.com/Fanduzi/DeltaScope/issues/74)：`dml.impact.*` 进入 Rule Catalog 且默认关闭。Default Policy 不启用它们。默认 UPDATE/DELETE 仍带语句 impact 对象。
- [#75](https://github.com/Fanduzi/DeltaScope/issues/75)：`audit -h` 与 `query-access analyze -h` 打印帮助。Host 为 `-H` / `--host`。
- [#76](https://github.com/Fanduzi/DeltaScope/issues/76)：HTTP `/v1/audit` 把 MCP 的 `connection_ref` 报成点名 `connection_id` 的 `invalid_request`，而不是不透明的 `invalid_json`。

## 非目标

- 不在 Fail Threshold 触发时抬高 Verdict。
- 不统一空 SQL 退出码。
- 不把 npm `engines` 降到 Node 20。
- 不增加 MCP Query Access 工具。
- 不在 Default Policy 中启用 `dml.impact.*`。
- 不执行 SQL，不做授权，也不是 SQL syntax 或 grammar coverage 声明。

## 规则目录事实

| 指标 | 数量 |
|------|------:|
| 规则总数 | **376** |
| blocker | 73 |
| warning | 144 |
| notice | 159 |

总数包含三条默认关闭的 `dml.impact.*` Catalog 行。它不是 Default Policy 计数，也不是 Loaded。

## 语料与目录事实

- 支持的 rule-and-dialect fixture coverage：**586/586**、**100.0%**、**286** 个 YAML fixture；这不是 SQL syntax 或 grammar coverage。
- PostgreSQL ALTER TABLE 配置项：**53**。
- DDL 覆盖目录：**407** 条（mysql 62，tidb 55，postgresql 290，parser_upgrade_candidate 18）。

## 决策记录

- [2026-09-04 named public signals](../decisions/2026-09-04-named-public-signals.md)（[#65](https://github.com/Fanduzi/DeltaScope/issues/65)–[#74](https://github.com/Fanduzi/DeltaScope/issues/74)）是本次补丁的 Accepted 边界记录。
