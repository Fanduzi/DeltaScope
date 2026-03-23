# 产品架构

DeltaScope 通过三种产品入口暴露同一个审计引擎：`deltascope` CLI、`deltascope-server` HTTP 服务，以及 `pkg/deltascope` Go 库。三种入口共享相同的规则评估路径、相同的严重级别模型，以及相同的离线优先保证。

---

## 设计理念：一条路径，三种入口

CLI、HTTP 服务和库均是对同一套应用逻辑的轻量适配层。这带来了几个实际意义：

- 离线模式产生的发现与元数据感知模式产生的发现完全一致——相同的规则 ID、相同的级别、相同的消息格式。
- 发现结果在各入口间可复现：通过 CLI 和通过 Go 库审计同一个 SQL 文件，产生的结果相同。
- 不存在"仅在线"规则。每条使用元数据的规则在离线模式下注册也是安全的；当未附加快照时，它只是空操作。离线审计永远不会因注册了元数据依赖规则而失败。
- 策略配置的评估方式与入口无关。无论是通过 CLI 还是内嵌库使用，`deltascope.yaml` 文件控制的是同一套规则集。

---

## 共享审计流程

```text
SQL text / file / stdin / HTTP body
                |
                v
      +----------------------+
      | CLI / HTTP / Library |
      +----------------------+
                |
                v
      +----------------------+
      | Policy / Config Load |
      +----------------------+
                |
                v
      +----------------------+
      | Parser + Extractor   |
      | normalized statements|
      +----------------------+
                |
      +---------+----------+
      |                    |
      v                    v
+----------------+   +----------------------+
| Offline facts  |   | Optional metadata    |
| only           |   | enrichment           |
+----------------+   | instance + schema    |
      |              +----------------------+
      +---------+----------+
                |
                v
      +----------------------+
      | Rule Evaluation      |
      | blocker/warning/...  |
      +----------------------+
                |
                v
      +----------------------+
      | Verdict + Findings   |
      +----------------------+
                |
      +---------+---------+----------------+
      |                   |                |
      v                   v                v
  CLI markdown        CLI JSON        HTTP / library
  or exit code        for agents      structured result
```

---

## 审计流水线阶段

### 1. 解析（Parse）

SQL 文本被传递给 TiDB 解析器适配器（`internal/infrastructure/parser/tidb`），生成有类型的 AST（抽象语法树）。TiDB 解析器同时用于 MySQL 和 TiDB 两种方言，因为它是 MySQL 语法的严格超集。方言特定行为通过根据当前激活方言设置的解析器模式标志来控制。

### 2. 提取（Extract）

遍历 AST 并将其规范化为 `[]spec.Statement`——一种与解析器解耦的中立模型，使下游规则评估不依赖解析器内部实现。每个 `spec.Statement` 包含：

- 语句类型（DDL 或 DML）及子类型（CREATE TABLE、ALTER TABLE、DELETE 等）
- 提取的结构化细节（列、索引、子句、受影响的表）
- 原始 SQL 字符串

### 3. 补充（Enrich）（可选）

当提供了 `MetadataProvider`（即元数据感知模式已激活）时，补充阶段会将实时信息附加到每条语句：

- `spec.InstanceFacts` 附加到每条语句：MySQL/TiDB 版本、默认字符集、InnoDB 配置变量。
- `spec.TableSnapshot` 按目标表附加：从 `information_schema` 加载的当前列定义、索引定义、主键状态及表选项。

对于无法解析到快照的语句（例如尚不存在的表），将以可用的信息进行补充。规则能够优雅地处理缺失的快照。

### 4. 评估（Evaluate）

规则注册表（Rule Registry）以确定性顺序将已注册的规则应用于补充后的语句。存在两种评估作用域：

- **语句级规则（Statement-Scoped Rules）** 对每个 `spec.Statement` 独立应用。大多数规则属于语句级规则。
- **全局规则（Global Rules）** 执行一次，可访问批次中的所有语句。它们检测跨多条语句的模式，例如多个 ALTER TABLE 操作针对同一张表的情况。

注册表在运行全局规则之前，先对每条语句完成所有语句级规则的执行。

### 5. 报告（Report）

所有规则的发现结果汇总到 `report.Result` 中。结果包含：

- **裁定（Verdict）**：`pass`、`review` 或 `reject`（参见 [核心概念——发现与裁定](./core-concepts.zh-CN.md#发现与裁定)）
- **摘要（Summary）**：按级别统计的发现数量
- **按语句的发现（Per-Statement Findings）**：按语句索引分组的发现结果
- **全局发现（Global Findings）**：由全局规则产生的发现结果

`report.Result` 随后由激活的输出适配器渲染（CLI Markdown、CLI JSON 或 HTTP/库的结构化结果）。

---

## 层级边界

```
cmd/deltascope | cmd/deltascope-server        ← process entrypoints (flag binding only)
internal/interfaces/cli | http                ← transport adapters (request/response translation)
internal/application/audit | policy           ← orchestration: parse → extract → enrich → evaluate
internal/domain/spec | rule | policy | report ← core domain: normalized types and rule semantics
internal/infrastructure/parser | config | metadata | output  ← external adapters
pkg/deltascope                                ← stable public API facade
```

**核心约束**：领域包（`internal/domain/...`）不得导入基础设施包（`internal/infrastructure/...`）。依赖关系只能向内流动。基础设施将外部依赖适配到领域接口，但不定义领域行为。

**公共 API 边界**：`pkg/deltascope` 是稳定的公共 API 外观层。新的公共类型应属于 `pkg/deltascope`，而非 `internal/` 包。内部包可以自由演进而不破坏公共 API。

---

## 严重级别模型

三种产品入口均使用相同的三个严重级别：

| 级别 | 含义 |
|---|---|
| `blocker` | SQL 不得按现状执行。该问题是策略违规，或是具有较高数据丢失、服务中断或数据损坏风险的模式。 |
| `warning` | SQL 在执行前应经过审查。该问题属于策略关注点、高风险模式，或需要明确签字确认的情况。 |
| `notice` | 仅供参考。该问题值得知晓，但在执行 SQL 之前不需要采取行动。 |

发现的严重级别由规则的默认级别决定，可在策略配置中按规则覆盖。这意味着同一项检查，在严格的生产环境中可以是 `blocker`，在开发环境中可以是 `notice`，仅需修改配置，无需任何代码变更。
