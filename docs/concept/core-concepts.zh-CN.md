# 核心概念

## 审计请求

审计请求（Audit Request）是提交给 DeltaScope 的工作单元。它包含：

- **SQL 文本**：待评估的一条或多条 SQL 语句，可以是字符串、文件或标准输入流
- **方言（Dialect）**：`mysql` 或 `tidb`；控制解析器行为及方言特定规则的激活
- **配置路径**（可选）：YAML 策略文件的路径；若省略，则使用 `policy.Default()`
- **元数据提供者（Metadata Provider）**（可选）：连接配置，用于激活实时实例和 Schema 信息的补充

三种产品入口均接受相同的请求结构：

| 入口 | 请求提交方式 |
|---|---|
| CLI（`deltascope audit`） | 命令行标志和参数 |
| HTTP 服务（`POST /v1/audit`） | JSON 请求体 |
| Go 库（`pkg/deltascope`） | 传递给 `deltascope.Audit()` 的 `deltascope.Request` 结构体 |

三种入口均运行相同的审计流水线，并产生结构上完全一致的发现结果。

---

## 发现与裁定

### 发现（Finding）

发现是单条规则评估的结果。每条发现包含：

- **规则 ID**：稳定的点分格式标识符，例如 `dml.where.require` 或 `ddl.column.comment.require`。规则 ID 在主版本内跨版本保持稳定。
- **级别（Level）**：`blocker`、`warning` 或 `notice` 之一（详见下文）。
- **消息（Message）**：对规则所检测内容的可读说明。
- **建议（Suggestion）**（可选）：解决该问题的推荐操作。
- **位置（Location）**（可选）：问题在原始 SQL 文本中的行号和列号。
- **元数据（Metadata）**（可选）：包含附加上下文的键值对（例如表名、列名、当前值）。

JSON 格式的发现示例：

```json
{
  "rule_id": "dml.where.require",
  "level": "blocker",
  "message": "UPDATE statement is missing a WHERE clause",
  "suggestion": "Add a WHERE clause to restrict the rows affected",
  "location": { "line": 1, "column": 1 }
}
```

### 裁定（Verdict）

裁定是整个审计请求的汇总结论。它由所有语句的全部发现综合计算得出：

| 裁定 | 条件 |
|---|---|
| `reject` | 存在至少一条级别为 `blocker` 的发现 |
| `review` | 无 blocker，但存在至少一条级别为 `warning` 的发现 |
| `pass` | 不存在 blocker 或 warning；只有 `notice` 的结果也仍然是 `pass` |

裁定反映的是整个请求的结果，而非单条语句的结果。任意语句中只要有一个 blocker，无论其他语句是否全部通过，最终裁定均为 `reject`。

### --fail-on 与退出码

`--fail-on` 标志控制哪种裁定级别会使 CLI 以退出码 1 退出。这允许你在不修改规则或策略的情况下，调整 CI 门禁的严格程度。

| `--fail-on` 值 | 退出码 1 的条件 | 退出码 0 的条件 |
|---|---|---|
| `blocker`（默认） | 存在任何 blocker 发现 | 无 blocker（允许 warning 和 notice） |
| `warning` | 存在任何 blocker 或 warning 发现 | 无 blocker 或 warning（允许 notice） |
| `notice` | 存在任何级别的发现 | 完全无发现 |
| `none` | 永不退出 1 | 始终退出 0 |

**CLI 退出码参考：**

| 退出码 | 含义 |
|---|---|
| `0` | 审计完成；发现未超过 `--fail-on` 阈值 |
| `1` | 审计完成；发现超过 `--fail-on` 阈值 |
| `2` | 用户输入错误：无效标志、SQL 格式错误、配置文件无法读取 |
| `3` | 运行时错误或内部故障 |

---

## 离线优先

离线优先（Offline-First）是默认运行模式。DeltaScope 在任何阶段均无需数据库连接，即可完成 SQL 的解析、规范化和评估。

这意味着：

- 开发者可以在无法访问目标数据库的笔记本电脑上运行 `deltascope audit`。
- CI 流水线无需连接预发或生产实例，即可审计 SQL 迁移脚本。
- AI 智能体无需将数据库凭证注入其执行上下文，即可调用该库。

离线路径并非功能降级模式。所有支持离线的规则均以完整精度运行。需要实时 Schema 信息的规则会正常注册，但在未附加元数据时会优雅地空操作（no-op）——它们永远不会阻塞离线审计，也不会产生虚假错误。

元数据感知模式（参见 [元数据感知模式](./metadata-aware-mode.zh-CN.md)）在相同的离线流水线之上叠加实时数据补充，而非取代离线路径。

---

## 策略与规则

**规则（Rules）** 是 DeltaScope 内置的检查项。它们随产品版本一同发布，在主版本内的补丁和次要版本之间保持稳定。规则按领域组织：

- `ddl.*` — CREATE TABLE 治理、ALTER TABLE 限制、对象生命周期管理
- `dml.*` — WHERE/LIMIT 要求、子查询防护、INSERT 限制

**策略（Policy）** 是控制规则在运行时行为的 YAML 配置文件：

- 通过规则 ID 启用或禁用单条规则
- 覆盖规则的默认严重级别
- 提供规则级别的参数（例如允许的最大列数、必需的注释格式）

策略变更无需修改任何代码即可改变评估行为。使用不同的策略文件，同一个二进制文件可为不同团队或环境执行不同的规范。

`policy.Default()` 是随产品附带的基准策略。查看方法：

```bash
deltascope config show-default
```

生成本地副本以便自定义：

```bash
deltascope config init
```

验证配置文件：

```bash
deltascope config lint --config ./deltascope.yaml
```

---

## 规则评估顺序

规则按**注册顺序确定性地执行**。这意味着：

- 在给定相同 SQL 和策略的情况下，发现结果始终以相同顺序产生。
- 添加或移除规则不会改变其他规则发现结果的顺序。

存在两种评估作用域：

**语句级规则（Statement-Scoped Rules）** 对每条语句独立应用。例如 `dml.where.require` 对每条 DML 语句各执行一次，仅产生该语句的发现结果。

**全局规则（Global Rules）** 执行一次，可访问批次中的所有语句。它们能够检测跨多条语句的模式。

全局规则示例：`ddl.alter.merge.mysql.require` — 该规则检查批次中的所有 ALTER TABLE 语句，当多个 ALTER TABLE 操作针对同一张表时发出警告。由于必须在看到所有语句后才能作出判断，该规则作为全局规则在所有语句级规则完成后运行。

---

## 规则 ID

规则 ID 遵循稳定的点分格式：

```
ddl.<area>.<check>
dml.<area>.<check>
```

示例：

| 规则 ID | 检查内容 |
|---|---|
| `dml.where.require` | DML 语句必须包含 WHERE 子句 |
| `ddl.column.comment.require` | 列必须有注释 |
| `ddl.table.comment.require` | 表必须有注释 |
| `ddl.alter.merge.mysql.require` | 针对同一张表的多个 ALTER TABLE 应合并执行 |
| `ddl.index.key_length.max_bytes.require` | 索引键长度不得超过实例限制 |

规则 ID 在主版本内保持稳定。重命名或删除规则属于主版本变更。

探索规则的方法：

```bash
deltascope rules list                      # list all rules
deltascope rules explain dml.where.require # detailed info for one rule
deltascope rules list --search "where"     # full-text search
```
