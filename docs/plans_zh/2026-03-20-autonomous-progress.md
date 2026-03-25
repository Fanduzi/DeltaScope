# DeltaScope 自主进度摘要

## 在原始 v1 基线之后新增的内容

仓库已经超越最初的 library/CLI v1 基线，并增加了十二个主要离线 DDL/产品化批次：

1. 列与审计列治理
2. create-table 索引治理
3. action 级 alter 限制
4. 更强的主键语义
5. 更丰富的 alter 语义
6. source-aware alter checks
7. create-table superset 补完
8. HTTP API 服务交付
9. 审计能力补完与元数据感知覆盖收口
10. CLI 补完与产品界面收口
11. CLI 元数据 E2E 与 live-smoke 风险收口
12. 发布就绪与发布界面收口

## 新检查点

- `34704ea` `feat: expand ddl column and index governance`
- `1e29699` `feat: add alter action restriction rules`
- `adeb082` `feat: add table option ddl rules`
- `2802ba8` `feat: add primary key semantic rules`
- `c155de0` `refactor: tighten alter domain contract`
- `7d13bff` `fix: normalize alter extraction edge cases`
- `65bcec9` `refactor: narrow alter type-family rule naming`
- `3be386d` `feat: audit alter-added index prefixes`
- `0dd633b` `docs: add milestone 3-5 planning docs`
- `739553d` `refactor: remove redundant alter rename flag`
- `6403f26` `refactor: narrow explicit alter change facts`
- `5f9b47c` `refactor: prepare source-aware alter rules`
- `2900bfe` `feat: add explicit alter column rules`
- `cf2705d` `feat: extend alter index lifecycle checks`
- `17825cb` `docs: pin create-table superset rule surface`
- `b953bda` `feat: add create-table identifier governance`
- `eb413bd` `feat: add create-table type-family governance`
- `6af0652` `feat: deepen create-table redundant index checks`
- `a647fb6` `feat: close create-table object-shape gaps`
- `7460ddd` `docs: define http api contracts`
- `0abd3bf` `feat: add http api service adapter`
- `ce56373` `docs: add audit capability matrix baseline`
- `da0c768` `feat: add metadata-aware domain specs`
- `7c6ee34` `feat: add metadata-aware audit providers`
- `86eefe3` `feat: add metadata-backed ddl existence rules`
- `54d418f` `feat: add source-aware alter compatibility rules`
- `9812b92` `feat: close metadata and lifecycle audit gaps`
- `4848698` `docs: add cli completion plan artifacts`
- `c47e330` `feat: add metadata-aware audit request plumbing`
- `d80168d` `feat: add audit connection flag parsing`
- `240a48d` `feat: wire metadata-aware cli audit`
- `a4ecab1` `feat: add shipped rule catalog metadata`
- `92a0e2d` `feat: add cli rule catalog commands`
- `d1569e3` `feat: add cli config lint commands`
- `5d843fc` `feat: add cli capabilities command`
- `0fe28d8` `feat: close cli help and output gaps`
- `2cc1b90` `docs: add cli metadata e2e plan artifacts`
- `c63f72e` `test: add cli metadata e2e fixtures`
- `2743e9e` `test: add cli metadata e2e harness`
- `8babfbf` `test: add mysql cli metadata e2e coverage`
- `62271c4` `test: add tidb cli metadata e2e coverage`
- `7301352` `docs: add cli metadata e2e usage targets`
- `5a37116` `docs: add product docs structure`
- `4d7b148` `docs: move capability matrix into reference docs`
- `7d6c74f` `docs: add concept and architecture references`
- `57078cd` `docs: add recipes and product references`
- `eb85956` `docs: rewrite english landing page`
- `b1e8f0f` `docs: rewrite chinese landing page`
- `972973f` `build: add release workflow`
- `c87a2e0` `build: add install script`
- `2f944e5` `build: expand make targets`

## 当前离线 DDL 覆盖

- 表注释存在性与最大长度
- 表名长度
- create-table 标识符模式与保留关键字治理
- 引擎与字符集 allowlist
- create-table 的 row-format 与 auto-increment 初始值治理
- 外键、分区、`CREATE TABLE ... LIKE` 与 `CREATE TABLE ... AS SELECT` 限制
- 主键存在性、最大列数、bigint/unsigned/auto-increment/not-null 语义
- 审计时间戳列模式
- 列注释/默认值/not-null/float-double 规则
- varchar 与 char 长度限制
- blob/text、json、bit 与 timestamp 类型族治理
- 列 charset/collation allowlist，以及 charset-collation 一致性检查
- 索引数量、索引宽度、命名前缀、精确重复索引检测、左前缀冗余，以及 unique-overlap 冗余
- 更丰富的离线 `ALTER TABLE` 覆盖：
  - drop/rename/change 操作的 action 级限制
  - rename-index 禁止规则
  - `MODIFY COLUMN` 与 `CHANGE COLUMN` 的显式 nullability/default/auto-increment 变更禁止
  - `MODIFY COLUMN` 与 `CHANGE COLUMN` 的目标类型族 allowlist
  - alter-added unique/secondary/fulltext 索引前缀检查
  - 在显式启用时，alter-added 索引宽度与精确重复检查
- HTTP API 服务覆盖：
  - `GET /healthz`
  - `GET /version`
  - `POST /v1/audit` 返回稳定的公开 JSON 结果契约
- Audit Completion 覆盖：
  - 用能力矩阵驱动验收，而不是主观的“差不多完成”判断
  - 面向 DDL 与 DML 的可选元数据感知 denylist 治理
  - 基于元数据的表选项兼容性
  - alter-added 冗余索引生命周期检查
  - 基于元数据的粗粒度行大小与索引键长度保护
  - 中英文发布界面文档，以及 changelog/security 页面
- CLI Completion 覆盖：
  - 带有类 MySQL 连接参数、`--ask-password`、方言自动检测与 schema 推断的元数据感知 CLI 审计
  - 面向解释的已发布规则目录，以及 `rules list/show/search`
  - `config lint`、`config show-default` 与 `capabilities`
  - help/示例、元数据感知 JSON context，以及中英文 CLI 文档
- CLI Metadata E2E 覆盖：
  - 仅通过公开 CLI 对 MySQL 与 TiDB 做 Docker 驱动的 live smoke
  - 在真实目标上验证方言自动检测、schema 推断、schema 歧义以及限定 schema 的 DML 覆盖
  - 验证基于元数据的存在性检查，以及在两个引擎上都验证至少一个基于实例事实的 sizing 规则路径
  - 通过本地 `Makefile` 目标与文档，将该测试套件与 `go test ./...` 分离
- Release Readiness 覆盖：
  - 产品文档已位于 `admin / concept / dev / recipe / reference` 之下
  - 两个根 README 现在都是产品落地页，而不是开发优先入口
  - 能力矩阵现在是稳定的 reference 文档
  - 发布打包统一在 `.github/workflows/release.yml` + `.goreleaser.yml`
  - `install.sh` 与 README 安装说明共享同一套压缩包命名契约
  - `Makefile` 现在暴露 `test`、`build`、`build-cli`、`build-server` 与 CLI E2E 入口
  - 仓库默认版本目标现已指向 `v0.6.0`

## 下一里程碑还可能是什么

Milestone 6、CLI Completion、CLI Metadata E2E 与 Release Readiness 现在都已完成。下一里程碑应来自发布后的产品扩展，而不是基础界面补齐：

- 更深入的在线/运行时风险建模
- 服务加固与运维打磨
- 当 workflow 落到 `origin/main` 后，执行真实远端 release 并验证产物

## 建议的下一步

下一条工作流应先验证真实远端 `v0.6.0` 发布路径，然后再决定是优先深化在线/运行时审计，还是优先做服务加固。无论如何，CLI、HTTP 与未来适配器都应继续共享同一套公开审计/结果契约。