# DeltaScope v0.9.2 发布说明

## 概览

DeltaScope `v0.9.2` 是一个补丁版本，包含文档和 AI Agent Skill 改进。无二进制文件变更。

## 更新内容

### AI Agent Skill：多平台安装引导

当检测到本地未安装 `deltascope` 时，Skill 现在会根据操作系统给出对应的安装命令，而不是仅提供通用链接：

- **macOS** — Homebrew（推荐）：`brew tap Fanduzi/deltascope && brew install --cask deltascope`
- **Linux** — curl 安装到 `~/.local/bin`，无需 sudo
- **Windows** — PowerShell 一键脚本，自动从 GitHub Releases 下载最新版本

所有命令均展示给用户确认后再执行，不会静默安装。

### AI Agent Skill：通过 `npx skills update` 保持更新

`skills/README.md`、`README.md` 和 `README_ZH.md` 中新增了 `npx skills update` 命令说明，方便用户在安装后随时拉取最新版本的 Skill。

### 文档：AI Agent Skill 章节改进

- 恢复 Quick Start 示例和 Release Contract 章节
- 将「Skill」章节重命名为「AI Agent Skill」，表述更清晰
- 更新 `skills/README.md`，明确支持 Claude Code、Codex、Cursor 及 40+ AI 工具

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

**Linux：**

```bash
curl -fsSL https://raw.githubusercontent.com/Fanduzi/DeltaScope/v0.9.2/install.sh | \
  DELTASCOPE_VERSION=v0.9.2 sh
```

**MCP launcher（无需安装）：**

```bash
npx -y @fanduzi/deltascope-mcp --version
```

## 兼容性

无行为变更。[v0.9.1](release-notes-v0.9.1.zh-CN.md) 的所有兼容性说明均适用。
