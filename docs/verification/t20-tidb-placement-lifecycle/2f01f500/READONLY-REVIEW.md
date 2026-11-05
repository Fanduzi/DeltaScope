# T20 / #99：固定产品终审与资源事件取证报告

关联：[T20 / #99](https://github.com/Fanduzi/DeltaScope/issues/99)。
这是 2026-11-04 UTC 完成的只读终审报告的脱敏发布版，结论与本地冻结报告一致。
本报告区分产品验收、资源归因和取证隐私处理；其中一项通过不代表其他项通过。

## 两条独立裁决

**A．产品验收：PASS，限于冻结的 T20 定向补修合同。**
累计源码、文档和归档证据审查没有发现本轮范围内的产品阻塞。
保存资源现场、确认测试没有 Docker 启停路径后，独立执行的 T20/T14 定向测试通过。
历史门禁的准确口径是 **8 项在最终产品执行、12 项按已审查的源码等价关系复用**，
不是“20 项均在最终 SHA 重跑”，也不是所有原始命令 rc=0。

**B．资源事件：确认当前端点缺失，已找到大部分对象与实际批量清理的具体关联；仍部分未归因。**
相关任务已完成的工具记录及展开清单命中 **143 个缺失容器、106 个缺失卷名、10 个缺失网络名**。
删除输出被 `tail` 截断，管道 rc=0 不能证明每个 Docker 子命令逐项成功。
对应清理的用户授权未由现有记录确认，另 **1 个容器、5 个匿名卷名**仍未闭合。
业务数据损失、备份可用性及数据可恢复性均未确认，也不能承诺无数据损失。

## 被审产品及范围

| 项目 | 冻结身份 |
|---|---|
| 产品分支 | `milestone/mysql-tidb-ddl-completion` |
| 产品 SHA | `2f01f500d296bb29ee518bc113a58ec26172dc1a` |
| 证据 / 被审 HEAD | `b33a83ec11e569657dc4916ff060deb44701c1fb` |
| 本轮补修基线 | `6ac1e918b3558d6d611d7d8a59a770d73341c2f9` |
| milestone 基线 | `9a28cc99c38770e6e40738515994f864166419f9` |
| 本轮提交数 | 5 |
| 产品净改动 | 23 个路径 |
| 独立证据提交 | 774 个路径 |

父链为 `b33a83ec → 2f01f500 → bb5e347c → 5fc29853 → 9012677d → 6ac1e918`。
终审结束时 tracked/staged 均干净，原 13 个未跟踪状态条目保留，
其他既有 worktree 的完整 listing 与开始时一致。不得把本轮 5 个提交解释为整个 milestone 的提交数。

本报告通过独立的 `review/t20-readonly-final-report` 分支发布，基于远端 main
`9a28cc99c38770e6e40738515994f864166419f9`。
这个发布分支只增加本报告；上述产品及证据 SHA 是被审身份，
**本次报告推送不代表这些本地提交已经推送，也不构成产品交付或合并**。
报告发布范围可用以下命令核对：

```text
git diff --name-only 9a28cc99c38770e6e40738515994f864166419f9...review/t20-readonly-final-report
```

预期唯一新增路径：
`docs/verification/t20-tidb-placement-lifecycle/2f01f500/READONLY-REVIEW.md`。

## 累计产品审查

Standards 与 Spec 两轴均通过本次限定范围的审查：

- `CollectPlacementBindings` 覆盖直接绑定、CREATE 成员、ALTER 直接目标、
  ADD/REORGANIZE 成员及替换分区方案；DEFAULT 保留独立位置，
  存在性查询去重不丢失绑定出现位置。unknown 不吞掉后位 missing。
- 目录加载、适用性、两条规则、ordered state 与引用维护消费相同结构化范围。
  已证拒绝不再产生暗示绑定落地的 warning；可应用时采用条件性措辞，
  policy existence 与 topology 证据仍分开。
- T14 的方法、成员和边界判断继续约束发布。
  拒绝的 CREATE/ADD/REORGANIZE 保持前态；未决操作不发布新成员绑定，
  并撤回可能变化的候选策略“引用集合已穷尽”知识。
- ADD 保留原成员；REORGANIZE 只退休被替换成员引用；
  未参与成员及表级绑定保留。省略/DEFAULT 不复制旧成员 override，
  表级继承另行保留。
- 逐项正引用与引用集合穷尽性分开。已证非空子集可以拒绝 DROP；
  未读且空的集合不能授权 DROP。未知引用不成为假 bound-drop 反证。
- `PartitionPlacements` 深拷贝隔离前序投影；
  不可变性控制另外用独立字面 map 检查 provider 原件。
- 子分区保持明确的 `tidb_subpartition` 能力边界。
  状态归一化仅用于已验证的 TiDB 8.5.0、CREATE TABLE、主 RANGE 形状，
  不建立被忽略子分区或其 refs，不遮蔽可独立分析的父级事实。
- 原部分目录、topology gap、provider 错误、取消/deadline 控制保留。
  findings 仍限制为有界身份，未增加原始 region/constraints 输出。

相关合同位于被审树的 `docs/work-plans/t20-tidb-placement-lifecycle/CONTRACT.md`。
实施补修所需 ADR 已同步到
`docs/decisions/2026-11-12-tidb-placement-policy-lifecycle.md`，相关 L2、
inventory/corpus 亦已核对。
本次只读终审及报告发布不改变产品、架构或公共合同，因此无需新增 ADR，
也没有把任务流水账写进 ADR。

## 本次实际独立执行

所选 audit 测试没有 Docker/testcontainers 调用；
T14 的进程调用仅用于当前测试二进制的有界隔离子测试。
检查 `TestMain`/`init`、imports、fake providers 和 companion 路径后执行：

```text
GOPROXY=off GOSUMDB=off go test ./internal/application/audit -run "^(TestT20|TestT14)" -count=1 -v
```

结果 **rc=0**，执行前后 HEAD 为 `b33a83ec...`，tracked/index 干净。
没有添加或修改测试，没有重跑数据库 Golden、重搭 PD/TiKV 或执行未知副作用聚合脚本。
本次另执行了只读 whitespace 检查与归档核对；不将其算作历史门禁重跑。

## 20 项历史门禁身份

| 门禁 | 实际执行 SHA | 原始 rc | 最终产品采用方式 |
|---|---|---:|---|
| 01 全仓 Go test | `5fc29853` | 0 | 经两步已审查的等价关系复用 |
| 02 vet | `2f01f500` | 0 | 最终产品实跑 |
| 03 internal race | `bb5e347c` | 0 | 复用 |
| 04 PG unit gates | `bb5e347c` | 0 | 复用 |
| 05 SQL corpus | `bb5e347c` | 0 | 复用 |
| 06 Query Access corpus | `bb5e347c` | 0 | 复用 |
| 07 inventory | `bb5e347c` | 0 | 复用 |
| 08 coverage catalog | `bb5e347c` | 0 | 复用 |
| 09 parser/unsupported | `bb5e347c` | 0 | 复用 |
| 10 diagnostic evidence | `bb5e347c` | 0 | 复用 |
| 11 privacy | `bb5e347c` | 0 | 复用 |
| 12 docs examples | `bb5e347c` | 0 | 复用 |
| 13 decision record | `2f01f500` | 0 | 最终产品实跑，范围 `6ac1e918..2f01f500` |
| 14 committed-diff L2 | `2f01f500` | 0 | 最终产品实跑，相同非空范围 |
| 15 Golden validator tests | `bb5e347c` | 0 | 复用 |
| 16 scoped lint | `2f01f500` | **1** | 保留失败，以诊断身份无新增作为约定门槛 |
| 17 fullrepo lint | `2f01f500` | **1** | 同上 |
| 18 lint identity comparison | `2f01f500` | 0 | 最终产品实跑 |
| 19 T20 Golden | `2f01f500` | 0 | 最终产品实跑，本次未重跑 |
| 20 artifact validator | `2f01f500` | 0 | 最终产品实跑，本次独立核对归档内容 |

复用依据由 Git 对象及原始回执重新核对：

1. `5fc29853 → bb5e347c` 仅删除所有分支均已 return 后的不可达 `return nil`。
2. `bb5e347c → 2f01f500` 仅删除无调用者的私有 helper，
   并将对同一 `alter.Action` 的互斥条件改为对应 tagged switch，分支体不变。
3. 除此之外，完整 tracked tree 没有输入变化，包括测试源码、文档、
   Makefile、脚本、fixtures、manifest、生成及配置文件；
   不只是非测试 Go 文件一致。
4. 12 项复用回执的 command/stdout/stderr/rc/head-before/head-after
   与原始执行目录逐字节一致。01 的源执行身份仍是 `5fc29853`。
5. docs examples、inventory、validator 的文档/数据输入未变；
   ADR/L2 在最终产品执行，未用 Go 字节相同替代提交范围检查。

这是源码语义等价复用，不是完整环境重演证明。
原 runner 每次 attempt 开始检查 tracked/index，每门前后检查 HEAD，
但没有每门结束的 dirty-tree hash，也没有每门单独的 Python/linter
二进制 hash 或完整继承环境。可取得的 Go 基线与 Golden build 均为 1.26.1，
CGO/PG tags/命令配置明确。历史 provenance 缺口没有补造为完整证据。

## Golden、lint 与 whitespace

直接核对 manifest、旧案例及最终 artifact 的 `cases[].assertions`：

| 集合 | 案例 | 断言 |
|---|---:|---:|
| 原集合保留 | 25 | 286 |
| 新增 | 10 | 211 |
| 合计 | **35** | **497** |

原 25 格定义未变；required set 与 executed set 一致，
无重复 ID、无失败案例或断言。永久测试、companion 和 native probe 不计入这些分母。
`artifact.head_sha` 与 `cli.build.head_sha` 均为完整产品 SHA `2f01f500...`。

```text
artifact SHA-256:
eee3c43c660d9ba1a10d0e67da5b72eab2b41f2007c7352371554ec650e8014b
CLI SHA-256:
adbec424f57663d75f5f1fc72bfdee63944b5ff7dd48e582565e162d7e83b833
```

lint 沿用原 `file:linter:message` 身份算法，去除 line/column：
scoped 原始诊断 **22→22**、身份 **18→18**；
fullrepo 原始诊断 **39→39**、身份 **33→33**，均零新增/零移除。
这不改变两条原始 lint 命令 rc=1。

产品差异 whitespace rc=0；证据 README/tools rc=0。
证据全差异 rc=2 的 **2,412 条诊断来自 17 个 `development/` 原始日志/patch 路径**：
261 trailing whitespace、2,142 patch 上下文中的 space-before-tab、
9 EOF blank line。未为美化检查改写历史材料，未放宽配置。
真实合同属于产品差异检查；未把不存在的证据目录合同路径的空匹配当作验证。

## 资源口径与逐对象结果

先保存易失记录，后执行测试。现场保存起点：
**2026-11-04T22:23:35.215893Z**，本地偏移 **+08:00**。
有界 events 查询成功，但 daemon 仅返回 256 条，
集中在 **17:18:57–17:19:03Z**，覆盖 Golden 收尾。
不能由有限历史中没有事件推断更早没有删除。
另保全 78 个 Desktop 当前/轮转日志、原任务执行记录及当前同口径清单。

当前为本地 Docker Desktop，context `desktop-linux`，Server 29.7.2；
默认 socket 与用户 socket 指向同一端点。
历史基线未同时封存 daemon ID、宿主稳定身份、版本及数据盘位置，
因此没有宣称完整证明环境从未切换/迁移。

| 类别 | 基线 | 当前 | 保留 | 缺失 | 新增 |
|---|---:|---:|---:|---:|---:|
| 容器 | 148 | 8 | 4 个完整 ID | 144 个完整 ID | 4 个完整 ID |
| 卷 | 116 | 8 | 5 个同名项，原身份/内容未证 | 111 个名称 | 3 个名称 |
| 网络 | 19 | 12 | 9 个完整 ID | 10 个完整 ID | 3 个完整 ID |

排除了 running/all 混用、短 ID、列顺序、过滤器、命令失败当空列表及把停止视为删除等假差异。
原 live 基线的清单命令 rc=0；不是复用一次更早清理的旧快照。
历史 daemon 身份缺口仍保留，但后续清单再次包含 147 个基线完整容器 ID，
增强了环境连续性与实际消失的证据。
同名卷存在不等于原卷内容仍在。

## 清理关联与证据强度

相关 BinlogServer 测试任务保留的命令回执带实际纳秒时间：

| 快照 | 完成 UTC | 全部容器 | 其中基线完整 ID |
|---|---|---:|---:|
| cp0001 after | 2026-11-04 16:39:01.848107 | 190 | 147 |
| cp0002 before | 2026-11-04 16:46:10.149579 | 6 | 4 |

两次均由全量容器选择后 inspect 得到完整 ID，rc=0。
**143 个基线容器在该窗口消失**，北京时间为 11 月 5 日 00:39:01–00:46:10，
早于 Golden 收尾。另一个缺失测试数据库容器在 cp0001 前已缺失，窗口必须分开。

已完成工具记录包括：

- 按名称前缀生成 184 个容器名称和 136 个卷名的展开清单。
- 第一次 `xargs -a` 在 macOS 失败；其回显 rc=0 来自管道末端，不能当删除成功。
- 后续实际执行读取清单的容器 `rm -v`、`volume rm`，并执行显式名称的 `network rm`。
- 另有 builder prune 和一次临时诊断容器调用，亦为历史记录，本轮未执行。

| 当前缺失类别 | 总数 | 命中实际清理目标 | 未匹配 |
|---|---:|---:|---:|
| 容器 | 144 | 143 | 1 |
| 卷名 | 111 | 106 | 5 |
| 网络名 | 10 | 10 | 0 |

展开清单的冻结指纹：

```text
container names SHA-256:
c23f46f681b67c8afbc0196fc47418ad27b423bda44a49955b3caf6bb15512b8
volume names SHA-256:
743831e0ae7c16564b32ac6749550ac7c95139823bab07945294399bc11ed11d
```

容器有完整 ID 前后清单支撑，卷/网络清理参数是名称。
输出经 `tail` 截断，缺少每个删除子命令的完整输出及独立 rc；
不能把上述关联扩张为全部对象的逐项 destroy 回执、原卷内容身份或自然人身份。
**现有记录没有确认这次清理的用户授权。**
代理交接、成功执行和文件存在都不替代用户授权。
更早的一次审批对象清单与本次缺失集合不匹配，不能拿来替代本次授权。

## 未闭合影响与取证隐私

剩余 1 个测试数据库容器没有与其完整 ID、时间窗相符的删除回执。
5 个匿名卷名不在 volume-rm 清单；`rm -v` 可能处理匿名卷，
但缺少历史 mount 关系，不能据此认定就是这 5 个。
精确日志检索只找到更早的 volume-contents GET 记录，没有删除、内容或挂载归属证明。

83 个缺失卷有历史 Compose 挂载用途声明：
61 指向 `/var/lib/mysql`，19 指向 `/s3mock`，1 指向 `/data`，2 指向 `/hold`。
这些是相关 fixture 的持久挂载声明，不是历史 inspect、实际数据内容或业务关键性证明；
另 28 个卷名缺少这种声明关系。
容器消失不能证明宿主 bind mount 目录被删；同名重建不能证明恢复。
业务影响与可恢复性仍需进一步授权后的专门工作。

本轮没有启动辅助容器、读取卷内容、挂载/复制数据盘、创建同名卷、
初始化数据库或恢复备份。补充 Desktop settings 读取未完成，不计作成功环境证明。
首次识别差异的旧脚本未封存独立起止时间，未编造其精确发现时刻，
也未用提交时间代替资源操作时间。

**取证隐私处理未通过。**
曾在工具回显中出现 fixture 默认凭据，随后历史命令筛选又漏掉了
`-p…` 和 DSN 写法中的数据库凭据。已明确向用户说明，
后续改为只输出白名单字段，本报告不包含这些值。
没有执行历史命令、打开其引用的凭据文件、连接数据库或自动轮换凭据。
不能因产品通过而掩盖这项取证失误。

## 发布与保全边界

原始日志、工具记录、逐对象完整清单及可复算材料继续保持受限本地保存：
目录 0700、文件 0600。本公开报告不包含凭据、完整环境变量、私人配置、
宿主绝对路径或原始宿主日志。

冻结的本地完整报告 SHA-256：

```text
c13737f53f01284a3c2af92a70245f872e3887c7c5674fd0a450671477736323
```

公开摘要提供结论、分母、身份、时间窗、证据强度和指纹；
它不假装已公开全部原始材料，也不单独提供完整复现包。
如审核者需要逐对象或原始回执复核，应按授权范围另外提供脱敏材料。

本次发布仅修改这份 Markdown 报告，验证采用文档 diff、隐私白名单检查和远端 ref 核对。
未运行产品测试/Golden，未修改 milestone/main/workfree 引用，
未创建 PR、合并、关闭 issue、发布版本、部署或进入 T21。
没有远端 CI 成功结论；报告可访问与 CI/产品发布是不同事实。
