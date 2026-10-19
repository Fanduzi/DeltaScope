# T06 技术收口索引（issue #85）

T06技术收口材料齐备。本文件是 T06（#85，四个版本 CREATE TABLE 基础语义）的
最终导航索引：把每个验收维度映射到 [T06-base-traceability.md](T06-base-traceability.md)
的逐维度行、冻结的伞证据和已接受的切片决策记录。表述基于用户确认的技术接受——
不代表 "#85 已关闭"或任何外部评审结论；#79/#85/#86 均保持 open。

## 版本与证据锚定（三个 SHA 必须区分）

- **T06/A17 实现基线（收口时的生产代码）**：`4f58ac548aecd495edc707c35868c179686b0e35`。
  R1 终验确认当时的生产文件与该基线逐字节一致；这不禁止后续 owner 的已授权同层扩展。
- **实际测试代码**：`ee06c8e4d45f8d80ae1218b9de1041d9211f3037`（= 实现基线 +
  A17/R1 测试与证明脚本；45 条记录命令全部在该 SHA 上以 rc=0 完成）。
- **仅证据 HEAD**：`7a65fe162695ce1248d6edfb825b04e6b81b48b8`（只新增
  `docs/verification/` 证据树，无任何源码/测试变化）。

### 冻结数字（不重新推导）

- T06 Golden 冻结值：**496** 例（8 baseline database + 252 CLI + 236 anchored
  metadata 结构证明）；T06 收口时原 validator 合同为 **728** 项。后续任务可追加
  独立验证项，但不能删减原合同，也不能把 T07 的新增分母混入 T06 的 496。
- A17 额外原生表证明：**4 次独立运行**（每锚点一次），每次 24 列 / 21 个目标
  faithful pair，**不属于 496**。

### 伞证据（最新、与最终 SHA 绑定）

[`docs/verification/t06-a17/ee06c8e4d45f8d80ae1218b9de1041d9211f3037/`](../../docs/verification/t06-a17/ee06c8e4d45f8d80ae1218b9de1041d9211f3037/)：

- [README.md](../../docs/verification/t06-a17/ee06c8e4d45f8d80ae1218b9de1041d9211f3037/README.md) — 渲染自 checked-summary 的证据说明；
- [golden/T06/artifact.json](../../docs/verification/t06-a17/ee06c8e4d45f8d80ae1218b9de1041d9211f3037/golden/T06/artifact.json)、
  [cases/](../../docs/verification/t06-a17/ee06c8e4d45f8d80ae1218b9de1041d9211f3037/golden/T06/cases/)、
  [policies/](../../docs/verification/t06-a17/ee06c8e4d45f8d80ae1218b9de1041d9211f3037/golden/T06/policies/) — 496 例原始工件；
- [commands/04-focused-a17](../../docs/verification/t06-a17/ee06c8e4d45f8d80ae1218b9de1041d9211f3037/commands/04-focused-a17/) — A17 聚焦测试原始记录（仅 A17，见 §2）；
- [commands/09-make-test](../../docs/verification/t06-a17/ee06c8e4d45f8d80ae1218b9de1041d9211f3037/commands/09-make-test/) — 全套件记录；
- [anchors/{mysql57,mysql80,mysql84,tidb85}/](../../docs/verification/t06-a17/ee06c8e4d45f8d80ae1218b9de1041d9211f3037/anchors/) — 4 次严格原生证明；
- [sensitivity/](../../docs/verification/t06-a17/ee06c8e4d45f8d80ae1218b9de1041d9211f3037/sensitivity/) — 断言敏感度（22 条子观测）。

### 历史证据（不重写）

[`docs/verification/t06-a17/4f58ac548aecd495edc707c35868c179686b0e35/`](../../docs/verification/t06-a17/4f58ac548aecd495edc707c35868c179686b0e35/)
的 Golden 是在 a0819096 dirty 工作树上跑的，不是最终 4f58ac5 的复跑——见该目录
[PROVENANCE-CORRECTION.md](../../docs/verification/t06-a17/4f58ac548aecd495edc707c35868c179686b0e35/PROVENANCE-CORRECTION.md)；
原始记录保留原样，绝不重写。

## 1. 源事实（source facts）

每维度对应 [T06-base-traceability.md](T06-base-traceability.md) 同名行 + 切片 ADR +
已接受切片证据 README（目录身份引用，见下方说明）：

| 维度 | 已接受事实 | 追踪/证据 |
|---|---|---|
| A1 | PRIMARY KEY 存在性（inline + table-level） | traceability「Explicit PRIMARY KEY presence」行；`TestT06A1*`；[ADR](../../docs/decisions/2026-10-06-ddl-create-table-pk-golden-proof.md)；[切片证据](../../docs/verification/t06-a1/4e51cbe0d17b035ac950f16176b8cea7f2dacaa4/README.md) |
| A2 | PK 成员隐含 NOT NULL / 显式 NULL 冲突 | traceability A2 行；`TestT06A2*`；[ADR](../../docs/decisions/2026-10-06-ddl-create-primary-key-nullability.md)；[切片证据](../../docs/verification/t06-a2/9d12aec81ecdca0be94252ae6b568127817cdf5d/README.md) |
| A3/A4 | 声明层与目录层 default 身份（`DEFAULT NULL` vs 存储 `'NULL'` 字面量） | traceability A3/A4 行；`TestT06A3*`/`TestT06A4*`；[ADR-A3](../../docs/decisions/2026-10-06-ddl-default-null-literal-fidelity.md)、[ADR-A4](../../docs/decisions/2026-10-07-ddl-provider-default-null-fidelity.md)；[A3 证据](../../docs/verification/t06-a3/d3ddf9c13e5c216a455c86d21c30293001123bb1/README.md)、[A4 证据](../../docs/verification/t06-a4/527fd48dde8190b3c3037ef2335c97ec7ad2853e/README.md) |
| A5 | 声明 CHAR/VARCHAR 长度 | traceability A5 行；`TestT06A5*`；[ADR](../../docs/decisions/2026-10-07-ddl-character-length-policy-proof.md)；[切片证据](../../docs/verification/t06-a5/a43a8a8121d803e049f6663acb8d1024fb66919d/README.md) |
| A7 | charset/collation 声明事实（声明与目录解析/继承层分离） | traceability A7 行；`TestT06A7*`；[ADR](../../docs/decisions/2026-10-07-ddl-create-table-collation-policy.md)；[切片证据](../../docs/verification/t06-a7/2c8003111c869f8b7b8d4749384d294e2b9b8787/README.md) |
| A8 | comment 字节/码点事实 | traceability A8 行；`TestT06A8*`；[ADR](../../docs/decisions/2026-10-07-ddl-comment-value-and-length-fidelity.md)；[切片证据](../../docs/verification/t06-a8/767a19c995234e44aea28e1648e77e7118b50ec6/README.md) |
| A9 | 类型化 current-time 角色（created/updated） | traceability A9 行；`TestT06A9*`；[ADR](../../docs/decisions/2026-10-08-ddl-explicit-audit-time-column-proof.md)；[切片证据](../../docs/verification/t06-a9/b9af1238c9bf99eaced7b39834de52f24a4dc866/README.md) |
| A10 | AUTO_INCREMENT 声明/初始值选项 vs 观察到的分配器 | traceability A10 行；`TestT06A10*`；[ADR](../../docs/decisions/2026-10-10-ddl-auto-increment-declaration-proof.md)；[切片证据](../../docs/verification/t06-a10/bffcc261dc7958022ae3ae462d44e13bb83a55f4/README.md) |
| A11 | 单成员 PK BIGINT/UNSIGNED | traceability A11 行；`TestT06A11*`；[ADR](../../docs/decisions/2026-10-14-ddl-primary-key-bigint-unsigned-proof.md)；[切片证据](../../docs/verification/t06-a11/4dc51cbd424d2e1f13a9658622820c64685da340/README.md) |
| A12 | 普通列 nullability | traceability A12 行；`TestT06A12*`；[ADR](../../docs/decisions/2026-10-15-ddl-ordinary-column-nullability-proof.md)；[切片证据](../../docs/verification/t06-a12/f015fb474f8cc40a60dff4e5355925669bc14b70/README.md) |
| A13 | 类型族策略事实（五个 forbid 族） | traceability A13 行；`TestT06A13*`；[ADR](../../docs/decisions/2026-10-17-ddl-type-family-forbid-proof.md)；[切片证据](../../docs/verification/t06-a13/2eed4a98728e7bd0c89ef4db6b5149e5ec155252/README.md) |
| A14 | 声明表选项（ENGINE/CHARSET/ROW_FORMAT allowlist） | traceability A14 行；`TestT06A14*`；[ADR](../../docs/decisions/2026-10-17-ddl-table-option-allowlist-proof.md)；[切片证据](../../docs/verification/t06-a14/ded0ed51bcbf4e34952438e53f6b08073425b800/README.md) |
| A15/R1 | 字段感知粗略 row-size 估计 + 声明 `COMPRESSED` 身份 | traceability A15 行；`TestT06A15*`；[ADR](../../docs/decisions/2026-10-18-ddl-row-size-fact-resolution.md)；[切片证据](../../docs/verification/t06-a15/700e90e4fb2c9a789156f739932727f4e3cfb020/README.md) |
| A17/R1 | 五类声明类型参数/成员字面量保真（DECIMAL/FLOAT/DOUBLE/ENUM/SET） | traceability A17 行；`TestT06A17*`；[ADR](../../docs/decisions/2026-10-18-ddl-column-type-parameter-fidelity.md)；[切片证据（即伞证据）](../../docs/verification/t06-a17/ee06c8e4d45f8d80ae1218b9de1041d9211f3037/README.md)；伞证据 `anchors/` 四次原生证明 |

**切片证据目录说明**：上表每个 `t06-aXX/<sha>/` 是该切片被接受时冻结的证据
目录身份引用——目录内记录的是该切片自己当时运行的原始 SHA/构建身份，**不**
表示那些历史运行测的是本索引所列的目录 SHA。各切片的原始运行/构建身份以各自
README 记录为准；A17 的最终 tested/evidence SHA（`ee06c8e4`/`7a65fe16`）如上文
权威有效。

T06 manifest 见 [testdata/ddl-golden/T06.json](../../testdata/ddl-golden/T06.json)；
模块 README 中枚举的已接受切片测试入口
（[internal/application/audit/README.md](../../internal/application/audit/README.md)、
[internal/infrastructure/parser/tidb/README.md](../../internal/infrastructure/parser/tidb/README.md)、
[internal/infrastructure/metadata/mysql/README.md](../../internal/infrastructure/metadata/mysql/README.md) 等）
是各维度的直接代码侧链接。

## 2. 策略面（dangerous / safe / threshold / off）

- [T06.json](../../testdata/ddl-golden/T06.json) 的隔离 profile（`t06-a*-*-isolated`
  与 `all-rules-disabled`）固定了每条规则的 dangerous/safe/off 角色，并以逐语句
  归因钉死（exact rule_id/message/metadata/1:1 location）。
- **threshold 只适用于有数值边界的策略**：字符/码点长度（A5/A8）与 row-size
  估计（A15）有 boundary 角色（at/above/below limit 精确值）；存在性/布尔策略
  （PK、NOT NULL、注释存在、allowlist 等）使用各自适用的 dangerous/safe、
  `required=false`/`forbid=false`/`require_explicit=false` 与 disabled 控制，不虚构数值阈值。
- 有策略证据的维度：A1/A2/A5/A7–A15（见上表）。
- **A17 是保真-only**，不引入新策略；无精度/成员治理、无 scale-shrink 或
  ENUM-order 治理、无 `FLOAT(M,D)` 废弃声明。
- 引用：伞证据 [cases/](../../docs/verification/t06-a17/ee06c8e4d45f8d80ae1218b9de1041d9211f3037/golden/T06/cases/)
  与 [policies/](../../docs/verification/t06-a17/ee06c8e4d45f8d80ae1218b9de1041d9211f3037/golden/T06/policies/)。
- Go 接缝为各 `TestT06A*`；注意
  [commands/04-focused-a17](../../docs/verification/t06-a17/ee06c8e4d45f8d80ae1218b9de1041d9211f3037/commands/04-focused-a17/)
  只跑 A17 聚焦项，A1–A15 的 Go 回归在
  [commands/09-make-test](../../docs/verification/t06-a17/ee06c8e4d45f8d80ae1218b9de1041d9211f3037/commands/09-make-test/)
  全套件中覆盖，各切片各自的原始接受运行记录见上方切片证据目录。
- 496 为冻结分母，本文档不扩大计数。

## 3. 批次消费（batch state consumption）

- A1：PK 前置状态被 follower 消费（`applyCreateTable` →
  `TableSnapshot.PrimaryKey`/`HasPrimaryKey`）。
- A3/A4：`DEFAULT NULL`/存储字面量在 drop-column 接缝中的消费
  （`dropOtherColumnMayReference`）。
- A8：派生 shape 中 `Table.Comment` 携带。
- A11–A14、A17/R1：真实 follower target、完整列集、克隆隔离
  （`TestT06A17DerivedPreStatePreservesType` 等）。
- T05 的有界 unknown/invalidation 语义保持不变：本任务**不**声称任意 ALTER
  被模拟。链接：[testdata/ddl-golden/T05.json](../../testdata/ddl-golden/T05.json) +
  已接受 ADR（[rename 身份迁移](../../docs/decisions/2026-10-20-ddl-rename-table-identity-migration.md)、
  [change/rename column](../../docs/decisions/2026-10-21-ddl-change-rename-column-identity.md)
  及 T05 有序状态/删除系列）。
- 生成列/default-expression/可见性依赖属 T07（#86），不是 T06 缺陷。

## 4. 合法性 ≠ 策略

- 合法 fixture（无 PK、超长声明长度、越界 charset/collation、缺 comment、
  受限表选项）可同时产生 product blocker 与 native driver CREATE 成功——
  两者是独立事实，逐例记录于锚定 case。
- 已知原生 negative（如 utf8mb4+latin1_bin ERROR 1253、显式 PK NULL
  ERROR 1171）单列记录，不与策略结论混读。
- A15 粗略估计不是物理存储/原生接受的预测。
- 目录归一化表示无法恢复声明 provenance（`COLUMN_DEFAULT IS NULL` 无法区分
  省略 DEFAULT 与显式 DEFAULT NULL；`COLUMN_TYPE` 无法区分显式
  `DECIMAL(12,0)` 与省略 scale）——这是永久 catalog 边界，不是待补缺口。

## 5. 锚点与所有者边界

- 四个固定锚点：MySQL **5.7.44 / 8.0.46 / 8.4.10**、TiDB **8.5.0**。
- T06 接受范围为源/普通语义；以下维度各有所有者、保持移交：
  - 生成列 / 表达式默认值 / 不可见列 → **T07（#86）**
  - ADD/MODIFY 长度治理 → **T08（#87）**
  - 索引族 → T09/T10；约束 → T11/T12；分区 → T14；表选项枚举 → T15；
    生命周期（LIKE/SELECT/temporary）→ T16；TiDB 专项行 → 各自 T1x/T2x 所有者。
- 不承诺物理 DECIMAL 存储模型、运行时取值矩阵或 sql_mode 矩阵。

## 维护记录

- A16 的唯一源事实残留已由 A17 关闭：**缺少专用 Precision/Scale 字段不是
  残留**——`Column.Type` 是已接受的参数/成员表示。
- 已知的非阻塞 A15-R1 nil-label 问题：保持现状，相关测试未改动，列为遗留
  维护项。
- 历史 4f58ac5 证据的 provenance 差异见
  [PROVENANCE-CORRECTION.md](../../docs/verification/t06-a17/4f58ac548aecd495edc707c35868c179686b0e35/PROVENANCE-CORRECTION.md)；
  本索引不重写历史原始记录。
