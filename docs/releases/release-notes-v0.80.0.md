# DeltaScope v0.80.0 Release Notes

## Summary

v0.80.0 extends selected PostgreSQL non-permission DDL deep coverage with 36 new PostgreSQL-only rules across 6 families: composite type attributes, extension members, publication/subscription, foreign objects, annotation lifecycle, and event trigger/rewrite rule. SQL corpus expansion to 469/469 targets (100% coverage). Full public surface coverage across SDK, CLI, HTTP, and MCP. The PostgreSQL DDL completion census now covers 38 selected non-permission DDL forms classified as `finding_covered`. This milestone does not claim full PostgreSQL DDL grammar coverage.

## Selected PostgreSQL DDL Deep Coverage

v0.80.0 systematically extends DeltaScope's PostgreSQL offline audit to 6 additional non-permission DDL families. The PostgreSQL DDL completion census now covers 38 forms classified as `finding_covered`, spanning composite type attributes, extension members, publication/subscription, foreign objects, annotation lifecycle, and event trigger/rewrite rule operations. Many PG DDL families remain intentionally deferred (see Non-Goals).

## New Rules

| Rule ID | Dialects | Level | Triggers |
|---------|----------|:------:|----------|
| `ddl.pg.alter_type.add_attribute.notice` | PostgreSQL | notice | `ALTER TYPE ... ADD ATTRIBUTE` |
| `ddl.pg.alter_type.drop_attribute.warn` | PostgreSQL | warning | `ALTER TYPE ... DROP ATTRIBUTE` |
| `ddl.pg.alter_type.alter_attribute_type.warn` | PostgreSQL | warning | `ALTER TYPE ... ALTER ATTRIBUTE ... TYPE` |
| `ddl.pg.alter_type.rename_attribute.notice` | PostgreSQL | notice | `ALTER TYPE ... RENAME ATTRIBUTE` |
| `ddl.pg.alter_extension.add_member.notice` | PostgreSQL | notice | `ALTER EXTENSION ... ADD TABLE` |
| `ddl.pg.alter_extension.drop_member.warn` | PostgreSQL | warning | `ALTER EXTENSION ... DROP TABLE` |
| `ddl.pg.create_publication.notice` | PostgreSQL | notice | `CREATE PUBLICATION` |
| `ddl.pg.alter_publication.notice` | PostgreSQL | notice | `ALTER PUBLICATION` |
| `ddl.pg.drop_publication.warn` | PostgreSQL | warning | `DROP PUBLICATION` |
| `ddl.pg.create_subscription.notice` | PostgreSQL | notice | `CREATE SUBSCRIPTION` |
| `ddl.pg.alter_subscription.notice` | PostgreSQL | notice | `ALTER SUBSCRIPTION` |
| `ddl.pg.alter_subscription.disable.warn` | PostgreSQL | warning | `ALTER SUBSCRIPTION ... DISABLE` |
| `ddl.pg.drop_subscription.warn` | PostgreSQL | warning | `DROP SUBSCRIPTION` |
| `ddl.pg.create_foreign_table.notice` | PostgreSQL | notice | `CREATE FOREIGN TABLE` |
| `ddl.pg.alter_foreign_table.notice` | PostgreSQL | notice | `ALTER FOREIGN TABLE` |
| `ddl.pg.drop_foreign_table.warn` | PostgreSQL | warning | `DROP FOREIGN TABLE` |
| `ddl.pg.create_foreign_server.notice` | PostgreSQL | notice | `CREATE SERVER` |
| `ddl.pg.alter_foreign_server.notice` | PostgreSQL | notice | `ALTER SERVER` |
| `ddl.pg.drop_foreign_server.warn` | PostgreSQL | warning | `DROP SERVER` |
| `ddl.pg.create_user_mapping.notice` | PostgreSQL | notice | `CREATE USER MAPPING` |
| `ddl.pg.alter_user_mapping.notice` | PostgreSQL | notice | `ALTER USER MAPPING` |
| `ddl.pg.drop_user_mapping.warn` | PostgreSQL | warning | `DROP USER MAPPING` |
| `ddl.pg.create_foreign_data_wrapper.notice` | PostgreSQL | notice | `CREATE FOREIGN DATA WRAPPER` |
| `ddl.pg.alter_foreign_data_wrapper.notice` | PostgreSQL | notice | `ALTER FOREIGN DATA WRAPPER` |
| `ddl.pg.drop_foreign_data_wrapper.warn` | PostgreSQL | warning | `DROP FOREIGN DATA WRAPPER` |
| `ddl.pg.comment_on.notice` | PostgreSQL | notice | `COMMENT ON` |
| `ddl.pg.comment_on.remove.notice` | PostgreSQL | notice | `COMMENT ON ... IS NULL` |
| `ddl.pg.security_label.notice` | PostgreSQL | notice | `SECURITY LABEL` |
| `ddl.pg.security_label.remove.notice` | PostgreSQL | notice | `SECURITY LABEL ... IS NULL` |
| `ddl.pg.create_event_trigger.notice` | PostgreSQL | notice | `CREATE EVENT TRIGGER` |
| `ddl.pg.alter_event_trigger.notice` | PostgreSQL | notice | `ALTER EVENT TRIGGER` |
| `ddl.pg.alter_event_trigger.disable.warn` | PostgreSQL | warning | `ALTER EVENT TRIGGER ... DISABLE` |
| `ddl.pg.drop_event_trigger.warn` | PostgreSQL | warning | `DROP EVENT TRIGGER` |
| `ddl.pg.create_rule.notice` | PostgreSQL | notice | `CREATE RULE` |
| `ddl.pg.alter_rule.notice` | PostgreSQL | notice | `ALTER RULE` |
| `ddl.pg.drop_rule.warn` | PostgreSQL | warning | `DROP RULE` |

### Composite Type Attribute Lifecycle

```bash
deltascope audit --dialect postgresql --sql "ALTER TYPE address ADD ATTRIBUTE zip_code text"
# → [notice] ddl.pg.alter_type.add_attribute.notice

deltascope audit --dialect postgresql --sql "ALTER TYPE address DROP ATTRIBUTE zip_code"
# → [warning] ddl.pg.alter_type.drop_attribute.warn

deltascope audit --dialect postgresql --sql "ALTER TYPE address ALTER ATTRIBUTE zip_code TYPE varchar(10)"
# → [warning] ddl.pg.alter_type.alter_attribute_type.warn

deltascope audit --dialect postgresql --sql "ALTER TYPE address RENAME ATTRIBUTE zip_code TO postal_code"
# → [notice] ddl.pg.alter_type.rename_attribute.notice
```

### Extension Member Lifecycle

```bash
deltascope audit --dialect postgresql --sql "ALTER EXTENSION pg_trgm ADD TABLE trgm_test"
# → [notice] ddl.pg.alter_extension.add_member.notice

deltascope audit --dialect postgresql --sql "ALTER EXTENSION pg_trgm DROP TABLE trgm_test"
# → [warning] ddl.pg.alter_extension.drop_member.warn
```

### Publication / Subscription Lifecycle

```bash
deltascope audit --dialect postgresql --sql "CREATE PUBLICATION pub_users FOR TABLE users"
# → [notice] ddl.pg.create_publication.notice

deltascope audit --dialect postgresql --sql "ALTER PUBLICATION pub_users ADD TABLE orders"
# → [notice] ddl.pg.alter_publication.notice

deltascope audit --dialect postgresql --sql "DROP PUBLICATION pub_users"
# → [warning] ddl.pg.drop_publication.warn

deltascope audit --dialect postgresql --sql "CREATE SUBSCRIPTION sub_users CONNECTION 'host=pg port=5432 dbname=mydb' PUBLICATION pub_users"
# → [notice] ddl.pg.create_subscription.notice

deltascope audit --dialect postgresql --sql "ALTER SUBSCRIPTION sub_users DISABLE"
# → [warning] ddl.pg.alter_subscription.disable.warn

deltascope audit --dialect postgresql --sql "DROP SUBSCRIPTION sub_users"
# → [warning] ddl.pg.drop_subscription.warn
```

### Foreign Object Lifecycle

```bash
deltascope audit --dialect postgresql --sql "CREATE FOREIGN TABLE remote_users (id int, name text) SERVER pg_remote"
# → [notice] ddl.pg.create_foreign_table.notice

deltascope audit --dialect postgresql --sql "DROP FOREIGN TABLE remote_users"
# → [warning] ddl.pg.drop_foreign_table.warn

deltascope audit --dialect postgresql --sql "CREATE SERVER pg_remote FOREIGN DATA WRAPPER postgres_fdw OPTIONS (host 'remote-host')"
# → [notice] ddl.pg.create_foreign_server.notice

deltascope audit --dialect postgresql --sql "DROP SERVER pg_remote"
# → [warning] ddl.pg.drop_foreign_server.warn

deltascope audit --dialect postgresql --sql "CREATE USER MAPPING FOR current_user SERVER pg_remote OPTIONS (user 'remote_user')"
# → [notice] ddl.pg.create_user_mapping.notice

deltascope audit --dialect postgresql --sql "DROP USER MAPPING FOR current_user SERVER pg_remote"
# → [warning] ddl.pg.drop_user_mapping.warn

deltascope audit --dialect postgresql --sql "CREATE FOREIGN DATA WRAPPER pg_fdw"
# → [notice] ddl.pg.create_foreign_data_wrapper.notice

deltascope audit --dialect postgresql --sql "DROP FOREIGN DATA WRAPPER pg_fdw"
# → [warning] ddl.pg.drop_foreign_data_wrapper.warn
```

### Annotation Lifecycle

```bash
deltascope audit --dialect postgresql --sql "COMMENT ON TABLE users IS 'User accounts table'"
# → [notice] ddl.pg.comment_on.notice

deltascope audit --dialect postgresql --sql "COMMENT ON TABLE users IS NULL"
# → [notice] ddl.pg.comment_on.remove.notice

deltascope audit --dialect postgresql --sql "SECURITY LABEL FOR selinux ON TABLE users IS 'system_u:object_r:sepgsql_table_t:s0'"
# → [notice] ddl.pg.security_label.notice

deltascope audit --dialect postgresql --sql "SECURITY LABEL FOR selinux ON TABLE users IS NULL"
# → [notice] ddl.pg.security_label.remove.notice
```

### Event Trigger / Rewrite Rule Lifecycle

```bash
deltascope audit --dialect postgresql --sql "CREATE EVENT TRIGGER trg_ddl ON ddl_command_start EXECUTE FUNCTION log_ddl()"
# → [notice] ddl.pg.create_event_trigger.notice

deltascope audit --dialect postgresql --sql "ALTER EVENT TRIGGER trg_ddl DISABLE"
# → [warning] ddl.pg.alter_event_trigger.disable.warn

deltascope audit --dialect postgresql --sql "DROP EVENT TRIGGER trg_ddl"
# → [warning] ddl.pg.drop_event_trigger.warn

deltascope audit --dialect postgresql --sql "CREATE RULE notify_insert AS ON INSERT TO users DO ALSO NOTIFY users_changed"
# → [notice] ddl.pg.create_rule.notice

deltascope audit --dialect postgresql --sql "DROP RULE notify_insert ON users"
# → [warning] ddl.pg.drop_rule.warn
```

## Normalization

| Statement | Normalized Operation | Object Type |
|-----------|---------------------|-------------|
| `ALTER TYPE address ADD ATTRIBUTE zip_code text` | `alter_type` | composite_type |
| `ALTER TYPE address DROP ATTRIBUTE zip_code` | `alter_type` | composite_type |
| `ALTER TYPE address ALTER ATTRIBUTE zip_code TYPE varchar(10)` | `alter_type` | composite_type |
| `ALTER TYPE address RENAME ATTRIBUTE zip_code TO postal_code` | `alter_type` | composite_type |
| `ALTER EXTENSION pg_trgm ADD TABLE trgm_test` | `alter_extension` | extension |
| `ALTER EXTENSION pg_trgm DROP TABLE trgm_test` | `alter_extension` | extension |
| `CREATE PUBLICATION pub_users FOR TABLE users` | `create_publication` | publication |
| `ALTER PUBLICATION pub_users ADD TABLE orders` | `alter_publication` | publication |
| `DROP PUBLICATION pub_users` | `drop_publication` | publication |
| `CREATE SUBSCRIPTION sub_users CONNECTION ... PUBLICATION pub_users` | `create_subscription` | subscription |
| `ALTER SUBSCRIPTION sub_users DISABLE` | `alter_subscription` | subscription |
| `DROP SUBSCRIPTION sub_users` | `drop_subscription` | subscription |
| `CREATE FOREIGN TABLE remote_users (...) SERVER pg_remote` | `create_foreign_table` | foreign_table |
| `ALTER FOREIGN TABLE remote_users ADD COLUMN email text` | `alter_foreign_table` | foreign_table |
| `DROP FOREIGN TABLE remote_users` | `drop_foreign_table` | foreign_table |
| `CREATE SERVER pg_remote FOREIGN DATA WRAPPER postgres_fdw` | `create_foreign_server` | foreign_server |
| `ALTER SERVER pg_remote OPTIONS (SET host 'new-host')` | `alter_foreign_server` | foreign_server |
| `DROP SERVER pg_remote` | `drop_foreign_server` | foreign_server |
| `CREATE USER MAPPING FOR current_user SERVER pg_remote` | `create_user_mapping` | user_mapping |
| `ALTER USER MAPPING FOR current_user SERVER pg_remote OPTIONS (SET user 'new_user')` | `alter_user_mapping` | user_mapping |
| `DROP USER MAPPING FOR current_user SERVER pg_remote` | `drop_user_mapping` | user_mapping |
| `CREATE FOREIGN DATA WRAPPER pg_fdw` | `create_foreign_data_wrapper` | foreign_data_wrapper |
| `ALTER FOREIGN DATA WRAPPER pg_fdw OPTIONS (ADD debug 'true')` | `alter_foreign_data_wrapper` | foreign_data_wrapper |
| `DROP FOREIGN DATA WRAPPER pg_fdw` | `drop_foreign_data_wrapper` | foreign_data_wrapper |
| `COMMENT ON TABLE users IS 'description'` | `comment_on` | - |
| `COMMENT ON TABLE users IS NULL` | `comment_on` | - |
| `SECURITY LABEL FOR selinux ON TABLE users IS 'label'` | `security_label` | - |
| `SECURITY LABEL FOR selinux ON TABLE users IS NULL` | `security_label` | - |
| `CREATE EVENT TRIGGER trg_ddl ON ddl_command_start EXECUTE FUNCTION log_ddl()` | `create_event_trigger` | event_trigger |
| `ALTER EVENT TRIGGER trg_ddl DISABLE` | `alter_event_trigger` | event_trigger |
| `DROP EVENT TRIGGER trg_ddl` | `drop_event_trigger` | event_trigger |
| `CREATE RULE notify_insert AS ON INSERT TO users DO ALSO NOTIFY users_changed` | `create_rule` | rule |
| `ALTER RULE notify_insert ON users RENAME TO notify_insert_v2` | `alter_rule` | rule |
| `DROP RULE notify_insert ON users` | `drop_rule` | rule |

## Quality

- SQL corpus: 278 policy rules, 469/469 supported targets, 100% coverage
- PostgreSQL DDL deep coverage census: 38/38 selected non-permission DDL forms `finding_covered`, 1 `parser_error`
- Public surface coverage verified across SDK, CLI, HTTP, and MCP for all 36 new rules
- Default policy dialect isolation maintained

## Privacy Boundary

Expected corpus outputs do not contain:
- Subscription connection strings (`CONNECTION '...'` values)
- Foreign object option values (`OPTIONS (...)` contents)
- Comment text (`IS '...'` values)
- Security label text (`IS '...'` values)
- Event trigger function bodies (`EXECUTE FUNCTION ...` bodies)
- Rewrite rule bodies (`DO ...` clause contents)

Rules emit structural lifecycle facts (operation, object type, object name) without preserving or exposing sensitive payload values.

## Non-Goals

- No full PostgreSQL DDL support claim. Selected non-permission DDL families are covered; many remain deferred.
- No live privilege/role validation. GRANT/REVOKE rules emit informational notices only.
- No DCL execution or runtime database firewall behavior.
- DeltaScope does not execute migrations.
- Deferred: `DROP SUBSCRIPTION ... WITH (drop_slot = true)` remains a genuine parser error.
- Deferred PG DDL families: broader PostgreSQL grammar, aggregate/operator/conversion/statistics lifecycle, and other out-of-scope families remain intentionally not covered.
- Permissions/DCL remain out of scope for runtime validation: GRANT/REVOKE, roles/users, default privileges (except pre-existing v0.60.0 table-level DCL support).

## Install

**macOS (recommended):**

```bash
brew tap fanduzi/deltascope
brew install --cask deltascope
```

**Generic installer:**

```bash
curl -fsSL https://raw.githubusercontent.com/Fanduzi/DeltaScope/v0.80.0/install.sh | \
  DELTASCOPE_VERSION=v0.80.0 sh
```

## Upgrade

If you previously installed v0.70.0:

```bash
# Homebrew
brew upgrade --cask deltascope

# Generic installer (re-run with new version)
curl -fsSL https://raw.githubusercontent.com/Fanduzi/DeltaScope/v0.80.0/install.sh | \
  DELTASCOPE_VERSION=v0.80.0 sh
```

## Verification

```bash
deltascope --version
# Should output v0.80.0

deltascope audit --dialect postgresql --sql "CREATE PUBLICATION pub_users FOR TABLE users"
# Should produce ddl.pg.create_publication.notice finding

deltascope audit --dialect postgresql --sql "CREATE FOREIGN TABLE remote_users (id int) SERVER pg_remote"
# Should produce ddl.pg.create_foreign_table.notice finding

deltascope audit --dialect postgresql --sql "COMMENT ON TABLE users IS 'description'"
# Should produce ddl.pg.comment_on.notice finding

deltascope audit --dialect postgresql --sql "CREATE EVENT TRIGGER trg_ddl ON ddl_command_start EXECUTE FUNCTION log_ddl()"
# Should produce ddl.pg.create_event_trigger.notice finding
```
