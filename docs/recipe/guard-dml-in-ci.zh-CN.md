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
          curl -L https://github.com/Fanduzi/DeltaScope/releases/download/v0.6.2/deltascope_0.6.2_linux_amd64.tar.gz \
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
    - curl -L https://github.com/Fanduzi/DeltaScope/releases/download/v0.6.2/deltascope_0.6.2_linux_amd64.tar.gz
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
