# T10 / #89 — Advanced index forms and visibility changes

T10 最终技术验收：**ACCEPTED**（独立评审宣判）。

本分支为**独立证据分支**，仅发布已验收的 T10-R6 证据归档，与产品里程碑分支 `milestone/mysql-tidb-ddl-completion` 相互独立，不代表产品代码发布，也不代表 T11–T26 任何验收。

## Evidence archive

- File: `t10/t10-advanced-index-evidence-r6.tar.gz`
- SHA-256: `a9167199b68296fd730b83fe8b255e5fade480b2c293b85756763cdea535b092`
- Size: 146,245 bytes
- Manifest: 85 payload rows（NEW=9 / REUSED=76），manifest/payload-index/reuse-ledger 均经独立核验

## Frozen acceptance identity (T10 验收时的本地快照)

- HEAD: `9d2c439c46f6faed434e91c8b54ebc82abbf0787`
- Tree: `bcec2a29997276ca8d3a173da15ce7cb4c66ab4c`
- Parent: `d6af124165ff05ba1ca18948a7aa20c8f03df58f`
- Branch: `milestone/mysql-tidb-ddl-completion`

说明：T10 的 16 个实现与冻结 Oracle 提交已存在于远端里程碑基点 `d0c7cf3373167db7065d3027088a43c56ed6dbf5` 的祖先集中；上述冻结 HEAD 为验收时的本地快照，不是当前远端 milestone 分支 HEAD。

## Accepted verification evidence

- Golden Path: 126/126 cases PASS，3,187 assertions（受测 HEAD 与冻结身份一致；本发布不含重新运行）
- Constructed-control checks: 33/33 PASS
- R6 补证项 ER1（batch-6 生成器可执行、输出与 fixture 字节一致）/ ER2（历史 R4、R5 重建、R6 补测三类记录区分 + 四组补测）/ ER3（R5 README 核算勘误）— 全部关闭

## Stated boundaries（如实保留）

- 多值索引成员级查询语义、表达式等价性判定为有界未审计
- TiDB FULLTEXT/SPATIAL 不支持行为与解析器错误相区分
- 不含优化器/查询性能结论
- R4 历史性能输入不能证明与 R5 重建输入字节一致（如实区分历史测量与重建 fixture）
- R1–R5 归档未在本证据分支公开（仅 R6）
