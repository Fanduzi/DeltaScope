# MySQL / TiDB DDL 覆盖盘点

日期：2026-09-21。代码基线：`main`，`9a28cc99c38770e6e40738515994f864166419f9`。

## 结论

**MySQL/TiDB 确实存在重要 DDL 审计与测试缺口，不能用当前 100% 规则 fixture 覆盖率支持“DDL 已测全，可以收官”的判断。** 差异既来自合并 fixture，也来自真实语义遗漏。最优先的问题是多目标 DDL 漏查表黑名单，以及分区删除/清空、TiDB TTL 等有效语句静默通过。

本报告是研究结果和修复建议，不改变产品支持范围，不把建议直接变成已接受的需求。未修改实现、既有 fixture、配置或历史决策；未创建 issue、提交、推送或发布。

## 范围、方法与证据

- 审计 E2E 配置为 MySQL `8.4`、TiDB `v8.5.0`，见 [compose](../../docker/cli-e2e-compose.yaml)。Query Access 的 5.7/8.0/8.4/8.5 配置是另一条能力线，不能借作 DDL 全版本支持证据。
- 外部分母来自 MySQL 8.4 手册和 PingCAP 官方 `release-8.5` 文档，完整来源及版本限制见 [官方基线](2026-09-21-mysql-tidb-ddl-official-baseline.md)。未追求最新版本或扩张最低兼容版本。
- 从官方家族和关键变体反查 census、SQL corpus、parser/extractor、领域规则单测、SDK/transport 测试；再用当前源码构建 CLI，运行 **153 个离线审计探针**（MySQL 69、TiDB 84）。其中 8 个使用显式黑名单配置，其余使用默认策略。
- 全部输入、配置、返回码、JSON 和 stderr 保存在 [探针原始证据](2026-09-21-mysql-tidb-ddl-probes.json)。下文的 `mysql.*` / `tidb.*` 是该文件中的唯一 ID。
- 探针没有连接或执行数据库 SQL。输入含厂商不适用和错误语法对照，因此不能把探针数量作为有效 SQL 覆盖率；需要表/列/分区存在的语句只验证静态路径。此次没有重跑 Docker E2E，没有验证生产误报率或全部语法组合。
- `--fail-on none` 用于分开观察 Verdict 和退出阈值；不会把 `pass` 解释成可安全执行。解析失败探针必须查看 diagnostics/exit code，不能把空 Verdict 当作 pass。

## 当前测试库存：分母是什么

| 指标 | MySQL | TiDB | PostgreSQL（对照，未做同等外部盘点） |
|---|---:|---:|---:|
| 全部 SQL corpus YAML | 32 | 24 | 230 |
| 其中独立 DDL SQL/YAML 对 | 20 | 14 | 226 |
| DDL fixture 有 include 断言 | 18 | 12 | 219 |
| DDL fixture 有 exclude 断言 | 13 | 8 | 8 |
| DDL fixture 带模拟 metadata | 3 | 3 | 2 |
| 现有目录中的代表形式 | 62 | 55 | 290 |
| finding_covered | 47 | 46 | 279 |
| normalized_silent | 0 | 0 | 6 |
| parser_error | 15 | 9 | 5 |

库存来自 [SQL corpus](../../testdata/sql-corpus/README.md)、[当前目录 JSON](../reference/ddl-coverage-catalog.json) 和 `make sql-corpus-report`。这些是文件/目录计数，不是所有单测总数，也不能据 PostgreSQL 文件多就判定它语义完善。

`make sql-corpus-report` 当前为 373 个 Default Policy rule IDs、586/586 个受支持 rule×dialect targets、286 个 YAML。三个 opt-in impact Catalog 规则不等于 Default Policy；另有两个明确 deferred 的规则不进入该覆盖分母：`ddl.alter.add_index.redundant_unique_overlap.forbid`、`ddl.pg.alter.add_column.non_null_default.rewrite.warn`。后者等规则仍有领域单测，不能表述为完全无测试。

门禁的核心是“fixture 的 findings.include 至少提到一次该规则/方言”。[覆盖门禁](../../internal/application/audit/corpus_coverage_test.go) 与 [库存实现](../../internal/application/audit/corpus_inventory_test.go) 不要求同一规则具有危险、安全、边界、关闭配置、metadata 缺失等成组样例。

现有 census 的 `finding_covered` 表示出现 finding；通用 notice 也算。`normalized_silent=0` **只对当前列入 census 的 MySQL/TiDB 形式成立**，不覆盖本次新发现的 ALTER 子动作。历史 [v0.200.0 决策](../decisions/2026-05-26-v0.200.0-mysql-tidb-ddl-normalized-silent-coverage-pack.md) 本就只承诺将当时跟踪的 silent 项转为 notice，没有承诺所有 DDL 的风险覆盖。

## 已确认、优先修复的缺口

### P1-A：多目标 DDL 只检查第一个表，黑名单可漏查

使用同一配置：

```yaml
rules:
  ddl.table.drop.forbid:
    enabled: false
  ddl.table.denylist.forbid:
    enabled: true
    level: blocker
    params:
      tables: [sensitive]
```

| SQL | MySQL / TiDB 实测 | 探针后缀 |
|---|---|---|
| `DROP TABLE sensitive` | reject，黑名单 blocker | `denylist_single` |
| `DROP TABLE harmless, sensitive` | pass，零 finding | `denylist_second` |
| `DROP TABLE sensitive, harmless` | reject，黑名单 blocker | `denylist_first` |
| `RENAME TABLE harmless TO harmless_old, sensitive TO sensitive_old` | pass，仅 rename notice | `denylist_rename_second` |

这些多目标语法为两种数据库支持，见 [官方基线的多目标证据](2026-09-21-mysql-tidb-ddl-official-baseline.md)。默认策略的 blanket DROP blocker 会遮蔽 DROP 场景，所以必须在允许普通删表、保护指定表的配置下测试；RENAME 也不能遗漏后续对象。

代码证据：[extractDropTable](../../internal/infrastructure/parser/tidb/extractor.go) 只把第一个表放入 `DDL.Table`，其他表仅保留数量；`extractRenameTable` 虽保存各 rename action，但 [denylist rule](../../internal/domain/rule/ddl/denylist_rules.go) 仅读取 `DDL.Table`。已有 extractor 多目标测试只断言第一个表和 `multiple_targets=2`，不能证明所有对象都被政策检查。

**收官验收建议**：两种方言逐目标检查；保护对象置于第一/中间/最后均拦截；跨 schema、多个 rename 源/目标的合同明确；至少保留一个全允许对照。此项是已存在黑名单能力的正确性缺口，不需要先承诺完整 DDL 语法。

### P1-B：破坏性 ALTER 子动作静默 pass

MySQL 和 TiDB 的 `DROP PARTITION`、`TRUNCATE PARTITION`、`EXCHANGE PARTITION`，以及 MySQL `DISCARD TABLESPACE` 均在本次默认离线探针中得到 `pass`、零 finding、零 unsupported、零 diagnostic。

```sql
ALTER TABLE t DROP PARTITION p0;
ALTER TABLE t TRUNCATE PARTITION p0;
ALTER TABLE t EXCHANGE PARTITION p0 WITH TABLE staging;
```

DROP/TRUNCATE 分区会删除数据，EXCHANGE 涉及两个对象，见 [官方分区基线](2026-09-21-mysql-tidb-ddl-official-baseline.md)。这与离线无法判断某列是否存在不同：操作类别本身可以从 SQL 确定。

代码证据：`alterActionName` 对未枚举 AST 类型返回 `alter_<number>`；[extractAlterSpec](../../internal/infrastructure/parser/tidb/extractor.go) 没有为这些动作完整投影分区/对象语义。普通 ALTER 可进入审计，但不自动得到 Unsupported。领域的 `HasPartition` 检查主要由 CREATE TABLE 填充，不能替代分区生命周期分析。

**收官验收建议**：先确定最小合同——危险操作要有明确风险 finding，尚未覆盖的操作应明确未审计；不能只补一个“期待静默 pass”的 fixture 当作修复。普通安全对照、混合 ALTER、第二对象与 metadata 不可用必须一起验证。

### P1-C：TiDB TTL census 用错形状，掩盖能解析但没审计的路径

| 输入 | 实测 |
|---|---|
| 当前 census：`ALTER TABLE users TTL = 'created_at + INTERVAL 30 DAY'` | parser_error |
| 官方形状：`ALTER TABLE t TTL = created_at + INTERVAL 30 DAY` | pass，零 finding/diagnostic |
| `ALTER TABLE t REMOVE TTL` | pass，零 finding/diagnostic |
| `ALTER TABLE t TTL_ENABLE = 'OFF'` | pass，零 finding/diagnostic |

官方语法使用表达式，TTL 会自动删除过期数据，见 [TiDB 8.5 TTL](https://raw.githubusercontent.com/pingcap/docs/release-8.5/time-to-live.md)。不能从带引号对照推断“TTL 需要 parser 升级”。受影响证据包括 [census](../../internal/application/audit/cross_dialect_ddl_coverage_census_test.go)、[可行性分类](../../internal/application/audit/ddl_parser_error_feasibility_census_test.go)、目录及支持说明。

**收官验收建议**：保留无效 SQL 的失败测试；另加有效 TTL 的独立路径及 CREATE/ALTER、enable/disable/remove、特性注释变体，明确风险与未审计合同，再更新目录分类。不要直接删除失败用例或宣称“TTL 已支持”。

### P1-D：已有治理在等价或相关语法入口之间存在明显差异

- 两种方言 `ALTER TABLE t ALTER COLUMN c SET DEFAULT 1` / `DROP DEFAULT` 均静默 pass；通过 MODIFY/CHANGE 改 default 则有已有字段变化规则。前者没有规范化到有意义的 default 动作。
- `CREATE TABLE ... FOREIGN KEY` 触发 `ddl.table.foreign_key.forbid`；`ALTER TABLE ... ADD CONSTRAINT ... FOREIGN KEY` 只有通用 notice，默认仍 pass。现有 ALTER constraint fixture 主要验证 notice/action，不证明 FK 禁止政策跨入口一致。
- 相同 9 列索引与坏前缀：`CREATE INDEX bad ON t(a,b,c,d,e,f,g,h,i)` 仅 notice/pass；`ALTER TABLE t ADD INDEX bad (...)` 有列数及前缀 warning/review。需要明确独立 CREATE INDEX 与嵌套 ADD INDEX 的治理范围，不能称它们都已做同等风险审计。

探针：`set_default`、`drop_default`、`create_fk_control`、`add_fk`、`create_bad_index`、`alter_bad_index`。代码路径见 [extractor](../../internal/infrastructure/parser/tidb/extractor.go)、[ALTER 规则](../../internal/domain/rule/ddl/alter_semantic_rules_test.go)、[索引规则](../../internal/domain/rule/ddl/index_rules_test.go)。这里的 P1 是收官优先级建议；规则是否扩展到某一入口仍须明确合同，不能把当前差异全部当成已承诺功能的回归。

## 覆盖矩阵

状态：**较强**=多层测试证据存在，仍非穷尽；**浅层**=解析/通用 notice 为主；**缺口**=探针及代码证实语义或断言不足；**边界**=明确失败或厂商不适用；**待核**=不凭搜索缺失断言完全无测试。

证据缩写均链接到现有文件；探针可在 JSON 用对应后缀定位。

| DDL 家族/变体 | MySQL | TiDB | 已有证据、缺口与建议 |
|---|---|---|---|
| CREATE TABLE 基础列、主键、默认值、注释 | 较强 | 较强 | [extractor 测试][extract-tests] + [大 corpus][mysql-pack]；正常与危险对照存在，但多数规则集中在大样例 |
| CREATE TABLE inline/table PK、复合 PK | 较强 | 较强 | `column_primary_key`、`primary_key_semantics`、`no_primary_key_control` 两方言 fixtures；保留逐语句语义 |
| CREATE TABLE JSON、生成列 | 有代表样例 | 缺独立对照 | MySQL `create_table_json_generated`；TiDB 库存无对应独立 representative pack，不能把 MySQL 运行代替 TiDB |
| CREATE TABLE 分区 HASH/RANGE/LIST/COLUMNS | 有代表样例 | 主要合并 HASH | [MySQL boundary][mysql-boundary] 有 RANGE/LIST；建表禁分区规则不覆盖后续分区 ALTER |
| CREATE TABLE LIKE / AS SELECT | 有规则与合并 fixture | LIKE 可测；CTAS 厂商不支持 | 不将 TiDB CTAS parser 接受解释为数据库支持；应保留方言负例 |
| 临时表、IF NOT EXISTS | 有基础路径，专项语义待核 | 本地/全局临时表需细分 | 本次 temporary 仅普通 CREATE 规则；没有证明临时表特有风险覆盖 |
| ADD COLUMN、FIRST/AFTER、多列 | 有 extractor/规则测试 | 同共享路径 | 存在性有 metadata fixture；位置、同批依赖与字段全部保真未形成完整矩阵 |
| DROP COLUMN | notice + 可配置/metadata 规则 | 同左 | pass+notice 是策略现状，不等于没测；应区分默认政策和可开启禁止项 |
| MODIFY/CHANGE 类型、NULL/default、自增 | 较强，但多为合并样例 | 同左 | [兼容性][compat-tests] 有窄化、类型族、离线跳过；[语义单测][semantic-tests] 有触发与未变化对照 |
| RENAME COLUMN | 已有禁止规则 | 已有禁止规则 | 探针 reject；跨 schema/引号/组合仍需列出验收行 |
| ALTER COLUMN SET/DROP DEFAULT | 缺口：静默 | 缺口：静默 | P1-D，不能借 MODIFY default 测试作替代 |
| ADD INDEX/KEY/UNIQUE/FULLTEXT、显式 CONSTRAINT | 有专项修复及 fixtures | 解析/策略测试存在，FULLTEXT 有厂商边界 | [索引 action 决策][index-adr]；TiDB parser 接受不意味着实际可用 FULLTEXT |
| 独立 CREATE INDEX：前缀、宽索引 | 浅层，与 ALTER 不一致 | 同左 | P1-D；独立 create_bad_index 对照可复现 |
| 表达式、前缀长度、排序、SPATIAL/多值索引 | 浅层/待核 | 需按兼容性排除不适用项 | expression/SPATIAL 探针只有 notice，未证明关键索引语义保留；本次未穷举多值索引 |
| DROP/RENAME INDEX | 有禁止/notice/metadata 测试 | 同左 | 有 [语义规则测试][semantic-tests]，保留单独与嵌套入口对照 |
| ALTER INDEX VISIBLE/INVISIBLE | 缺口：静默 | 缺口：静默 | `index_visible/index_invisible`；未找到对应 MySQL/TiDB 高层 fixture |
| ADD/DROP PRIMARY KEY | ADD 浅层，DROP 有 blocker | 同左且 clustered 限制需区分 | ADD 只看到 constraint notice；不能证明 PK 语义规则全适用 |
| ADD/DROP FOREIGN KEY | ADD 浅层；CREATE/ALTER 不一致 | 同左 | P1-D；DROP 仅通用 notice，与完整依赖验证不同 |
| ADD CHECK、DROP CHECK、ENFORCED | ADD 浅层，DROP/状态静默 | 同左，真实约束开关需核验 | `add_check/drop_check/check_not_enforced`；不把有 notice 当约束语义测试 |
| ENGINE/ROW_FORMAT/charset | 有 metadata 兼容规则，离线可静默 | 需区分被忽略的 MySQL 选项 | [兼容性单测][compat-tests] 已明确离线跳过；这类静默不是一概判 bug |
| CONVERT TO CHARACTER SET | 离线静默 | 离线静默 | charset 有抽取/规则基础，但本次未证明在线数据转换语义；需独立 snapshot 对照 |
| ALGORITHM/LOCK | 明确已接受但无安全政策 | 实际算法语义不同 | [既有决策][index-adr] 已明确不做策略规则；不要将其默认为新增必做功能 |
| ADD/REORGANIZE/COALESCE PARTITION | 缺口：静默 | 缺口：静默，按分区类型判适用 | 当前 census 无对应动作明细；探针与源代码共同支持 |
| DROP/TRUNCATE PARTITION | 缺口：静默 | 缺口：静默 | P1-B，数据删除类别优先 |
| EXCHANGE PARTITION | 缺口：静默 | 缺口：静默 | P1-B，需保留两个对象、validation 语义与配置保护 |
| REMOVE PARTITIONING / PARTITION BY | 缺口：静默 | 缺口：静默 | 新旧布局与重建风险不能由 CREATE HasPartition 代替 |
| DISCARD/IMPORT TABLESPACE、FORCE | 缺口：静默 | 先排除不适用/忽略项 | MySQL 专项，DISCARD 优先；其他操作风险等级待定 |
| DROP/TRUNCATE TABLE 单目标 | 较强 | 较强 | [生命周期单测][lifecycle-tests] + metadata fixture；普通 DROP 默认 blocker |
| 多目标 DROP/RENAME | 已复现政策漏查 | 已复现政策漏查 | P1-A；已有 multiple_targets 数量测试并不充分 |
| DATABASE CREATE/ALTER/DROP | 有 AST、规则和跨入口测试 | 同左，placement 另核 | [数据库 AST 测试][db-tests]、生命周期 fixtures；charset/加密等组合非穷尽 |
| VIEW CREATE/REPLACE/DROP | 有禁止规则 | 有禁止规则 | 基础形状有证据；definer/security/check option 全组合未核完 |
| ALTER VIEW | parser_error | 当前 parser_error | MySQL 有效语法边界；TiDB 版本语义按官方再核，不能复用 PostgreSQL 证据 |
| PROCEDURE CREATE/DROP | 通用 notice | 厂商不支持却可被 parser 接受 | 现有 census/notice 不能用作 TiDB 执行支持证明 |
| FUNCTION/TRIGGER/EVENT | 多个已记录 parser_error | 多数为厂商不支持 | 保留明确未审计和无敏感信息泄漏合同；不为追数量强行支持 TiDB |
| 独立 TABLESPACE/LOGFILE GROUP/SERVER/SRS | TABLESPACE 已知错误，其余待核 | 不直接适用 | 官方分母中列出，本次未逐变体探测；不是零缺口声明 |
| TTL CREATE/ALTER/ENABLE/REMOVE | 不适用 | 有效语法静默，目录样例失真 | P1-C；CREATE 与 feature comments 还需补独立验收 |
| PLACEMENT POLICY 生命周期/绑定 | 不适用 | 通用 notice | 有 lifecycle fixture；库/分区绑定、DEFAULT、混合 table options、脱敏需细分 |
| SEQUENCE CREATE/ALTER/DROP | 不适用 | 通用 notice | 已有 fixture 和 no-leak；RESTART/CYCLE/范围未证明风险检查 |
| RESOURCE GROUP CREATE/ALTER | MySQL 语法 parser_error | kind=unknown，pass，无未审计提示 | TiDB 合法 RU_PER_SEC 形状新缺口；与 DROP 的 notice 不一致 |
| CACHE/NOCACHE、ATTRIBUTES | 不适用 | 静默 | 缺独立语义/状态前提样例；不必默认所有项都新增 blocker |
| SHARD_ROW_ID_BITS/PRE_SPLIT/AUTO_ID_CACHE | 不适用 | 静默 | table options 没有完整抽取；优先明确支持边界 |
| AUTO_RANDOM/CLUSTERED/NONCLUSTERED | 不适用 | parser 接受，仍套普通 PK 政策 | 本次有 auto_increment/unsigned blocker；是否为团队政策或误报需决定，不能直接判 bug |
| SET TIFLASH REPLICA | 不适用 | 静默 | 有效扩展需独立分类及成本/副本影响边界 |

## fixture 为什么会显得“很多规则都测了”

1. MySQL、TiDB 各有 `rule_coverage_offline.expected.yaml` 一次 include 100 个规则 ID；metadata 大样例一次 include 22 个。它们提供广度，但不是每种 DDL 的独立验收。
2. [MySQL/TiDB runner][runner] 将整批 SQL 的 findings 合并成 rule ID 集合再断言；某规则若在正确语句漏报、另一语句误报，集合可能不变。当前 schema/runner 不提供通用的逐语句精确 finding 数量、level 和 Verdict 断言。
3. `findings.include: []` 不等于“必须零 finding”；exclude 只排除列出的 ID，未列出的误报可能通过。需要安全对照时不能只写空 include。
4. metadata fixture 使用快照 provider，不等于数据库版本/权限/连接的 E2E。反过来，Go 领域单测具有正反例，也不能因 YAML 少就抹掉其价值。
5. 本次扩大外部分母后，现有门禁仍然全部通过，而新探针暴露静默路径。这直接说明门禁覆盖的是既有规则库存，不是官方 DDL 完整性。

## 建议的有限收尾顺序

1. **先锁住 P1-A 多目标黑名单回归**，在共享语义路径修复，避免给 CLI/HTTP/MCP 各加补丁。
2. **给未覆盖动作明确状态**：分区破坏性动作、TTL、TiDB unknown resource-group 不能靠泛化 pass 充当覆盖证据；正式决定新增 finding 或未审计合同后再实现。
3. **核对 P1-D 的跨语法政策范围**：默认值、FK、独立索引，用成对 SQL 明确哪些应一致，哪些有意不同；有意不同要记录并展示边界。
4. **补最小逐语句 fixture 断言**：危险、安全、阈值边界、开关、metadata 存在/缺失；保留大 pack 作广度回归，不为追平 PostgreSQL 拆出几百个无意义文件。
5. **修正事实目录与文档**：TTL 有效/无效分开；官方支持、parser 支持、notice、风险检查分列。当前 `ddl-coverage.md` MySQL/TiDB 61/54 与 JSON 62/55 的漂移也应同步。

收官判断：完成以上高风险缺口的合同与回归后，可按明确范围进入维护；无需承诺所有厂商语法、所有版本或所有物理运行风险。表空间专用对象等待核项目不能静默视为已验收，也不自动成为必须实现的需求。

## 本次验证与复现

已执行并通过：

```sh
make sql-corpus-gates sql-corpus-report ddl-coverage-catalog-test
go test ./internal/infrastructure/parser/tidb ./internal/domain/rule/ddl ./internal/application/audit -count=1
go build -o /tmp/deltascope-ddl-inventory ./cmd/deltascope
```

153 个探针是观察性研究，不是 153 个“正确结果通过”的测试。复现任一默认策略行：

```sh
/tmp/deltascope-ddl-inventory audit --dialect mysql \
  --sql 'ALTER TABLE t DROP PARTITION p0' --format json --fail-on none
```

复现配置行时，将 JSON 的 `config_yaml` 写入临时文件，追加 `--config <文件>`。输入 SQL 仅送入 DeltaScope 静态审计，不应在数据库执行。

正式决策记录：本次不需要。原因是只记录观察证据、未决问题与建议，没有接受新的公开合同、支持提升或能力延期。后续修复若决定未支持动作的返回合同、跨语法政策或正式延期范围，应创建/更新 ADR 并随实现提交。

[extract-tests]: ../../internal/infrastructure/parser/tidb/extractor_test.go
[mysql-pack]: ../../testdata/sql-corpus/mysql/ddl/findings/rule_coverage_offline.expected.yaml
[mysql-boundary]: ../../testdata/sql-corpus/mysql/ddl/boundary/
[compat-tests]: ../../internal/domain/rule/ddl/alter_compatibility_rules_test.go
[semantic-tests]: ../../internal/domain/rule/ddl/alter_semantic_rules_test.go
[index-adr]: ../decisions/2026-08-30-mysql-tidb-alter-index-action-normalization.md
[lifecycle-tests]: ../../internal/domain/rule/ddl/object_lifecycle_rules_test.go
[db-tests]: ../../internal/infrastructure/parser/tidb/database_lifecycle_ast_test.go
[runner]: ../../internal/application/audit/corpus_test.go
