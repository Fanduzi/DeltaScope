# DeltaScope v0.110.0 发行说明

## 概述

v0.110.0 将两个先前延迟的 PostgreSQL DDL 边界形态 — `CREATE TRANSFORM` 和 `CREATE ACCESS METHOD` — 提升为受支持的生命周期发现，采用有界身份。PostgreSQL DDL 长尾普查达到 57/57 形态 `finding_covered`，零个 `unsupported_boundary`。

## 变更内容

v0.110.0 是一次聚焦提升发布。两个在 v0.100.0 中有意延迟的 DDL 形态现已成为受支持形态：

- **CREATE TRANSFORM** — 触发 `ddl.pg.create_transform.notice`。对象身份使用 `type@language`（如 `jsonb@plpython3u`）。`FROM SQL WITH FUNCTION` 和 `TO SQL WITH FUNCTION` 子句中的函数名不会被输出。
- **CREATE ACCESS METHOD** — 触发 `ddl.pg.create_access_method.notice`。对象身份仅使用访问方法名称（如 `heap2`）。`HANDLER` 函数名不会被输出。

### 长尾普查

| 分类 | 数量 |
|------|:----:|
| finding_covered | 57 |
| normalized_silent | 0 |
| unsupported_boundary | 0 |
| parser_error | 0 |

### 载荷安全

发现中不会输出 handler、function、body、query、definition 或 options 载荷。对象身份使用有界令牌：

- Transform 身份：`type@language`（如 `jsonb@plpython3u`）— 不含函数名。
- Access method 身份：仅名称（如 `heap2`）— 不含处理函数引用。

## 使用

```bash
# 离线审核 — 无需数据库连接
deltascope audit --dialect postgresql --sql "CREATE TRANSFORM FOR jsonb LANGUAGE plpython3u (FROM SQL WITH FUNCTION jsonb_to_plpython(jsonb), TO SQL WITH FUNCTION plpython_to_jsonb(internal))"

deltascope audit --dialect postgresql --sql "CREATE ACCESS METHOD heap2 TYPE TABLE HANDLER heap_tableam_handler"
```

两条规则均为离线规则，不需要数据库连接。

## 非目标

- 不新增 CREATE TRANSFORM 和 CREATE ACCESS METHOD 以外的对象族。
- 不声称完整 PostgreSQL DDL 支持。仅覆盖选定的长尾对象生命周期族；大量形态仍延迟处理。
- 不扩展 DCL/权限支持，仅保留已有的表级 GRANT/REVOKE 支持。
- 不执行实时 DDL 或迁移结果验证。
- 不声称 v1.0/稳定 API 合约。
- DeltaScope 不执行迁移。

## 安装

**macOS（推荐）：**

```bash
brew tap fanduzi/deltascope
brew install --cask deltascope
```

**通用安装脚本：**

```bash
curl -fsSL https://raw.githubusercontent.com/Fanduzi/DeltaScope/v0.110.0/install.sh | \
  DELTASCOPE_VERSION=v0.110.0 sh
```

## 升级

从 v0.100.0 升级：

```bash
# Homebrew
brew upgrade --cask deltascope

# 通用安装脚本（使用新版本重新运行）
curl -fsSL https://raw.githubusercontent.com/Fanduzi/DeltaScope/v0.110.0/install.sh | \
  DELTASCOPE_VERSION=v0.110.0 sh
```

## 验证

```bash
deltascope --version
# 应输出 v0.110.0

# 离线审核 — CREATE TRANSFORM
deltascope audit --dialect postgresql --sql "CREATE TRANSFORM FOR jsonb LANGUAGE plpython3u (FROM SQL WITH FUNCTION jsonb_to_plpython(jsonb), TO SQL WITH FUNCTION plpython_to_jsonb(internal))" --format json
# 发现应包含 rule_id: "ddl.pg.create_transform.notice"

# 离线审核 — CREATE ACCESS METHOD
deltascope audit --dialect postgresql --sql "CREATE ACCESS METHOD heap2 TYPE TABLE HANDLER heap_tableam_handler" --format json
# 发现应包含 rule_id: "ddl.pg.create_access_method.notice"
```
