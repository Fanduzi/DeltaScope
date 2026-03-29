# DeltaScope v0.10.0 发布说明

发布日期：2026-03-29

## 概览

DeltaScope `v0.10.0` 聚焦 HTTP 服务面加固，新增认证、中间件治理、限流能力和 Prometheus 监控指标。

## 新功能

### HTTP 适配层迁移到 Gin，并引入中间件链

- HTTP 适配层迁移为 Gin，实现仍保持 `/healthz`、`/version`、`/v1/audit` 的既有契约。
- 新增中间件链：request ID、panic 恢复、请求超时上下文、认证、限流、结构化访问日志。

### HTTP API Key 认证

- 新增可选 `X-API-Key` 鉴权。
- 新增明确错误语义：
  - 缺少 key 返回 `401 auth_required`
  - key 无效返回 `403 auth_invalid`

### 限流与可观测性

- 新增可选限流（按 `api-key` 或 `ip`）并返回 `429 rate_limited`。
- 新增 `/metrics` 监控端点，提供 Prometheus 指标：
  - `deltascope_http_requests_total`
  - `deltascope_http_request_duration_seconds`

### 代理环境 IP 限流加固

- 新增 `-trusted-proxies` 参数，显式配置可信代理 CIDR。
- 默认不信任任何代理，降低伪造转发头带来的风险。

## Bug 修复

- 为限流器键空间增加过期回收，降低高基数键在长时间运行下的内存增长风险。
- 移除库层 `NewHandler` 中的 Gin 全局模式副作用，改为在 server 入口设置 release mode。

## 破坏性变更

无。

## 升级说明

常用 HTTP 参数示例：

```bash
deltascope-server \
  -listen 127.0.0.1:8083 \
  -auth-enabled \
  -auth-keys 'k1,k2' \
  -rate-limit-enabled \
  -rate-limit-rps 10 \
  -rate-limit-burst 20 \
  -rate-limit-key api-key
```

若在反向代理后使用 `-rate-limit-key ip`，请配置可信代理网段：

```bash
deltascope-server -rate-limit-key ip -trusted-proxies '10.0.0.0/8,192.168.0.0/16'
```
