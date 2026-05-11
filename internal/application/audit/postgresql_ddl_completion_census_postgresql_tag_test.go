//go:build postgresql

package audit

import "testing"

// PostgreSQL DDL Completion Census 2.0 (v0.70.0 Task 1)
//
// Characterizes non-privilege PostgreSQL DDL families that are candidates
// for coverage in v0.70.0. Each case is classified as one of:
//   - finding_covered:      audit already emits findings
//   - normalized_silent:    parser/extractor normalizes the form but no rule fires
//   - unsupported_boundary: parser succeeds but extractor marks the statement unsupported
//   - parser_error:         PG parser cannot parse the statement
//
// This file must not modify production code. It only observes current behavior.

var pgDDLCompletionCensusCases = []struct {
	Name     string
	SQL      string
	Expected ddlCoverageClass
}{
	// ===== Trigger lifecycle =====

	{Name: "CREATE TRIGGER",
		SQL:      "CREATE TRIGGER trg_users_ai AFTER INSERT ON users FOR EACH ROW EXECUTE FUNCTION log_change()",
		Expected: ddlCoverageUnsupportedBoundary},
	{Name: "CREATE CONSTRAINT TRIGGER",
		SQL:      "CREATE CONSTRAINT TRIGGER trg_chk AFTER INSERT ON users DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION check_fn()",
		Expected: ddlCoverageUnsupportedBoundary},
	{Name: "DROP TRIGGER",
		SQL:      "DROP TRIGGER trg_users_ai ON users",
		Expected: ddlCoverageUnsupportedBoundary},
	{Name: "DROP TRIGGER IF EXISTS",
		SQL:      "DROP TRIGGER IF EXISTS trg_users_ai ON users",
		Expected: ddlCoverageUnsupportedBoundary},

	// ===== Function/Procedure lifecycle =====

	{Name: "CREATE FUNCTION",
		SQL:      "CREATE FUNCTION add(a int, b int) RETURNS int LANGUAGE sql AS $$ SELECT a + b $$",
		Expected: ddlCoverageUnsupportedBoundary},
	{Name: "CREATE OR REPLACE FUNCTION",
		SQL:      "CREATE OR REPLACE FUNCTION add(a int, b int) RETURNS int LANGUAGE sql AS $$ SELECT a + b $$",
		Expected: ddlCoverageUnsupportedBoundary},
	{Name: "CREATE FUNCTION SECURITY DEFINER",
		SQL:      "CREATE FUNCTION add(a int, b int) RETURNS int LANGUAGE sql SECURITY DEFINER AS $$ SELECT a + b $$",
		Expected: ddlCoverageUnsupportedBoundary},
	{Name: "DROP FUNCTION",
		SQL:      "DROP FUNCTION add(int, int)",
		Expected: ddlCoverageUnsupportedBoundary},
	{Name: "DROP FUNCTION IF EXISTS",
		SQL:      "DROP FUNCTION IF EXISTS add(int, int)",
		Expected: ddlCoverageUnsupportedBoundary},
	{Name: "CREATE PROCEDURE",
		SQL:      "CREATE PROCEDURE insert_user(name text) LANGUAGE sql AS $$ INSERT INTO users(name) VALUES(name) $$",
		Expected: ddlCoverageUnsupportedBoundary},
	{Name: "DROP PROCEDURE",
		SQL:      "DROP PROCEDURE insert_user(text)",
		Expected: ddlCoverageUnsupportedBoundary},

	// ===== Advanced view lifecycle =====

	{Name: "CREATE OR REPLACE VIEW",
		SQL:      "CREATE OR REPLACE VIEW v_users AS SELECT id FROM users",
		Expected: ddlCoverageUnsupportedBoundary},
	{Name: "CREATE TEMP VIEW",
		SQL:      "CREATE TEMP VIEW v_temp AS SELECT id FROM users",
		Expected: ddlCoverageUnsupportedBoundary},
	{Name: "CREATE VIEW WITH CHECK OPTION",
		SQL:      "CREATE VIEW v_users AS SELECT id FROM users WITH CHECK OPTION",
		Expected: ddlCoverageUnsupportedBoundary},
	{Name: "CREATE VIEW WITH LOCAL CHECK OPTION",
		SQL:      "CREATE VIEW v_users AS SELECT id FROM users WITH LOCAL CHECK OPTION",
		Expected: ddlCoverageUnsupportedBoundary},
	{Name: "DROP VIEW CASCADE",
		SQL:      "DROP VIEW v_users CASCADE",
		Expected: ddlCoverageFindingCovered},
	{Name: "ALTER VIEW RENAME TO",
		SQL:      "ALTER VIEW v_users RENAME TO v_people",
		Expected: ddlCoverageUnsupportedBoundary},
	{Name: "ALTER VIEW SET SCHEMA",
		SQL:      "ALTER VIEW v_users SET SCHEMA app",
		Expected: ddlCoverageFindingCovered},

	// ===== RLS / Policy lifecycle =====

	{Name: "CREATE POLICY",
		SQL:      "CREATE POLICY users_policy ON users AS PERMISSIVE FOR SELECT TO PUBLIC USING (true)",
		Expected: ddlCoverageUnsupportedBoundary},
	{Name: "ALTER POLICY",
		SQL:      "ALTER POLICY users_policy ON users USING (user_id = current_user)",
		Expected: ddlCoverageUnsupportedBoundary},
	{Name: "DROP POLICY",
		SQL:      "DROP POLICY users_policy ON users",
		Expected: ddlCoverageUnsupportedBoundary},
	{Name: "ALTER TABLE ENABLE RLS",
		SQL:      "ALTER TABLE users ENABLE ROW LEVEL SECURITY",
		Expected: ddlCoverageUnsupportedBoundary},
	{Name: "ALTER TABLE DISABLE RLS",
		SQL:      "ALTER TABLE users DISABLE ROW LEVEL SECURITY",
		Expected: ddlCoverageUnsupportedBoundary},
	{Name: "ALTER TABLE FORCE RLS",
		SQL:      "ALTER TABLE users FORCE ROW LEVEL SECURITY",
		Expected: ddlCoverageUnsupportedBoundary},
	{Name: "ALTER TABLE NO FORCE RLS",
		SQL:      "ALTER TABLE users NO FORCE ROW LEVEL SECURITY",
		Expected: ddlCoverageUnsupportedBoundary},

	// ===== Selected ALTER object lifecycle =====

	{Name: "ALTER SCHEMA RENAME TO",
		SQL:      "ALTER SCHEMA app RENAME TO app_new",
		Expected: ddlCoverageUnsupportedBoundary},
	{Name: "ALTER SCHEMA OWNER TO",
		SQL:      "ALTER SCHEMA app OWNER TO app_owner",
		Expected: ddlCoverageUnsupportedBoundary},
	{Name: "ALTER INDEX RENAME TO",
		SQL:      "ALTER INDEX idx_users_email RENAME TO idx_users_email_v2",
		Expected: ddlCoverageFindingCovered},
	{Name: "ALTER INDEX SET TABLESPACE",
		SQL:      "ALTER INDEX idx_users_email SET TABLESPACE pg_default",
		Expected: ddlCoverageUnsupportedBoundary},
	{Name: "ALTER MATERIALIZED VIEW RENAME TO",
		SQL:      "ALTER MATERIALIZED VIEW mv_stats RENAME TO mv_stats_v2",
		Expected: ddlCoverageUnsupportedBoundary},
	{Name: "ALTER MATERIALIZED VIEW SET SCHEMA",
		SQL:      "ALTER MATERIALIZED VIEW mv_stats SET SCHEMA app",
		Expected: ddlCoverageFindingCovered},
}

func TestPostgreSQLDDLCompletionCensus(t *testing.T) {
	t.Parallel()
	runDDLCensus(t, "PostgreSQL", pgDDLCompletionCensusCases)
}
