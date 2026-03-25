# HTTP API 服务实施计划

> **给 Claude 的提示：** 必需子技能：使用 superpowers:executing-plans 逐任务实施此计划。

**目标：** 在现有离线审计引擎之上添加薄 HTTP API 服务。

**架构：** 保持库优先核心；添加新的 HTTP 接口适配器加上一个小服务器入口点，委托给 CLI 使用的相同应用/审计流程。

**技术栈：** Go, 标准 `net/http` 或选择的路由器, 现有策略/配置堆栈, Go testing

---

### 任务 1：定义 HTTP 表面和契约

**文件：**
- 创建：`internal/interfaces/http/README.md`
- 创建：`docs/plans/http-api-contract-notes.md`

**步骤 1：** 确定请求/响应/错误形状
**步骤 2：** 记录端点和状态码行为
**步骤 3：** 提交

### 任务 2：添加 HTTP 请求/响应绑定层

**文件：**
- 创建：`internal/interfaces/http/handler.go`
- 创建：`internal/interfaces/http/handler_test.go`

**步骤 1：** 编写失败的处理器测试
**步骤 2：** 实施最小绑定和响应映射
**步骤 3：** 重新运行测试
**步骤 4：** 提交

### 任务 3：添加服务器接线和配置集成

**文件：**
- 创建：`internal/interfaces/http/server.go`
- 创建：`cmd/deltascope-server/main.go`
- 修改：根据需要修改相关配置文档/文件

**步骤 1：** 如果实际可行则编写失败的烟雾测试
**步骤 2：** 实施服务器接线和配置加载/热重载路径
**步骤 3：** 运行针对性测试
**步骤 4：** 提交

### 任务 4：验证并记录服务里程碑

**文件：**
- 修改：`README.md`
- 修改：交接/进度文档

**步骤 1：** 运行完整验证
**步骤 2：** 记录 curl 示例和服务使用
**步骤 3：** 提交
