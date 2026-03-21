# Metadata Infrastructure Module

Infrastructure adapters for optional metadata-aware auditing.

## Files

| File | Responsibility |
|------|---------------|
| mysql/ | MySQL-protocol metadata providers for MySQL and TiDB |

## Exports

- No package-level exports; concrete providers live in submodules.

## Dependencies
- Upstream: `internal/application/audit`
- Downstream: `internal/infrastructure/metadata/mysql`, `database/sql`, driver packages

## Update Rule
- If members/interfaces/dependencies change, update this file in same change.
