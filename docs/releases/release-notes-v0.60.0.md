# DeltaScope v0.60.0 Release Notes

## Summary

v0.60.0 adds PostgreSQL table-level privilege DCL narrow support. DeltaScope now normalizes `GRANT ... ON TABLE` and `REVOKE ... ON TABLE` through the audit pipeline, adding four PostgreSQL-only findings for offline migration review. Supports multiple privileges, multiple grantees, schema-qualified table names, `GRANT ALL PRIVILEGES`, and `REVOKE ... CASCADE`. DeltaScope does not perform live validation of any kind — no grantee/role existence checks, no table/object existence checks, no grantor permission checks, no effective privilege computation, no role inheritance resolution, no ownership verification, and no RLS/policy evaluation. This is narrow table-level privilege DCL support, not broad governance or admin DCL support.

## Normalized Forms

| SQL | Normalized Operation |
|-----|---------------------|
| `GRANT SELECT ON users TO reader` | `grant_table_privilege` |
| `GRANT SELECT, INSERT ON users TO reader, writer` | `grant_table_privilege` (privileges=[SELECT, INSERT], grantees=[reader, writer]) |
| `GRANT ALL PRIVILEGES ON users TO admin` | `grant_table_privilege` (all_privileges=true) |
| `GRANT SELECT ON public.users TO reader` | `grant_table_privilege` (schema=public) |
| `REVOKE SELECT ON users FROM reader` | `revoke_table_privilege` |
| `REVOKE INSERT, UPDATE ON users FROM writer, editor` | `revoke_table_privilege` (privileges=[INSERT, UPDATE], grantees=[writer, editor]) |
| `REVOKE ALL PRIVILEGES ON users FROM admin` | `revoke_table_privilege` (all_privileges=true) |
| `REVOKE SELECT ON users FROM reader CASCADE` | `revoke_table_privilege` (cascade=true) |

## New PostgreSQL-Only Rules

| Rule ID | Trigger | Default Level |
|---------|---------|---------------|
| `ddl.pg.grant.table_privilege.notice` | Any table-level `GRANT` | notice |
| `ddl.pg.grant.table_privilege.all.warn` | `GRANT ALL PRIVILEGES ON TABLE` | warning |
| `ddl.pg.revoke.table_privilege.notice` | Any table-level `REVOKE` | notice |
| `ddl.pg.revoke.table_privilege.cascade.warn` | `REVOKE ... ON TABLE ... CASCADE` | warning |

## Duplicate Findings

`GRANT ALL PRIVILEGES ON TABLE` triggers both `ddl.pg.grant.table_privilege.notice` and `ddl.pg.grant.table_privilege.all.warn`. `REVOKE ... ON TABLE ... CASCADE` triggers both `ddl.pg.revoke.table_privilege.notice` and `ddl.pg.revoke.table_privilege.cascade.warn`. These duplicate findings are intentional — each rule addresses a distinct concern (the operation itself vs. the over-permission / cascade side-effect risk).

## Explicit Unsupported/Deferred Boundaries

| SQL | Status |
|-----|--------|
| `GRANT ... ON ALL TABLES IN SCHEMA` | Not supported |
| Sequence privileges (`GRANT ... ON SEQUENCE`) | Not supported |
| Role membership (`GRANT role TO role`) | Not supported |
| `ALTER DEFAULT PRIVILEGES` | Not supported |

## Live Validation Boundary

DeltaScope does not perform live validation of any kind for table privileges:
- No grantee/role existence verification
- No table/object existence verification
- No grantor permission checks
- No effective privilege computation
- No role inheritance resolution
- No ownership verification
- No RLS/policy evaluation

## Test Coverage

- AST census tests documenting stable parser facts for all table privilege DCL forms.
- Parser/extractor normalization tests for all supported GRANT/REVOKE variants.
- Corpus fixtures covering all four new rules' trigger forms.
- Service-level tests through `AuditSQL` for representative table privilege DCL variants.
- Public surface tests across all four surfaces: `pkg/deltascope` Audit, CLI Execute, HTTP handler, and MCP audit_sql tool.

## Non-Goals

- DeltaScope does not perform live validation of any kind for table privileges.
- `ALL TABLES IN SCHEMA`, sequence privileges, role membership, and `ALTER DEFAULT PRIVILEGES` are not supported.
- This is narrow table-level privilege DCL support — not broad governance or admin DCL support.
- No MySQL/TiDB behavior changes.
- No default policy changes beyond the four new PostgreSQL-only rule entries.

## Install

**macOS (recommended):**

```bash
brew tap fanduzi/deltascope
brew install --cask deltascope
```

**Generic installer:**

```bash
curl -fsSL https://raw.githubusercontent.com/Fanduzi/DeltaScope/v0.60.0/install.sh | \
  DELTASCOPE_VERSION=v0.60.0 sh
```
