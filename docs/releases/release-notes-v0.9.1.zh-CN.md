# DeltaScope v0.9.1 发布说明

## 概览

DeltaScope `v0.9.1` 是一个补丁版本，修复了 v0.9.0 中损坏的 CI 发布流水线，确保 Homebrew Cask 和 GitHub Release 构建产物正常发布。

## 修复内容

### CI 发布流水线（`npm publish --dry-run` 误报失败）

发布工作流的 `Verify MCP launcher package contract` 步骤调用了 `npm publish --dry-run`，当该版本已发布到 npm registry 时，即使在 dry-run 模式下也会报错。这导致 v0.9.0 打 tag 后 CI 直接失败，GoReleaser 未能执行，Homebrew Cask 也未能更新。

修复方式：删除多余的 `npm publish --dry-run` 调用。包内容已通过 `npm pack --dry-run` 完成验证。

## 安装 / 升级

**macOS（推荐）：**

```bash
brew tap Fanduzi/deltascope
brew install --cask deltascope
```

**Linux / 手动安装：**

```bash
curl -fsSL https://raw.githubusercontent.com/Fanduzi/DeltaScope/v0.9.1/install.sh | \
  DELTASCOPE_VERSION=v0.9.1 sh
```

**MCP launcher（无需安装）：**

```bash
npx -y @fanduzi/deltascope-mcp --version
```

## 兼容性

无行为变更。[v0.9.0](release-notes-v0.9.0.zh-CN.md) 的所有兼容性说明均适用。
