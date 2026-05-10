//go:build postgresql

package audit

import (
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

var crossDialectPGDDLCensusCases = []struct {
	Name     string
	SQL      string
	Expected ddlCoverageClass
}{
	// Table lifecycle
	{Name: "CREATE TABLE", SQL: "CREATE TABLE users (id bigint primary key)", Expected: ddlCoverageFindingCovered},
	{Name: "ALTER TABLE ADD COLUMN", SQL: "ALTER TABLE users ADD COLUMN email text", Expected: ddlCoverageFindingCovered},
	{Name: "DROP TABLE", SQL: "DROP TABLE users", Expected: ddlCoverageFindingCovered},

	// View lifecycle
	{Name: "CREATE VIEW", SQL: "CREATE VIEW v_users AS SELECT id FROM users", Expected: ddlCoverageFindingCovered},
	{Name: "DROP VIEW", SQL: "DROP VIEW v_users", Expected: ddlCoverageFindingCovered},

	// Schema lifecycle
	{Name: "CREATE SCHEMA", SQL: "CREATE SCHEMA app", Expected: ddlCoverageNormalizedSilent},
	{Name: "CREATE SCHEMA IF NOT EXISTS", SQL: "CREATE SCHEMA IF NOT EXISTS app", Expected: ddlCoverageNormalizedSilent},
	{Name: "DROP SCHEMA", SQL: "DROP SCHEMA app", Expected: ddlCoverageFindingCovered},
	{Name: "DROP SCHEMA CASCADE", SQL: "DROP SCHEMA app CASCADE", Expected: ddlCoverageFindingCovered},

	// Trigger lifecycle
	{Name: "CREATE TRIGGER", SQL: "CREATE TRIGGER trg_users_ai AFTER INSERT ON users FOR EACH ROW EXECUTE FUNCTION log_change()", Expected: ddlCoverageUnsupportedBoundary},
	{Name: "DROP TRIGGER", SQL: "DROP TRIGGER trg_users_ai ON users", Expected: ddlCoverageUnsupportedBoundary},

	// Routine lifecycle
	{Name: "CREATE FUNCTION", SQL: "CREATE FUNCTION log_change() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RETURN NEW; END $$", Expected: ddlCoverageUnsupportedBoundary},
	{Name: "DROP FUNCTION", SQL: "DROP FUNCTION log_change()", Expected: ddlCoverageUnsupportedBoundary},

	// Privilege/DCL
	{Name: "GRANT SELECT ON TABLE", SQL: "GRANT SELECT ON TABLE users TO reader", Expected: ddlCoverageFindingCovered},
	{Name: "REVOKE SELECT ON TABLE", SQL: "REVOKE SELECT ON TABLE users FROM reader", Expected: ddlCoverageFindingCovered},
}

func TestCrossDialectPostgreSQLDDLCoverageCensus(t *testing.T) {
	t.Parallel()
	runDDLCensus(t, "PostgreSQL", crossDialectPGDDLCensusCases)
}

func TestCrossDialectPGDDLCensusDialectIsolation(t *testing.T) {
	t.Parallel()

	pgOnlyCases := []struct {
		Name string
		SQL  string
	}{
		{Name: "DROP SCHEMA CASCADE", SQL: "DROP SCHEMA app CASCADE"},
		{Name: "CREATE FUNCTION", SQL: "CREATE FUNCTION log_change() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RETURN NEW; END $$"},
	}

	for _, tc := range pgOnlyCases {
		res := classifyDDLCensusResult(t, tc.Name, tc.SQL, "PostgreSQL", spec.DialectPostgreSQL)
		t.Logf("pg-isolation: %-40s -> %s", tc.Name, res.Classification)
	}
}
