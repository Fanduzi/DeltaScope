# DeltaScope v0.61.0 发行说明

## 概要

v0.61.0 带来全面的代码库质量改进：数据库连接池泄漏修复、MCP 服务三层 panic 恢复、静态分析集成（修复 903 个代码质量问题）、Context 传播支持（超时与取消）、1522 个测试的并行执行，以及性能优化（slice 预分配、strings.Builder 字符串拼接、markdown 渲染器 Builder 复用池）。代码库所有文件均在 800 行以下。无新规则、无解析器变更、无公共 API 变更。

## 质量改进

| 领域 | 变更 |
|------|------|
| 数据库连接 | 连接池泄漏修复，完善生命周期管理（`SetMaxOpenConns`、`SetMaxIdleConns`、`SetConnMaxLifetime`） |
| MCP 稳定性 | 三层 panic 恢复：工具处理器、服务处理器、进程级 |
| 静态分析 | golangci-lint v2 集成，15 个活跃 linters，自动修复 903 个问题 |
| Context 传播 | `context.Context` 超时与取消支持贯穿所有审计层 |
| 测试性能 | 1522 个测试并行执行（`t.Parallel()`） |
| 运行时性能 | Slice 预分配、`strings.Builder` 替代字符串拼接、markdown 渲染器 Builder 复用池 |

## 性能基准

热路径优化包括规则评估中的 slice 预分配、输出渲染器中 `strings.Builder` 替代 `fmt.Sprintf` 字符串拼接，以及 markdown 渲染器的 sync.Pool Builder 复用。

## 非目标

- 无新规则 ID、解析器功能或公共 API 变更。
- 无 MySQL/TiDB/PostgreSQL 审计行为变更。
- 无发布资产命名或安装工作流变更。

## 安装

**macOS（推荐）：**

```bash
brew tap fanduzi/deltascope
brew install --cask deltascope
```

**通用安装：**

```bash
curl -fsSL https://raw.githubusercontent.com/Fanduzi/DeltaScope/v0.61.0/install.sh | \
  DELTASCOPE_VERSION=v0.61.0 sh
```
