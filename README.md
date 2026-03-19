# DeltaScope

DeltaScope is an offline SQL review engine for MySQL and TiDB. The first release focuses on library and CLI usage for DDL and DML auditing without requiring a live database connection.

## Architecture

DeltaScope uses a DDD-leaning structure. Interfaces drive application use cases, application orchestrates domain behavior, and infrastructure provides parser, config, and output adapters. The core review model is built around normalized statement specifications and rule findings.

### Modules

| Module | Description | Doc |
|--------|-------------|-----|
| cmd/deltascope | CLI process entrypoint | [README](/Users/fan/GolangProjects/deltascope/cmd/deltascope/README.md) |
| internal/interfaces/cli | CLI adapter layer | [README](/Users/fan/GolangProjects/deltascope/internal/interfaces/cli/README.md) |
| internal/application | Use-case orchestration layer | [README](/Users/fan/GolangProjects/deltascope/internal/application/README.md) |
| internal/application/audit | Application parse/audit orchestration | [README](/Users/fan/GolangProjects/deltascope/internal/application/audit/README.md) |
| internal/application/policy | Application policy loader | [README](/Users/fan/GolangProjects/deltascope/internal/application/policy/README.md) |
| internal/domain | Core domain types and rules | [README](/Users/fan/GolangProjects/deltascope/internal/domain/README.md) |
| internal/domain/spec | Normalized statement specifications | [README](/Users/fan/GolangProjects/deltascope/internal/domain/spec/README.md) |
| internal/domain/rule | Rule findings and severity model | [README](/Users/fan/GolangProjects/deltascope/internal/domain/rule/README.md) |
| internal/domain/rule/ddl | Tier-1 DDL rule catalog | [README](/Users/fan/GolangProjects/deltascope/internal/domain/rule/ddl/README.md) |
| internal/domain/rule/dml | Tier-1 DML rule catalog | [README](/Users/fan/GolangProjects/deltascope/internal/domain/rule/dml/README.md) |
| internal/domain/policy | Policy configuration model | [README](/Users/fan/GolangProjects/deltascope/internal/domain/policy/README.md) |
| internal/domain/report | Audit result aggregation and verdict | [README](/Users/fan/GolangProjects/deltascope/internal/domain/report/README.md) |
| internal/infrastructure | Infrastructure adapter layer | [README](/Users/fan/GolangProjects/deltascope/internal/infrastructure/README.md) |
| internal/infrastructure/parser | Parser adapter namespace | [README](/Users/fan/GolangProjects/deltascope/internal/infrastructure/parser/README.md) |
| internal/infrastructure/parser/tidb | TiDB parser adapter | [README](/Users/fan/GolangProjects/deltascope/internal/infrastructure/parser/tidb/README.md) |
| internal/infrastructure/config/viper | YAML config adapter | [README](/Users/fan/GolangProjects/deltascope/internal/infrastructure/config/viper/README.md) |
| internal/infrastructure/output | Output renderer namespace | [README](/Users/fan/GolangProjects/deltascope/internal/infrastructure/output/README.md) |
| internal/infrastructure/output/markdown | Markdown renderer | [README](/Users/fan/GolangProjects/deltascope/internal/infrastructure/output/markdown/README.md) |
| internal/infrastructure/output/json | JSON renderer | [README](/Users/fan/GolangProjects/deltascope/internal/infrastructure/output/json/README.md) |
| configs | Example configuration files | [README](/Users/fan/GolangProjects/deltascope/configs/README.md) |
| pkg/deltascope | Stable public package surface | [README](/Users/fan/GolangProjects/deltascope/pkg/deltascope/README.md) |
