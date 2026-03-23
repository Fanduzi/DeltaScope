# HTTP API 参考

`deltascope-server` 是一个轻量 JSON 适配层，底层使用与 CLI 和 `pkg/deltascope` Go 库完全相同的审计引擎。所有请求和响应均为 JSON 格式。服务器是无状态的，每次审计请求都会重新读取配置文件，因此策略变更无需重启即可生效。

## 服务器启动

### 启动参数

```
-listen string   HTTP 监听地址（默认 "127.0.0.1:8083"）
-config string   YAML 策略配置文件路径（可选）
-version         打印服务器构建版本并退出
```

### 启动命令

```bash
# 离线模式——使用内置默认策略
deltascope-server -listen 127.0.0.1:8083

# 指定自定义策略配置
deltascope-server -listen 127.0.0.1:8083 -config ./deltascope.yaml
```

---

## 接口列表

### GET /healthz

返回服务器健康状态。可用于存活探针（liveness probe）。

**请求：**

```bash
curl http://127.0.0.1:8083/healthz
```

**响应（200）：**

```json
{"status": "ok"}
```

---

### GET /version

返回服务器构建版本字符串。

**请求：**

```bash
curl http://127.0.0.1:8083/version
```

**响应（200）：**

```json
{"version": "v0.6.1"}
```

---

### POST /v1/audit

审计一条或多条 SQL 语句。请求体必须是单个 JSON 对象。

#### 请求

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `sql` | string | 是 | 待审计的一条或多条 SQL 语句 |
| `dialect` | string | 否 | `mysql` 或 `tidb`，省略时默认为 `mysql` |

> **注意：** 服务器启用了 `DisallowUnknownFields`。传入上述列表之外的额外字段将返回 `400 invalid_json` 错误。

#### 成功响应（200）

所有合法审计请求均返回 `200`，无论 SQL 是否通过审计。审计结果通过响应体中的 `verdict` 字段体现。

**拒绝示例——存在审计发现：**

```json
{
  "verdict": "reject",
  "summary": {
    "statements": 2,
    "blockers": 1,
    "warnings": 1,
    "notices": 0
  },
  "statements": [
    {
      "index": 1,
      "kind": "DELETE",
      "raw_sql": "DELETE FROM users",
      "findings": [
        {
          "rule_id": "dml.where.require",
          "level": "blocker",
          "message": "DELETE statement is missing a WHERE clause",
          "suggestion": "Add a WHERE clause to restrict the rows affected",
          "location": {
            "line": 1,
            "column": 1
          }
        }
      ]
    },
    {
      "index": 2,
      "kind": "CREATE TABLE",
      "raw_sql": "CREATE TABLE t (id INT) COMMENT='t'",
      "findings": [
        {
          "rule_id": "ddl.table.primary_key.require",
          "level": "blocker",
          "message": "table t does not have a primary key",
          "suggestion": "Add a PRIMARY KEY constraint"
        }
      ]
    }
  ],
  "global_findings": []
}
```

#### 通过响应（200）

没有规则触发时，`findings` 数组被省略，`verdict` 为 `pass`。

```json
{
  "verdict": "pass",
  "summary": {
    "statements": 1,
    "blockers": 0,
    "warnings": 0,
    "notices": 0
  },
  "statements": [
    {
      "index": 1,
      "kind": "CREATE TABLE",
      "raw_sql": "CREATE TABLE users (\n  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT 'primary key',\n  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'created time',\n  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT 'updated time',\n  PRIMARY KEY (id)\n) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='user records'",
      "findings": []
    }
  ],
  "global_findings": []
}
```

#### 错误响应

| HTTP 状态码 | 错误码 | 触发条件 |
|-------------|--------|----------|
| 400 | `invalid_json` | 请求体不是合法 JSON、包含未知字段，或包含多个 JSON 对象 |
| 400 | `bad_request` | `sql` 字段为空，或 `dialect` 值无法识别 |
| 500 | `config_invalid` | 服务器配置文件加载失败 |

**错误响应格式：**

```json
{
  "error": {
    "code": "bad_request",
    "message": "sql is required"
  }
}
```

#### curl 示例

```bash
# 审计 SQL——dialect 默认为 mysql
curl -s -X POST http://127.0.0.1:8083/v1/audit \
  -H 'Content-Type: application/json' \
  -d '{"sql": "DELETE FROM users WHERE id = 1"}'

# 指定 TiDB 方言进行审计
curl -s -X POST http://127.0.0.1:8083/v1/audit \
  -H 'Content-Type: application/json' \
  -d '{"sql": "ALTER TABLE users ADD COLUMN email VARCHAR(255)", "dialect": "tidb"}'

# 触发错误：SQL 为空
curl -s -X POST http://127.0.0.1:8083/v1/audit \
  -H 'Content-Type: application/json' \
  -d '{"sql": ""}'
# 返回：{"error":{"code":"bad_request","message":"sql is required"}}

# 触发错误：无效 JSON
curl -s -X POST http://127.0.0.1:8083/v1/audit \
  -H 'Content-Type: application/json' \
  -d 'not json'
# 返回：{"error":{"code":"invalid_json","message":"request body must be valid JSON"}}

# 触发错误：包含未知字段
curl -s -X POST http://127.0.0.1:8083/v1/audit \
  -H 'Content-Type: application/json' \
  -d '{"sql": "SELECT 1", "unknown_field": "value"}'
# 返回：{"error":{"code":"invalid_json","message":"request body must be valid JSON"}}
```

---

## 响应字段参考

### Result

`POST /v1/audit` 返回的顶层响应对象。

| 字段 | 类型 | 说明 |
|------|------|------|
| `verdict` | string | `pass`、`review` 或 `reject` |
| `summary` | object | 汇总计数：statements、blockers、warnings、notices |
| `statements` | array | 各语句的审计结果；为空时省略 |
| `global_findings` | array | 来自全局规则（跨语句检查）的发现；为空时省略 |

### StatementResult

`statements` 数组中的单个条目，表示一条 SQL 语句的审计结果。

| 字段 | 类型 | 说明 |
|------|------|------|
| `index` | int | 该语句在输入中的位置，从 1 开始计数 |
| `kind` | string | 语句类型，例如 `CREATE TABLE`、`ALTER TABLE`、`DELETE`、`UPDATE` |
| `raw_sql` | string | 该语句的原始 SQL 文本 |
| `normalized_sql` | string | 规范化后的 SQL（空白符标准化）；不可用时省略 |
| `findings` | array | 该语句的规则发现；为空时省略 |

### Finding

`findings` 数组中的单条规则结果。

| 字段 | 类型 | 说明 |
|------|------|------|
| `rule_id` | string | 稳定的规则标识符，例如 `dml.where.require` |
| `level` | string | `blocker`、`warning` 或 `notice` |
| `message` | string | 问题的人类可读描述 |
| `suggestion` | string | 建议的修复措施；不可用时省略 |
| `location` | object | 原始 SQL 中的位置 `{"line": N, "column": N}`；不可用时省略 |
| `metadata` | object | 规则特定的附加键值上下文；为空时省略 |

### Summary

每个审计结果均附带的汇总计数。

| 字段 | 类型 | 说明 |
|------|------|------|
| `statements` | int | 请求中 SQL 语句的总数 |
| `blockers` | int | 所有语句中 blocker 级别发现的总数 |
| `warnings` | int | 所有语句中 warning 级别发现的总数 |
| `notices` | int | 所有语句中 notice 级别发现的总数 |
