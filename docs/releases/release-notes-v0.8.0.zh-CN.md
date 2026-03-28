# DeltaScope v0.8.0 发布说明

## 概览

DeltaScope `v0.8.0` 把 MCP onboarding 提升为正式发布面。这次发布保留 `v0.7.0` 引入的官方 `deltascope-mcp` 服务，并进一步补齐可发布的 npm launcher、更清晰的 quick start 文档，以及纳入发布流水线的 npm 交付能力，让用户既可以通过 `npx` 直接接入 MCP 客户端，也可以继续使用原生二进制。

## 亮点

- 可正式发布的 npm launcher：`@fanduzi/deltascope-mcp`
- 面向 Claude Code、Codex 和通用 stdio 客户端的复制即用 quick start
- 独立的 DeltaScope MCP 使用文档，中英双语
- release workflow 现在会校验并发布 npm launcher
- launcher 增加 checksum 校验、cache metadata 与锁恢复

## 新增与改进

### 面向 MCP 客户端的 npm launcher

- DeltaScope 现在提供 `@fanduzi/deltascope-mcp` 作为面向 `npx` 用户的推荐 launcher 包
- launcher 会下载匹配版本的 DeltaScope release archive，先用官方 checksums 文件校验，再缓存原生 `deltascope-mcp` 并把 stdio 转发给它
- launcher 支持版本覆盖和 release base 覆盖，方便受控环境接入，同时仍会基于官方 DeltaScope checksums 做校验

### 更快的接入文档

- README 现在直接给出 Claude Code、Codex 与通用 stdio 的 MCP quick start 片段
- 新增独立 MCP 使用文档，覆盖 launcher 前提、代理配置、direct connection、`connection_ref` 和本地 YAML 配置
- 英文和中文文档都把原生二进制与 npm launcher 明确成两条正式支持的接入路径

### 发布流水线集成

- release workflow 现在会在 tag 发布前校验 launcher package contract
- tag 发布时会和 Go release assets 一起发布 npm launcher，并附带 provenance
- package 版本与 Git tag 现在在 release 时强制一致

### Launcher 加固

- launcher bootstrap 日志统一走 `stderr`，保证 MCP `stdout` 只承载协议流
- 首次下载的 archive 会在解压前用官方 DeltaScope checksums 校验
- cache metadata、锁超时和 stale lock 恢复降低了首次安装卡死或缓存损坏的风险

## 安装 / 升级

推荐给 MCP 客户端的 launcher：

```bash
npx -y @fanduzi/deltascope-mcp --version
```

原生二进制安装：

```bash
curl -fsSL https://raw.githubusercontent.com/Fanduzi/DeltaScope/v0.8.0/install.sh | \
  DELTASCOPE_VERSION=v0.8.0 sh
```

## 兼容性

- Launcher 运行时：Node.js `24+`
- 支持的原生目标：`darwin`、`linux`
- 支持的架构：`amd64`、`arm64`
- 支持的数据库方言：`MySQL`、`TiDB`

## 已知限制

- launcher 首次运行仍依赖 GitHub release 可达性、Node 24+ 和系统 `tar`
- 代理环境下仍可能需要显式设置 `NODE_USE_ENV_PROXY=1`
- MCP transport 仍然只支持 stdio
