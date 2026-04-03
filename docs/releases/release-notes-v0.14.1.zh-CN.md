# DeltaScope v0.14.1 发布说明

发布日期：2026-04-04

## 概览

DeltaScope `v0.14.1` 是一个面向接口对等和发布面收敛的兼容性补丁版本。它增强了已交付的 HTTP 和 CLI 接入面的可发现性与自动化可用性，但不改变核心审计引擎契约。

## 更新内容

### HTTP Discovery 端点

HTTP adapter 现在除了执行审计，还提供只读 discovery 端点：

- `GET /v1/rules`
- `GET /v1/rules/{rule_id}`
- `GET /v1/capabilities`

这让 HTTP 接入面在发现能力上更接近已有的 CLI 与 MCP 流程。

### 更安全的 CLI Secret 输入

CLI 现在支持：

- `--password-env`
- `--password-file`

这让脚本和本地自动化可以在不把密码直接放进进程参数的前提下传递数据库口令。

### 发布与支持元数据同步

这个补丁还同步更新了版本化安装示例、package metadata、landing page 上的 latest-release 引用，以及 `SECURITY.md` 中的支持版本声明，确保正式发布路径保持一致。

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
curl -fsSL https://raw.githubusercontent.com/Fanduzi/DeltaScope/v0.14.1/install.sh | \
  DELTASCOPE_VERSION=v0.14.1 sh
```

## 兼容性

没有破坏性变更。`v0.14.1` 是建立在 `v0.14.0` 之上的兼容性补丁版本。
