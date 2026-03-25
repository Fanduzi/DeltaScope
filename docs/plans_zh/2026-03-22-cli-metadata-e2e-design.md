# DeltaScope CLI 元数据 E2E 设计

## 目标

通过对真实 MySQL 和 TiDB 实例增加可重复执行的端到端测试，补齐元数据感知 CLI 路径上剩余的 live-smoke 风险，同时保持默认单元测试工作流无容器且足够快速。

## 成功标准

- 可以通过已交付的 CLI 对真实 MySQL 容器和真实 TiDB 容器执行 DeltaScope。
- E2E 套件覆盖关键的元数据感知 CLI 行为：
  - 方言自动检测
  - 显式方言不匹配
  - schema 推断
  - schema 歧义失败
  - 带 schema 限定的 SQL 跳过推断
  - 基于元数据的存在性/兼容性行为
  - create-table 的部分元数据行为
- 该套件可以通过稳定脚本与 Make 目标在本地运行。
- 默认 `go test ./...` 继续与容器化 E2E 解耦。
- 当该套件变绿后，可以从 handoff/progress 文档中去掉“尚未做 live smoke”风险。

## 为什么需要这个里程碑

CLI Completion 已经交付了元数据感知接线、schema 解析、规则目录命令、配置工具以及帮助/输出收口。当前剩余的不确定性，不在单元测试中的代码路径覆盖，而在于 CLI 是否能在真实 MySQL/TiDB 服务上完整走通连接与元数据路径。这个里程碑就是为了去除这部分不确定性。

## 推荐方向

使用 Docker Compose + shell 驱动的 CLI 断言，而不是尝试在 Go 测试中编排容器生命周期。

原因：

- 被测对象是 CLI，因此 shell 执行才是最诚实的测试界面
- Docker 很适合构建可重复的 MySQL/TiDB 元数据 fixture
- 基于 JSON 输出做 shell 断言，比把容器生命周期嵌入 `go test` 更简单、更透明

## 测试环境形状

推荐结构：

- `docker/cli-e2e-compose.yaml`
- `docker/mysql/init.sql`
- `docker/tidb/init.sql`
- `scripts/test_cli_metadata_e2e.sh`
- 用于便捷本地执行的 `Makefile` 目标

初始化数据应刻意制造：

- 一个唯一 schema 目标，例如 `app.users`
- 一个歧义表场景，例如同时存在 `app.users` 与 `archive.users`
- 至少一张适合做存在性与 alter 兼容性检查的表

## 断言策略

只把 CLI 本身作为公开测试界面。

优先采用 JSON 断言，以保持稳定：

- 退出码
- `context.mode`
- `context.dialect`
- `context.schema`
- 期望 `rule_id` finding 的存在

Markdown 可以做 1 到 2 个 smoke 断言，但不应成为主断言媒介。

## 覆盖范围

### MySQL 覆盖

- 连接参数存在时进入元数据感知模式
- 方言自动检测结果为 `mysql`
- schema 推断可解析唯一目标
- 歧义时产生用户错误并要求传入 `--schema`
- 像 `app.users` 这样的限定 SQL 会跳过 schema 推断
- 至少触发一个基于元数据的存在性规则
- 至少触发一个基于元数据的兼容性或大小规则
- 在对象尚不存在时，create-table 仍能以部分元数据模式工作

### TiDB 覆盖

- 方言自动检测结果为 `tidb`
- schema 推断正确解析
- 歧义时产生同样的用户契约
- 限定 SQL 跳过 schema 推断
- 至少触发一个基于元数据的存在性规则
- 至少有一个基于实例事实的规则证明 TiDB 元数据模式确实生效

## 范围外

- 为这套测试加 GitHub Actions 自动化
- 性能测试
- 超出 MySQL 与 TiDB 的更大兼容性矩阵
- 交互式密码提示自动化

密码提示已有单元测试覆盖；E2E 套件可直接使用显式 `--password` 以保证自动化可重复。

## 预期结果

完成该里程碑后，DeltaScope 应具有可信的真实实例证据，证明元数据感知 CLI 路径在 MySQL 与 TiDB 上可以端到端工作；CLI 风险表述也应从“尚未在线证明”升级为“已通过可重复本地 E2E 证明”。