# Metadata Infrastructure Module

Infrastructure adapters for optional metadata-aware auditing.

## Files

| File | Responsibility |
|------|---------------|
| mysql/ | MySQL-protocol metadata providers for MySQL and TiDB |
| postgresql/ | PostgreSQL metadata provider over `database/sql` using pgx stdlib |

## Exports

- No package-level exports; concrete providers live in submodules.

## Dependencies
- Upstream: `internal/application/audit`
- Downstream: `internal/infrastructure/metadata/mysql`, `internal/infrastructure/metadata/postgresql`, `database/sql`, `github.com/go-sql-driver/mysql`, `github.com/jackc/pgx/v5/stdlib`

## Update Rule
- If members/interfaces/dependencies change, update this file in same change.
