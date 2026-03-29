# HTTP Gin + Auth Hardening 设计文档

## 目标

定义 DeltaScope 下一阶段 HTTP 里程碑：将当前 `net/http` 适配层迁移到 Gin，并补齐生产可用的最小中间件能力，第一优先级是 API Key 认证。

本里程碑只增强 HTTP 可运维性和安全基线，不改变核心审核域模型与结果契约。

## 现状

当前 HTTP 服务：

- 基于 `net/http` 的轻量实现
- 路由仅有 `GET /healthz`、`GET /version`、`POST /v1/audit`
- 没有认证中间件
- 没有 timeout/recovery/request-id 中间件链
- 尚无限流与 metrics 暴露

接口可用，但缺少生产防护。

## 问题定义

DeltaScope HTTP 当前“功能可用但治理不足”，需要一次里程碑完成三件事：

1. 采用更清晰的 middleware 组织方式
2. 引入基础请求认证
3. 增加标准请求生命周期控制（recover、timeout、request-id、日志）

## 非目标

本里程碑不包含：

- 重构审核结果 JSON 契约
- 增加新的 SQL 规则
- 引入完整用户体系和 RBAC
- 接入 OAuth2/JWT IdP
- 多租户授权模型

## 方案比较

### 方案 A：保留 `net/http` 并手写中间件

优点：

- 迁移成本最低
- 不引入新框架依赖

缺点：

- middleware 组织体验一般
- 后续扩展效率不高

### 方案 B：迁移到 Gin

优点：

- 基于 `net/http`，生态兼容性好
- middleware 组织清晰
- 社区成熟，团队上手快

缺点：

- 引入新依赖
- 需要改写路由装配层

### 方案 C：迁移到 Fiber

优点：

- 性能表现强
- 开发体验较好

缺点：

- 基于 `fasthttp`，与当前标准库生态差异更大
- 在当前阶段迁移/维护成本更高

## 推荐

选择方案 B（Gin）。

理由：

- 当前瓶颈更可能在 SQL 解析与规则评估，而不是路由层
- Gin 能快速补齐认证和治理中间件
- 同时保持与现有 `net/http` 生态的兼容性，风险可控

## 设计

### 1. 适配层边界

保持业务层框架无关：

- `internal/application/*` 与 `internal/domain/*` 不引入 Gin 依赖
- Gin 仅作为 `internal/interfaces/http` 传输适配层

禁止在业务逻辑中传递 `gin.Context`。

### 2. 路由契约（保持不变）

保留现有端点：

- `GET /healthz`
- `GET /version`
- `POST /v1/audit`

保持现有响应结构和错误 envelope 语义，除非后续明确做版本化。

### 3. 认证模型

第一阶段采用 `X-API-Key`：

- 缺失 key -> `401`
- key 无效 -> `403`
- 放行路径可配置（默认 `/healthz`、`/version`）

配置项：

- `http.auth.enabled`
- `http.auth.keys`
- `http.auth.allow_paths`

轮换策略：

- 支持多 key 同时生效
- 服务端双 key 窗口
- 客户端迁移后下线旧 key

### 4. 中间件链

初版顺序：

1. request-id
2. recover
3. timeout
4. auth
5. access log

约束：

- timeout 必须返回稳定 JSON 错误结构
- 日志禁止记录原始 key

### 5. 面向未来的可替换性

降低后续框架替换成本：

- 认证核心逻辑从 Gin wrapper 中解耦
- 请求上下文元数据模型保持通用
- 通过 HTTP 黑盒契约测试锁定外部行为

后续若要切 Fiber，只需改适配层。

## 测试策略

新增/补齐 HTTP 黑盒测试：

- 路由兼容：`healthz/version/audit`
- 认证结果：200/401/403
- allow-path 放行
- timeout 错误 envelope
- panic recovery 行为

## 风险

1. mux -> Gin 迁移期间出现兼容性回归
2. 认证默认策略过严导致调用方中断
3. timeout 设置不当影响长 SQL 审核请求

## 发布策略

1. 先落地 Gin 适配且保持行为不变
2. 认证中间件加配置开关（开发默认可关闭）
3. 中间件与测试一起落地
4. 发布迁移说明和调用示例

## 成功标准

- Gin 版 HTTP 服务通过现有契约测试
- 开启认证后 `/v1/audit` 正确返回 401/403 语义
- middleware 链可观测且稳定
- 文档包含接入示例与轮换说明
