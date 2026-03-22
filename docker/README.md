# Docker Module

Container assets for DeltaScope local end-to-end environments.

## Files

| File | Responsibility |
|------|---------------|
| cli-e2e-compose.yaml | Defines the local MySQL, TiDB, and helper-client services used by CLI metadata e2e |
| mysql/init.sql | Seeds MySQL with deterministic schemas/tables for inference, ambiguity, and compatibility scenarios |
| tidb/init.sql | Seeds TiDB with deterministic schemas/tables for inference, ambiguity, and compatibility scenarios |

## Exports

- Local Docker Compose assets only
- `docker compose -f docker/cli-e2e-compose.yaml up -d mysql`
- `docker compose -f docker/cli-e2e-compose.yaml up -d tidb mysql-client`

## Dependencies
- Upstream: local developers and `scripts/test_cli_metadata_e2e.sh`
- Downstream: Docker Engine, Docker Compose, MySQL image, PingCAP TiDB image

## Notes

- MySQL fixtures cover unique-schema inference, ambiguous-schema failures, compatibility checks, and partial create-table metadata behavior.
- TiDB fixtures cover unique-schema inference, ambiguous-schema failures, existence checks, and one instance-fact-backed sizing path.

## Update Rule
- If members/interfaces/dependencies change, update this file in same change.
