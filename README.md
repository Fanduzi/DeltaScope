# DeltaScope

DeltaScope is an offline SQL review engine for MySQL and TiDB. The first release focuses on library and CLI usage for DDL and DML auditing without requiring a live database connection.

## What It Does

- audits DDL and DML for MySQL and TiDB
- runs fully offline against SQL text plus policy config
- returns `blocker`, `warning`, and `notice` findings with a final verdict of `reject`, `review`, or `pass`
- supports both a stable Go package API and the `deltascope` CLI

Current DDL coverage includes:

- table comment, table name length, and primary-key rules
- audit timestamp column patterns
- column comment, default, not-null, and float/double guidance
- varchar length limits
- create-table index count, index width, naming-prefix, and duplicate-index checks

## Install And Run

```bash
go test ./...
go run ./cmd/deltascope version
```

Audit inline SQL:

```bash
go run ./cmd/deltascope audit --sql "delete from users"
```

Audit a file:

```bash
go run ./cmd/deltascope audit --file ./change.sql
```

Audit from stdin:

```bash
cat ./change.sql | go run ./cmd/deltascope audit
```

Use JSON output for agents and scripts:

```bash
go run ./cmd/deltascope audit --sql "delete from users" --format json
```

Control the non-zero threshold:

```bash
go run ./cmd/deltascope audit --sql "create table users (id bigint, primary key (id))" --fail-on warning
```

Exit codes:

- `0`: audit finished and did not reach the configured failure threshold
- `1`: audit finished, but findings reached `--fail-on`
- `2`: user input error, such as invalid flags or unreadable config/file input
- `3`: internal/runtime error

## Configuration

Generate a usable template:

```bash
go run ./cmd/deltascope config init > deltascope.yaml
```

Use a policy file:

```bash
go run ./cmd/deltascope audit --config ./deltascope.yaml --sql "update users set name = 'delta'"
```

The v1 config model is rule-ID keyed YAML. The checked-in [deltascope.example.yaml](/Users/fan/GolangProjects/deltascope/configs/deltascope.example.yaml) matches `deltascope config init`.

## Library Usage

```go
result, err := deltascope.Audit(ctx, deltascope.Request{
    SQL:     "delete from users",
    Dialect: deltascope.DialectMySQL,
})
```

The public API lives in [pkg/deltascope](/Users/fan/GolangProjects/deltascope/pkg/deltascope/README.md).

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
