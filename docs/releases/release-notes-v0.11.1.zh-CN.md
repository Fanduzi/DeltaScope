# DeltaScope v0.11.1 发布说明

## 概览

DeltaScope `v0.11.1` 聚焦首次使用体验。这个补丁版本让安装选择更清晰、CLI 的终端引导更友好，并让首页与参考文档和真实的安装 / 规则列表体验保持一致。

## 更新内容

### 安装体验：macOS 以 Homebrew 为优先

顶层安装说明现在明确区分平台：

- **macOS：** 使用 Homebrew 安装
- **Linux / 其他环境：** 使用便携安装脚本 `install.sh`

这一说明现在已在 README、landing 页首屏和发布文档中保持一致。

### 便携安装脚本：默认值更安全、行为更可预期

`install.sh` 现在为交互式安装提供了更好的控制：

- 默认只安装 `deltascope`
- 为交互式用户提示选择要安装的二进制
- 在拷贝二进制之前提示安装目录
- 在下载发布产物前打印安装摘要
- 在调用 `sudo` 前先明确提示
- 当用户已经是 `root` 时完全跳过 `sudo`
- 保留对旧版本发布包的兼容性，避免在不存在 `deltascope-mcp` 的版本上安装失败

### CLI：更友好的首次终端体验

有两个直接在终端使用的 CLI 场景现在更清晰了：

- `deltascope audit` 在从交互式终端读取粘贴 SQL 之前，会先打印：

```text
Waiting for SQL from stdin. Press Ctrl+D to finish.
```

- `deltascope rules list` 和 `deltascope rules search` 现在会以对齐的 ASCII 表格渲染规则，而不是 Markdown 项目列表。

示例：

```text
# DeltaScope Rules

RULE ID                     LEVEL    KIND  SUMMARY
--------------------------  -------  ----  ----------------------------------------
ddl.table.comment.require  warning  ddl   Require DDL table comment require
dml.where.require          blocker  dml   Require DML where require
```

### 文档对齐

中英文参考文档现在已经和当前 CLI 的规则发现输出契约保持一致。

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
curl -fsSL https://raw.githubusercontent.com/Fanduzi/DeltaScope/v0.11.1/install.sh | \
  DELTASCOPE_VERSION=v0.11.1 sh
```

## 兼容性

没有破坏性变更。这个版本只改进安装与 CLI 可用性，不改变核心审计契约、内置规则 ID 或 API surface。
