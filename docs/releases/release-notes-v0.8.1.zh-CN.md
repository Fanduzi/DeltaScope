# DeltaScope v0.8.1 发布说明

## 概览

DeltaScope `v0.8.1` 是一个面向 MCP launcher 发布链的元数据修复版本。它不改变 `v0.8.0` 的功能，而是修正 npm 包在 CI 里执行 provenance 校验时需要的 repository 元数据。

## 亮点

- 修复 `@fanduzi/deltascope-mcp` 的 npm provenance 校验
- DeltaScope 引擎和 MCP server 的运行时能力、合同均无变化
- 面向发布的安装链接已切换到 `v0.8.1`

## 修复内容

### npm launcher 包元数据

- `packages/deltascope-mcp/package.json` 现在声明了规范的仓库地址 `https://github.com/Fanduzi/DeltaScope`
- 这样可以让 package metadata 与 `npm publish --provenance` 生成的 GitHub Actions provenance bundle 保持一致
- CI 发包不再因为 repository URL 校验失败而被 npm 拒绝

## 安装 / 升级

推荐给 MCP 客户端的 launcher：

```bash
npx -y @fanduzi/deltascope-mcp --version
```

原生二进制安装：

```bash
curl -fsSL https://raw.githubusercontent.com/Fanduzi/DeltaScope/v0.8.1/install.sh | \
  DELTASCOPE_VERSION=v0.8.1 sh
```

## 兼容性

- Launcher 运行时：Node.js `24+`
- 支持的原生目标：`darwin`、`linux`
- 支持的架构：`amd64`、`arm64`
- 支持的数据库方言：`MySQL`、`TiDB`
