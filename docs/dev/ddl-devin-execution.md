# Devin DDL execution contract

Date: 2026-09-21. Normative remote entry: [spec #79](https://github.com/Fanduzi/DeltaScope/issues/79). Each child has a standalone first-path card. This document records the same execution constraints for the local workspace; implementation has not started.

## Devin execution constraints — Golden Path First

This section is normative and narrows execution of this issue. It does not reduce the accepted official-DDL scope. Implement one ticket and one inspectable behavior at a time; do not treat parent #79 as permission for a single 31-ticket coding run.

### 0. Checkout and source of truth

- Remote baseline verified on 2026-09-21: `origin/main = 9a28cc99c38770e6e40738515994f864166419f9`. Local planning commits `4d282a2` / `7a46ebc` have NOT been pushed and cannot be assumed available to Devin. The published parent and child issue bodies contain the governing contract.
- In a fresh clone, fetch remote main, record its actual SHA and inspect differences if it no longer matches the baseline. Start/reuse one milestone checkout on `milestone/mysql-tidb-ddl-completion`. If a branch/worktree already contains previous ticket work, preserve it. Do not reset/clean the workspace, substitute an old baseline, or fabricate access to unpublished commits.
- Before a dependent ticket, verify both the native blocker relationships and the implementing commits/artifacts present in THIS checkout. A closed issue without its code is not an available dependency. Missing prerequisite code, Docker, compiler/CGO or required database topology is an external blocker; report it rather than simulating a pass.
- No automatic push/tag/release or mass-closing issues. Deliver a local commit/diff plus proof. Changes must preserve unrelated work. If the executor cannot carry local commits to its next session, report that delivery blocker; do not silently assume permission to publish source.

### 1. Freeze the first observable behavior before editing production code

For the ticket's Golden Path card, record: input SQL; exact dialect and target version; schema/metadata and policy profile; enabled rule IDs and parameters; expected per-statement findings (IDs, levels, counts and target identity); coverage and gaps; Verdict; process/SDK/transport outcome. A new risk rule's stable ID/level belongs in this expectation BEFORE implementation; reuse an existing semantically equivalent rule instead of adding duplicates. Attach the expectation to the task evidence so code cannot redefine its own oracle.

Only inspect/refactor the path needed for that input. Start with the existing parser adapter -> normalized statement facts -> shared application/policy -> SDK/result -> real CLI. For a bug, enumerate all consumers/callers of the fact being changed, including siblings outside the reported surface, before editing. Do not introduce a generic execution engine, new plugin/interface layer, unrelated cleanup or dependencies to accommodate hypothetical future forms. Parser upgrades require a demonstrated unparseable valid input and versioned compatibility evidence first.

### 2. Proof entrypoint and exact commands

**T01 is immediately runnable** using the self-contained build + Python oracle in #80. No new Make target, Docker fixture, target_version flag or coverage field is needed to start it.

**T02 must deliver** a thin `make ddl-golden TASK=T02 ARTIFACT_DIR=/tmp/ddl-golden` entrypoint around the existing test/build/database mechanisms, and demonstrate its own first real database case. The target is NOT present on the baseline: do not report it as an existing or already-passing command. Later tickets add their own case to that same entrypoint, not a new test framework per ticket.

For T03–T30 the required proof command is:

```sh
make ddl-golden TASK=Txx ARTIFACT_DIR=/tmp/ddl-golden
```

Replace `Txx` with the ticket's literal ID. This must build the current checkout's CLI, execute a real subprocess with SQL/config/connection inputs, parse its JSON and assert the frozen oracle. Metadata/DB paths must use the named disposable real instance and verify the relevant setup/effect/metadata facts. An internal function call or a unit-test-only mock does not satisfy this proof. Existing Go/Python test machinery is sufficient; a Go test using a real CLI subprocess and real database is acceptable.

For T31:

```sh
make ddl-golden TASK=all ARTIFACT_DIR=/tmp/ddl-golden
make test
make pg-unit-test-gates
```

Then run the existing applicable lint/release/transport/privacy/database gates specified by the inventory. Release commands must be nonpublishing; do not tag to satisfy a test.

Every proof writes an inspectable artifact with task ID, git HEAD, build options, actual executable, executed input/case IDs, policy/metadata profile, raw JSON/stderr/exit status, expected/asserted result, and actual database version/digest where used. Redact credentials; do not attach real user SQL. Missing target, zero executed cases, missing oracle fields, skipped required profiles, stale binary or missing database are failures, not passing evidence. Fail if the executed case set differs from the manifest's required set.

After EVERY meaningful change, run the Golden Path FIRST, then focused regression tests. Only once both pass add the next SQL form or boundary case; repeat. Before the task commit run its full acceptance set, `make test`, the applicable PostgreSQL gate and required actual DB cases. Success of the first seed does not close the rest of the ticket's manifest rows.

### 3. Policy isolation and anti-false-green requirements

- A focused golden profile may disable unrelated rules to isolate one semantic check. Assert that its subject rule exists, enable it explicitly with frozen parameters, and retain a separate default-policy regression. Never change repository defaults just to make the proof pass.
- Assert per statement, not the union of rule IDs across a batch. Specify zero findings explicitly for clean controls; empty include lists are not a zero assertion. Findings on the wrong object/line do not count.
- Do not weaken severity, replace supported behavior with unsupported, increment fake warning counts, delete negative fixtures, accept any nonzero error as the expected error, use skip/tolerated failures, or regenerate expectations from actual output to get green.
- Existing catalog changes are deterministic consequences of verified semantic changes; investigate every diff. A 100% figure over a smaller denominator is not evidence of improvement.
- Query and effect SQL executed by tests is restricted to fixed, synthetic, disposable objects. Product analysis remains nonexecuting. DB syntax negatives must fail for the intended syntax/version reason, not missing setup or permission failures.

### 4. Shared public oracle

| Outcome | coverage | Verdict | CLI | SDK / HTTP / MCP |
|---|---|---|---|---|
| All applicable checks complete, no policy finding | complete | pass | 0 | nil / 200 / isError=false |
| Known policy blocker | complete or unverified if another check lacks evidence | reject | 1 at blocker/warning/notice threshold, 0 at none unless independent errors | nil / 200 / false unless an independent error exists |
| Only evidence gaps for enabled applicable checks | unverified | review | warning/notice=1; blocker/none=0 | nil / 200 / false |
| Recognized but semantically unsupported action | incomplete | at least review; preserve reject | 1 even fail-on none | unsupported error / 400 / true |
| Parser failure with valid siblings | incomplete | at least review; preserve reject | 2 even fail-on none | parser error / 400 / true |
| Wholly unparseable input | incomplete | preserve existing empty Verdict behavior | 2 | existing parser error envelope / 400 / true |

Finding counts must equal actual findings, not synthetic gap weight. `coverage.status` is present at result and retained statement levels; aggregate incomplete > unverified > complete. `evidence_gaps` use stable rule_id/reason_code/required_facts and bounded safe identities. Do not put raw SQL, credentials or stored-body text into new diagnostics. Keep original top-level statement counts and map body findings back to their source definition. Version-independent checks do not require a target_version; conflicts with observed online version are input errors.

### 5. Change boundary and stop rule

The card gives module ownership, not permission to edit everything under those directories. Keep every diff explainable by the current input or its regression. Preserve old callers during unavoidable shared type changes; do not rewrite all adapters in the same patch merely for style.

If the SAME blocker survives TWO attempted fixes, stop editing that path. Re-read input normalization, parser output, ownership/state transition and consumer assumptions. Record both attempted fixes and the first broken assumption. Fix at that root boundary or report the external blocker. Do not try a third local guard or continue unrelated tickets to appear productive.

Finish each work cycle with:

- Golden Path: pass / fail / blocked (blocked is NOT pass)
- Proof command, tested HEAD, executed case/profile count and actual result/artifact
- Root blocker, if any; unknown facts remain explicit
- Next smallest change within the current ticket

Task completion additionally requires its remaining acceptance rows, focused regressions, current-checkout evidence, changed module READMEs/file headers where required, ADR/public-contract notes, and Standards + Spec review. Do not claim the entire milestone complete from one golden seed.

## Task-specific first paths

### T01 first Golden Path

- **允许修改的责任范围**：共享DDL目标抽取、DDL spec与denylist；必要时相关target consumer；现有SDK/transport测试与文档。具体改动仍须能追溯到本路径或其回归；不是目录内任意重构授权。
- **输入前提 / fixture profile**：精确隔离政策：读取现有catalog的rule_id后关闭全部规则，只开启ddl.table.denylist.forbid，level=blocker，tables=[sensitive]。必须断言目标rule存在。这样未来新增metadata规则不会污染该oracle；默认政策另跑回归。无需DB/版本新字段。
- **真实输入**：`DROP TABLE harmless, sensitive`
- **独立oracle**：两个方言均1个顶层statement、恰好1个denylist blocker、metadata.table=sensitive、line=1、Verdict=reject、fail-on blocker退出1，无diagnostics/unsupported。
- **必须同时通过的反例**：DROP TABLE harmless, other -> pass/exit0/0 denylist findings；完整16-case脚本还覆盖首/中/后目标与RENAME源/目标。
- **本轮非目标**：不加coverage/version字段，不重写policy loader，不升级parser，不做Prospective Schema State，不新增数据库环境，不把每个目标拆成独立顶层statement。
- **证明命令**：使用下方完整命令。

### T02 first Golden Path

- **允许修改的责任范围**：官方清单、已有fixture/数据库脚本/Make验证入口；baseline状态文件及文档。具体改动仍须能追溯到本路径或其回归；不是目录内任意重构授权。
- **输入前提 / fixture profile**：只使用独立可销毁DB；固定4镜像版本并记录digest/observed version。Docker或架构不可用是外部blocker。
- **真实输入**：`CREATE TABLE golden_t (id INT PRIMARY KEY); ALTER TABLE golden_t ADD COLUMN c INT; DROP TABLE golden_t;`
- **独立oracle**：4个anchor各自执行3个fixture DDL全部成功；在ADD后直接从该数据库metadata确认c存在，在DROP后确认golden_t不存在；记录每个DDL真实返回码。清单行不得未分类或无owner。 同一批输入还必须送入刚构建的真实CLI（所有规则关闭的隔离profile，不使用尚未存在的target_version/coverage字段）：保留3条statement、无diagnostics/unsupported、pass/exit0；不是仅用DB client证明产品路径。
- **必须同时通过的反例**：CREATE TABLE golden_broken ( -> 真正syntax error而非权限/连接错误；区分client stdout和服务器状态；只此测试执行固定fixture SQL。
- **本轮非目标**：不改审计规则，不改153旧结果为新oracle，不构建通用数据库执行平台；不要假装一次测试覆盖所有补丁。
- **证明命令**：`make ddl-golden TASK=T02 ARTIFACT_DIR=/tmp/ddl-golden`

### T03 first Golden Path

- **允许修改的责任范围**：application audit completeness、domain/public结果、错误和renderers；一条共享语义路径。具体改动仍须能追溯到本路径或其回归；不是目录内任意重构授权。
- **输入前提 / fixture profile**：固定mysql方言，关闭所有finding政策；CREATE SEQUENCE是共享parser可识别但不属于该MySQL产品的明确方言边界，不会被后续TiDB序列任务提升为MySQL支持。
- **真实输入**：`CREATE SEQUENCE golden_seq START WITH 1; ALTER TABLE t ADD COLUMN c INT;`
- **独立oracle**：2个顶层statement描述；第一条coverage=incomplete并有明确mysql方言不适用的安全unsupported证据，第二条complete；总coverage=incomplete、Verdict至少review；CLI fail-on none仍1、SDK非nil unsupported、HTTP400、MCP isError=true；无伪造风险finding。
- **必须同时通过的反例**：只审计ADD COLUMN对照为complete/pass/exit0；parser负例保持exit2；全部规则关闭不能关闭incomplete。
- **本轮非目标**：不实现MySQL sequence来变绿，不实现分区风险规则、版本输入或state engine，不破坏性重命名JSON。分区/resource-group临时unsupported另有回归，后续所属任务明确升级预期；永久MySQL sequence边界始终保留。
- **证明命令**：`make ddl-golden TASK=T03 ARTIFACT_DIR=/tmp/ddl-golden`

### T04 first Golden Path

- **允许修改的责任范围**：audit版本输入/metadata事实与evidence gaps、四入口投影及已有阈值判断。具体改动仍须能追溯到本路径或其回归；不是目录内任意重构授权。
- **输入前提 / fixture profile**：只启用现有 ddl.alter.modify_column.compatibility.require(required=true,requires_metadata=true)；使用mysql 8.4.10；旧列长度未知。
- **真实输入**：`ALTER TABLE t MODIFY COLUMN c VARCHAR(20)`
- **独立oracle**：无metadata时coverage=unverified、Verdict=review、至少1条该rule_id的evidence_gap、0虚构兼容finding；warning/notice阈值exit1，blocker/none为0，SDK nil error、HTTP200、MCP isError=false。
- **必须同时通过的反例**：真实旧列VARCHAR(10)->20：complete且无窄化finding；旧列VARCHAR(200)->20：complete/reject且compatibility blocker；禁用该规则不产生该gap。
- **本轮非目标**：不把provider错误降级成gap，不默认最新版，不新增查询执行能力，不改变unrelated Query Access版本合同。
- **证明命令**：`make ddl-golden TASK=T04 ARTIFACT_DIR=/tmp/ddl-golden`

### T05 first Golden Path

- **允许修改的责任范围**：application audit批次状态与既有schema spec/metadata seam，禁止新的通用执行引擎。具体改动仍须能追溯到本路径或其回归；不是目录内任意重构授权。
- **输入前提 / fixture profile**：隔离已知空schema；开启ADD列存在性与索引引用所需检查；服务器快照不包含t。
- **真实输入**：`CREATE TABLE t (id INT PRIMARY KEY); ALTER TABLE t ADD COLUMN c INT; CREATE INDEX idx_c ON t(c);`
- **独立oracle**：3个原始statement；后两条基于前序确定事实，不产生“t或c不存在”finding；读metadata次数/顺序不得每句覆盖已派生state；对照真实fixture执行后的schema含c与idx_c。
- **必须同时通过的反例**：CREATE→未知/unsupported变更→依赖其事实的ALTER：相关结论unverified/incomplete而非继续用旧状态；其它确定对象保持可分析。
- **本轮非目标**：不执行用户SQL，不模拟数据行或事务rollback，不提前实现所有DDL家族；只实现本路径实际需要的状态。
- **证明命令**：`make ddl-golden TASK=T05 ARTIFACT_DIR=/tmp/ddl-golden`

### T06 first Golden Path

- **允许修改的责任范围**：CREATE TABLE提取、已有column/PK/table规则、corpus和批次事实。具体改动仍须能追溯到本路径或其回归；不是目录内任意重构授权。
- **输入前提 / fixture profile**：隔离只启用ddl.table.primary_key.require(required=true,level=blocker)；每个anchor使用其合法INT定义。
- **真实输入**：`CREATE TABLE t (id INT)`
- **独立oracle**：1个statement、complete、exactly 1 primary_key.require blocker、reject、fail-on blocker exit1；实际DB接受DDL，证明团队规范与语法合法性不同。
- **必须同时通过的反例**：CREATE TABLE t(id INT PRIMARY KEY) -> complete/pass/exit0、0 findings；inline/table PK两形式等价。
- **本轮非目标**：不以本路径代替所有column/default条目，不修改默认政策降低阈值，不进入尚未归属本任务的高级列/约束扩展。
- **证明命令**：`make ddl-golden TASK=T06 ARTIFACT_DIR=/tmp/ddl-golden`

### T07 first Golden Path

- **允许修改的责任范围**：生成列/表达式默认值/可见性提取和相关state、规则、fixture。具体改动仍须能追溯到本路径或其回归；不是目录内任意重构授权。
- **输入前提 / fixture profile**：真实表支持所选生成列语法；只启用需要验证的列存在/依赖检查；四anchor先记录适用性。
- **真实输入**：`CREATE TABLE t (a INT, b INT GENERATED ALWAYS AS (a + 1) STORED); ALTER TABLE t DROP COLUMN a;`
- **独立oracle**：第一条保留b依赖a的语义；第二条不能在已知生成列依赖存在时声称无风险：相关policy violation或明确依赖风险结果，记录稳定rule_id/level并在改生产代码前冻结oracle。DB fixture证明依赖关系及预期拒绝原因。
- **必须同时通过的反例**：删除无依赖普通列c不出现“被b依赖”finding；不可见列/表达式default不同版本不允许共享一个未验证positive。
- **本轮非目标**：不执行表达式、不把表达式文本放diagnostics、不实现索引优化器；新rule ID须先在manifest定下。
- **证明命令**：`make ddl-golden TASK=T07 ARTIFACT_DIR=/tmp/ddl-golden`

### T08 first Golden Path

- **允许修改的责任范围**：ALTER column标准化及既有explicit-default/nullability规则与state。具体改动仍须能追溯到本路径或其回归；不是目录内任意重构授权。
- **输入前提 / fixture profile**：版本固定、t.c=INT且default已知；隔离启用显式default改变禁止政策及相应AST支持。
- **真实输入**：`ALTER TABLE t ALTER COLUMN c SET DEFAULT 1`
- **独立oracle**：原来静默的SET DEFAULT得到完整动作；按启用的default-change forbid生成1个对应blocker/reject，fail-on blocker exit1；对应MODIFY COLUMN c INT DEFAULT 1给出同语义结论。
- **必须同时通过的反例**：DROP DEFAULT同维度独立oracle；不触及default的操作不触发default-change；旧状态不足用gap而非“无变化”。
- **本轮非目标**：不统一改写所有SQL文本，不修改PG的已有规则语义，不靠匹配raw SQL关键字发finding。
- **证明命令**：`make ddl-golden TASK=T08 ARTIFACT_DIR=/tmp/ddl-golden`

### T09 first Golden Path

- **允许修改的责任范围**：独立/嵌套索引提取、共享index政策及metadata/state消费。具体改动仍须能追溯到本路径或其回归；不是目录内任意重构授权。
- **输入前提 / fixture profile**：t含a到i共9列；隔离只开启index列数上限8及相应ALTER列数规则，禁用无关命名规则。
- **真实输入**：`CREATE INDEX idx_wide ON t(a,b,c,d,e,f,g,h,i); ALTER TABLE t ADD INDEX idx_wide2(a,b,c,d,e,f,g,h,i);`
- **独立oracle**：2个statement，各自exactly1列数warning、metadata指向本statement索引；overall review，warning阈值exit1；不是把两个findings在批次中合并凑数。
- **必须同时通过的反例**：每种形式8列索引均0列数finding；禁用对应规则后均无该finding；不得靠generic notice满足oracle。
- **本轮非目标**：不提前实现优化器收益估计、cache、全新索引注册框架；不要重命名旧rule ID掩盖差异。
- **证明命令**：`make ddl-golden TASK=T09 ARTIFACT_DIR=/tmp/ddl-golden`

### T10 first Golden Path

- **允许修改的责任范围**：高级index spec与适用规则、版本/engine边界和fixture。具体改动仍须能追溯到本路径或其回归；不是目录内任意重构授权。
- **输入前提 / fixture profile**：真实t有idx_c普通可见索引，目标版本明确，相关可见性风险政策先冻结。
- **真实输入**：`ALTER TABLE t ALTER INDEX idx_c INVISIBLE`
- **独立oracle**：能定位t/idx_c与visibility=false；所选风险规则有稳定ID/level，状态影响可见；MySQL8.0/8.4及适用TiDB用DB目录验证；5.7明确版本不适用而非假complete。
- **必须同时通过的反例**：VISIBLE恢复与不存在索引分别验证；expression/prefix/descending/SPATIAL多值等另外逐manifest行验收。
- **本轮非目标**：不因首个visibility种子通过就关单，不把TiDB fulltext/spatial parser接受当实际支持，不承诺查询性能。
- **证明命令**：`make ddl-golden TASK=T10 ARTIFACT_DIR=/tmp/ddl-golden`

### T11 first Golden Path

- **允许修改的责任范围**：PK/UNIQUE/FK spec、CREATE/ALTER共享constraint规则、依赖状态。具体改动仍须能追溯到本路径或其回归；不是目录内任意重构授权。
- **输入前提 / fixture profile**：parent(id)已存在且键合法；隔离只开ddl.table.foreign_key.forbid(forbid=true,blocker)。
- **真实输入**：`ALTER TABLE child ADD CONSTRAINT fk_c FOREIGN KEY (c) REFERENCES parent(id)`
- **独立oracle**：与CREATE TABLE内同一FK相同：恰好1 foreign_key.forbid blocker、reject、完整引用目标及原始位置；不仅是add_constraint.notice。
- **必须同时通过的反例**：无FK的ADD COLUMN不触发；关foreign_key.forbid后不再该blocker，命名/元数据按剩余规则独立判断。
- **本轮非目标**：不一并实现DB授权系统、执行FK验证查询以外用户SQL，不能丢弃ON DELETE/UPDATE就称完整支持。
- **证明命令**：`make ddl-golden TASK=T11 ARTIFACT_DIR=/tmp/ddl-golden`

### T12 first Golden Path

- **允许修改的责任范围**：CHECK约束结构与执行属性、版本事实和规则。具体改动仍须能追溯到本路径或其回归；不是目录内任意重构授权。
- **输入前提 / fixture profile**：真实t(id INT)与合法chk_id约束；版本/执行开关明确，隔离激活CHECK enforcement变更风险政策。
- **真实输入**：`ALTER TABLE t ALTER CHECK chk_id NOT ENFORCED`
- **独立oracle**：受支持版本保留约束目标和enforced=false，稳定风险finding而非静默；DB metadata确认执行状态变化。5.7或TiDB具体不适用行按真实厂商事实返回边界。
- **必须同时通过的反例**：ENFORCED反向与普通列变更不能触发同一“关闭约束”finding；旧版本接受但忽略CHECK必须揭示，不能报告执行有效。
- **本轮非目标**：不求解任意CHECK表达式，不把权限错误当语法负例；不可偷偷删除旧版本矩阵。
- **证明命令**：`make ddl-golden TASK=T12 ARTIFACT_DIR=/tmp/ddl-golden`

### T13 first Golden Path

- **允许修改的责任范围**：分区目标和动作、既有多目标保护、risk规则/state。具体改动仍须能追溯到本路径或其回归；不是目录内任意重构授权。
- **输入前提 / fixture profile**：真实InnoDB/TiDB range表t，有p0/p1与少量专用数据；隔离只开新drop-partition forbid政策，级别blocker，事先固定rule_id。
- **真实输入**：`ALTER TABLE t DROP PARTITION p0`
- **独立oracle**：1个statement，complete且恰好1对应blocker，table=t/partition=p0，reject/exit1；无unsupported。仅DB测试执行该fixture后p0与其数据消失，p1保留。
- **必须同时通过的反例**：非删除动作不触发drop-partition规则；规则关闭消除该finding但不让未支持语义假complete。EXCHANGE还须两个方向保护目标回归。
- **本轮非目标**：不执行用户SQL，不用generic warning代替已确认blocker，不以早期T03的unsupported结果交付本任务。
- **证明命令**：`make ddl-golden TASK=T13 ARTIFACT_DIR=/tmp/ddl-golden`

### T14 first Golden Path

- **允许修改的责任范围**：分区定义/维护语义、版本engine前提与state。具体改动仍须能追溯到本路径或其回归；不是目录内任意重构授权。
- **输入前提 / fixture profile**：真实range分区p0<200且数据100/150；支持该重组的版本固定；先冻结copy/reorg风险规则。
- **真实输入**：`ALTER TABLE t REORGANIZE PARTITION p0 INTO (PARTITION p1 VALUES LESS THAN (120), PARTITION p2 VALUES LESS THAN (200))`
- **独立oracle**：所有源/目标分区和边界保真，输出该变更具体风险或已验证政策结果；metadata/批次后续看到p1/p2。DB fixture确认数据保留与分区归属。
- **必须同时通过的反例**：错误/重叠边界不能以parser success当安全；未支持engine维护动作显式边界。
- **本轮非目标**：不自己执行分区算法，不预测精确耗时，不用单一RANGE例子替代HASH/KEY/LIST/REMOVE等清单。
- **证明命令**：`make ddl-golden TASK=T14 ARTIFACT_DIR=/tmp/ddl-golden`

### T15 first Golden Path

- **允许修改的责任范围**：table options/online-DDL提取、compatibility规则和版本证据。具体改动仍须能追溯到本路径或其回归；不是目录内任意重构授权。
- **输入前提 / fixture profile**：真实InnoDB表t，版本固定，已知列/engine；隔离开启online/rebuild相关新增规则并冻结其ID/level。
- **真实输入**：`ALTER TABLE t ADD COLUMN c INT, ALGORITHM=COPY, LOCK=EXCLUSIVE`
- **独立oracle**：保留三个语义部分，不能只输出ADD COLUMN notice；按操作/engine/版本给出具体copy/exclusive风险warning（MySQL适用路径），risk位置对应原SQL。
- **必须同时通过的反例**：INSTANT/LOCK=NONE按实际可用性给不同结论，TiDB算法声明按其语义判定，不照搬MySQL；缺前提给gap。
- **本轮非目标**：不承诺锁时长、不强制所有DDL只能在线、不用非参数化SQL执行提交内容。
- **证明命令**：`make ddl-golden TASK=T15 ARTIFACT_DIR=/tmp/ddl-golden`

### T16 first Golden Path

- **允许修改的责任范围**：表生命周期/临时身份/条件子句、metadata/state，复用T01所有目标。具体改动仍须能追溯到本路径或其回归；不是目录内任意重构授权。
- **输入前提 / fixture profile**：隔离已知schema，snapshot明确t不存在，profile允许CREATE/DROP且开相关存在性检查。
- **真实输入**：`CREATE TABLE IF NOT EXISTS t (id INT); DROP TABLE IF EXISTS t; CREATE TABLE t (id INT);`
- **独立oracle**：3条按已知前序状态完成，不能把第二/第三条都查初始snapshot；DB对照最终t存在。unknown初始状态不能同样宣称已知终态。
- **必须同时通过的反例**：临时/持久同名、全允许/保护对象和多表目标序列分别测；TiDB CTAS厂商不支持不计作positive。
- **本轮非目标**：不从IF EXISTS猜存在、不顺便新增事务执行或迁移回滚；一条生命周期种子不覆盖全部临时类型。
- **证明命令**：`make ddl-golden TASK=T16 ARTIFACT_DIR=/tmp/ddl-golden`

### T17 first Golden Path

- **允许修改的责任范围**：database/schema对象与默认属性state、生命周期规则。具体改动仍须能追溯到本路径或其回归；不是目录内任意重构授权。
- **输入前提 / fixture profile**：隔离唯一库名golden_db，不触及预置系统库；版本固定，profile明确允许必要fixture创建。
- **真实输入**：`CREATE DATABASE golden_db CHARACTER SET utf8mb4; ALTER DATABASE golden_db CHARACTER SET latin1; DROP DATABASE golden_db;`
- **独立oracle**：3条库身份/默认字符集变化和删除作用域明确；DROP风险按现有数据库政策保持；DB测试观察中间metadata并确认最终删除，不检查错误的连接默认库。
- **必须同时通过的反例**：IF条件已知/未知分别验证；同名跨scope与系统schema保护不绕过；TiDB忽略/不适用选项显式登记。
- **本轮非目标**：不把schema动态选择做成执行任意库名的测试脚本，测试只用专用对象。
- **证明命令**：`make ddl-golden TASK=T17 ARTIFACT_DIR=/tmp/ddl-golden`

### T18 first Golden Path

- **允许修改的责任范围**：view AST、依赖/声明属性、规则/state与源位置。具体改动仍须能追溯到本路径或其回归；不是目录内任意重构授权。
- **输入前提 / fixture profile**：真实t(id INT)与v，目标版本的ALTER VIEW语法已经官方/DB验证；关闭默认view blanket forbid以检验语义。
- **真实输入**：`ALTER VIEW v AS SELECT id FROM t`
- **独立oracle**：合法MySQL形式不再parser_error；明确target=v和source=t、coverage/政策结论，后续view依赖可用；SQL SECURITY/DEFINER仍保留声明而非凭空授权。
- **必须同时通过的反例**：错误SELECT定义保持失败；缺t的metadata给gap/对应规则，不能因禁用blanket规则而丢掉coverage。
- **本轮非目标**：不执行SELECT、不扩张Query Access能力、不用字符串截取绕开view parser。
- **证明命令**：`make ddl-golden TASK=T18 ARTIFACT_DIR=/tmp/ddl-golden`

### T19 first Golden Path

- **允许修改的责任范围**：TiDB TTL AST/options、自动删除风险规则、census及state。具体改动仍须能追溯到本路径或其回归；不是目录内任意重构授权。
- **输入前提 / fixture profile**：TiDB8.5.0真实t(created_at DATETIME PRIMARY KEY)满足TTL必要前提；冻结启用TTL规则为warning。
- **真实输入**：`ALTER TABLE t TTL = created_at + INTERVAL 30 DAY`
- **独立oracle**：合法形式完整抽取TTL时间列和interval；specific TTL warning，review，warning阈值exit1，不能unsupported或0finding。DB metadata记录TTL定义；自动删除只在单独受控fixture验证。
- **必须同时通过的反例**：整个表达式被引号包裹的既有错误例保留parser_error；REMOVE TTL及TTL_ENABLE OFF不能被误报“开启TTL”。
- **本轮非目标**：不修掉负例就声称能力提升，不启动生产调度，不让regex从错误SQL提取finding。
- **证明命令**：`make ddl-golden TASK=T19 ARTIFACT_DIR=/tmp/ddl-golden`

### T20 first Golden Path

- **允许修改的责任范围**：TiDB placement对象/绑定、已批准safe projection、state。具体改动仍须能追溯到本路径或其回归；不是目录内任意重构授权。
- **输入前提 / fixture profile**：TiDB拓扑前提明确，policy p有效且真实表t存在；不具备所需拓扑则先报告blocker。
- **真实输入**：`ALTER TABLE t PLACEMENT POLICY = p`
- **独立oracle**：明确t和p引用及适用性；binding检查和改变的风险结果不是仅generic notice；DB metadata可定位绑定，后续DEFAULT能解除。
- **必须同时通过的反例**：不存在/未知policy分别给对应错误或gap；mixed ALTER其他action不能被placement吞掉；region约束不泄漏。
- **本轮非目标**：不模拟缺失PD/TiKV拓扑，不用mock-only替代关键真实验证，不把整个网络/集群平台作为本任务新增框架。
- **证明命令**：`make ddl-golden TASK=T20 ARTIFACT_DIR=/tmp/ddl-golden`

### T21 first Golden Path

- **允许修改的责任范围**：TiDB sequence定义/restart/options、state与风险规则。具体改动仍须能追溯到本路径或其回归；不是目录内任意重构授权。
- **输入前提 / fixture profile**：TiDB真实sequence seq，当前配置已知；固定restart-risk规则及允许输出属性。
- **真实输入**：`ALTER SEQUENCE seq RESTART WITH 1`
- **独立oracle**：明确seq和restart行为，按配置给出复用值风险warning，不声称一定与业务数据冲突；DB fixture确认重启语义且结果位置正确。
- **必须同时通过的反例**：安全CREATE普通序列不出现同一restart警告；unknown当前位置不编造碰撞结论，cycle等另行覆盖。
- **本轮非目标**：不读取用户表推断全局唯一性、不在产品分析路径取nextval、不把sequence参数原文写诊断。
- **证明命令**：`make ddl-golden TASK=T21 ARTIFACT_DIR=/tmp/ddl-golden`

### T22 first Golden Path

- **允许修改的责任范围**：TiDB RU资源组AST/spec/规则，明确区别MySQLCPU组。具体改动仍须能追溯到本路径或其回归；不是目录内任意重构授权。
- **输入前提 / fixture profile**：TiDB8.5.0环境满足resource-control前提；固定CREATE/ALTER参数规则而非unknown-kind兜底。
- **真实输入**：`CREATE RESOURCE GROUP golden_rg RU_PER_SEC = 100; ALTER RESOURCE GROUP golden_rg RU_PER_SEC = 200;`
- **独立oracle**：2条均kind=ddl、coverage complete（必要前提具备），分别保留create/alter对象与RU变化；合法定义至少具有其具体政策检查证据而非unknown/pass；DB读取RU元数据。
- **必须同时通过的反例**：MySQL模式不能接受同一RU语法为本地支持；DROP/safe参数/非法参数独立oracle。
- **本轮非目标**：不把资源控制性能建模加入范围、不重用MySQLCPU字段伪装RU事实。
- **证明命令**：`make ddl-golden TASK=T22 ARTIFACT_DIR=/tmp/ddl-golden`

### T23 first Golden Path

- **允许修改的责任范围**：TiDB ID/layout/cache/replica相关事实与规则/state。具体改动仍须能追溯到本路径或其回归；不是目录内任意重构授权。
- **输入前提 / fixture profile**：TiDB8.5.0真实表，默认auto_random策略明确；第一条种子隔离只开启适用PK检查。
- **真实输入**：`CREATE TABLE t (id BIGINT PRIMARY KEY AUTO_RANDOM)`
- **独立oracle**：AUTO_RANDOM和clustered身份保真；不能仅因未写AUTO_INCREMENT而误判数据库能力不合法。团队明确要求AUTO_INCREMENT时仍可有具名政策finding，并说明它是政策不是解析不支持。DB metadata验证真实定义。
- **必须同时通过的反例**：普通AUTO_INCREMENT表仍按旧政策检查；CACHE/TiFlash等次路径需真实前提证据不能skip算完成。
- **本轮非目标**：不一口气改MySQLPK默认规范、不把缺TiFlash服务当不适用、不为一个形状重写整个metadata provider。
- **证明命令**：`make ddl-golden TASK=T23 ARTIFACT_DIR=/tmp/ddl-golden`

### T24 first Golden Path

- **允许修改的责任范围**：MySQL CPU资源组版本/属性规范化与风险检查。具体改动仍须能追溯到本路径或其回归；不是目录内任意重构授权。
- **输入前提 / fixture profile**：MySQL8.0/8.4主机能力经fixture预检可用；选择一个实际可用VCPU，fixture记录实际取值。
- **真实输入**：`CREATE RESOURCE GROUP golden_rg TYPE = USER VCPU = 0 THREAD_PRIORITY = 0`
- **独立oracle**：适用环境输出具体CPU资源组语义、object identity与检查；DB metadata吻合；若VCPU0不可用需记录选择后替换fixture的值及原因，不改oracle为随便成功。
- **必须同时通过的反例**：5.7明确厂商版本边界，TiDB RU形状不同；无法获得CPU前提时gap/外部blocker而非假complete。
- **本轮非目标**：不提升宿主权限、不修改实际生产线程调度、不把版本失败改成无条件notice。
- **证明命令**：`make ddl-golden TASK=T24 ARTIFACT_DIR=/tmp/ddl-golden`

### T25 first Golden Path

- **允许修改的责任范围**：InnoDB tablespace语义、文件敏感字段及engine边界。具体改动仍须能追溯到本路径或其回归；不是目录内任意重构授权。
- **输入前提 / fixture profile**：仅专用容器里新建可丢弃InnoDB表t；版本/engine已知，默认discard风险为blocker。
- **真实输入**：`ALTER TABLE t DISCARD TABLESPACE`
- **独立oracle**：保留目标和discard动作，specific blocker/reject/exit1，不静默、不依赖SQL执行来判风险；DB效果验证只针对专用fixture，之后容器销毁。
- **必须同时通过的反例**：IMPORT与普通ALTER不是同一个删除finding；NDB/logfile动作返回明确engine边界，不绕入InnoDB。
- **本轮非目标**：不访问SQL中的任意本地路径、不泄漏文件路径、不修改宿主文件、不承诺完整NDB。
- **证明命令**：`make ddl-golden TASK=T25 ARTIFACT_DIR=/tmp/ddl-golden`

### T26 first Golden Path

- **允许修改的责任范围**：官方低频对象与版本/engine applicability及安全projection。具体改动仍须能追溯到本路径或其回归；不是目录内任意重构授权。
- **输入前提 / fixture profile**：第一条选择明确其他engine关联的SERVER语法；无真实远端连接，不做网络连接。
- **真实输入**：`CREATE SERVER golden_srv FOREIGN DATA WRAPPER mysql OPTIONS (HOST '192.0.2.1', DATABASE 'app', USER 'u', PASSWORD 'golden-secret')`
- **独立oracle**：在已批准非InnoDB运行语义边界内明确incomplete/unsupported且不静默pass；diagnostics/findings不得出现golden-secret、连接options原文；无任何出站访问。
- **必须同时通过的反例**：范围内SRS合法定义另有complete语义验证；旧版本不适用列入清单；不能以此SERVER边界代表所有低频对象完成。
- **本轮非目标**：不添加FEDERATED执行能力、不安装任意插件、不把提交的URL/主机作为实际连接目标。
- **证明命令**：`make ddl-golden TASK=T26 ARTIFACT_DIR=/tmp/ddl-golden`

### T27 first Golden Path

- **允许修改的责任范围**：存储定义parser/body边界、静态作用域、共享审计与source map。具体改动仍须能追溯到本路径或其回归；不是目录内任意重构授权。
- **输入前提 / fixture profile**：MySQL8.4.10，隔离只开dml.where.require(required=true,blocker)，t存在；routine生命周期禁用无关规则。
- **真实输入**：`CREATE PROCEDURE p() BEGIN DELETE FROM t; END;`
- **独立oracle**：1个顶层definition statement；body DELETE恰好1 dml.where.require blocker、reject/exit1；原始body行列可定位、无新增raw body副本；不能把BEGIN/END片段当独立顶层SQL。
- **必须同时通过的反例**：CREATE PROCEDURE p() BEGIN DELETE FROM t WHERE id=1; END; -> 无该finding；含字符串分号/注释和常见DELIMITER包装不误切；3MySQL实际创建验证。
- **本轮非目标**：不执行routine、不预测运行分支、不把定义body效果写入外层schema state；动态body留给明确boundary。
- **证明命令**：`make ddl-golden TASK=T27 ARTIFACT_DIR=/tmp/ddl-golden`

### T28 first Golden Path

- **允许修改的责任范围**：body AST控制流/local绑定、资源限制与不完整证据。具体改动仍须能追溯到本路径或其回归；不是目录内任意重构授权。
- **输入前提 / fixture profile**：MySQL目标版本明确，仅启用同一DML WHERE政策；动态内容包含测试秘密标记。
- **真实输入**：`CREATE PROCEDURE p(IN s TEXT) BEGIN SET @q=s; PREPARE stmt FROM @q; EXECUTE stmt; END;`
- **独立oracle**：保留1个定义，body dynamic effect明确incomplete，至少review、SDK错误、CLI fail-on none仍1；不能执行或从字符串猜finding；原有可确定static sibling风险仍保留。
- **必须同时通过的反例**：纯静态安全body complete；静态危险body reject；未知分支与循环不伪造执行次数；深度上限触发显式incomplete非panic/timeout。
- **本轮非目标**：不实现SQL解释器、不加proof-engine插件体系、不放宽no-leak、不用timeout跳过作为通过。
- **证明命令**：`make ddl-golden TASK=T28 ARTIFACT_DIR=/tmp/ddl-golden`

### T29 first Golden Path

- **允许修改的责任范围**：trigger事件上下文、OLD/NEW绑定和body共享分析。具体改动仍须能追溯到本路径或其回归；不是目录内任意重构授权。
- **输入前提 / fixture profile**：专用MySQL t与audit_log；只启用dml.where.require；触发器只在DB fixture受控创建/触发。
- **真实输入**：`CREATE TRIGGER tr BEFORE INSERT ON t FOR EACH ROW DELETE FROM audit_log`
- **独立oracle**：1个顶层trigger，body有1 dml.where.require blocker/reject；trigger target=t、body target=audit_log区分，源位置正确。
- **必须同时通过的反例**：使用OLD/NEW的合法安全body不能把OLD/NEW当数据库表缺失；非法上下文与TiDB厂商边界显式报告。
- **本轮非目标**：不在产品中触发INSERT、不复用outer目标覆盖body目标、不假设创建trigger等于执行body。
- **证明命令**：`make ddl-golden TASK=T29 ARTIFACT_DIR=/tmp/ddl-golden`

### T30 first Golden Path

- **允许修改的责任范围**：event声明/调度状态与body共享分析，隔离event fixture。具体改动仍须能追溯到本路径或其回归；不是目录内任意重构授权。
- **输入前提 / fixture profile**：专用MySQL环境；首条fixture事件使用DISABLE，防止未经控制执行；profile只开dml.where.require。
- **真实输入**：`CREATE EVENT golden_ev ON SCHEDULE EVERY 1 DAY DISABLE DO DELETE FROM t`
- **独立oracle**：1个顶层event、保留DISABLE及schedule；body静态DELETE仍产生1 where blocker/reject，即使目前禁用也不能跳过body审计。DB创建后验证event disabled。
- **必须同时通过的反例**：WHERE安全body没有该blocker；ENABLE风险独立分析；scheduler不可用/权限错不能计syntax负例；TiDB边界明确。
- **本轮非目标**：不启动宿主scheduler、不用真实日级等待、不执行提交body、不把所有事件都按相同notice处理。
- **证明命令**：`make ddl-golden TASK=T30 ARTIFACT_DIR=/tmp/ddl-golden`

### T31 first Golden Path

- **允许修改的责任范围**：验收证据/库存/CI文档闭环，仅修必需gate问题。具体改动仍须能追溯到本路径或其回归；不是目录内任意重构授权。
- **输入前提 / fixture profile**：所有native blockers的代码和验证证据在同一checkout可用；不是只看issue已closed。
- **真实输入**：`运行T01到T30固定golden记录与全部manifest版本行；保留每个case/profile/check的执行记录。`
- **独立oracle**：executed_cases>0且精确匹配manifest期望集合；missing/skip/in-scope unsupported=0；4个anchor真实版本/digest匹配，全部assertion成功；PG与现有必需gate通过。
- **必须同时通过的反例**：删除一个fixture、强制一个DB不可达、返回0case、把不支持路径改pass，都应使验证器非零；不能修改分母变绿。
- **本轮非目标**：不做新feature、无关重构、push/tag/release；清单遗漏必须补owner任务并重新阻塞关闭，不能在最后一单偷偷做一批新功能。
- **证明命令**：`make ddl-golden TASK=all ARTIFACT_DIR=/tmp/ddl-golden`

## T01 self-contained proof

```sh
set -eu
proof_dir="$(mktemp -d)"
export DELTASCOPE_BIN="$proof_dir/deltascope"
go build -o "$DELTASCOPE_BIN" ./cmd/deltascope
python3 - <<'PY_GOLDEN'
import json, os, pathlib, subprocess, sys, tempfile
binary = os.environ["DELTASCOPE_BIN"]
failures = []
with tempfile.TemporaryDirectory(prefix="ddl-t01-") as tmp:
    cfg = pathlib.Path(tmp) / "policy.yaml"
    catalog = json.loads(subprocess.check_output([binary, "rules", "list", "--format", "json"], text=True))
    ids = sorted({r["rule_id"] for r in catalog["rules"]})
    assert "ddl.table.denylist.forbid" in ids, "required rule must exist"
    policy = "rules:\n" + "".join("  " + json.dumps(rid) + ":\n    enabled: false\n" for rid in ids if rid != "ddl.table.denylist.forbid")
    policy += "  ddl.table.denylist.forbid:\n    enabled: true\n    level: blocker\n    params:\n      tables: [sensitive]\n"
    cfg.write_text(policy)
    cases = [
        ("single", "DROP TABLE sensitive", True),
        ("second", "DROP TABLE harmless, sensitive", True),
        ("first", "DROP TABLE sensitive, harmless", True),
        ("middle", "DROP TABLE harmless, sensitive, other", True),
        ("all_allowed", "DROP TABLE harmless, other", False),
        ("rename_source", "RENAME TABLE harmless TO harmless_old, sensitive TO sensitive_old", True),
        ("rename_destination", "RENAME TABLE harmless TO harmless_old, other TO sensitive", True),
        ("rename_allowed", "RENAME TABLE harmless TO harmless_old, other TO other_old", False),
    ]
    for dialect in ("mysql", "tidb"):
        for name, sql, blocked in cases:
            run = subprocess.run([binary, "audit", "--dialect", dialect, "--sql", sql,
                "--config", str(cfg), "--format", "json", "--fail-on", "blocker"],
                text=True, capture_output=True, timeout=20)
            try:
                result = json.loads(run.stdout)
                statements = result.get("statements", [])
                findings = [f for st in statements for f in st.get("findings", [])]
                deny = [f for f in findings if f.get("rule_id") == "ddl.table.denylist.forbid"]
                assert len(statements) == 1, "must retain exactly one top-level statement"
                assert not result.get("diagnostics") and not result.get("unsupported"), "supported SQL cannot become unaudited"
                assert len(deny) == (1 if blocked else 0), "one denylist finding for this distinct protected object"
                if blocked:
                    assert result.get("verdict") == "reject" and run.returncode == 1, "blocker must reject and exit 1"
                    assert deny[0].get("level") == "blocker", "must not downgrade severity"
                    assert deny[0].get("metadata", {}).get("table") == "sensitive", "must identify protected target"
                    assert deny[0].get("location", {}).get("line") == 1, "must preserve original source line"
                else:
                    assert result.get("verdict") == "pass" and run.returncode == 0, "allowed control must pass without broad rejection"
                print("PASS", dialect, name)
            except (AssertionError, ValueError) as exc:
                failures.append((dialect, name, str(exc), run.returncode, run.stdout[:500]))
                print("FAIL", dialect, name, str(exc))
for item in failures:
    print("EVIDENCE", repr(item))
print("cases=16 failures=" + str(len(failures)))
sys.exit(1 if failures else 0)
PY_GOLDEN
```

Measured baseline: 16 cases, 8 failures, exit 1. Expected after implementation: 16 cases, 0 failures, exit 0. No application fix was made while preparing this contract.
