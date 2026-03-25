# DeltaScope v1 决策日志

## 目的

本文件记录在没有同步用户反馈的情况下构建 `DeltaScope` v1 过程中遇到的实施期决策、权衡与问题。

## 决策 1：v1 的交付形态

- 决策：优先构建 `library + CLI`，将 HTTP API 与 MCP 延后到后续版本。
- 原因：第一批目标用户是本地运行的 AI coding agents；离线消费比长期运行的服务形态更重要。

## 决策 2：v1 的运行时模型

- 决策：v1 仅支持离线模式，不要求实时数据库连接。
- 原因：本地开发者和 agent 环境通常无法访问生产元数据，而确定性的离线审计比部分在线行为更有价值。

## 决策 3：配置模型

- 决策：使用单一 YAML 配置文件，通过 Viper 加载，按领域分组，并以规则 ID 为键。
- 原因：这种方式便于人工编辑，与 Cobra/Viper 兼容，也比平铺的大写配置 flag 更易扩展。

## 决策 4：架构风格

- 决策：采用偏 DDD 的结构：`interfaces -> application -> domain <- infrastructure`。
- 原因：这次重写应避免 `gAudit` 那种以 checker 为中心的结构，并把 parser/config/CLI 等关注点排除在核心领域之外。

## 决策 5：审计模型

- 决策：规则针对以 `StatementSpec` 为根的统一领域模型求值，而不是直接针对 parser AST 类型。
- 原因：这样可以让规则保持 parser-neutral，也为未来 HTTP API 与 MCP 复用提供更好的基础。

## 决策 6：结果契约

- 决策：finding 使用 `blocker`、`warning` 和 `notice` 三个级别；最终 verdict 为 `reject`、`review` 或 `pass`。
- 原因：这些命名比通用的运行时严重级别更能表达治理语义。

## 决策 7：文档契约

- 决策：将 `three-level-doc` 作为主动开发门槛引入。
- 原因：仓库还是新的，现在建立 L1/L2/L3 结构远比以后补改更便宜。

## 决策 8：Task 2 之后的领域跟进

- 问题：最初的 Task 2 实现把策略参数建模得过窄，对全局 finding 的结果命名也不够清晰，而且 statement kind/dialect 仍然是原始字符串。
- 决策：
  - 将规则参数泛化为 `Params map[string]any`
  - 将结果级 findings 重命名为 `GlobalFindings`
  - 引入类型化的 `Kind` 与 `Dialect`
- 原因：这些都是 Task 3 及后续依赖的基础类型；若等到配置加载和 API 接线成型之后再修，成本会更高。

## 决策 9：Viper 对规则 ID 的加载方式

- 问题：像 `dml.where.require` 这样的规则 ID 带有点号，而 Viper 默认会把点号视为嵌套路径分隔符。
- 决策：使用 `viper.NewWithOptions(viper.KeyDelimiter("::"))` 创建 Viper 实例，让点分规则 ID 保持为字面量 map key。
- 原因：配置设计本来就是按 rule ID 键控。在这个加载器里，保留点分 ID 比沿用 Viper 默认的嵌套键行为更重要。

## 决策 10：kind 建模继续留在 `statement.go`

- 问题：Task 4 最初提到 `internal/domain/spec/kind.go`，但在 Task 2 的后续工作里，`Kind` 和 `Dialect` 已经被放进了 `statement.go`。
- 决策：继续把 `Kind` 与 `Dialect` 保留在 `statement.go` 中，不再创建重复的 `kind.go`。
- 原因：为同一组类型再建一个文件只会增加噪音，不会改善所有权；Task 4 只需消费已有领域类型来完成分类。

## 决策 11：parser 模块与 Go 版本

- 问题：当前最新的 `github.com/pingcap/tidb/pkg/parser` 模块要求更高版本的 Go toolchain，而且其 driver 导入路径使用的是 `pkg/parser/test_driver`，而不是旧代码库里常见的 `pkg/types/parser_driver`。
- 决策：
  - 直接依赖 `github.com/pingcap/tidb/pkg/parser`
  - 导入 `github.com/pingcap/tidb/pkg/parser/test_driver`
  - 接受模块 `go` 版本提升到 `1.26.1`
- 原因：这样可以让 DeltaScope 与当前 parser 模块边界保持一致，而不是复制 `gAudit` 中过时的 TiDB 集成细节。

## 决策 12：由 application 拥有解析结果

- 问题：如果 `internal/application/audit` 直接返回基础设施 parser 的结果，就会让承载 TiDB AST 的类型从 application 契约中泄漏出来。
- 决策：
  - 原始 AST node 只保留在 application 包内
  - 对外暴露 `ParsedSQL` 和 `ParsedStatement` 这类由 application 拥有的类型
  - AST node 继续作为未导出字段存在，供后续提取工作使用
- 原因：Task 5 仍然需要原始 parser node，但偏 DDD 的边界不应让基础设施结果类型或 AST 符号越出 application 流程。

## 决策 13：提取逻辑放在 `internal/application/audit`

- 问题：原始 Task 5 计划中列出了 `internal/infrastructure/parser/tidb/extractor.go`，但 Task 4 已经把携带 AST 的 parsed statement 放在 application-owned 契约之后了。
- 决策：
  - 在 `internal/application/audit` 中实现提取
  - 基础设施仅负责 parsing
  - 避免在 `internal/infrastructure/parser/tidb` 再做第二层 extractor
- 原因：提取过程消费的是 application-owned parsed statement 内部隐藏的 AST，并生成领域 `Statement` 值。把这层放进 infrastructure 会重复边界工作，也会削弱 application/domain 的接缝。

## 决策 14：第一轮提取保持 DDL 约束与 DML join 形状分离

- 问题：一个天真的首版 extractor 会把主键和其他约束误标为普通索引，同时也无法区分“没有 join”和“有 join 但没有 ON”。
- 决策：
  - 将 `PrimaryKey` 与 `Indexes` 分开建模
  - 在 `DDL.Constraints` 中保留其他非索引约束
  - 增加 `HasJoin`，与 `HasJoinOn` 并存
  - 对未知但可解析的语句，允许其通过提取层，而不是硬失败
- 原因：后续 DDL / DML 规则需要这些区分；提取层不应通过过早拒绝可解析语句，把策略判断硬编码进去。

## 决策 15：registry 强制规则 ID 正确性

- 问题：规则 ID 是配置与输出的核心，但一个天真的 registry 会把它当成“建议值”，从而允许重复或不匹配的标识符滑过去。
- 决策：
  - 在注册时拒绝空规则 ID
  - 在注册时拒绝重复的 statement/global rule ID
  - 当 finding 的 rule ID 为空时，用已注册规则 ID 自动补齐
  - 拒绝那些声称与已注册规则 ID 不同的 finding
  - 把 report-flow 的集成覆盖放在 `internal/application/audit`，而不是 domain/rule 单测里
- 原因：Task 7 / 8 会引入很多真实规则，规则 ID 的正确性必须在引擎边界被强制，而不能只靠约定。

## 决策 16：第一批 DDL 规则聚焦 create-table 结构

- 问题：Task 7 很容易扩散成一堆浅层检查，尤其是像 audit columns 和 alter restrictions 这类需要比 v1 当前模型更丰富的提取元数据的内容。
- 决策：
  - 第一批 DDL 规则只针对 `CREATE TABLE`
  - 先交付四个高信号规则：表注释必填、表名最大长度、主键必填、主键最大列数
  - audit-column 与 alter restrictions 留到后续批次，等 extractor 能安全捕捉更多形状后再做
- 原因：这样既能证明规则架构的可行性，也能贴近 `gAudit` 的核心价值，同时避免对领域模型尚未暴露的元数据写出脆弱规则。

## 决策 17：DML 规则需要显式操作语义

- 问题：原始 `spec.DML` 只暴露了 `HasWhere`、`InsertRows` 这类 flag，却没有明确指出一条语句到底是 `INSERT`、`UPDATE` 还是 `DELETE`。
- 决策：
  - 增加 `spec.DMLOperation`
  - 在每条提取出的 DML 语句上记录该操作类型
  - 在 DML 规则中使用基于 operation 的适用性判断，而不是从偶发字段推断行为
- 原因：Tier-1 DML 规则在 mutation 语句与 insert-family 语句之间差异很大。把 operation 编码进领域模型，比依赖 `InsertRows > 0` 之类启发式方法更干净也更安全。

## 决策 18：在公开 API 存在前先重命名 insert-select 元数据

- 问题：早期的 `spec.DML.IsSelectInto` 字段名具有误导性，因为它表达的是 `INSERT ... SELECT`，而不是 `SELECT ... INTO`。
- 决策：
  - 将该字段重命名为 `IsInsertSelect`
  - 增加 `HasOnDuplicate`
  - 保持 subquery 检测仍然以 statement 为中心，而 `IsInsertSelect` 专门留给独立的 insert-select 规则
- 原因：这仍然是内部领域模型，现在修正命名可以避免把令人困惑的术语泄漏到未来的公开 API 和输出契约中。

## 决策 19：夜间流程不应阻塞在异步 reviewer 周转上

- 问题：夜间实现流程仍遵循 subagent-driven review 模式，但 reviewer 的返回速度有时比编码循环慢，不应该让独立的后续任务因此停滞数小时。
- 决策：
  - 保留正式 reviewer 检查点
  - 当前一任务没有已知阻塞缺陷时，在本地验证后允许控制器继续进入下一个独立任务
  - 在本日志中明确记录这种交接，并在合并前协调后续 reviewer 发现
- 原因：用户要求的是无人值守的夜间推进。空等 reviewer 延迟只会浪费开发窗口，并不会提升代码质量。

## 决策 20：公开包拥有自己的结果契约

- 问题：如果 `pkg/deltascope` 直接暴露 `internal/domain/*` 的结果类型，就会让外部消费者与内部领域重构耦合，使公开 API 比看起来更不稳定。
- 决策：
  - 保持 `pkg/deltascope` 的 request/result 类型与内部领域包分离
  - 在包边界将内部 report 与 finding 类型映射为公开等价类型
- 原因：v1 是 library-first。一个小而稳定的公开契约，比省掉一层薄映射更有价值。

## 决策 21：省略公开 dialect 时默认为 MySQL

- 问题：库的第一批消费者与未来 CLI 的大多数场景都会针对 MySQL 语法；要求每次 inline audit 都显式传 dialect，会增加摩擦，但安全收益不大。
- 决策：
  - 将空的公开 dialect 视为 MySQL
  - 仍然拒绝未知的非空 dialect 值
- 原因：这样可以让本地常见路径更顺畅，同时在调用方显式设置 dialect 时仍保留严格校验。

## 决策 22：Task 7 的验收按完成的 DDL 批次，而不是穷尽所有已规划 DDL 关注点

- 问题：原始 Task 7 提示把 audit columns、column constraints、index constraints 和 alter restrictions 都放在同一批次，但当前提取出的 DDL 模型只支持安全的一小片范围，如果硬塞更多内容，只会得到脆弱规则。
- 决策：
  - 将现有聚焦 `CREATE TABLE` 的 DDL 规则集视作第一批已完成的 Task 7 工作
  - 剩余 DDL 关注点保留为显式后续规则批次，而不是硬塞进同一提交中做浅实现
  - 在日志中保留该决策，以便后续 reviewer 与晨间交接能够区分“有意分批”与“静默遗漏”
- 原因：v1 仍然需要更广的 DDL 覆盖，但把所有已规划关注点都压进早期一个批次，只会过度适配当前 extractor，或者牺牲规则质量。明确记录拆分方式，能让这种权衡更可见，也能保持夜间开发继续推进。

## 决策 23：v1 公开 API 保持 request-driven 与 config-path based

- 问题：Task 9 需要一个稳定的公开库接口，但暴露内部 policy/domain 类型会把实现细节泄漏到 CLI、HTTP API 和 MCP 这些本该位于其上的界面中。
- 决策：
  - 暴露单一公开入口 `Audit(ctx, request)`
  - 保持公开 request 形状尽量小：SQL 文本、dialect 和可选 config path
  - policy 加载仍留在 application service 内部，而不是让调用方传入内部 policy struct
- 原因：这样可以让公开界面保持狭窄且稳定，同时仍满足 v1 对默认策略审计与基于文件的覆盖需求。

## 决策 24：renderer 消费 domain report 结果，而不是公开包结果

- 问题：Task 10 需要 Markdown 与 JSON renderer，但如果让基础设施直接依赖公开 `pkg/deltascope` 类型，就会反转预期依赖方向，并让 renderer 与库包装层耦合。
- 决策：
  - 让 renderer 针对 `internal/domain/report.Result` 实现
  - 让公开包保持为 application service 之上的转换层
  - CLI 与未来适配器可以选择调用内部服务或公开包，但 renderer 的依赖继续朝内收敛
- 原因：输出格式化属于 infrastructure，但它仍应消费核心结果契约，而不是更高层包装器。这样既保持偏 DDD 的依赖方向，也让公开 API 可以继续作为轻量 facade 演进。

## 决策 25：公开 finding level 是类型化的，不是自由字符串

- 问题：Task 9 最初在 `pkg/deltascope` 中把 finding severity 暴露为原始字符串，这削弱了公开契约，而领域层其实已有封闭的 severity 集合。
- 决策：
  - 在 `pkg/deltascope` 中暴露公开 `Level` 类型
  - 在包边界将内部规则级别映射成该类型
- 原因：这样可以让库 API 更明确、更稳定，同时不把内部领域包直接暴露出去。

## 决策 26：`config init` 从默认策略模型渲染 YAML

- 问题：Task 11 要求 `deltascope config init` 输出一个可用 YAML 模板，但 CLI 包在运行时无法可靠地通过相对路径嵌入或读取 `configs/deltascope.example.yaml`。
- 决策：
  - 从 `internal/domain/policy.Default()` 生成模板
  - 以确定性的排序顺序输出规则
- 原因：这样可以让命令保持自包含，避免运行时路径假设，并保证输出模板与真实默认规则集保持一致。

## 决策 27：审计阈值失败返回退出码 1，且不额外污染 stderr

- 问题：在 CLI 中，由 findings 达到 `--fail-on` 导致的非零退出，其实是正常的审计结果，而不是工具/运行时失败。如果再额外打印一行 stderr 错误，会让脚本与 agent 更难集成。
- 决策：
  - 渲染后的审计结果继续输出到 stdout
  - 当达到配置的 finding 阈值时返回退出码 `1`
  - 对该情况不额外打印 stderr 错误
- 原因：这样既保留约定好的退出码契约，也让自动化能继续友好消费成功的审计输出。

## 决策 28：不可读或无效的 CLI 配置文件属于用户错误

- 问题：最终 review 发现，对错误的 `--config` 输入，系统把它报告成了内部/运行时失败，即便坏路径或坏格式其实是用户提供的。
- 决策：
  - 将 Viper 配置查找与解析失败归类为 CLI 用户错误
  - 这类失败保留在 stderr，并返回退出码 `2`
- 原因：这与文档中的退出码契约一致，也避免自动化把用户输入错误误判成工具故障。

## 决策 29：`--quiet` 将 Markdown 输出切换为扁平 finding 列表

- 问题：第一版 CLI 定义了 `--quiet`，但它在行为上并没有实际效果，这会让 flag 产生误导。
- 决策：
  - 在 `--quiet` 下保持 JSON 输出不变
  - 让 Markdown quiet 模式输出一个扁平的逐 finding 列表，而不是完整报告包装
  - 当审计没有 finding 时输出 `pass`
- 原因：这样能给这个 flag 一个最小但有意义、对机器友好的行为，而不必额外新增第二套完整 renderer 栈。

## 决策 30：第二轮 DDL 扩展先做列治理，而不是 alter 限制

- 问题：下一轮 DDL 里程碑需要继续缩小与 `gAudit` 的差距，但当前提取模型仍缺少实现高质量第二批 alter/index 规则所需的安全、细粒度元数据。
- 决策：
  - 优先做列治理规则
  - 用离线事实扩展 DDL column spec，例如长度、nullability、default 与 current-timestamp 语义
  - alter restrictions 与更深的索引规则留到后续批次，等提取模型更丰富之后再做
- 原因：列规则能立刻增加高价值的离线覆盖，而且可以安全地基于当前 create-table 形状实现，无需发明脆弱元数据。

## 决策 31：审计列检测保持基于模式、与列名无关

- 问题：`gAudit` 风格的审计要求关注 created/updated timestamp 行为，但如果把精确列名写死，规则在不同 schema 中就会不够可移植。
- 决策：
  - 将审计列视为语义模式，而不是固定名称
  - 要求存在一个带 `DEFAULT CURRENT_TIMESTAMP` 的 time-like 列
  - 要求存在一个同时带 `DEFAULT CURRENT_TIMESTAMP` 与 `ON UPDATE CURRENT_TIMESTAMP` 的 time-like 列
- 原因：这样既保留治理意图，也能对使用不同审计列命名的团队保持适用性。

## 决策 32：create-table 索引规则需要类型化的索引元数据

- 问题：列治理之后，下一个主要 DDL 缺口是索引策略，但领域模型起初只保留了索引名与索引列。
- 决策：
  - 增加 `spec.IndexKind`
  - 将提取出的索引分类为 `primary`、`secondary`、`unique` 或 `fulltext`
  - 让规则继续只消费领域 `Index` 值，而不是 AST 约束类型
- 原因：前缀与重复索引检查都需要稳定的语义分类，而这正是解锁它们所需的最小模型扩展。

## 决策 33：精确重复索引检测是安全第一步，而不是完整冗余分析

## 决策 34：元数据感知审计保持在同一条引擎路径上

- 问题：更深的审计覆盖现在需要实时实例事实与表快照，但如果再引入一条独立的“online mode”管线，就会把规则行为拆成两套，也会让 CLI / HTTP 更难理解。
- 决策：
  - 保持一套审计引擎
  - 让元数据感知模式通过 `spec.Statement.Metadata` 附加 schema、instance facts 与目标表快照
  - 规则在这些事实存在时按需消费，不存在时诚实跳过
- 原因：这样既保留离线优先契约，又能在不分叉架构的前提下提供更强检查。

## 决策 35：对象范围治理使用显式 denylist 参数

- 问题：验收矩阵中仍然存在针对 DDL 与 DML 的 DB/table blocklist 缺口。
- 决策：
  - 增加 `ddl.table.denylist.forbid` 与 `dml.table.denylist.forbid`
  - 支持 `schemas`、`tables` 与 `qualified_tables` 参数
  - shipped defaults 保持为空，使规则已交付但在未配置前不生效
- 原因：这样可以补齐受保护表治理缺口，而无需发明第二套策略子系统。

## 决策 36：基于元数据的 alter 选项兼容性必须显式且狭窄

- 问题：alter-column 兼容性已有覆盖，但 `ALTER TABLE ...` 选项变更仍缺少与当前 schema 的元数据对比。
- 决策：
  - 增加 `ddl.alter.table_option.compatibility.require`
  - 只对显式的 `engine`、`charset`、`collation`、`row_format` 与 `auto_increment` 选项变更，与当前 snapshot 做比较
- 原因：这覆盖了真实审计缺口，同时仍然对 snapshot 能证明的内容保持诚实。

## 决策 37：alter-added 冗余索引应复用 create-table 逻辑

- 问题：`ALTER TABLE` 中新增索引已经有宽度/重复检查，但更深入的 left-prefix 与 unique-overlap 冗余分析仍落后于 create-table。
- 决策：
  - 复用现有 create-table 冗余索引规则
  - 在求值前，将 `snapshot.Indexes + alter-added indexes` 投影成一个临时生命周期视图
- 原因：这样可以在 create-table 与 alter-table 之间保持冗余语义一致，而不需要写第二套算法。

## 决策 38：sizing 检查是粗粒度预检保护，而不是精确存储模拟器

- 问题：剩余矩阵缺口要求基于实例事实进行 row-size 与 index-size 检查，但做精确的引擎/运行时模拟既脆弱又与收益不成比例。
- 决策：
  - 增加基于元数据的粗粒度 row size 与 index key length 保护
  - 对 charset/default-row-format/large-prefix 上下文要求 instance facts
  - 在文档中将其定位为保守的 preflight checks，而不是精确的执行期预测
- 原因：这样能够诚实地补齐基线覆盖缺口，同时给用户可执行信号，而不假装已经解决完整的存储引擎分析。

- 问题：更广泛的冗余索引分析虽然有价值，但当前离线模型只能在不虚构优化器或实时 schema 语义的前提下，安全判断精确重复签名。
- 决策：
  - 先为 create-table 规则交付精确重复索引检测
  - 对 alter-added indexes 复用相同的精确签名逻辑
  - 更广泛的 left-prefix / redundancy 分析留到后续里程碑
- 原因：精确重复索引是高信号且可由当前 parser-neutral 事实诚实表达；更广泛的冗余分析则需要更丰富的语义与更谨慎的误报控制。

## 决策 34：source-aware alter facts 必须显式，而不是推断出来的

- 问题：`MODIFY COLUMN` 与 `CHANGE COLUMN` 会带完整的目标定义，但这并不能证明语句显式修改了哪些语义。
- 决策：
  - 保持 `AlterColumn.Change` 只表示语句局部显式触及的 nullability、default 与 auto-increment 变更
  - 不把目标 type 或 unsigned 形状标记为显式变更事实
  - 当下游规则需要目标侧策略检查时，让它们单独检查目标 `Definition`
- 原因：这样可以让 source-aware alter 模型保持诚实，避免下游策略过度声称离线提取其实并不掌握的源到目标真相。

## 决策 35：目标侧 alter 规则应保持为目标侧

- 问题：早期 alter 规则命名逐渐漂向“compatibility”语言，即使实现实际上只判断提取出的目标列类型是否落在保守的允许族集合中。
- 决策：
  - 让 target-type-family 规则明确地命名为 allowlist
  - 只有那些语句能清楚表达的语义，才使用 explicit-change forbid 规则
  - 真正的 source-to-target compatibility 规则留到模型能诚实支持时再做
- 原因：稳定的 rule ID 是产品表面的一部分；它们应描述真实行为，而不是未来希望具备的语义。

## 决策 36：alter-added 索引生命周期检查应包裹 create-table 索引规则

- 问题：Milestone 3 需要更强的 alter-index 治理，但如果把 create-table 索引规则体复制到 alter 专用规则中，立即就会产生漂移。
- 决策：
  - 将 alter-added index payload 投影成临时的 parser-neutral 索引列表
  - 通过 wrapper 复用现有的 create-table 前缀、宽度与精确重复索引规则体
  - 只有在对应策略显式启用时，才注册 alter-added 宽度/重复规则
- 原因：这样可以让 alter-index 治理与 create-table 行为保持一致，避免重复规则逻辑，并对默认 shipped policy 中是否启用保持诚实。

- 问题：`gAudit` 具有更广的冗余索引关注点，但完整的冗余分析需要表达式、前缀长度与 left-prefix 语义，而这些目前都还没有被 DeltaScope 提取出来。
- 决策：
  - 暂时只实现精确重复检测
  - 比较索引 kind 以及按顺序排列的索引列
  - 更广的冗余索引分析留到后续批次
- 原因：精确重复索引是高信号且可安全离线检测的，而更广的冗余逻辑在当前模型下很容易过度声称能力。

## 决策 37：在更丰富的 alter 建模之前，先交付 alter restrictions

- 问题：`gAudit` 覆盖了若干 alter 相关治理开关，但 DeltaScope 当前的 alter 模型只暴露了规范化 action 加上单个相关 name。
- 决策：
  - 先交付 action-level alter forbid 规则
  - 保持其粗粒度且由策略驱动
  - 更丰富的类型/存在性分析留到 alter 模型超出 `Action + Name` 之后再做
- 原因：这样可以立刻捕获有意义的离线治理行为，同时不假装当前模型已经能安全回答更深层的 alter 问题。

## 决策 38：`modify_column` 默认允许，但 rename/change 不允许

- 问题：如果默认禁止所有 alter action，会让 DeltaScope 对常见的迭代式 schema 变更过于嘈杂；但如果把 rename 风格操作也都放开，就会漏掉 `gAudit` 已经保护的一些高风险模式。
- 决策：
  - 默认禁止 `drop_primary_key`、`rename_table`、`rename_column` 与 `change_column`
  - 默认允许 `drop_column`、`drop_index` 与 `modify_column`，但仍保持可由策略覆盖
- 原因：这样既能让默认策略对高风险结构重写保持严格，又不会让所有 alter 工作流都立刻变成 blocker。

## 决策 39：create-table 选项规则值得在 `spec.DDL` 中增加少量 shape 布尔值

- 问题：若干 `gAudit` 表级规则依赖于 create-table 是否使用 `LIKE`、`AS SELECT` 或 partitioning，但领域模型最初没有办法表达这些形状。
- 决策：
  - 在 `spec.DDL` 中增加 `HasReferTable`、`HasSelect` 与 `HasPartition`
  - 保持它们专用于 create-table shape，而不是引入更大的对象种类层级
- 原因：这些布尔值以最小的模型增长解锁了若干高价值离线规则，同时仍把 parser 细节挡在规则层之外。

## 决策 40：表 engine/charset 规则在 v1 中保持 allowlist 语义

- 问题：`gAudit` 带有更丰富的 charset 推荐语义，但 DeltaScope 当前离线模型只保留了显式 option 值，而没有推荐类元数据。
- 决策：
  - 将 engine 和 charset 规则实现为严格 allowlist
  - 要求显式值存在且属于配置列表
  - 如有需要，推荐式 guidance 留到后续批次
- 原因：allowlist 简单、明确且适合离线执行。它已经覆盖当前最重要的治理行为，而不需要提前引入更复杂的策略模型。

## 决策 41：主键语义规则首先针对单列约定

- 问题：团队通常期待的不只是“有主键”，而是“单列 bigint unsigned auto-increment 主键”；但复合主键会让这些语义变复杂。
- 决策：
  - 增加针对 bigint、unsigned、auto-increment 与 not-null 的 PK 语义规则
  - 仅当规范化后的主键恰好只有一列时，才应用 bigint/unsigned/auto-increment 规则
  - `not_null` 检查则对所有主键列都有效
- 原因：这样可以干净地捕获主流约定，而不必为复合主键虚构误导性的语义。

## 决策 42：更丰富的 alter 建模仍保持每条 alter 记录只有一个规范化 subject name

- 问题：早期 rich-alter 草案让 `Alter.Name` 与 rename/change payload 字段并存竞争，这会迫使下游规则去猜哪个 name 才是权威。
- 决策：
  - 保持 `Alter.Name` 作为每条规范化 alter 记录的权威 subject 标识符
  - 仅将 `AlterColumn.OldName` / `AlterIndex.OldName` 用于记录 rename-or-change 历史
  - 通过 `Definition` 携带目标列/索引形状，而不是重复 create-table 的字段集合
- 原因：这样可以让领域契约保持精简、parser-neutral，并为后续规则匹配提供稳定预期。

## 决策 43：多列 alter 规格应规范化为“每个语义目标一条 alter 记录”

- 问题：TiDB AST 可以用一条 `ALTER TABLE ... ADD (...)` spec 编码多个新增列，但规则层期望的是“每个语义 action target 对应一个 `spec.Alter`”。
- 决策：
  - 在提取阶段展开 multi-column add spec
  - 为每个新增列输出一条规范化的 `spec.Alter`，每条都带规范化的 `Name` 与 `AlterColumn.Definition`
  - 不把非索引 `ADD CONSTRAINT` payload 塞进 `AlterIndex`
- 原因：这样可以避免静默数据丢失，让规则表面保持统一，也能阻止应用层 AST 特性泄漏进领域契约。

## 决策 44：alter 目标类型规则必须描述 allowlist，而不是 compatibility

- 问题：第一版语义化 alter 规则名称使用了 `compatible.require`，但实现实际上只检查提取出的目标列类型是否落在保守的允许族集合中。
- 决策：
  - 将这些规则重命名为 `ddl.alter.modify_column.target_type_family.allowlist` 与 `ddl.alter.change_column.target_type_family.allowlist`
  - 保持其行为明确是 target-side、offline-conservative
  - 在文档中说明：除非团队有意放宽，否则 `ddl.alter.change_column.forbid` 仍然是更严格的默认门槛
- 原因：诚实的 rule ID 与文档比“听起来更强”的命名更重要。当前模型并不能证明 source-to-target compatibility，因此导出的接口也不应暗示已经具备这种能力。

## 决策 45：alter-added 索引规则应通过投影复用 create-table 索引治理

- 问题：Milestone 2 需要对 `ALTER TABLE ... ADD CONSTRAINT` 引入的索引做离线治理，但如果把 create-table 前缀规则的实现复制到 alter 专用规则中，就会立即产生漂移。
- 决策：
  - 将 alter-added index payload 投影为临时的 parser-neutral `DDL.Indexes` 列表
  - 复用现有的 create-table 索引前缀规则构造器与行为
  - 将 Task 5 的范围限定为 alter-added unique/secondary/fulltext 前缀检查
- 原因：投影可以保持逻辑复用干净、避免 AST 泄漏，并把新的 alter 表面收窄到当前领域模型能够诚实支持的行为。

## 决策 46：source-aware alter facts 只能编码语句显式证明的内容

- 问题：早期 Milestone 3 草案试图在每个 `MODIFY COLUMN` / `CHANGE COLUMN` 上都标记 `TouchesType` 与 `TouchesUnsigned`，但这些语法总会携带完整目标定义，即使实际意图变更可能只是 nullability 或 default。
- 决策：
  - 保持 `AlterColumn.Change` 只编码语句局部显式触及的内容
  - 移除会过度声称能力的 flag，避免让模型假装语句证明了更多事实
  - 目标 type 与 unsigned 形状继续保留在 `AlterColumn.Definition` 中，但不标记为显式 touched facts
- 原因：下游规则必须能信任每个 flag 的语义。当模型无法诚实证明一类变更关系时，就应只暴露目标形状，把比较决策留给后续更明确的层。

## 决策 47：rename 意图通过名称推导，而不是再加第二层 change flag

- 问题：Milestone 3 Task 1 的早期草案曾在 `AlterColumnChange` 下增加 `Renames` flag，但 rename 意图其实已经可由 `OldName` 与 `Definition.Name` 推导。
- 决策：
  - 删除这个重复的 rename flag
  - 将 rename 推导保留为现有名称字段的派生事实
- 原因：一个事实来源就够了。若在领域模型中重复表示 rename 意图，后续规则就必须决定信任哪一种表示。

## 决策 48：Milestone 4 先固定 breadth-first create-table rule ID，再实现行为

- 问题：剩余的 create-table 工作主要是 breadth-oriented，后续任务在实现前就需要稳定的 rule ID，否则配置、文档与测试会在里程碑中途频繁抖动。
- 决策：
  - 预先固定 Milestone 4 的 rule ID，覆盖四类 create-table 关注点：
    - 标识符与关键字治理
    - 更广的类型族以及 charset/collation 治理
    - 更深的冗余索引分析
    - 剩余的 create-table 对象形状覆盖
  - 即使规则体尚不存在，也先在 `internal/domain/rule/ddl/common.go` 中定义这些 ID
  - 在文档中明确它们是 Milestone 4 的计划界面，而不是已交付行为
- 原因：当前 create-table 更多是 coverage-completion 问题，因此命名稳定性比再多做一层实现前抽象更重要。

## 决策 49：剩余 create-table 命名保持字面且以 family-first 为主

- 问题：Milestone 4 会快速增加大量 breadth 规则，如果命名含糊，就很难判断某条规则到底是 hard forbid、pattern requirement、allowlist 还是 redundancy heuristic。
- 决策：
  - 标识符合法性规则保持在 `*.name.pattern.require`
  - 保留字治理放在 `*.name.keyword.forbid`
  - 类型族限制保持字面命名，例如 `ddl.column.blob_text.forbid`、`ddl.column.json.forbid` 与 `ddl.column.timestamp.forbid`
  - charset/collation 规则明确写出它们是 allowlist 还是 pair-coherence 检查，例如 `ddl.column.charset.allowlist` 与 `ddl.column.charset_collation.match.require`
  - create-table 专属的对象形状补充也保持字面命名，例如 `ddl.table.row_format.allowlist` 与 `ddl.table.auto_increment.init_value.require`
  - 更深入的冗余索引规则要明确写出所采用的启发式，例如 `ddl.index.redundant_left_prefix.forbid`
- 原因：现有规则表面已经在使用 `*.allowlist`、`*.forbid`、`*.max_length` 这样的 family-first 风格；继续沿用它比现在引入更抽象的策略名更清晰。

## 决策 50：未命名 secondary index 在提取后仍应保持未命名

- 问题：MySQL/TiDB 允许使用 `KEY (col)` 语法而不给 secondary index 显式命名，但早期 extractor 会把这类索引规范化成类似 `key` 的合成名称，从而掩盖真实的治理问题。
- 决策：
  - 对未命名的非主键索引，在提取后的领域模型中保留空名称
  - 仅将 `"primary"` 保留为主键的合成标识符
- 原因：标识符与关键字治理应该评估语句真实声明了什么，而不是提取阶段虚构出的占位名称。

## 决策 51：列 collation 必须从列选项中提取，不能只看 field-type 元数据

- 问题：TiDB parser 会把显式列级 `COLLATE` 子句保存在 `ColumnOptionCollate` 中，而在相关 `CREATE TABLE` 形状下，`FieldType.GetCollate()` 仍然为空。
- 决策：
  - 继续使用 `FieldType` 提取显式列 charset
  - 另外在提取阶段读取 `ColumnOptionCollate` 来捕捉显式列 collation
- 原因：如果没有这条分离的提取路径，DeltaScope 就会静默漏掉显式 collation override，后续任何 charset/collation 治理都会不完整。

## 决策 52：row-format allowlist 规则只在显式设置 row format 时才求值

- 问题：对 `ROW_FORMAT` 直接套用通用的 table-option allowlist 行为，会让所有没有显式 row-format 子句的 create-table 语句看起来都不合法，这比预期中的离线检查更严格。
- 决策：
  - 为 table-option allowlist 规则增加 `require_explicit` 支持
  - 保持 engine 与 charset 规则默认仍要求显式值
  - 将 `ddl.table.row_format.allowlist` 配置为 `require_explicit: false`
- 原因：`ROW_FORMAT` 治理的目标是限制显式 row-format 选择，而不是强迫每条语句都写出 row format，只要引擎默认值可接受即可。

## 决策 53：HTTP 适配器应直接复用公开 audit 契约

- 问题：service 里程碑需要 JSON request/response 处理，但如果从更深层的内部类型去映射 HTTP 响应，就会在 `pkg/deltascope` 之外再造一份外部契约。
- 决策：
  - 在 `pkg/deltascope.Audit` 之上构建 HTTP adapter
  - 让 `POST /v1/audit` 直接返回公开的 `pkg/deltascope.Result` JSON 形状
- 原因：library contract 本来就是预期中的稳定外部表面。直接复用它可以保持 CLI、library 与 HTTP 输出对齐，并减少 adapter 特有漂移。

## 决策 54：服务配置热重载采用“按请求重载”实现

- 问题：Milestone 5 需要具备配置驱动的长运行服务行为，但现有 audit 流程本来就在提供 config path 时按每次调用从磁盘重载策略。
- 决策：
  - 在 server 启动时先验证配置路径一次
  - 保持每个 HTTP audit 请求继续走与 CLI/library 相同的配置加载路径
  - 在文档中说明：文件更新会在下一次请求生效，而不是通过 watcher 专用内存缓存来生效
- 原因：这样可以保持只有一条策略加载路径，避免再长出一套长运行配置子系统，同时仍然能为小型服务提供即时配置生效行为。

## 决策 55：能力矩阵是 Audit Completion 的权威验收来源

- 问题：下一个里程碑关注审计完整性，这很容易凭直觉判断“差不多完成”，而不是依据稳定清单。
- 决策：
  - 维护一份专门的 audit capability matrix 文档
  - 将每项重要能力标记为 covered、enhanced、gap 或 deferred
  - 后续规则工作由矩阵中的 gap 驱动，而不是靠临时直觉
- 原因：这样可以让里程碑变得可度量，也让未来关于“审计完整性”的说法可被审计。

## 决策 56：元数据感知访问继续放在 `deltascope audit` 上，而不是再开一条在线命令

- 问题：CLI completion 需要 live metadata access，但如果把它拆成第二条顶级 online 命令，就会分叉 help、示例、错误与长期适配器行为。
- 决策：
  - 保持只有一个 `deltascope audit` 命令
  - 只有在传入 connection flags 时才进入 metadata-aware mode
  - 在线自动检测 dialect，在安全时推断 schema，在存在歧义时诚实失败
- 原因：这样可以保持一致的 audit UX，也符合 application 层使用的一套引擎架构。

## 决策 57：已发布规则目录项由默认策略 rule ID 加解释模板生成

- 问题：CLI 需要 `rules list/show/search`，但如果手工维护第二份已发布规则清单，就会很快与真实默认策略表面发生漂移。
- 决策：
  - 从 `policy.Default().Rules` 派生目录项集合
  - 通过目录模板附加 summary、example、config snippet、remediation hint 等面向解释的元数据
  - 保持 rule execution 与 rule explanation 仅通过 `rule_id` 关联
- 原因：这样可以让目录始终覆盖 shipped surface，同时避免为“究竟发布了哪些规则”维护第二份脆弱事实源。

## 决策 58：元数据感知 CLI live smoke 保持在 Docker 驱动 e2e 中，而不是塞进 `go test ./...`

- 问题：元数据感知 CLI 路径虽然在功能上已完成，但围绕真实 MySQL/TiDB 连接、dialect 自动检测、schema 推断以及基于元数据 finding 的风险，单元测试与 fake provider 仍无法完全消除。
- 决策：
  - 增加一层基于 Docker Compose 的 shell e2e，只通过已发布 CLI 去驱动真实 MySQL 和 TiDB 容器
  - 保持它挂在显式的 `make test-e2e-cli*` 入口下，而不是折叠进 `go test ./...`
- 使用确定性的 fixture schema 去验证唯一 schema 推断、歧义失败、qualified-schema SQL、存在性检查，以及两个引擎上的至少一条 instance-fact-backed sizing 路径
- 原因：这样可以为元数据感知 CLI 表面提供可信的 live proof，同时又不拖慢默认 Go 测试循环，也不把公开行为验证绑定到内部 test double 上。

## 决策 59：发布打包必须只有一条可信路径

- 问题：release readiness 一度开始漂向“一个 workflow 加上一条第二手动打包路径”，这会很快让产物命名与安装文档发生漂移。
- 决策：
  - 使用 `.github/workflows/release.yml` 下基于 tag 驱动的单一路径 GitHub Actions workflow
  - 在该 workflow 中使用 GoReleaser 打包，而不是维护第二套手写 archive script
  - 将 workflow 输出名、checksums、安装文档与 installer 逻辑视为同一份产物契约
- 原因：可发布性取决于整条交付链，而不只是本地测试能否通过。

## 决策 60：发布文档应面向产品，而不是面向计划

- 问题：仓库里已经积累了很多实施计划文档，但面向外部的文档表面仍然像开发中的工作笔记，而不是打磨后的产品入口。
- 决策：
  - 将文档重组到 `admin`、`concept`、`dev`、`recipe` 与 `reference` 之下
  - 将 audit capability matrix 移到 `docs/reference`
  - 将两个根 README 重写为 landing page，并把 L1 模块图下移到文件后部
- 原因：准备发布的文档应帮助运维者和评估者先找到稳定产品信息，而不是先读计划工件。

## 决策 61：installer 与 README 必须共享同一套 archive 契约

- 问题：安装说明很容易与实际发布的 archive 命名漂移，尤其是当 OS/arch 命名在 workflow、文档和 shell script 中被重复编码时。
- 决策：
  - 统一使用 `deltascope_<version-without-v>_<os>_<arch>.tar.gz`
  - 让 `install.sh` 严格按这一命名模式解析
  - 在 `README.md`、`README_ZH.md` 与 `CHANGELOG.md` 中写清同一模式
- 原因：用户不应该还要猜文档、installer 与 release assets 指的是不是同一套产物家族。

## 决策 62：Release Readiness 不包含新的审计范围与新的分发渠道

- 问题：release-readiness 里程碑很容易继续膨胀进新的审计规则、HTTP 增强、MCP 工作或 package-manager 分发，然后永远收不拢。
- 决策：
  - 本里程碑不新增审计规则
  - 不扩展 HTTP service
  - 不增加 MCP server
  - 暂不新增 Homebrew、apt、yum 或其他 package-manager 分发路径
- 原因：这个里程碑的目标是让现有产品可发布，而不是在首个打磨版发布前再次扩大范围。

## 开放跟踪项

- 未来决策：如果真实的配置加载与规则求值开始暴露明显痛点，policy params 是否应从 `map[string]any` 迁移到更强类型的值模型。
- 未来决策：在不把 TiDB AST 关注点泄漏进领域层的前提下，`StatementSpec` 应保留多少 parser-specific 的位置信息/细节。
