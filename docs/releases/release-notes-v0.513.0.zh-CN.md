# DeltaScope v0.513.0 发行说明

发布日期：2026-11-15

## 概述

v0.513.0 完成证据驱动的语义验收计划：PostgreSQL 官方 DDL 验收、跨全部方言语句面的官方 DML 分母、两项长期版本适用性欠账兑现，以及首个 parser-upgrade 候选毕业为真实审计。canonical golden 集扩至 **50 任务 —— 3,360 用例 · 59,177 断言 —— 全部在单一 git HEAD（`c9192cb3`）通过**。

DeltaScope 仍是静态分析工具：不执行提交的 SQL、不返回查询结果、不做授权决策。

## PostgreSQL 17 DDL 验收（#115，T33–T49）

- **官方分母**：从 PostgreSQL 17 命令清单蒸馏出 143 行 inventory，每行带按锚点划分的 cell 与绑定 `postgres:17.7`（digest 锁定 `postgres@sha256:2006...`，linux/arm64）的 568 个探针证据 cell。
- **作用域锚点**：manifest 可声明 `anchor_scope`——`postgresql` scope 只证明 postgres17 锚点 + postgresql 方言，`tidb` scope 只证明 tidb85 + tidb 方言，默认 flat scope 保持四锚点分母诚实。baseline 对 scope 漂移 fail-closed。
- **13 个家族 manifest**（T35–T47）覆盖 create-table、alter-table 子动作矩阵、drop/truncate/cluster/reindex、索引、视图/物化视图、类型/域/例程、触发器/规则/策略/事件触发器、序列/统计信息/扩展、FDW/服务器/外部表、文本搜索、发布/订阅（`wal_level=logical` 逻辑复制），以及角色/授权/注解/表空间/大对象/ALTER SYSTEM。
- **残余分类**（T48）：全部 silent 与 parser_error cell 以 golden 证据分类；`restart: on-failure` 修复消除了 TiKV region-unavailable 启动竞态（此前会阻塞 T23S 聚合）。

## 官方 DML 验收（#133，T50–T56）

- **44 行 DML 分母**（17 MySQL、18 TiDB、9 PostgreSQL），全部官方文档化 DML 动词按 cell 探针分类。
- **T51** MySQL 变动动词：四锚点 INSERT/REPLACE/UPDATE/DELETE/DO——63 用例、696 断言。
- **T52** MySQL 查询/工具动词：SELECT 家族、UNION、8.0+ 形态（TABLE/VALUES/WITH/EXCEPT/INTERSECT/FOR SHARE）及 HANDLER/CALL/LOAD DATA/XML/IMPORT TABLE 边界——49 用例、480 断言。
- **T53** TiDB 原生 DML（新 `tidb` scope）：IMPORT INTO 钉为显式 unsupported 边界并附活锚点原生拒绝证据——26 用例、256 断言。
- **T54** PostgreSQL DML（postgres17）：INSERT/UPDATE/DELETE 变体（RETURNING、ON CONFLICT、USING、FROM）被审计；SELECT/MERGE/COPY/TABLE/VALUES/WITH 保持显式 unsupported，但 psql 在活锚点上逐条执行——vendor 可执行但审计方明确拒绝，绝不静默——32 用例、346 断言。

## 欠账兑现与 parser 毕业

- **T57（#141）**：`ALTER TABLESPACE <name> ENGINE [=] <engine>` 由 parser-upgrade 候选毕业为可审计——`AlterAction="engine"`，`ENGINE=InnoDB` 触发 `ddl.mysql.tablespace.alter.warn`，NDB 保持显式 engine 边界，裸 `ENGINE` 保持 parser error。**MySQL parser-upgrade 候选归零**；五个 PostgreSQL 形态保留为候选，待 libpg_query 升级。
- **T58（#142）**：`srs.version_applicability` 前提在活锚点上得证——observed 5.7.44 与 8.4.10 钉 `version_inapplicable` + `gap_fact`，observed 8.0.46 正常审计（`ddl.mysql.srs.drop.absent`），tidb85 保持 vendor 边界，离线 `--target-version` pin 保持 `version_unproven`（pin 仅是冲突检查，从不是观测事实）。
- **T59（#143）**：`alter_instance.version_applicability` 前提以钉死边界方式结清——活 tidb85 证据挂到全部 subaction cell，promotion 前提延期，因为 TiDB 未文档化 ALTER INSTANCE 的审计适用版本。

## Bug 修复与工具

- **fix(rules)**：evidence-gap `required_facts` 现按字典序输出——消除了 map 遍历序导致 `table.columns`/`table.auto_random_bits` 顺序抖动的 flake。
- **lint 基线清零**：golangci-lint CI 门转绿（errcheck/unparam/unused/gocritic/gosec/staticcheck，48 文件），无语义变更。

## 验证

- `TASK=all` 聚合 @ `c9192cb3`：50 canonical 任务，3360/3360 用例，59,177 断言，单 HEAD，artifact valid。
- `go test ./...` 全绿（含 `-tags postgresql`）；896/896 golden 契约用例；`ddl-inventory-gate` 绿。
- 决策记录：DML 验收收口、ALTER TABLESPACE ENGINE 毕业、SRS 活版本门证据、ALTER INSTANCE 前提延期。

## 破坏性变更

无。
