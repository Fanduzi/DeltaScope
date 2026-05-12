//go:build postgresql

package audit

import "testing"

// PostgreSQL DDL Deep Coverage Census (v0.80.0 Task 1)
//
// Characterizes non-privilege PostgreSQL DDL families beyond v0.70.0 that are
// candidates for deep coverage. Each case is classified as one of:
//   - finding_covered:      audit already emits findings
//   - normalized_silent:    parser/extractor normalizes the form but no rule fires
//   - unsupported_boundary: parser succeeds but extractor marks the statement unsupported
//   - parser_error:         PG parser cannot parse the statement
//
// This file must not modify production code. It only observes current behavior.

var pgDDLDeepCoverageCensusCases = []struct {
	Name     string
	SQL      string
	Expected ddlCoverageClass
}{
	// ===== Composite type attribute operations =====
	// Parser handles ALTER TYPE variants; extractor marks them unsupported
	// with specific feature flags (alter_type_add_attribute, etc).

	{Name: "ALTER TYPE ADD ATTRIBUTE",
		SQL:      "ALTER TYPE address ADD ATTRIBUTE country text",
		Expected: ddlCoverageFindingCovered},
	{Name: "ALTER TYPE DROP ATTRIBUTE",
		SQL:      "ALTER TYPE address DROP ATTRIBUTE city",
		Expected: ddlCoverageFindingCovered},
	{Name: "ALTER TYPE ALTER ATTRIBUTE TYPE",
		SQL:      "ALTER TYPE address ALTER ATTRIBUTE street TYPE varchar(255)",
		Expected: ddlCoverageFindingCovered},
	{Name: "ALTER TYPE RENAME ATTRIBUTE",
		SQL:      "ALTER TYPE address RENAME ATTRIBUTE street TO line1",
		Expected: ddlCoverageFindingCovered},

	// ===== Extension member mutation =====
	// Parser handles ALTER EXTENSION ADD/DROP member; extractor marks unsupported.

	{Name: "ALTER EXTENSION ADD TABLE",
		SQL:      "ALTER EXTENSION pg_trgm ADD TABLE users",
		Expected: ddlCoverageUnsupportedBoundary},
	{Name: "ALTER EXTENSION DROP TABLE",
		SQL:      "ALTER EXTENSION pg_trgm DROP TABLE users",
		Expected: ddlCoverageUnsupportedBoundary},

	// ===== Publication lifecycle =====
	// Parser handles all publication DDL; extractor marks them unsupported.

	{Name: "CREATE PUBLICATION FOR ALL TABLES",
		SQL:      "CREATE PUBLICATION pub_all FOR ALL TABLES",
		Expected: ddlCoverageUnsupportedBoundary},
	{Name: "ALTER PUBLICATION ADD TABLE",
		SQL:      "ALTER PUBLICATION pub_all ADD TABLE users",
		Expected: ddlCoverageUnsupportedBoundary},
	{Name: "ALTER PUBLICATION DROP TABLE",
		SQL:      "ALTER PUBLICATION pub_all DROP TABLE users",
		Expected: ddlCoverageUnsupportedBoundary},
	{Name: "DROP PUBLICATION",
		SQL:      "DROP PUBLICATION pub_all",
		Expected: ddlCoverageUnsupportedBoundary},

	// ===== Subscription lifecycle =====
	// Parser handles most subscription DDL; extractor marks unsupported.
	// DROP SUBSCRIPTION ... WITH (drop_slot) is a genuine parser error.

	{Name: "CREATE SUBSCRIPTION",
		SQL:      "CREATE SUBSCRIPTION sub CONNECTION 'postgres://example' PUBLICATION pub_all",
		Expected: ddlCoverageUnsupportedBoundary},
	{Name: "ALTER SUBSCRIPTION DISABLE",
		SQL:      "ALTER SUBSCRIPTION sub DISABLE",
		Expected: ddlCoverageUnsupportedBoundary},
	{Name: "ALTER SUBSCRIPTION ENABLE",
		SQL:      "ALTER SUBSCRIPTION sub ENABLE",
		Expected: ddlCoverageUnsupportedBoundary},
	{Name: "DROP SUBSCRIPTION WITH drop_slot",
		SQL:      "DROP SUBSCRIPTION sub WITH (drop_slot = true)",
		Expected: ddlCoverageParserError},

	// ===== Foreign table lifecycle =====

	{Name: "CREATE FOREIGN TABLE",
		SQL:      "CREATE FOREIGN TABLE ft_users (id bigint) SERVER srv OPTIONS (table_name 'users')",
		Expected: ddlCoverageUnsupportedBoundary},
	{Name: "ALTER FOREIGN TABLE OPTIONS",
		SQL:      "ALTER FOREIGN TABLE ft_users OPTIONS (SET table_name 'users_v2')",
		Expected: ddlCoverageUnsupportedBoundary},
	{Name: "DROP FOREIGN TABLE",
		SQL:      "DROP FOREIGN TABLE ft_users",
		Expected: ddlCoverageUnsupportedBoundary},

	// ===== Foreign server lifecycle =====

	{Name: "CREATE SERVER",
		SQL:      "CREATE SERVER srv FOREIGN DATA WRAPPER postgres_fdw",
		Expected: ddlCoverageUnsupportedBoundary},
	{Name: "ALTER SERVER OPTIONS",
		SQL:      "ALTER SERVER srv OPTIONS (SET host 'db')",
		Expected: ddlCoverageUnsupportedBoundary},
	{Name: "DROP SERVER",
		SQL:      "DROP SERVER srv",
		Expected: ddlCoverageUnsupportedBoundary},

	// ===== User mapping lifecycle =====

	{Name: "CREATE USER MAPPING",
		SQL:      "CREATE USER MAPPING FOR app SERVER srv OPTIONS (user 'app')",
		Expected: ddlCoverageUnsupportedBoundary},
	{Name: "ALTER USER MAPPING",
		SQL:      "ALTER USER MAPPING FOR app SERVER srv OPTIONS (SET user 'app2')",
		Expected: ddlCoverageUnsupportedBoundary},
	{Name: "DROP USER MAPPING",
		SQL:      "DROP USER MAPPING FOR app SERVER srv",
		Expected: ddlCoverageUnsupportedBoundary},

	// ===== Foreign data wrapper lifecycle =====

	{Name: "CREATE FOREIGN DATA WRAPPER",
		SQL:      "CREATE FOREIGN DATA WRAPPER fdw HANDLER fdw_handler",
		Expected: ddlCoverageUnsupportedBoundary},
	{Name: "ALTER FOREIGN DATA WRAPPER OPTIONS",
		SQL:      "ALTER FOREIGN DATA WRAPPER fdw OPTIONS (SET key 'value')",
		Expected: ddlCoverageUnsupportedBoundary},
	{Name: "DROP FOREIGN DATA WRAPPER",
		SQL:      "DROP FOREIGN DATA WRAPPER fdw",
		Expected: ddlCoverageUnsupportedBoundary},

	// ===== Metadata annotation: COMMENT ON =====

	{Name: "COMMENT ON TABLE IS",
		SQL:      "COMMENT ON TABLE users IS 'user accounts'",
		Expected: ddlCoverageUnsupportedBoundary},
	{Name: "COMMENT ON TABLE IS NULL",
		SQL:      "COMMENT ON TABLE users IS NULL",
		Expected: ddlCoverageUnsupportedBoundary},

	// ===== Metadata annotation: SECURITY LABEL =====

	{Name: "SECURITY LABEL ON TABLE IS",
		SQL:      "SECURITY LABEL FOR selinux ON TABLE users IS 'system_u:object_r:sepgsql_table_t:s0'",
		Expected: ddlCoverageUnsupportedBoundary},
	{Name: "SECURITY LABEL ON TABLE IS NULL",
		SQL:      "SECURITY LABEL FOR selinux ON TABLE users IS NULL",
		Expected: ddlCoverageUnsupportedBoundary},

	// ===== Event trigger lifecycle =====

	{Name: "CREATE EVENT TRIGGER",
		SQL:      "CREATE EVENT TRIGGER trg_ddl ON ddl_command_end EXECUTE FUNCTION log_ddl()",
		Expected: ddlCoverageUnsupportedBoundary},
	{Name: "ALTER EVENT TRIGGER DISABLE",
		SQL:      "ALTER EVENT TRIGGER trg_ddl DISABLE",
		Expected: ddlCoverageUnsupportedBoundary},
	{Name: "ALTER EVENT TRIGGER ENABLE",
		SQL:      "ALTER EVENT TRIGGER trg_ddl ENABLE",
		Expected: ddlCoverageUnsupportedBoundary},
	{Name: "ALTER EVENT TRIGGER RENAME TO",
		SQL:      "ALTER EVENT TRIGGER trg_ddl RENAME TO trg_ddl_v2",
		Expected: ddlCoverageUnsupportedBoundary},
	{Name: "DROP EVENT TRIGGER",
		SQL:      "DROP EVENT TRIGGER trg_ddl",
		Expected: ddlCoverageUnsupportedBoundary},

	// ===== Rewrite rule lifecycle =====

	{Name: "CREATE RULE",
		SQL:      "CREATE RULE users_insert AS ON INSERT TO users DO NOTHING",
		Expected: ddlCoverageUnsupportedBoundary},
	{Name: "ALTER RULE RENAME TO",
		SQL:      "ALTER RULE users_insert ON users RENAME TO users_insert_ignore",
		Expected: ddlCoverageUnsupportedBoundary},
	{Name: "DROP RULE",
		SQL:      "DROP RULE users_insert_ignore ON users",
		Expected: ddlCoverageUnsupportedBoundary},
}

func TestPostgreSQLDDLDeepCoverageCensus(t *testing.T) {
	t.Parallel()
	runDDLCensus(t, "PostgreSQL", pgDDLDeepCoverageCensusCases)
}
