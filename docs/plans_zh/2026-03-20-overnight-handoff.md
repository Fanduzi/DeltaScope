# DeltaScope 夜间交接

## 已完成

- 通过 `pkg/deltascope.Audit(ctx, request)` 构建了离线审计核心
- 增加了 Tier-1 DDL 规则与 Tier-1 DML 规则
- 扩展了离线 DDL 覆盖，包括：
  - 针对 bigint、unsigned、auto-increment 与 not-null 要求的更强主键语义
  - 审计列要求
  - 列注释/默认值/not-null/类型规则
  - create-table 索引数量、前缀与重复索引规则
  - drop/rename/change 操作的 action 级 alter 限制
  - 针对注释长度、引擎/字符集、外键、分区、`LIKE` 与 `AS SELECT` 的 create-table 选项/对象形状规则
- 完成了 `Rich Alter Semantics` 里程碑，包括：
  - 更丰富的解析器中立 alter 模型
  - 对列、索引、rename 以及 table-option 变更的更丰富 alter 提取
  - 用于 rename-index 禁止规则的语义化 alter 规则
  - 面向 `MODIFY COLUMN` 与 `CHANGE COLUMN` 的保守目标类型族 allowlist
  - alter-added unique/secondary/fulltext 索引前缀检查
- 完成了 `Source-Aware Alter Checks` 里程碑，包括：
  - 用于显式 nullability/default/auto-increment 变更的 statement-local alter change facts
  - 将目标形状与显式变更事实分离的 source-aware 提取
  - 面向 `MODIFY COLUMN` 与 `CHANGE COLUMN` 的显式 alter-column 变更禁止规则
  - 用于宽度与精确重复检查的 alter-added 索引生命周期包装器
- 完成了 `Complete Create-Table Superset` 里程碑，包括：
  - create-table 标识符模式与保留关键字治理
  - 更广的 create-table 类型族规则，涵盖 blob/text、json、bit、timestamp 以及超大 `char`
  - 列 charset/collation allowlist 与 charset-collation 一致性检查
  - 针对左前缀与 unique-overlap 场景的更深入 create-table 冗余索引分析
  - create-table row-format 与 auto-increment 初始值 table-option 检查
- 完成了 `HTTP API Service` 里程碑，包括：
  - `GET /healthz`
  - `GET /version`
  - `POST /v1/audit` 返回稳定公开 JSON 审计结果形状
  - 基于同一离线审计核心的轻量 `cmd/deltascope-server` 入口
- 增加了 Markdown 与 JSON 渲染器
- 构建了 Cobra CLI，包括：
  - `audit`
  - `config init`
  - `version`
- 将根 `README.md` 扩展为可用的 v1 指南
- 让 `configs/deltascope.example.yaml` 与 `deltascope config init` 保持一致
- 规划了接下来三个里程碑，并提交了对应的 design、implementation 与 task-prompt 文档：
  - `Source-Aware Alter Checks`
  - `Complete Create-Table Superset`
  - `HTTP API Service`
- 完成了 `Audit Completion` 里程碑，包括：
  - 以显式审计能力矩阵作为验收基线
  - 将可选元数据感知审计事实拆分为实例事实与目标表快照
  - 针对 create/alter/drop/truncate 的基于元数据的存在性规则，以及列/索引/主键生命周期检查、adaptive-hash 提示与 row-count 提示
  - 针对 change/modify column 的 source-aware alter 兼容性，以及基于元数据的 table-option 兼容性
  - 基于 `schemas`、`tables` 与 `qualified_tables` 的对象范围 DDL/DML denylist 治理
  - 针对 snapshot + added indexes 的 alter-added 冗余索引生命周期检查，复用 create-table 冗余逻辑
  - 基于元数据的粗粒度 row-size 与 index-key-length 检查
  - 发布界面文档，包括 `README.md`、`README_ZH.md`、`CHANGELOG.md` 与 `SECURITY.md`
- 完成了 `CLI Completion` 里程碑，包括：
  - 带有 MySQL 风格 TCP/socket 参数、`--ask-password`、方言自动检测、schema 推断以及诚实的歧义/降级错误的元数据感知 `deltascope audit`
  - 已发布规则目录元数据层，以及 `rules list`、`rules show` 与 `rules search`
  - `config lint`、`config show-default` 与 `capabilities`
  - CLI help/示例、元数据感知 JSON context、quiet-output 稳定性，以及中英文文档更新
- 完成了 `CLI Metadata E2E` 里程碑，包括：
  - 针对 MySQL 8.4 与 TiDB v8.5.0 的 Docker 驱动元数据感知 CLI live smoke 覆盖
  - 用确定性 fixture 验证方言自动检测、schema 推断、schema 歧义错误、限定 schema SQL、基于元数据的存在性检查，以及基于实例事实的 sizing 行为
  - 已交付 `scripts/test_cli_metadata_e2e.sh` harness，以及 `make test-e2e-cli`、`make test-e2e-cli-mysql`、`make test-e2e-cli-tidb`
  - 针对本地 live e2e 使用的 README / README_ZH / 模块文档收口
- 完成了 `Release Readiness` 里程碑，包括：
  - 位于 `docs/admin`、`docs/concept`、`docs/dev`、`docs/recipe` 与 `docs/reference` 下的产品文档信息架构
  - 将 `README.md` 与 `README_ZH.md` 重写为产品落地页，同时保留 L1 架构/模块图
  - 将审计能力矩阵从 `docs/plans` 迁移到 `docs/reference/audit-capability-matrix.md`
  - 产品级与实现级 ASCII 架构文档
  - 通过 `.github/workflows/release.yml` 与 `.goreleaser.yml` 建立唯一可信、基于 tag 的 GitHub Actions 发布路径
  - 与发布对齐的 `install.sh` 与小而稳定的 `Makefile` 操作入口面
  - 将默认发布目标版本对齐到 `v0.6.0`

## 进行中

- `Milestone 3: Source-Aware Alter Checks` 已完成
- `Milestone 4: Complete Create-Table Superset` 已完成
- `Milestone 5: HTTP API Service` 已完成
- `Milestone 6: Audit Completion` 已完成
- 下一阶段的活跃工作应超越当前的 parity / coverage，转向新的产品方向：
  - 更深入的在线风险估计
  - MCP server / agent adapter 工作
  - 服务加固，如 auth、中间件与运维打磨

## 关键提交

- `35f1926` `feat: add Tier-1 DML rules`
- `6a80dac` `feat: add public audit API`
- `ea84b71` `feat: add audit result renderers`
- `2440bca` `feat: add deltascope cobra cli`
- `a8f5cc1` `fix: tighten cli config error handling`
- `091f428` `docs: finalize v1 usage and verification`
- `f933f4b` `docs: finalize v1 README and examples`
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

## 验证运行

- `go test ./...`
- `go run ./cmd/deltascope audit --sql "delete from t"`
- `go run ./cmd/deltascope audit --sql "delete from t" --format json`
- `go run ./cmd/deltascope config init`
- `go run ./cmd/deltascope config lint --file ./configs/deltascope.example.yaml`
- `go run ./cmd/deltascope config show-default`
- `go run ./cmd/deltascope rules list --kind dml --level blocker`
- `go run ./cmd/deltascope rules show dml.where.require`
- `go run ./cmd/deltascope rules search metadata`
- `go run ./cmd/deltascope capabilities`
- `go run ./cmd/deltascope version`
- `diff -u configs/deltascope.example.yaml <(go run ./cmd/deltascope config init)`
- `make test-e2e-cli-mysql`
- `make test-e2e-cli-tidb`
- `make -n test`
- `make -n build`
- `make -n test-e2e-cli`
- `sh -n install.sh`
- `go run github.com/goreleaser/goreleaser/v2@v2.12.7 check`
- `/Users/fan/.codex/skills/check-three-level-doc/scripts/check_three_level_doc.sh`

## 决策与问题

- 权威决策历史见 [2026-03-20-deltascope-v1-decisions.md](/Users/fan/GolangProjects/deltascope/docs/plans/2026-03-20-deltascope-v1-decisions.md)
- reviewer 子代理比实现循环更慢也更不稳定，因此部分夜间进度是在本地验证后继续推进，而不是空等审查结果
- 曾出现一次 CLI race：某个并发 worker 在验证期间重写了 `internal/interfaces/cli`；我随后对文件进行了协调，并重新运行测试与手动 smoke 检查
- Milestone 3 立即暴露了另一条“诚实性边界”：`MODIFY/CHANGE COLUMN` 语法包含完整目标定义，但这并不能证明语句显式触及了 type 或 unsigned 语义
- 我通过收窄 `AlterColumn.Change` 修复了这一点：它现在只表示显式的 nullability/default/auto-increment 变更；目标形状仍保留在 `Definition` 上
- Milestone 3 Task 4 有意跳过了 unsigned-transition 规则，因为当前离线模型仍无法诚实表达该转换
- Milestone 3 Task 5 只通过投影复用 create-table 规则的方式扩展 alter-added 索引生命周期检查；它仍未声称具备实时存在性或完整 rename/drop 生命周期语义
- Milestone 4 有意将 `blob_text/json/bit` forbid 作为“已交付但默认放松”的规则处理：这些规则存在于默认模板中，但团队需要自行开启 `forbid: true` 才会真正强制执行
- Milestone 4 的 `ROW_FORMAT` allowlist 只在语句显式设置 row-format 时求值；它不会强制每个 create-table 语句都写出 row format
- Milestone 5 有意让 HTTP 服务保持轻量且无状态；它复用公开 audit API，并在每个请求上重载配置，而不是再生长出一套独立的内存策略管理器
- `Audit Completion` 有意让元数据感知规则继续运行在同一条 audit 路径上，而不是引入第二套“online mode”引擎
- `Audit Completion` 同样把 sizing 检查定位为粗粒度、基于元数据的保护规则，而不是假装精确计算执行期存储结果
- `CLI Completion` 有意将元数据感知访问保留在现有 `audit` 命令上，而不是另开一棵 online-only 命令树
- `CLI Completion` 还通过默认策略中的 rule ID 派生已发布规则目录，以确保 `rules` 命令与实际 shipped surface 始终对齐
- `CLI Metadata E2E` 有意把真实实例 smoke 保留在 `go test ./...` 之外的 Docker 驱动 shell 层，这样默认反馈循环仍然快速且无容器依赖，而 live metadata 路径也能获得可重复证明
- `Release Readiness` 有意只增加一条可信发布路径；在同一个里程碑中不会再添加 package-manager 分发、新审计规则、HTTP 增强或 MCP server
- 发布 workflow 必须先落在 `origin/main` 上，之后再推第一个 `v0.6.0` tag；否则该 tag 不会追溯触发 release job

## 剩余缺口

- `Audit Completion` 已收口能力矩阵中的阻塞缺口
- `CLI Completion` 已收口 audit、rules、config inspection 与 capability discovery 的主要 CLI 缺口
- `CLI Metadata E2E` 已收口“元数据感知 CLI 尚未在真实目标上证明”的本地 MySQL/TiDB smoke 风险
- `Release Readiness` 已收口一个成熟 `v0.6.0` release candidate 在本地 docs/install/workflow/operator-surface 方面的阻塞项
- 剩余工作现在属于发布后深化：
  - 当 workflow 已在 `origin/main` 上可用时，执行远端 tag 并验证 GitHub Release 资产
  - 在粗粒度元数据 sizing 与 row-count 提示之外，做更丰富的运行时风险估计
  - 更广泛的在线安全检查与策略易用性打磨

## 下一阶段活跃工作

- Milestone 3 已关闭：目标是建立诚实的 source-aware alter facts，并交付第一批显式 source-aware alter 规则
- Milestone 4 已关闭：目标是让 create-table 的离线覆盖超过计划中的 `gAudit` superset 线
- Milestone 5 已关闭：目标是通过轻量 JSON HTTP 服务暴露同一套离线审计引擎
- Milestone 6 之后，最可能的下一步是：
  - 将发布 workflow push / merge 到远端默认分支，然后创建真实的 `v0.6.0` tag 并验证已发布资产
  - 决定首个成熟公开版本之后，是优先深化在线审计/风险建模，还是优先做服务加固
  - 继续保持 CLI、HTTP 与未来适配器共享同一套公开审计/结果契约