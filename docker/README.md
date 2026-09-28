# Docker Module

Container assets for DeltaScope local end-to-end environments.

## Files

| File | Responsibility |
|------|---------------|
| cli-e2e-compose.yaml | Defines the local MySQL, TiDB, and helper-client services used by CLI metadata e2e |
| query-access-builtin-compose.yaml | Defines isolated MySQL 5.7, 8.0, 8.4, and TiDB 8.5 services for builtin semantic evidence |
| query-access-builtin-mysql-init.sql | Seeds the aggregate/window evidence table in each MySQL profile |
| query-access-builtin-tidb-init.sql | Seeds the aggregate/window evidence table through the TiDB fixture client |
| ddl-golden-compose.yaml | Defines pinned golden-path anchors MySQL 5.7.44, 8.0.46, 8.4.10, and TiDB 8.5.0 (not 8.5.7) plus a TiDB fixture client; mysql57 runs under `platform: linux/amd64` because `mysql:5.7.44` ships amd64 only; all four anchors publish loopback host ports (`127.0.0.1:23357/23380/23384/24000`) so the host-built CLI can run live metadata and observed-version audit cases against them (#83 T04-A/T04-B). Two task-scoped auxiliary fixtures — `mysql84-4k` (`--innodb-page-size=4096`, port 23394) and `tidb85-12288` (`max-index-length=12288` via `tidb/ddl-golden-max-index-length.toml`, port 24001) — prove instance-fact-dependent rule bounds without touching the four pinned anchors |
| tidb/ddl-golden-max-index-length.toml | Auxiliary TiDB config for the `tidb85-12288` golden fixture: sets `max-index-length = 12288` so live `SHOW CONFIG` evidence exercises the configurable index-key bound (#83 T04-B-R1) |
| mysql/init.sql | Seeds MySQL with deterministic schemas/tables for inference, ambiguity, and compatibility scenarios |
| tidb/init.sql | Seeds TiDB with deterministic schemas/tables for inference, ambiguity, and compatibility scenarios |

## Exports

- Local Docker Compose assets only
- `docker compose -f docker/cli-e2e-compose.yaml up -d mysql`
- `docker compose -f docker/cli-e2e-compose.yaml up -d tidb mysql-client`
- `docker compose -f docker/query-access-builtin-compose.yaml up -d --wait mysql57 mysql80 mysql84 tidb85 tidb85-fixture`
- `docker compose -f docker/query-access-builtin-compose.yaml down -v --remove-orphans`
- `docker compose -f docker/ddl-golden-compose.yaml up -d --wait` (managed by `scripts/ddl_golden.py` via `make ddl-golden`)

## Dependencies
- Upstream: local developers and `scripts/test_cli_metadata_e2e.sh`
- Downstream: Docker Engine, Docker Compose, MySQL image, PingCAP TiDB image

## Notes

- MySQL fixtures cover unique-schema inference, ambiguous-schema failures, compatibility checks, and partial create-table metadata behavior.
- TiDB fixtures cover unique-schema inference, ambiguous-schema failures, existence checks, and one instance-fact-backed sizing path.
- The builtin semantic matrix has independent containers, ports, and fixture initialization for each version profile; its evidence tests fail when a required service is unavailable.
- The golden compose intentionally differs from the builtin matrix only in the TiDB tag: DDL golden proof requires TiDB `v8.5.0` exactly, while the builtin matrix pins `v8.5.7`. Anchor access runs through `docker exec`/the compose network, and `ddl_golden.py` owns lifecycle plus deterministic `down -v` cleanup; the loopback host ports exist only for host-CLI metadata audit cases.

## Update Rule
- If members/interfaces/dependencies change, update this file in same change.
