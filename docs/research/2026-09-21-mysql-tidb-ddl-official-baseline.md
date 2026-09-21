# MySQL / TiDB DDL 官方外部分母

核验日期：2026-09-21。用途：给本仓库 DDL 覆盖盘点提供独立于已有规则的检查清单；不是支持承诺，也不是“每行都已覆盖”的结论。

## 版本与证据范围

- 审计 E2E 使用 MySQL `8.4`、TiDB `v8.5.0`，见 [compose](../../docker/cli-e2e-compose.yaml)。MySQL 标签未锁定补丁版本。
- query-access builtin 场景另有 MySQL `5.7.44`、`8.0.46`、`8.4.10` 与 TiDB `v8.5.7`，见 [独立 compose](../../docker/query-access-builtin-compose.yaml)。这些配置不能替代 DDL 审计版本验收。
- 本文用 MySQL 8.4 手册和 PingCAP 官方 `release-8.5` 文档。后者为持续维护的版本族文档，不能把其中后续补丁新增能力自动归到 8.5.0；下列基础语法可供盘点，补丁相关算法/限制须以精确版本实测。
- 未调查最新稳定版，也不据此扩张最低支持版本。未运行真实数据库；文档有效语法、数据库运行成功、DeltaScope 解析成功、风险审计覆盖是四个不同结论。

## 普通 DDL 家族与关键变体

| 家族 | MySQL 8.4 外部分母 | TiDB 8.5 外部分母 / 厂商边界 | 证据 |
|---|---|---|---|
| 库 | CREATE / ALTER / DROP DATABASE，字符集与排序规则 | 同类操作；PLACEMENT POLICY 需单独列项 | [M总览][m-index]、[T总览][t-index]、[T ALTER DATABASE][t-db] |
| 建表 | 普通、临时、IF NOT EXISTS、LIKE、AS SELECT | 普通、临时、LIKE；AS SELECT 属厂商不支持项 | [M CREATE][m-create]、[T CREATE][t-create]、[兼容性][t-compat] |
| 列定义 | 类型、NULL、默认值、生成列、不可见列、自增、字符集 | 对应定义按 TiDB 语义验证；不能套用 MySQL 可执行性 | [M CREATE][m-create]、[T CREATE][t-create] |
| 列变更 | ADD / DROP / MODIFY / CHANGE / RENAME；FIRST / AFTER；默认值增删 | 有列变更；类型转换与同一语句修改同一对象有限制 | [M ALTER][m-alter]、[T ALTER][t-alter] |
| 普通索引 | CREATE / ADD / DROP / RENAME；复合、前缀、表达式、可见性 | 普通、复合、表达式、可见性；降序不应当成等价能力 | [M ALTER][m-alter]、[T INDEX][t-index-create]、[兼容性][t-compat] |
| 特殊索引 | FULLTEXT、SPATIAL、多值索引 | Self-Managed 不支持 FULLTEXT 使用与 SPATIAL；解析接受不等于执行支持 | [M ALTER][m-alter]、[兼容性][t-compat] |
| 约束 | 主键、唯一、外键、CHECK 的建立/删除；CHECK 执行状态 | 同类需核对；CLUSTERED 主键不能随意增删 | [M CREATE][m-create]、[T约束][t-constraints]、[兼容性][t-compat] |
| 表选项 | ENGINE、字符集转换、ROW_FORMAT、压缩、加密、统计、注释等 | 接受的 MySQL 选项可能无效；TiDB 专有选项另列 | [M ALTER][m-alter]、[T CREATE][t-create]、[兼容性][t-compat] |
| 在线 DDL | ALGORITHM 与 LOCK 组合，操作/引擎决定是否重建或并发 | ALGORITHM 是能力断言，并不选择实际算法 | [M在线操作][m-online]、[T ALTER][t-alter] |
| 表生命周期 | RENAME（含多表）、DROP（含多表）、TRUNCATE | 同类操作，不能用 DROP TABLE 的测试代替分区删除 | [M总览][m-index]、[T总览][t-index] |
| 视图 | CREATE / OR REPLACE / ALTER / DROP，安全属性 | CREATE / DROP 列入官方总览；其他形式需精确核验 | [M总览][m-index]、[T总览][t-index] |
| 存储过程/函数 | CREATE / ALTER / DROP | 厂商不支持 | [M总览][m-index]、[兼容性][t-compat] |
| 触发器、事件 | CREATE / DROP TRIGGER；CREATE / ALTER / DROP EVENT | 厂商不支持 | [M总览][m-index]、[兼容性][t-compat] |
| 表空间及专用对象 | TABLESPACE、LOGFILE GROUP、SERVER、SPATIAL REFERENCE SYSTEM；含引擎专有项 | CREATE TABLESPACE 厂商不支持；其余不假定适用 | [M总览][m-index]、[兼容性][t-compat] |

多目标不是非法边界：两种数据库均明确支持 `DROP TABLE t1,t2` 和 `RENAME TABLE t1 TO t1_old,t2 TO t2_old`。覆盖清单应包含敏感对象在首位、末位、跨库、旧名/新名等变体，避免把仅保留首对象误当成整条语句已分析。[MySQL DROP](https://dev.mysql.com/doc/refman/8.4/en/drop-table.html)、[MySQL RENAME](https://dev.mysql.com/doc/refman/8.4/en/rename-table.html)、[TiDB DROP](https://raw.githubusercontent.com/pingcap/docs/release-8.5/sql-statements/sql-statement-drop-table.md)、[TiDB RENAME](https://raw.githubusercontent.com/pingcap/docs/release-8.5/sql-statements/sql-statement-rename-table.md)

## 分区必须单独盘点

以下 SQL 为独立探针形状，要求存在匹配表/分区；不是可依次执行的完整迁移。MySQL 与 TiDB 的 DROP / TRUNCATE 都会移除分区中的数据；EXCHANGE 则交换分区与普通表的数据归属。[MySQL分区管理][m-part-manage]、[MySQL分区 ALTER][m-part]、[TiDB分区][t-part]

| 子类 | 探针形状 | 需要独立验证的语义 |
|---|---|---|
| 建分区表 | RANGE / LIST / HASH / KEY；COLUMNS 形式 | 不能只用一个 RANGE 样例代表全部；TiDB 不支持 SUBPARTITION |
| 删除分区 | `ALTER TABLE t DROP PARTITION p0` | 删除数据及分区定义；RANGE/LIST 与 HASH/KEY 的可执行性不同 |
| 清空分区 | `ALTER TABLE t TRUNCATE PARTITION p0` | 删除数据，保留分区结构 |
| 交换分区 | `ALTER TABLE t EXCHANGE PARTITION p0 WITH TABLE staging` | 两个对象及校验条件都必须保留 |
| 添加/重组 | ADD、REORGANIZE、COALESCE | 边界值、重分布/复制数据，与纯元数据修改区分 |
| 移除/改变分区方案 | REMOVE PARTITIONING、PARTITION BY | TiDB REMOVE PARTITIONING 会复制数据并重建索引 |
| 运维类分区操作 | CHECK、REBUILD、REPAIR、OPTIMIZE、IMPORT、DISCARD | MySQL 引擎适用性和 TiDB 不支持/忽略行为必须分开 |

这些差异来自厂商语义，不能将 TiDB 不适用项计作“待实现规则”，也不能用 parser 接受作为数据库支持的证据。[兼容性][t-compat]

## TiDB 扩展单列，不能借 MySQL fixture 代替

| 家族 | 最小变体 | 审计关注点 / 证据 |
|---|---|---|
| TTL | CREATE / ALTER TTL；ENABLE ON/OFF；REMOVE TTL；JOB INTERVAL | 会自动删除过期行；普通语法与 `/*T![ttl] ... */` 都要检查。[TTL][t-ttl] |
| Placement | CREATE / ALTER / DROP POLICY；库、表、分区绑定/恢复默认 | CREATE 与 ALTER 有独立文档；绑定对象与拓扑参数应保留。[CREATE][t-placement]、[ALTER][t-placement-alter] |
| Sequence | CREATE / ALTER / DROP；RESTART、步长、范围、CYCLE、CACHE | 序列定义与重启应分开。[CREATE][t-seq]、[ALTER][t-seq-alter]、[总览][t-index] |
| Resource group | CREATE / ALTER；RU_PER_SEC、优先级、BURSTABLE | 资源配额不是 MySQL CPU resource group 的等价物。[CREATE][t-rg]、[ALTER][t-rg-alter] |
| Cached table | CACHE / NOCACHE | 缓存表执行普通 DDL 有限制，应覆盖状态前提。[Cached tables][t-cache] |
| Table attributes | 表/分区 ATTRIBUTES 字符串；DEFAULT | `merge_option`、分区覆盖表属性的行为。[Attributes][t-attrs] |
| 物理与 ID 选项 | TiFlash REPLICA、SHARD_ROW_ID_BITS、AUTO_ID_CACHE、AUTO_RANDOM_BASE | 必须按单独操作分类；不能仅看 ALTER TABLE 顶层种类。[ALTER][t-alter] |

TTL 正确探针：`ALTER TABLE t TTL = created_at + INTERVAL 30 DAY`。整个右侧表达式不能替换成一个字符串常量后，还据解析失败判断“TTL 不支持”。可同时测 `TTL_ENABLE = 'OFF'`、`REMOVE TTL` 与注释形式。[TTL][t-ttl]

## 使用方式与未决问题

将每个变体映射到具体 SQL、逐语句断言和运行输出；区分“专门风险 finding”“通用 notice”“明确未审计”“静默 pass”。本清单不认定任何仓库 fixture 缺失；那需要本地搜索和运行证据。

这是研究基线，未改变产品范围、公开合同或正式推迟能力，因此无需决策记录。后续若决定将某项长期排除或新增支持，应记录该决定。

[m-index]: https://dev.mysql.com/doc/refman/8.4/en/sql-data-definition-statements.html
[m-create]: https://dev.mysql.com/doc/refman/8.4/en/create-table.html
[m-alter]: https://dev.mysql.com/doc/refman/8.4/en/alter-table.html
[m-online]: https://dev.mysql.com/doc/refman/8.4/en/innodb-online-ddl-operations.html
[m-part]: https://dev.mysql.com/doc/refman/8.4/en/alter-table-partition-operations.html
[m-part-manage]: https://dev.mysql.com/doc/refman/8.4/en/partitioning-management-range-list.html
[t-index]: https://raw.githubusercontent.com/pingcap/docs/release-8.5/sql-statements/sql-statement-overview.md
[t-db]: https://raw.githubusercontent.com/pingcap/docs/release-8.5/sql-statements/sql-statement-alter-database.md
[t-create]: https://raw.githubusercontent.com/pingcap/docs/release-8.5/sql-statements/sql-statement-create-table.md
[t-alter]: https://raw.githubusercontent.com/pingcap/docs/release-8.5/sql-statements/sql-statement-alter-table.md
[t-index-create]: https://raw.githubusercontent.com/pingcap/docs/release-8.5/sql-statements/sql-statement-create-index.md
[t-compat]: https://raw.githubusercontent.com/pingcap/docs/release-8.5/mysql-compatibility.md
[t-constraints]: https://raw.githubusercontent.com/pingcap/docs/release-8.5/constraints.md
[t-part]: https://raw.githubusercontent.com/pingcap/docs/release-8.5/partitioned-table.md
[t-ttl]: https://raw.githubusercontent.com/pingcap/docs/release-8.5/time-to-live.md
[t-placement]: https://raw.githubusercontent.com/pingcap/docs/release-8.5/sql-statements/sql-statement-create-placement-policy.md
[t-placement-alter]: https://raw.githubusercontent.com/pingcap/docs/release-8.5/sql-statements/sql-statement-alter-placement-policy.md
[t-seq]: https://raw.githubusercontent.com/pingcap/docs/release-8.5/sql-statements/sql-statement-create-sequence.md
[t-seq-alter]: https://raw.githubusercontent.com/pingcap/docs/release-8.5/sql-statements/sql-statement-alter-sequence.md
[t-rg]: https://raw.githubusercontent.com/pingcap/docs/release-8.5/sql-statements/sql-statement-create-resource-group.md
[t-rg-alter]: https://raw.githubusercontent.com/pingcap/docs/release-8.5/sql-statements/sql-statement-alter-resource-group.md
[t-cache]: https://raw.githubusercontent.com/pingcap/docs/release-8.5/cached-tables.md
[t-attrs]: https://raw.githubusercontent.com/pingcap/docs/release-8.5/table-attributes.md
