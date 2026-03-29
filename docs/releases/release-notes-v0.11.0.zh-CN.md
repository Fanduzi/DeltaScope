# DeltaScope v0.11.0 发布说明

## 概览

DeltaScope `v0.11.0` 带来两项重要能力：GitHub Actions Composite Action（让 SQL 审计在几分钟内接入 CI/CD 流水线），以及 HTTP 服务生产化加固（结构化日志、健康/就绪检查端点、优雅关闭）。

## 更新内容

### GitHub Actions Composite Action

DeltaScope 现在原生支持 GitHub Actions。在任何 workflow 中通过一行 `uses:` 引用即可接入 SQL 审计：

```yaml
- uses: Fanduzi/DeltaScope@v0.11.0
  with:
    files: migrations/*.sql
    fail-on: blocker
    token: ${{ secrets.GITHUB_TOKEN }}
```

Action 功能：
- 根据 runner 架构（`linux/amd64` 或 `linux/arm64`）从 GitHub Releases 下载对应二进制
- 展开 `files` glob，逐文件执行 `deltascope audit`
- 将 `has-issues: true/false` 和 `result`（JSON 数组）写入 step outputs
- 提供 `token` 时自动向当前 PR 发布审计摘要评论
- 若发现达到 `fail-on` 阈值的问题，以非零退出码使 CI 失败

完整 workflow 示例见 `docs/examples/github-actions.yml`，GitLab CI 模板见 `docs/examples/gitlab-ci.yml`。

### HTTP 服务：结构化 JSON 请求日志

每个 HTTP 请求现在会输出一行结构化 JSON 日志：

```json
{"timestamp":"2026-03-30T10:00:00Z","level":"info","msg":"http request","method":"POST","path":"/v1/audit","status":200,"duration_ms":12,"request_id":"a1b2c3d4"}
```

`timestamp` 记录请求到达时刻，确保延迟数据准确。

### HTTP 服务：健康与就绪检查端点

新增两个端点支持容器编排：

| 端点 | 用途 | 响应 |
|------|------|------|
| `GET /healthz` | 存活探针 — 进程是否存活 | `{"status":"ok"}` |
| `GET /readyz` | 就绪探针 — 引擎是否准备好服务 | `{"status":"ready"}` |

两个端点均绕过 API Key 认证和限流。

### HTTP 服务：优雅关闭

服务现在可以干净地处理 `SIGTERM` 和 `SIGINT`：

- 等待进行中的请求完成后再退出
- 关闭超时 15 秒（可通过 `--shutdown-timeout` 配置）
- 兼容 Kubernetes `terminationGracePeriodSeconds`

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
curl -fsSL https://raw.githubusercontent.com/Fanduzi/DeltaScope/v0.11.0/install.sh | \
  DELTASCOPE_VERSION=v0.11.0 sh
```

**MCP launcher（无需安装）：**

```bash
npx -y @fanduzi/deltascope-mcp --version
```

## 兼容性

无破坏性变更。v0.10.0 的所有 CLI flag、HTTP API 契约、MCP 工具和 Policy 配置文件均保持不变。
