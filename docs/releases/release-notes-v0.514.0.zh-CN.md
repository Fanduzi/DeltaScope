# DeltaScope v0.514.0 发行说明

发布日期：2026-11-16

## 概述

v0.514.0 将 PostgreSQL 已连接元数据的 DML 目标表存在性检查与 MySQL、TiDB 对齐，补充跨方言 DML 回归 corpus，并修复 Homebrew cask 的弃用指令。

DeltaScope 仍然只做静态分析：不执行提交的 SQL，不返回查询结果，也不决定执行权限。

## PostgreSQL DML 目标表存在性（#145，T60）

- 对已连接元数据且只有一个写入目标的 PostgreSQL `INSERT`、`UPDATE`、`DELETE` 审计，当元数据证实目标表不存在时，现在会产生 `dml.table.exists.require` **blocker**。规则是否启用及 finding level 仍由策略配置决定。
- 在所有方言中，带 schema 限定的 DML 目标均使用 SQL 中指定的 schema 读取元数据。未限定的 PostgreSQL 目标继续使用请求 schema；这不是 PostgreSQL `search_path` 解析。
- 离线 PostgreSQL 审计不变：没有 provider 就没有存在性事实，该规则不会虚构结论。未知元数据不等于目标不存在。
- 这是有意的审计结果变化：受影响的审计可能返回 `reject`，并触发调用方设置的 blocker fail threshold。结果结构和规则 ID 不变。

## 证据与回归覆盖（T61–T62）

- PostgreSQL live golden 用例固定缺失目标 blocker 与已存在目标对照，并通过 post-verification 证明审计未修改 fixture。
- MySQL `ALTER TABLESPACE ... ENGINE` 元数据用例固定目标存在与缺失的区别。这是在补强已发布审计切片的证据，不是扩大 parser 支持范围。
- 新增 MySQL、TiDB、PostgreSQL DML corpus fixture，覆盖写入 finding、正常对照、基于元数据的存在性检查，以及明确的 unsupported / parser-error 边界。不支持语法和解析错误仍属于不完整审计，不能算作检查成功。
- 保留的 **`6a4f14e7`** 上 `TASK=all` 聚合包含 **50 个任务、3,361/3,361 个用例、59,206 条断言**，全部来自单一 HEAD。这是既有行为证据基线，不表示已在 release tag 上重跑聚合。

## Homebrew

cask renderer 将弃用的 `postflight` 改为 `postflight_steps`，保留原有 trust / quarantine 提示。

## 兼容性与剩余边界

公共 API 和输出结构没有破坏性变更。新增的目标缺失 blocker 可能改变上述 verdict 和进程退出状态。MySQL / TiDB DML 行为不变。五个 PostgreSQL parser-upgrade 候选仍固定为 parser error，等待上游 binding 发布（#149）；本版本不提升 generic notice 或 mixed-coverage inventory 行的状态。

### 维护者说明

有界元数据契约与证据依据见 [PostgreSQL DML 目标表存在性对齐](../decisions/2026-11-16-postgresql-dml-table-existence-parity.md) 和 [tablespace ENGINE 元数据存在性证据](../decisions/2026-11-16-tablespace-engine-meta-existence-evidence.md)。
