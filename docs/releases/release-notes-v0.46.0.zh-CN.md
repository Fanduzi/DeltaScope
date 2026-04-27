# DeltaScope v0.46.0 发行说明

## 摘要

v0.46.0 清理了发布工作流中的 Homebrew cask 安装验证路径。成功的发布运行不再显示误导性的 Homebrew tap/cask 不可用错误注解。真实的 Homebrew 安装、版本和审计失败仍然会阻塞发布。

## 变更

- 发布工作流中的 `verify-homebrew-cask-install` job 现在使用条件清理探测（`if brew list --cask deltascope`）替代容忍失败的回退写法（`|| true`）。在全新 CI runner 上，清理步骤被静默跳过；在重复运行时，它会先移除之前的 cask 和 tap 再重新安装。
- 新增 `make release-workflow-hygiene-gates` 静态门控，强制要求条件清理探测、小写 tap 名称，并拒绝发布工作流中的容忍失败模式。该门控已包含在 `make release-contract-gates` 中。
- 在开发者测试文档中记录了 Homebrew 验证卫生契约。

## 发布工作流卫生

发布工作流在 macOS 上通过已发布的 tap 执行真实 Homebrew 安装。它验证：

- cask 可以从 `fanduzi/deltascope` 安装
- `deltascope --version` 包含发布 tag
- 二进制包含 PostgreSQL 支持
- PostgreSQL 审计冒烟测试通过

在 v0.46.0 之前，清理步骤使用 `brew uninstall --cask deltascope || true` 和 `brew untap Fanduzi/deltascope || true`。在全新 runner 上这些命令会向 stderr 输出错误信息，而 GitHub Actions 会将 stderr 提升为错误注解，即使退出码已被捕获。v0.46.0 的修复将无条件清理替换为条件探测，因此不再出现虚假注解。

## 验证

- `make release-contract-gates VERSION=v0.46.0` — 所有门控通过
- `make release-workflow-hygiene-gates` — 静态门控通过
- `make release-test-gates` — 所有测试通过

## 非目标

- 不涉及 SQL 审计行为变更。
- 不涉及 parser、规则或策略变更。
- 不涉及格式化器变更。
- 不涉及发布产物命名变更。
- 不涉及 npm launcher 行为变更。

## 安装

**macOS（推荐）：**

```bash
brew tap fanduzi/deltascope
brew install --cask deltascope
```

**通用安装脚本：**

```bash
curl -fsSL https://raw.githubusercontent.com/Fanduzi/DeltaScope/v0.46.0/install.sh | \
  DELTASCOPE_VERSION=v0.46.0 sh
```
