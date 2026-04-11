# DeltaScope v0.21.0 发行说明

发布日期：2026-04-11

## 概述

DeltaScope `v0.21.0` 是 **PostgreSQL DDL 覆盖包**——一个扩展 PostgreSQL DDL 标准化范围的版本，使常见迁移后续语句通过共享审核管线处理，不再返回能力边界错误。

不新增规则。已有规则 ID、级别和触发条件不变。本版本的价值在于已有的共享规则族和 metadata-aware 语义现在可以覆盖更多 PostgreSQL DDL 动作。

## 变更内容

### PostgreSQL ALTER TABLE 覆盖范围扩展

六种之前返回能力边界错误的常见 PostgreSQL `ALTER TABLE` 形式，现在通过共享 `spec.Alter` contract 进行标准化：

| PostgreSQL DDL | 标准化动作 | 含义 |
|----------------|----------|------|
| `ALTER TABLE ... ALTER COLUMN ... SET DEFAULT` | `set_default` | 分步上线中的列默认值设置现在可审计 |
| `ALTER TABLE ... ALTER COLUMN ... DROP DEFAULT` | `drop_default` | 列默认值移除现在可审计 |
| `ALTER TABLE ... ALTER COLUMN ... SET NOT NULL` | `set_not_null` | 回填后的非空约束施加现在可审计 |
| `ALTER TABLE ... ALTER COLUMN ... DROP NOT NULL` | `drop_not_null` | 非空约束放宽现在可审计 |
| `ALTER TABLE ... VALIDATE CONSTRAINT` | `validate_constraint` | 推荐的 `NOT VALID` → `VALIDATE` 模式中的约束验证步骤现在可审计 |
| `ALTER TABLE ... DROP CONSTRAINT` | `drop_constraint` | 约束移除现在可审计；主键删除在 metadata 可用时映射到已有的 `ddl.alter.drop_primary_key` 规则 |

### 共享规则复用

本版本不引入新规则。新标准化的 PostgreSQL DDL 动作通过已有的共享规则族处理：

- **Alter 语义规则**适用于 `set_default`、`drop_default`、`set_not_null` 和 `drop_not_null`，作为标准 alter 动作处理。
- **Metadata-aware 主键规则**在 `DROP CONSTRAINT` 目标为主键且 metadata 可用时适用。
- **`VALIDATE CONSTRAINT`** 是 supported 且 auditable 的，但没有专用规则。除非其他 finding 适用，否则产生干净的审计结果。

### 接口一致性

所有新标准化的 PostgreSQL DDL 动作已在四个接口上确认一致：

- **CLI**：`deltascope audit --dialect postgresql --sql "..."`
- **HTTP**：`POST /v1/audit` 并设置 `"dialect": "postgresql"`
- **MCP**：`audit_sql` 工具并设置 `"dialect": "postgresql"`
- **公共 Go API**：`deltascope.Audit(ctx, deltascope.Request{Dialect: deltascope.DialectPostgreSQL, ...})`

### 示例

审计分步迁移的后续步骤：

```bash
deltascope audit \
  --dialect postgresql \
  --sql "alter table users alter column status set default 'active';"
```

审计约束生命周期步骤：

```bash
deltascope audit \
  --dialect postgresql \
  --sql "alter table users validate constraint chk_amount;"
```

审计约束移除：

```bash
deltascope audit \
  --dialect postgresql \
  --sql "alter table orders drop constraint orders_pkey;"
```

## 重要说明

- **`VALIDATE CONSTRAINT`** 是 supported 且 auditable 的，但没有专用规则。除非同一语句上适用其他 finding，否则产生干净的审计结果。它不是"保证产生 finding"。
- **`DROP CONSTRAINT` 针对主键**（如 `DROP CONSTRAINT users_pkey`）仅在 metadata-aware 模式下触发已有的 `ddl.alter.drop_primary_key` 规则。在离线模式下，它作为普通 alter 动作通过。
- 本版本收窄了未支持的 PostgreSQL DDL 范围，但不声称已全面支持 PostgreSQL DDL。

## 安装 / 升级

```bash
curl -fsSL https://raw.githubusercontent.com/Fanduzi/DeltaScope/v0.21.0/install.sh | \
  DELTASCOPE_VERSION=v0.21.0 sh
```

macOS 用户可通过 Homebrew 安装：

```bash
brew tap Fanduzi/deltascope
brew install --cask deltascope
```

## 兼容性

无破坏性变更。`v0.21.0` 通过增加覆盖能力扩展了已有的审核 contract：

- 所有已有的 MySQL/TiDB/PostgreSQL 离线和 metadata-aware 行为不变
- 不引入新的规则 ID、严重级别或触发条件
- 新标准化的动作通过已有的规则族处理；无需修改策略 YAML
- CLI、HTTP、MCP 和 `pkg/deltascope` 公共 API contract 不变——只是从返回能力边界错误变为返回正常审计结果的 PostgreSQL DDL 形式集合扩大了
