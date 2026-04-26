# 在 GitLab CI 中使用 DeltaScope

DeltaScope 可以生成 GitLab Code Quality 报告，让 SQL 审计发现直接显示在合并请求的 Code Quality 小组件和差异标注中。

## 最小流水线

```yaml
stages:
  - test

deltascope_sql_audit:
  stage: test
  image: golang:1.26
  script:
    - go install github.com/Fanduzi/DeltaScope/cmd/deltascope@v0.45.0
    - deltascope audit --file migrations.sql --format gitlab-codequality --fail-on none > gl-code-quality-report.json
  artifacts:
    reports:
      codequality: gl-code-quality-report.json
    when: always
```

## 控制流水线通过/失败

`--fail-on none` 仅上报发现，不阻塞流水线。需要在发现 warning 或 blocker 时失败：

```bash
deltascope audit --file migrations.sql --format gitlab-codequality --fail-on warning > gl-code-quality-report.json
```

退出码：0 = 通过，1 = 存在达到阈值的发现，2 = 用户错误。

## 多方言支持

```bash
# MySQL（默认）
deltascope audit --dialect mysql --file migrations.sql --format gitlab-codequality

# TiDB
deltascope audit --dialect tidb --file migrations.sql --format gitlab-codequality

# PostgreSQL
deltascope audit --dialect postgresql --file migrations.sql --format gitlab-codequality
```

## 使用已发布二进制

如果不想在 CI 中执行 `go install`：

```yaml
deltascope_sql_audit:
  stage: test
  image: alpine:3.21
  before_script:
    - wget -q https://github.com/Fanduzi/DeltaScope/releases/download/v0.45.0/deltascope_0.45.0_linux_amd64.tar.gz
    - tar xzf deltascope_0.45.0_linux_amd64.tar.gz
  script:
    - ./deltascope audit --file migrations.sql --format gitlab-codequality --fail-on none > gl-code-quality-report.json
  artifacts:
    reports:
      codequality: gl-code-quality-report.json
    when: always
```

根据实际平台调整下载 URL。

## 字段映射

DeltaScope 将审计发现映射为 GitLab Code Quality 字段：

| DeltaScope | GitLab Code Quality |
|-----------|---------------------|
| 规则 ID | `check_name` |
| 消息 + 建议 | `description` |
| blocker → major、warning → minor、notice → info | `severity` |
| `--file` 路径或 `deltascope.sql` | `location.path` |
| 发现所在行号或 1 | `location.lines.begin` |
| SHA-256 哈希 | `fingerprint` |

Fingerprint 在不同运行之间保持稳定，GitLab 可据此跨流水线追踪发现。

## 限制

- 不支持的语句（解析器诊断）不会作为 Code Quality 问题输出，仍保留在 JSON 和 markdown 输出中。
- 不需要也不使用 GitLab API 集成。
- 内联 SQL（`--sql`）和标准输入使用合成路径 `deltascope.sql`。
- 本功能不宣称 GitLab 安全仪表盘支持。
