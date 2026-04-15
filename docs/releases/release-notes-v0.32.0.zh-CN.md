# DeltaScope v0.32.0 发行说明

发布日期：2026-04-14

## 概述

DeltaScope `v0.32.0` 是 **PostgreSQL 边界支持就绪门控**。这是一个决策里程碑——不是功能发布。未新增 PostgreSQL 支持行为、rule ID、CLI 标志或公共 API 字段。新增了 characterization 测试以记录 generated 和 identity 列的稳定 AST 事实，并做出共享契约决策：推荐 `v0.33.0` 作为窄事实保留里程碑。

## 变更内容

### Characterization 测试

在 `internal/infrastructure/parser/postgresql/parser_test.go` 中新增了 7 个 characterization 测试，用于记录 `pg_query_go/v6` 产生的稳定 AST 事实：

- `GeneratedWhen` 编码：`"a"`（ALWAYS）和 `"d"`（BY DEFAULT）——稳定的单字符字符串
- `CONSTR_IDENTITY` 与 `CONSTR_GENERATED` 约束类型区分明确且确定
- Identity 序列选项为 `[]*Node` 的 `DefElem` 节点，包含 `defname` 和 `Integer` 参数
- `CREATE TABLE` 和 `ALTER TABLE ADD COLUMN` 对 generated/identity 列产生相同的 AST 结构
- `ALTER TABLE ... SET GENERATED` 产生 `DefElem` 节点（非 `Constraint` 节点）

这些测试仅断言 AST 结构——不改变任何生产代码路径。

### 决策报告

在 `docs/plans/reports/2026-04-14-v0.32.0-pg-boundary-support-readiness-report.md` 中记录了：

- 完整的 unsupported 边界清单（generic 和 explicit）
- Generated/identity 列的 AST 事实覆盖表
- 共享契约决策：DeltaScope 已准备好进行窄事实保留，而非完整语义支持
- 推荐的 `v0.33.0` 字段：`spec.Column` 上的 `GeneratedWhen`（string）和 `IsIdentity`（bool）
- 延迟领域：generated expression 渲染、identity 序列选项规范化、rule 行为变更、ALTER TABLE 状态转换

## Surface 契约

无 surface 契约变更。本次发布未修改：

- CLI 标志或输出格式
- HTTP API 请求/响应结构
- MCP 工具签名
- `pkg/deltascope` 公共 Go API
- Corpus YAML schema
- Rule 配置或行为

## 未变更内容

- DeltaScope 未将 generated expression 或 identity 语义建模为受支持的能力。
- 未新增 rule ID、CLI 标志或公共 API 类型。
- 未更改 parser dependency 或 package version。
- MySQL 和 TiDB 的审核行为不变。
- 所有现存的 PostgreSQL unsupported 边界（v0.26.0、v0.30.0、v0.31.0）仍然有效。
- 生产环境 extractor、spec、rule 和 policy 代码未变更。

## 后续 / 下一个里程碑

推荐的下一个里程碑：**v0.33.0 PostgreSQL Generated/Identity 事实保留包**。

范围：在 `spec.Column` 上新增 `GeneratedWhen`（string）和 `IsIdentity`（bool）作为 `omitempty` 字段。这仅是窄事实保留——不包含 generated expression 渲染、identity 序列选项规范化或 rule 行为变更。

关于 v0.33.0 完整推荐（包括明确的非目标），请参阅就绪报告。

## 安装 / 升级

```bash
# macOS（推荐）
brew tap Fanduzi/deltascope
brew install --cask deltascope

# 通用安装脚本
curl -fsSL https://raw.githubusercontent.com/Fanduzi/DeltaScope/v0.32.0/install.sh | \
  DELTASCOPE_VERSION=v0.32.0 sh
```
