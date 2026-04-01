# DeltaScope v0.12.0 发布说明

## 概览

DeltaScope `v0.12.0` 增加了面向 schema object 的结构化 naming governance。这个版本保持现有的 offline-first 审计路径和稳定的 rule contract，同时让团队可以通过 policy configuration 为表、列、索引和显式命名约束声明命名约定，而不是依赖临时的人肉检查。

## 更新内容

### 结构化 Naming Governance

现在你可以用内置规则为 `CREATE TABLE` 相关 schema object 配置命名要求，支持：

- `prefix`
- `suffix`
- `contains`

这套治理能力覆盖：

- 表名
- 列名
- 索引名
- 显式命名的约束

这个版本保留了已有的 identifier 合法性检查，因此 naming governance 是对 pattern validation 的补充，而不是替代。

### 面向 Policy 的约束覆盖

约束命名治理继续遵循 DeltaScope 一贯的 policy model：

- foreign key naming 检查只会在 policy 允许 foreign key 时生效
- 默认内置的 `ddl.table.foreign_key.forbid` 基线仍会抑制 foreign key naming governance，除非团队显式启用

### 文档、示例与覆盖补齐

现在 release-facing 材料已经把 naming governance 作为一等工作流能力展示：

- config 示例展示了如何要求 naming prefix、suffix 和 contains 规则
- landing 和 README 入口现在统一指向 `v0.12.0`
- application 层和 CLI 层都补上了 config 驱动的 naming governance 端到端覆盖

## 安装 / 升级

**macOS（推荐）：**

```bash
brew tap Fanduzi/deltascope
brew install --cask deltascope
```

或升级：

```bash
brew upgrade --cask deltascope
```

**Linux / 其他环境：**

```bash
curl -fsSL https://raw.githubusercontent.com/Fanduzi/DeltaScope/v0.12.0/install.sh | \
  DELTASCOPE_VERSION=v0.12.0 sh
```

## 兼容性

没有破坏性变更。`v0.12.0` 只是在现有审计契约之上增加了可配置的 naming governance，同时保持 CLI、HTTP、MCP 和 Go library 的稳定 surface。
