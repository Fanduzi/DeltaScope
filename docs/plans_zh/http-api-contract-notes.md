# DeltaScope HTTP API 契约笔记

## 端点

### `GET /healthz`

- 返回 `200 OK`
- body:

```json
{"status":"ok"}
```

### `GET /version`

- 返回 `200 OK`
- body:

```json
{"version":"dev"}
```

### `POST /v1/audit`

- 请求 body:

```json
{
  "sql": "delete from users",
  "dialect": "mysql"
}
```

- `sql` 为必填
- `dialect` 为可选；空表示 `mysql`
- 响应 body 复用稳定的公共 `pkg/deltascope.Result` JSON 形状

## 错误形状

每个非 2xx 响应使用：

```json
{
  "error": {
    "code": "bad_request",
    "message": "..."
  }
}
```

支持的错误码：

- `bad_request`
- `invalid_json`
- `config_invalid`
- `internal`

## 状态映射

- `400` 用于格式错误的 JSON 或无效的审计输入
- `500` 用于服务器端配置/运行时故障
