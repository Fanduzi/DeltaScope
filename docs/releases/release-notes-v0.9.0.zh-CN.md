# DeltaScope v0.9.0 发布说明

## 概览

DeltaScope `v0.9.0` 新增 Homebrew Cask 分发渠道和 Claude Code Skill，让 DeltaScope 在 macOS 上更容易安装，并可在 AI 编码会话中直接审核 SQL。

## 亮点

- **Homebrew Cask** — 一条 `brew` 命令在 macOS 上安装 DeltaScope
- **Claude Code Skill** — 在 Claude Code、Codex、Cursor 等 40+ AI 工具中直接审核 SQL

## 新功能

### Homebrew Cask 分发

DeltaScope 现在可通过 Homebrew Cask 安装：

```bash
brew tap Fanduzi/deltascope
brew install --cask deltascope
```

这是 macOS 用户的推荐安装方式。每次发版时，GoReleaser 自动将 Cask 推送到 [Fanduzi/homebrew-deltascope](https://github.com/Fanduzi/homebrew-deltascope)。

### Claude Code Skill — `deltascope-review`

新的 Claude Code Skill 让你无需离开 AI 编码会话即可审核 SQL 片段或迁移文件：

```bash
# 通过 npx skills 安装（支持 Claude Code、Codex、Cursor 等 40+ AI 工具）
npx skills add Fanduzi/DeltaScope --skill deltascope-review -a claude-code
```

在 Claude Code 会话中调用：

```
/deltascope-review
```

粘贴 SQL 片段或指定文件路径——Claude 会将 SQL 写入临时文件（避免反引号和引号的 shell 转义问题），运行 `deltascope audit`，并返回结构化的违规报告和修复建议。

详见 [skills/README.md](../../skills/README.md)。

## 安装 / 升级

**macOS（推荐）：**

```bash
brew tap Fanduzi/deltascope
brew install --cask deltascope
```

**Linux / 手动安装：**

```bash
curl -fsSL https://raw.githubusercontent.com/Fanduzi/DeltaScope/v0.9.0/install.sh | \
  DELTASCOPE_VERSION=v0.9.0 sh
```

**MCP launcher（无需安装）：**

```bash
npx -y @fanduzi/deltascope-mcp --version
```

## 兼容性

- 支持的原生目标：`darwin`、`linux`
- 支持的架构：`amd64`、`arm64`
- 支持的数据库方言：`MySQL`、`TiDB`
- Claude Code Skill 需要本地安装 `deltascope` 二进制（brew 或手动安装）
