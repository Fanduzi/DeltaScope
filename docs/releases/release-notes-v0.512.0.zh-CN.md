# DeltaScope v0.512.0 发行说明

发布日期：2026-11-16

## 概要

v0.512.0 落地 MySQL/TiDB 官方 DDL 语义验收里程碑（#79，T01–T32）。MySQL 5.7.44、8.0.46、8.4.10 与 TiDB v8.5.0 官方清单内的每条 DDL 现在都落到显式分类——策略违规、证据缺口、unsupported 分析、parser 失败或已审计结果——不再因 parser 能识别就静默通过。

验收 oracle 可执行且版本化：31 个 canonical golden manifest 在锁 digest 的 anchor 上跑真实数据库用例（mysql:5.7.44、mysql:8.0.46、mysql:8.4.10 linux/amd64·arm64、pingcap/tidb:v8.5.0 arm64），`TASK=all` 聚合校验对分母收缩、产物缺失、用例跳过、git HEAD 混杂一律 fail-closed。里程碑终跑记录 2565 个用例、50875 条断言，全部通过 @`8816924f`。

DeltaScope 仍是静态分析：不执行提交的 SQL、不返回查询结果，也不做授权判定。

## 新审计切片

- **存储体（T27–T30）**：MySQL procedure、function、trigger、event 体内的静态 SQL 按成员扫描为 typed statement。控制流复合块（IF/CASE/循环）保持 member-initial 语义；有界可执行成员使已证字面绑定失效；`OLD.<col>`/`NEW.<col>` 词法绑定到事件语义并记录列级依赖；事件调度（AT/EVERY/STARTS/ENDS/ON COMPLETION/DISABLE ON SLAVE）提取为 typed 成员。成员级 bound 让兄弟语句继续被审计，而不是整体吞掉。
- **TiDB 家族（T19–T23S）**：TTL 生命周期（含原生拒绝证据与复合 ALTER 拒绝处理）；placement policy 生命周期、bind 与 `ALTER RANGE`；sequence；resource group（CREATE/ALTER/DROP 审计，`SET RESOURCE GROUP` 与 BACKGROUND 成员保持有界）；物理布局、AUTO_ID_CACHE 与缓存表（TiKV/TiFlash 车道）。
- **MySQL 低频 DDL（T24–T26）**：resource group（8.0.3+ 解析版本门 + 每次运行绑定的 live deployment evidence：`@@server_uuid`、nproc、CAP_SYS_NICE）；InnoDB tablespace 生命周期（occupancy 感知 drop、rename adjudication）；SERVER、空间参考系统与 ALTER INSTANCE（privilege premise adjudication + 按 form 版本门）。
- **有序预期 schema 状态**：迁移批次按 authored 顺序评估，带 proven-absence 通道、污染 barrier 与惰性白名单。

## 边界与版本语义

- 已识别但未支持的语句携带命名 `boundary` 标记（`unaudited`、`vendor`）与欠账 premise——例如 TiDB `ALTER INSTANCE RELOAD TLS` 持 `alter_instance.reload_tls` unaudited 边界，带 `object:instance` 与 `gap_fact:alter_instance.version_applicability`（T32）。MySQL 专属的 `ROTATE INNODB MASTER KEY` 保持 TiDB vendor 边界。
- 版本门按 form 生效：RELOAD TLS MySQL 8.0+、ROTATE 5.7.11+、SRS 8.0.4+、trigger FOLLOWS/PRECEDES 5.7.2+、resource group 8.0.3+——缺失或越界版本一律 unproven，绝不放行。
- 新规则含 `ddl.event.enabled_execution.warn`（ENABLED 默认 / DISABLE ON SLAVE 证据缺口）、`ddl.trigger.ordering.version`（5.7.2 前 ordering 拒绝）以及 trigger/event 生命周期 notice。

## 基础设施

- `make ddl-golden TASK=all` 顺序执行 canonical manifest 集（每任务独立 compose 生命周期）；`reaggregate` 从已落盘产物重建聚合，拒绝缺失或混 HEAD 证据；`anchors-baseline.json` 的 `required_all_tasks` 锁定 31 任务分母。
- `deployment_evidence` setup 步在审计前探测活 anchor 的 server UUID、版本、vCPU 与 capability 位——已提交的证据文件只保留 provenance 形状。
- DDL 清单（`testdata/ddl-inventory/inventory.yaml`）每行携带 file-bound 证据；能力矩阵与双语覆盖文档同步 audited-vs-boundary 分界。

## 非目标

- 不执行 SQL，不做授权，也不是 SQL syntax 或 grammar coverage 声明。
- schema 级 `ALTER DATABASE ... PLACEMENT POLICY` 保持显式 unaudited 边界；表级 placement 已审计。
- TiDB `ALTER INSTANCE` 在某切片证明 version applicability 前保持 unaudited；MySQL 侧才是已审计方言。
- MCP 仍没有 Query Access 工具。

## 规则目录事实

| 指标 | 数量 |
|------|------:|
| 规则总数 | **531** |
| blocker | 162 |
| warning | 183 |
| notice | 186 |

Catalog 现覆盖 517 条 DDL 与 14 条 DML，按方言分为 common（301）、PostgreSQL（191）、MySQL（13）、TiDB（36）。总数包含三条默认关闭的 `dml.impact.*` opt-in 规则。它不是 Default Policy 计数，也不是 Loaded。

## 语料与目录事实

- 支持的 rule-and-dialect fixture coverage：**790/790**、**100.0%**、**356** 个 YAML fixture；这不是 SQL syntax 或 grammar coverage。
- PostgreSQL ALTER TABLE 配置项：**53**。
- DDL 覆盖目录：**410** 条（mysql 63，tidb 57，postgresql 290，parser_upgrade_candidate 6）。

## 证据

- `TASK=all` 聚合 @`8816924f`：31 任务、2565 用例、50875 断言全过；`validate --task all` 产物 valid。
- `make test`：39 包零失败；golden 契约 890/890；清单 gate 全部 file-bound；PostgreSQL 门与 npm launcher 测试全绿。
- `docs/decisions/` 下每条边界提升与暂缓 premise 均有决策记录（T19–T32）。
