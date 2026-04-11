# 在 CI 中拦截风险 DML

将 DeltaScope 用作 CI 门控，在风险 DML 到达生产环境前捕获问题。当审查发现超过配置的 `--fail-on` 阈值时，DeltaScope 以退出码 `1` 退出，流水线步骤随即失败。

## 工作原理

1. DeltaScope 依据配置的策略审计 SQL。
2. 如果任何发现的严重级别达到或超过 `--fail-on` 阈值，进程以退出码 `1` 退出。
3. CI 流水线将退出码 `1` 视为步骤失败，阻止合并或部署。
4. 退出码 `0` 表示所有发现均低于阈值——步骤通过。
5. 退出码 `2` 表示输入或配置有误——视为流水线配置问题，而非审计发现。
6. 退出码 `3` 表示 DeltaScope 内部错误。

## 本地测试

提交前在本地测试 DML，与 CI 完全一致：

```bash
deltascope audit \
  --sql "UPDATE users SET status = 'disabled'" \
  --format json \
  --fail-on blocker

echo "Exit code: $?"
```

预期输出（JSON）：

```json
{
  "verdict": "reject",
  "summary": { "statements": 1, "blockers": 1, "warnings": 0, "notices": 0 },
  "explanation": {
    "summary": "Audit produced 1 finding(s) across 1 statement(s)",
    "reasons": ["UPDATE and DELETE statements must include a WHERE clause"]
  },
  "statements": [
    {
      "index": 0,
      "kind": "dml",
      "raw_sql": "UPDATE users SET status = 'disabled'",
      "explanation": {
        "summary": "Statement 1 has 1 finding(s)",
        "reasons": ["UPDATE and DELETE statements must include a WHERE clause"]
      },
      "findings": [
        {
          "rule_id": "dml.where.require",
          "level": "blocker",
          "message": "UPDATE and DELETE statements must include a WHERE clause",
          "suggestion": "add a WHERE clause that narrows the affected rows",
          "statement_kind": "dml",
          "location": { "line": 1, "column": 1 }
        }
      ]
    }
  ]
}
```

```
Exit code: 1
```

## GitHub Actions

以下是一个完整的工作流，在每次 Pull Request 时安装 DeltaScope 并审计 SQL 迁移文件：

```yaml
name: Audit SQL
on: [pull_request]

jobs:
  audit:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Install DeltaScope
        run: |
          curl -L https://github.com/Fanduzi/DeltaScope/releases/download/v0.24.0/deltascope_0.24.0_linux_amd64.tar.gz \
            -o /tmp/deltascope.tar.gz
          tar -xzf /tmp/deltascope.tar.gz -C /tmp
          install /tmp/deltascope /usr/local/bin/deltascope

      - name: Audit SQL migrations
        run: |
          deltascope audit \
            --file ./sql/changes.sql \
            --format json \
            --fail-on blocker
```

更严格的门控（同时拦截 warning）：

```yaml
      - name: Audit SQL migrations (strict)
        run: |
          deltascope audit \
            --file ./sql/changes.sql \
            --format json \
            --fail-on warning
```

仅审计 PR 中变更的 SQL 文件：

```yaml
      - name: Audit changed SQL files
        run: |
          git diff --name-only origin/${{ github.base_ref }}...HEAD \
            | grep '\.sql$' \
            | while read f; do
                echo "==> Auditing $f"
                deltascope audit --file "$f" --format json --fail-on blocker || exit 1
              done
```

## GitLab CI

```yaml
audit-sql:
  stage: validate
  image: ubuntu:22.04
  before_script:
    - apt-get update -qq && apt-get install -y -qq curl tar
    - curl -L https://github.com/Fanduzi/DeltaScope/releases/download/v0.24.0/deltascope_0.24.0_linux_amd64.tar.gz \
        -o /tmp/deltascope.tar.gz
    - tar -xzf /tmp/deltascope.tar.gz -C /tmp
    - install /tmp/deltascope /usr/local/bin/deltascope
  script:
    - |
      for f in ./sql/migrations/*.sql; do
        echo "==> Auditing $f"
        deltascope audit --file "$f" --format json --fail-on blocker || exit 1
      done
  rules:
    - if: '$CI_PIPELINE_SOURCE == "merge_request_event"'
```

## GitHub Actions 原生注解

使用 `--format github-actions` 生成 CI 内联注解，渲染在 GitHub Actions 工作流日志中：

```yaml
      - name: Audit SQL migrations (annotations)
        run: |
          deltascope audit \
            --file ./sql/changes.sql \
            --format github-actions \
            --fail-on blocker
```

## SARIF 输出用于 GitHub Code Scanning

使用 `--format sarif` 生成 SARIF 2.1.0 输出，集成 GitHub Code Scanning：

```yaml
      - name: Audit SQL migrations (SARIF)
        run: |
          deltascope audit \
            --file ./sql/changes.sql \
            --format sarif \
            --fail-on blocker > deltascope.sarif

      - name: Upload SARIF results
        uses: github/codeql-action/upload-sarif@v3
        if: always()
        with:
          sarif_file: deltascope.sarif
```

## CI 中的 PostgreSQL 迁移安全审计

在 CI 中使用迁移安全规则审计 PostgreSQL 迁移文件：

```bash
deltascope audit --dialect postgresql --file ./migrations.sql --format sarif > deltascope.sarif
```

### 识别方言误配

如果 CI 管道审计 SQL 时未显式设置 `--dialect`，DeltaScope 默认以 MySQL 模式运行。当遇到 PostgreSQL 专属语法时，会发出建议性通知（`dialect.postgresql.syntax.detected.notice`），但不会自动切换方言。

在 CI 中识别此通知：

```bash
# 检查输出是否包含 PostgreSQL 语法通知
deltascope audit --file ./migrations.sql --format json | jq '.global_findings[] | select(.rule_id == "dialect.postgresql.syntax.detected.notice")'
```

如果通知触发，要么在审计命令中添加 `--dialect postgresql`，要么确认 SQL 确实兼容 MySQL 并忽略该通知。

### 理解能力边界错误

当使用 `--dialect postgresql` 且 PG-capable 构建版本遇到尚未支持的 PostgreSQL 功能面（如复杂的 DDL 解析）时，DeltaScope 返回类型化的 `PostgreSQLCapabilityBoundaryError`。在 CI 中表现为退出码 `2`。通过错误消息可以区分——能力边界错误会明确说明请求的功能面和当前构建的支持能力。

### CI 中的 PostgreSQL DDL 覆盖范围（v0.21.0 / v0.23.0 / v0.24.0）

从 v0.21.0 开始，常见的 PostgreSQL 迁移后续 DDL 语句——`SET DEFAULT`、`DROP DEFAULT`、`SET NOT NULL`、`DROP NOT NULL`、`VALIDATE CONSTRAINT` 和 `DROP CONSTRAINT`——通过共享审核管线进行标准化处理，不再返回能力边界错误。这些语句在 CI 中产生正常的审计结果，减少了标准分步迁移序列上的误报工作流中断。

从 `v0.23.0` 开始，包含常见共享规则兼容约束形态的 PostgreSQL `CREATE TABLE` 语句，也可以在 CI 中保持正常审计路径，例如命名 `CHECK` / `UNIQUE` / `FOREIGN KEY`、内联 `CHECK`、内联 `UNIQUE` 与内联 `REFERENCES`。

从 `v0.24.0` 开始，这些建表外键形态携带更丰富的语义信息——解析器拥有的 `ReferencedTable` 和 `ReferencedColumns`——通过共享审核管线流转，继续在 CI 中产生正常审计结果，不新增规则 ID。

表述时请保持准确：

- 这是更广的 PostgreSQL `CREATE TABLE` 覆盖范围，不是完整 PostgreSQL DDL 支持
- 这是共享规则复用，不是新的规则包
- 内联 `REFERENCES` 仅是 parser-owned 的共享结构，不代表新的 metadata-aware 外键契约
- `v0.24.0` 深化了外键语义（`ReferencedTable`/`ReferencedColumns`）——这些是解析器拥有的结构事实，不是元数据真相

### Maintainer Confidence Targets（`v0.22.0` → `v0.24.0` release 线）

`v0.22.0` 建立了 **E2E & Release Confidence Pack**。进入 `v0.24.0` 后，维护者仍通过同一组规范化仓库入口完成验证：

- `make pg-unit-test-gates`
- `make pg-e2e-gates`
- `make pg-confidence-gates`
- `make release-surface-gates VERSION=v0.24.0`
- `make release-version-surface-gates VERSION=v0.24.0`

## --fail-on 策略说明

| 设置 | 退出码 1 的触发条件 | 适用场景 |
|------|-------------------|---------|
| `--fail-on blocker` | 存在任何 blocker 级别发现 | 大多数团队——仅硬性策略违规才阻断流水线 |
| `--fail-on warning` | 存在任何 warning 或 blocker 级别发现 | 严格团队——任何警告都阻断 |
| `--fail-on notice` | 存在任何级别的发现 | 最高门控——对任何偏差零容忍 |
| `--fail-on none` | 永不触发 | 仅审计模式——记录发现但不阻断流水线 |

建议从 `--fail-on blocker` 开始，随着团队对规则集建立信心后逐步收紧。

## 审计多个文件

遍历多个 SQL 文件，遇到第一个失败时立即退出：

```bash
for f in ./sql/migrations/*.sql; do
  echo "==> Auditing $f"
  deltascope audit \
    --file "$f" \
    --format json \
    --fail-on blocker \
  || { echo "FAILED: $f"; exit 1; }
done
echo "All files passed."
```

审计所有文件后再统一报告失败（收集全部发现）：

```bash
FAILED=0
for f in ./sql/migrations/*.sql; do
  echo "==> Auditing $f"
  deltascope audit --file "$f" --format json --fail-on blocker || FAILED=1
done
exit $FAILED
```

## 退出码说明

| 退出码 | 含义 | 处理方式 |
|--------|------|---------|
| `0` | 所有发现均低于 `--fail-on` 阈值（或无任何发现） | 流水线通过——继续执行 |
| `1` | 一个或多个发现超过 `--fail-on` 阈值 | 审计门控触发——阻止合并或部署 |
| `2` | 输入或配置有误（SQL 文件路径无效、参数未知、配置格式错误） | 修复流水线配置 |
| `3` | DeltaScope 内部错误 | 提交 Bug 报告；不要视为审计发现 |
