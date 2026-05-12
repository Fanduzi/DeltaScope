# DeltaScope v0.70.0 Release Notes

## Summary

v0.70.0 extends PostgreSQL coverage to selected non-permission DDL lifecycle families: RLS/Policy, Trigger, Function/Procedure, Advanced View, and selected ALTER object lifecycle (schema, index, materialized view). 34 new PostgreSQL audit rules across 5 lifecycle families, SQL corpus expansion to 433/433 targets (100% coverage), and full public surface coverage across CLI, HTTP, MCP, and SDK. This milestone covers 31 selected PostgreSQL non-permission DDL forms — it does not claim full PostgreSQL DDL grammar coverage.

## Selected PostgreSQL DDL Lifecycle Coverage

v0.70.0 systematically extends DeltaScope's PostgreSQL offline audit to selected non-permission DDL families. The PostgreSQL DDL completion census now covers 31 forms classified as `finding_covered`, spanning RLS/Policy, Trigger, Function/Procedure, View, and selected ALTER object lifecycle operations. Many PG DDL families remain intentionally deferred (see Non-Goals).

## New Rules

| Rule ID | Dialects | Level | Triggers |
|---------|----------|:------:|----------|
| `ddl.pg.create_policy.notice` | PostgreSQL | notice | `CREATE POLICY` |
| `ddl.pg.alter_policy.notice` | PostgreSQL | notice | `ALTER POLICY` |
| `ddl.pg.drop_policy.warn` | PostgreSQL | warning | `DROP POLICY` |
| `ddl.pg.alter.enable_rls.notice` | PostgreSQL | notice | `ALTER TABLE ... ENABLE ROW LEVEL SECURITY` |
| `ddl.pg.alter.disable_rls.warn` | PostgreSQL | warning | `ALTER TABLE ... DISABLE ROW LEVEL SECURITY` |
| `ddl.pg.alter.force_rls.notice` | PostgreSQL | notice | `ALTER TABLE ... FORCE ROW LEVEL SECURITY` |
| `ddl.pg.alter.no_force_rls.notice` | PostgreSQL | notice | `ALTER TABLE ... NO FORCE ROW LEVEL SECURITY` |
| `ddl.pg.create_trigger.notice` | PostgreSQL | notice | `CREATE TRIGGER` |
| `ddl.pg.create_constraint_trigger.warn` | PostgreSQL | warning | `CREATE CONSTRAINT TRIGGER` |
| `ddl.pg.drop_trigger.advisory` | PostgreSQL | notice | `DROP TRIGGER` |
| `ddl.pg.create_function.notice` | PostgreSQL | notice | `CREATE FUNCTION` |
| `ddl.pg.create_function.security_definer.warn` | PostgreSQL | warning | `CREATE FUNCTION ... SECURITY DEFINER` |
| `ddl.pg.create_or_replace_function.advisory` | PostgreSQL | notice | `CREATE OR REPLACE FUNCTION` |
| `ddl.pg.drop_function.advisory` | PostgreSQL | notice | `DROP FUNCTION` |
| `ddl.pg.create_procedure.notice` | PostgreSQL | notice | `CREATE PROCEDURE` |
| `ddl.pg.drop_procedure.advisory` | PostgreSQL | notice | `DROP PROCEDURE` |
| `ddl.pg.create_or_replace_view.advisory` | PostgreSQL | notice | `CREATE OR REPLACE VIEW` |
| `ddl.pg.create_temp_view.notice` | PostgreSQL | notice | `CREATE TEMP VIEW` / `CREATE TEMPORARY VIEW` |
| `ddl.pg.create_view.check_option.notice` | PostgreSQL | notice | `CREATE VIEW ... WITH CHECK OPTION` |
| `ddl.pg.drop_view.cascade.warn` | PostgreSQL | warning | `DROP VIEW ... CASCADE` |
| `ddl.pg.alter_view.rename.notice` | PostgreSQL | notice | `ALTER VIEW ... RENAME TO` |
| `ddl.pg.alter_view.set_schema.notice` | PostgreSQL | notice | `ALTER VIEW ... SET SCHEMA` |
| `ddl.pg.alter_schema.rename.notice` | PostgreSQL | notice | `ALTER SCHEMA ... RENAME TO` |
| `ddl.pg.alter_schema.owner.notice` | PostgreSQL | notice | `ALTER SCHEMA ... OWNER TO` |
| `ddl.pg.alter_index.rename.notice` | PostgreSQL | notice | `ALTER INDEX ... RENAME TO` |
| `ddl.pg.alter_index.set_tablespace.notice` | PostgreSQL | notice | `ALTER INDEX ... SET TABLESPACE` |
| `ddl.pg.alter_materialized_view.rename.notice` | PostgreSQL | notice | `ALTER MATERIALIZED VIEW ... RENAME TO` |
| `ddl.pg.alter_materialized_view.set_schema.notice` | PostgreSQL | notice | `ALTER MATERIALIZED VIEW ... SET SCHEMA` |

### RLS / Policy Lifecycle

```bash
deltascope audit --dialect postgresql --sql "CREATE POLICY users_select ON users FOR SELECT USING (true)"
# → [notice] ddl.pg.create_policy.notice

deltascope audit --dialect postgresql --sql "ALTER TABLE users ENABLE ROW LEVEL SECURITY"
# → [notice] ddl.pg.alter.enable_rls.notice

deltascope audit --dialect postgresql --sql "DROP POLICY users_select ON users"
# → [warning] ddl.pg.drop_policy.warn
```

### Trigger Lifecycle

```bash
deltascope audit --dialect postgresql --sql "CREATE TRIGGER trg_audit AFTER INSERT ON users FOR EACH ROW EXECUTE FUNCTION log_change()"
# → [notice] ddl.pg.create_trigger.notice

deltascope audit --dialect postgresql --sql "DROP TRIGGER trg_audit ON users"
# → [notice] ddl.pg.drop_trigger.advisory
```

### Function / Procedure Lifecycle

```bash
deltascope audit --dialect postgresql --sql "CREATE FUNCTION add(a int, b int) RETURNS int LANGUAGE sql AS \$\$ SELECT a + b \$\$"
# → [notice] ddl.pg.create_function.notice

deltascope audit --dialect postgresql --sql "CREATE FUNCTION add(a int, b int) RETURNS int LANGUAGE sql SECURITY DEFINER AS \$\$ SELECT a + b \$\$"
# → [notice] ddl.pg.create_function.notice, [warning] ddl.pg.create_function.security_definer.warn

deltascope audit --dialect postgresql --sql "DROP FUNCTION add(int, int)"
# → [notice] ddl.pg.drop_function.advisory
```

### Advanced View Lifecycle

```bash
deltascope audit --dialect postgresql --sql "CREATE OR REPLACE VIEW v_users AS SELECT id FROM users"
# → [notice] ddl.pg.create_or_replace_view.advisory

deltascope audit --dialect postgresql --sql "DROP VIEW v_users CASCADE"
# → [warning] ddl.pg.drop_view.cascade.warn
```

### Selected ALTER Object Lifecycle

```bash
deltascope audit --dialect postgresql --sql "ALTER SCHEMA app RENAME TO app_new"
# → [notice] ddl.pg.alter_schema.rename.notice

deltascope audit --dialect postgresql --sql "ALTER INDEX idx_old RENAME TO idx_new"
# → [notice] ddl.pg.alter_index.rename.notice

deltascope audit --dialect postgresql --sql "ALTER MATERIALIZED VIEW mv_stats SET SCHEMA archive"
# → [notice] ddl.pg.alter_materialized_view.set_schema.notice
```

## Normalization

| Statement | Normalized Operation | Object Type |
|-----------|---------------------|-------------|
| `ALTER SCHEMA app RENAME TO app_new` | `alter_schema` | schema |
| `ALTER SCHEMA app OWNER TO owner` | `alter_schema` | schema |
| `ALTER INDEX idx RENAME TO idx_new` | `alter_index` | index |
| `ALTER INDEX idx SET TABLESPACE ts` | `alter_index` | index |
| `ALTER MATERIALIZED VIEW mv RENAME TO mv_new` | `alter_materialized_view` | materialized_view |
| `ALTER MATERIALIZED VIEW mv SET SCHEMA s` | `alter_materialized_view` | materialized_view |

## Quality

- SQL corpus: 242 policy rules, 433/433 supported targets, 100% coverage
- PostgreSQL DDL completion census: 31/31 selected non-permission DDL forms `finding_covered`
- Public surface coverage verified across SDK, CLI, HTTP, and MCP
- AST characterization tests for all new lifecycle forms
- Default policy dialect isolation maintained

## Non-Goals

- No full PostgreSQL DDL support claim. Selected non-permission DDL families are covered; many remain deferred.
- No live privilege/role validation. GRANT/REVOKE rules emit informational notices only.
- No DCL execution or runtime database firewall behavior.
- DeltaScope does not execute migrations.
- Deferred PG DDL families: `ALTER TYPE` deep mutation, `ALTER DOMAIN` beyond rename, `ALTER EXTENSION` expanded lifecycle, `ALTER FOREIGN TABLE`/`SERVER`/`USER MAPPING`, `ALTER AGGREGATE`/`OPERATOR`/`CONVERSION`, `ALTER PUBLICATION`/`SUBSCRIPTION`, `ALTER STATISTICS`/`ALTER RULE`, `COMMENT ON`, `SECURITY LABEL`.
- Permissions/DCL remain out of scope for runtime validation.

## Install

**macOS (recommended):**

```bash
brew tap fanduzi/deltascope
brew install --cask deltascope
```

**Generic installer:**

```bash
curl -fsSL https://raw.githubusercontent.com/Fanduzi/DeltaScope/v0.70.0/install.sh | \
  DELTASCOPE_VERSION=v0.70.0 sh
```

## Upgrade

If you previously installed v0.64.0:

```bash
# Homebrew
brew upgrade --cask deltascope

# Generic installer (re-run with new version)
curl -fsSL https://raw.githubusercontent.com/Fanduzi/DeltaScope/v0.70.0/install.sh | \
  DELTASCOPE_VERSION=v0.70.0 sh
```

## Verification

```bash
deltascope --version
# Should output v0.70.0

deltascope audit --dialect postgresql --sql "CREATE POLICY users_select ON users FOR SELECT USING (true)"
# Should produce ddl.pg.create_policy.notice finding

deltascope audit --dialect postgresql --sql "ALTER SCHEMA app RENAME TO app_new"
# Should produce ddl.pg.alter_schema.rename.notice finding
```
