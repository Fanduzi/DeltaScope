# DeltaScope v0.6.2 发布说明

## 概览

`v0.6.2` 这次聚焦在 explainable audit results 的产品化落地。它为 finding、语句级结果与总体结果补上结构化 explanation，打通 CLI / HTTP / library 的输出形态，并同步刷新中英文文档，使公开契约与运行时行为一致。

## 亮点

- 审核摘要、语句结果、finding 级别的结构化 explanation
- 稳定公共 API 与 HTTP 输出同步支持 explainable findings
- CLI Markdown 渲染输出更丰富的解释信息
- 中英文文档统一按当前运行时契约校准
- 保持现有安装与发布流程不变的补丁版本发布

## 新增与改进

### 可解释审核结果

- 审核结果现在在总体与语句级别都包含聚合 `explanation`
- 单条 finding 现在可以携带 `summary`、`why`、`risk`、`suggestion` 等结构化解释字段
- metadata-aware finding 现在可以通过结构化 explanation metadata 暴露元数据可用性说明

### 公共接口面对齐

- `pkg/deltascope` 已将更丰富的 explanation 模型映射到稳定公共 API
- HTTP 适配层现在稳定返回更新后的审核结果结构
- Markdown CLI 渲染器现在会在面向人的输出中直接打印 explanation 细节

### 规则目录与可发现性

- 内置规则目录现在附带更丰富、偏 explanation 导向的元数据
- `rules list`、`rules show` 及相关文档更准确反映当前输出契约
- 规则示例与 metadata-aware 说明已按目录中的实际运行时输出对齐

### 文档刷新

- 英文与中文的 README、recipe、reference 页面已同步更新
- 文档现已统一反映 `omitempty` 行为、verdict 语义、explanation 字段与本地化交叉链接
- 发布安装示例已统一指向 `v0.6.2`

## 安装 / 升级

安装最新版：

```bash
curl -fsSL https://raw.githubusercontent.com/Fanduzi/DeltaScope/main/install.sh | sh
```

安装当前版本：

```bash
curl -fsSL https://raw.githubusercontent.com/Fanduzi/DeltaScope/v0.6.2/install.sh | \
  DELTASCOPE_VERSION=v0.6.2 sh
```

## 兼容性

- 支持操作系统：`darwin`、`linux`
- 支持架构：`amd64`、`arm64`
- 支持数据库方言：`MySQL`、`TiDB`

## 已知限制

- metadata-aware 联机检查仍依赖实时 schema 访问，离线模式下不可用
- 发布示例文档描述的是当前已发布行为，未来 minor 版本若公共接口继续扩展，文档仍可能继续调整
- MCP Server 仍不属于这一条发布线
