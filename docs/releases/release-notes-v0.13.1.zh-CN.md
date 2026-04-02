# DeltaScope v0.13.1 发布说明

## 概览

DeltaScope `v0.13.1` 是一个 landing page 补丁版本。它修复了 `v0.13.0` 引入的 JavaScript 语法错误，让首页再次能够正确渲染并切换本地化内容。

## 更新内容

### Landing Page JavaScript Hotfix

首页里的 DDL / CI 示例会把 SQL 字符串嵌入到内联 i18n 配置对象中。`v0.13.0` 里这些字符串包含了未转义的单引号，导致浏览器报错：

- `Uncaught SyntaxError: Unexpected string`

这个补丁版本对英文和中文配置块里的相关 SQL 示例都做了正确转义，因此首页脚本现在可以再次被正常解析。

### 没有产品契约变更

这个版本不会改变以下行为：

- CLI 审计行为
- HTTP metadata-aware 审计契约
- MCP launcher 行为
- 公共 Go API

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
curl -fsSL https://raw.githubusercontent.com/Fanduzi/DeltaScope/v0.13.1/install.sh | \
  DELTASCOPE_VERSION=v0.13.1 sh
```

## 兼容性

没有破坏性变更。`v0.13.1` 是建立在 `v0.13.0` 之上的文档与网站热修复版本。
