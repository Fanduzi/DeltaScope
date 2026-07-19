# Docker Module

Container assets for DeltaScope local end-to-end environments.

## Files

| File | Responsibility |
|------|---------------|
| cli-e2e-compose.yaml | Defines the local MySQL, TiDB, and helper-client services used by CLI metadata e2e |
| query-access-builtin-compose.yaml | Defines isolated MySQL 5.7, 8.0, 8.4, and TiDB 8.5 services for builtin semantic evidence |
| query-access-builtin-mysql-init.sql | Seeds the aggregate/window evidence table in each MySQL profile |
| query-access-builtin-tidb-init.sql | Seeds the aggregate/window evidence table through the TiDB fixture client |
| mysql/init.sql | Seeds MySQL with deterministic schemas/tables for inference, ambiguity, and compatibility scenarios |
| tidb/init.sql | Seeds TiDB with deterministic schemas/tables for inference, ambiguity, and compatibility scenarios |

## Exports

- Local Docker Compose assets only
- `docker compose -f docker/cli-e2e-compose.yaml up -d mysql`
- `docker compose -f docker/cli-e2e-compose.yaml up -d tidb mysql-client`
- `docker compose -f docker/query-access-builtin-compose.yaml up -d --wait mysql57 mysql80 mysql84 tidb85 tidb85-fixture`
- `docker compose -f docker/query-access-builtin-compose.yaml down -v --remove-orphans`

## Dependencies
- Upstream: local developers and `scripts/test_cli_metadata_e2e.sh`
- Downstream: Docker Engine, Docker Compose, MySQL image, PingCAP TiDB image

## Notes

- MySQL fixtures cover unique-schema inference, ambiguous-schema failures, compatibility checks, and partial create-table metadata behavior.
- TiDB fixtures cover unique-schema inference, ambiguous-schema failures, existence checks, and one instance-fact-backed sizing path.
- The builtin semantic matrix has independent containers, ports, and fixture initialization for each version profile; its evidence tests fail when a required service is unavailable.

## Update Rule
- If members/interfaces/dependencies change, update this file in same change.
