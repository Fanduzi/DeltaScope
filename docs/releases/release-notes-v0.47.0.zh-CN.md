# DeltaScope v0.47.0 发行说明

## 摘要

v0.47.0 为所有 CI 渲染器带来了源码位置保真。GitHub Actions、SARIF 和 GitLab Code Quality 输出现在携带原始文件路径和语句起始行号，内联注解直接指向触发告警的具体 SQL 语句，而非迁移文件的第一行。

## 变更

- 审核管线现在使用渐进式源码映射器填充每条解析语句的 `Line` 和 `Column` 字段。该映射器向前扫描原始 SQL 缓冲区，逐条匹配 `RawSQL` 文本并计数换行符。这取代了之前的语句索引回退机制，为多语句迁移文件生成正确的行号。
- `Finding.Location` 现在在评估层从语句位置填充（仅当规则未提供自定义位置时），因此所有 CI 渲染器自动获取源码坐标，无需逐渲染器修改。
- GitHub Actions 输出（`--format github-actions`）现在发出 `file=<path>,line=N,col=N` 并使用正确的语句起始行。当未提供 `--file` 路径时，`file=` 键被完全省略而非回退到空值。
- SARIF 输出（`--format sarif`）现在包含 `artifactLocation.uri`（文件路径）和每个结果的 `startLine`/`startColumn`。当未提供 `--file` 路径时，`artifactLocation` 被省略。
- GitLab Code Quality 输出（`--format gitlab-codequality`）之前已将 `--file` 传播到 `location.path`；源码映射器现在确保 `location.lines.begin` 携带正确的语句起始行号。
- 新增 `make release-source-location-smoke` 门控，验证 GitHub Actions、SARIF、GitLab Code Quality 和 TiDB SARIF 输出的源码位置传播。该门控已包含在 `make release-contract-gates` 中。
- 专用单元测试锁定渐进式源码映射器行为：多行第二条语句位置、前导换行处理、重复语句渐进匹配、空行跳过和无匹配回退。
- 公共 API（`pkg/deltascope`）测试验证 `Audit()` 为 MySQL、TiDB 和 PostgreSQL 方言返回带正确 `Line` 和 `Column` 的 `Finding.Location`。
- CLI 集成测试验证 TiDB SARIF 和 TiDB GitLab Code Quality 的源码位置保真。
- HTTP 和 MCP 集成测试验证结构化响应中 finding 包含 `location.line` 和 `location.column`。

## 验证

- `make release-contract-gates VERSION=v0.47.0` — 所有门控通过
- `make release-source-location-smoke` — 源码位置冒烟测试通过
- `make test` — 所有单元测试通过
- 渐进式源码映射器为多行迁移文件中第二条语句的 `DELETE` 生成行号 9（而非语句索引回退的行号 2）

## 非目标

- 不涉及新规则 ID、解析器特性或策略变更。
- 除语句位置传播外不涉及其他领域逻辑变更。
- 不涉及 MySQL/TiDB/PostgreSQL 审计行为变更。
- 除自动序列化的 `location` 字段外不涉及 HTTP/MCP 传输协议变更。
- 不涉及发布产物命名变更。

## 安装

**macOS（推荐）：**

```bash
brew tap fanduzi/deltascope
brew install --cask deltascope
```

**通用安装脚本：**

```bash
curl -fsSL https://raw.githubusercontent.com/Fanduzi/DeltaScope/v0.47.0/install.sh | \
  DELTASCOPE_VERSION=v0.47.0 sh
```
