# 完成 Create-Table 超集任务提示

> 用于 `Complete Create-Table Superset` 里程碑的逐任务实施和审查。
> 每个提示假设工作在 `/Users/fan/GolangProjects/deltascope` 内进行。

## 全局规则

- 遵循 `docs/plans/2026-03-20-complete-create-table-superset-design.md` 中的设计。
- 遵循 `docs/plans/2026-03-20-complete-create-table-superset-implementation.md` 中的实施顺序。
- 保持当前解析器中立的 DDL 流程。
- 不引入实时元数据依赖。
- 保持 `three-level-doc` 作为硬关卡。

## 任务主题

- 标识符和关键字治理
- 更广泛的类型族和字符集/排序规则
- 更深入的冗余索引分析
- 剩余 create-table 对象形状差距

## 验证模式

每个任务必须返回：
- 更改的文件
- 运行的测试
- 状态
- 提交哈希

目标验证在实施计划中定义，应按原样使用。
