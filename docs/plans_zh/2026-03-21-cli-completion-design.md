# DeltaScope CLI 补完设计

## 目标

让 CLI 成为 DeltaScope 完整的一等产品界面：暴露现有全部核心审计能力，增加带有类 MySQL 连接体验的元数据感知访问方式，并提供足够自解释的命令，使用户与 AI agent 不再需要阅读源码或 README 才能理解如何使用工具。

## 成功标准

- CLI 可以驱动离线审计与元数据感知审计两种模式。
- 元数据感知审计不需要把连接细节藏在配置文件里。
- CLI 暴露足够的规则目录与配置工具，使用户可以：
  - 查看可用规则，
  - 查看单条规则详情，
  - 搜索规则，
  - 校验配置，
  - 打印默认配置，
  - 查看当前产品能力。
- 命令面、帮助文本、示例、退出语义与输出模式整体上应显得完整，而不是临时拼接。
- 完成该里程碑后，不再存在“底层已实现但 CLI 无法访问”的主要缺口。

## 命令面

完整 CLI 应暴露：

- `deltascope audit`
- `deltascope rules list`
- `deltascope rules show <rule-id>`
- `deltascope rules search <keyword>`
- `deltascope config init`
- `deltascope config lint`
- `deltascope config show-default`
- `deltascope capabilities`
- `deltascope version`
- `deltascope --version`

这样可以保持 `audit` 为主操作，使用 `rules` 与 `config` 作为自然命令组，同时让 `capabilities` 与 `version` 在顶层易于发现。

## 审计模式

### 离线模式

当未提供数据库连接参数时，`audit` 的行为与现在一致：

- 通过 `--sql`、`--file` 或 stdin 获取 SQL 输入
- 通过默认策略或 `--config` 获取策略
- 通过 `--dialect` 指定方言

### 元数据感知模式

当提供任意连接参数时，`audit` 应当为同一套审计引擎增加实时实例事实与目标表快照。

CLI 应让 MySQL 用户感到熟悉：

- `-h, --host`
- `-P, --port`
- `-u, --user`
- `-p, --password`
- `--ask-password`
- `-D, --schema`
- `-S, --socket`

规则：

- `--password` 与 `--ask-password` 互斥。
- `--socket` 与基于 host/port 的 TCP 选择冲突。
- 只要出现连接参数，DeltaScope 即进入元数据感知模式。
- 在元数据感知模式中，DeltaScope 自动从实例事实检测方言。
- 如果用户同时传入 `--dialect`，DeltaScope 需要校验它与检测出的方言一致，不一致则失败。

## Schema 解析

CLI 不应强制用户每次都传 `--schema`。

解析顺序：

1. 如果给定 `--schema`，直接使用。
2. 否则，尝试通过跨 schema 查找目标对象名来推断 schema。
3. 如果目标只解析到一个 schema，则使用它，并在输出中说明该决策。
4. 如果目标存在于多个 schema 中，则失败，并提示用户传入 `--schema`。
5. 如果目标在任何地方都不存在，则需要保持诚实：
   - `CREATE TABLE` 可以在部分元数据上下文中继续执行，因为对象本来就预期尚不存在。
   - 需要真实目标对象的语句必须失败，而不是假装元数据可用。

## 规则目录元数据

现有规则引擎偏执行导向。CLI 补完需要在其上增加一层偏解释导向的能力。

每条已发布规则都应具备稳定的目录元数据，例如：

- `rule_id`
- summary
- description
- applies-to statement kinds
- default enabled state
- default level
- default params
- metadata-aware flag
- trigger example
- valid example
- config example
- suggestion hint

这些元数据应驱动：

- `rules list`
- `rules show`
- `rules search`
- 后续文档与工具能力

规则执行与规则描述应保持为两个不同关注点，只通过 `rule_id` 关联。

## 工具命令

### `rules list`

以紧凑目录字段和基础过滤能力列出已发布规则。

推荐过滤项：

- `--level`
- `--kind`
- `--enabled-only`

### `rules show <rule-id>`

展示：

- 核心规则元数据
- 默认配置
- 是否需要元数据
- 触发示例
- 合法示例
- 配置示例
- 简短修复建议

### `rules search <keyword>`

搜索规则 ID 与 summary，避免用户必须扫描完整列表。

### `config lint`

校验：

- YAML 语法
- 未知规则键
- 非法参数类型
- 非法枚举值

### `config show-default`

直接向 stdout 打印内置默认配置。

### `capabilities`

输出以下内容的简洁摘要：

- 支持的方言
- 支持的输入形式
- 支持的输出格式
- 离线与元数据感知模式
- 支持的元数据事实
- CLI 与 HTTP 等产品界面

## UX 补完

该里程碑不只是增加新命令，也包括 CLI 质量补完。

必须的 UX 改进：

- 为每个命令提供更完善的 help 文本与示例
- 在 `audit --help` 中给出清晰的在线/离线使用示例
- 更好的连接与 schema 解析错误
- 支持无回显密码提示
- 在使用元数据感知模式、schema 推断或方言自动识别时给出明确提示
- quiet 与 JSON 输出应继续对 shell 与 agent 保持可预测性

## 范围外

- HTTP 服务增强
- MCP server
- 完整的 request-file 输入模型
- `config explain`
- 高级凭据存储或 secret manager

## 预期结果

完成该里程碑后，DeltaScope 应拥有一个完整、可发现、且诚实的 CLI。用户应能审计 SQL、在需要时连接实时 MySQL/TiDB 元数据、查看已发布规则目录、校验配置，并且无需深入仓库内部即可理解工具能力。