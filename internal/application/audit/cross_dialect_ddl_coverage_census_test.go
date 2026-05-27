package audit

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

type ddlCoverageClass string

const (
	ddlCoverageParserError         ddlCoverageClass = "parser_error"
	ddlCoverageUnsupportedBoundary ddlCoverageClass = "unsupported_boundary"
	ddlCoverageNormalizedSilent    ddlCoverageClass = "normalized_silent"
	ddlCoverageFindingCovered      ddlCoverageClass = "finding_covered"
)

type ddlCensusCase struct {
	SQL      string
	Expected ddlCoverageClass
}

type ddlCensusResult struct {
	Name           string
	Dialect        string
	ParseOK        bool
	Kind           string
	Unsupported    bool
	UnsupportedFea string
	DDLOperation   string
	FindingRuleIDs []string
	Classification ddlCoverageClass
}

type ddlCensusSummary struct {
	Dialect             string
	Total               int
	FindingCovered      int
	NormalizedSilent    int
	UnsupportedBoundary int
	ParserError         int
	Unclassified        int
}

func summarizeDDLCensusResults(dialect string, results []ddlCensusResult) ddlCensusSummary {
	summary := ddlCensusSummary{Dialect: dialect, Total: len(results)}
	for _, r := range results {
		switch r.Classification {
		case ddlCoverageFindingCovered:
			summary.FindingCovered++
		case ddlCoverageNormalizedSilent:
			summary.NormalizedSilent++
		case ddlCoverageUnsupportedBoundary:
			summary.UnsupportedBoundary++
		case ddlCoverageParserError:
			summary.ParserError++
		default:
			summary.Unclassified++
		}
	}
	return summary
}

func (s ddlCensusSummary) assertArithmetic(t *testing.T) {
	t.Helper()
	sum := s.FindingCovered + s.NormalizedSilent + s.UnsupportedBoundary + s.ParserError + s.Unclassified
	if sum != s.Total {
		t.Fatalf("%s census arithmetic mismatch: total=%d buckets=%d", s.Dialect, s.Total, sum)
	}
}

var mysqlDDLCensusCases = []struct {
	Name     string
	SQL      string
	Expected ddlCoverageClass
}{
	// --- 1. Table lifecycle ---
	{Name: "CREATE TABLE", SQL: "CREATE TABLE users (id bigint primary key)", Expected: ddlCoverageFindingCovered},
	{Name: "CREATE TEMPORARY TABLE", SQL: "CREATE TEMPORARY TABLE tmp_users (id bigint primary key)", Expected: ddlCoverageFindingCovered},
	{Name: "CREATE TABLE LIKE", SQL: "CREATE TABLE users_copy LIKE users", Expected: ddlCoverageFindingCovered},
	{Name: "CREATE TABLE AS SELECT", SQL: "CREATE TABLE users_backup AS SELECT * FROM users", Expected: ddlCoverageFindingCovered},
	{Name: "RENAME TABLE", SQL: "RENAME TABLE users TO users_old", Expected: ddlCoverageFindingCovered},
	{Name: "DROP TABLE", SQL: "DROP TABLE users", Expected: ddlCoverageFindingCovered},
	{Name: "TRUNCATE TABLE", SQL: "TRUNCATE TABLE users", Expected: ddlCoverageFindingCovered},

	// --- 2. ALTER TABLE basics ---
	{Name: "ALTER TABLE ADD COLUMN", SQL: "ALTER TABLE users ADD COLUMN email varchar(255)", Expected: ddlCoverageFindingCovered},
	{Name: "ALTER TABLE DROP COLUMN", SQL: "ALTER TABLE users DROP COLUMN email", Expected: ddlCoverageFindingCovered},
	{Name: "ALTER TABLE MODIFY COLUMN", SQL: "ALTER TABLE users MODIFY COLUMN email varchar(500)", Expected: ddlCoverageFindingCovered},
	{Name: "ALTER TABLE CHANGE COLUMN", SQL: "ALTER TABLE users CHANGE COLUMN email user_email varchar(255)", Expected: ddlCoverageFindingCovered},
	{Name: "ALTER TABLE RENAME COLUMN", SQL: "ALTER TABLE users RENAME COLUMN email TO user_email", Expected: ddlCoverageFindingCovered},
	{Name: "ALTER TABLE RENAME TO", SQL: "ALTER TABLE users RENAME TO members", Expected: ddlCoverageFindingCovered},
	{Name: "ALTER TABLE ADD PRIMARY KEY", SQL: "ALTER TABLE users ADD PRIMARY KEY (id)", Expected: ddlCoverageFindingCovered},
	{Name: "ALTER TABLE DROP PRIMARY KEY", SQL: "ALTER TABLE users DROP PRIMARY KEY", Expected: ddlCoverageFindingCovered},
	{Name: "ALTER TABLE ADD INDEX", SQL: "ALTER TABLE users ADD INDEX idx_email (email)", Expected: ddlCoverageFindingCovered},
	{Name: "ALTER TABLE DROP INDEX", SQL: "ALTER TABLE users DROP INDEX idx_email", Expected: ddlCoverageFindingCovered},
	{Name: "ALTER TABLE ADD UNIQUE", SQL: "ALTER TABLE users ADD UNIQUE idx_email (email)", Expected: ddlCoverageFindingCovered},
	{Name: "ALTER TABLE ADD FOREIGN KEY", SQL: "ALTER TABLE orders ADD FOREIGN KEY (user_id) REFERENCES users(id)", Expected: ddlCoverageFindingCovered},
	{Name: "ALTER TABLE DROP FOREIGN KEY", SQL: "ALTER TABLE orders DROP FOREIGN KEY fk_user", Expected: ddlCoverageFindingCovered},

	// --- 3. Index lifecycle ---
	{Name: "CREATE INDEX", SQL: "CREATE INDEX idx_email ON users (email)", Expected: ddlCoverageFindingCovered},
	{Name: "CREATE UNIQUE INDEX", SQL: "CREATE UNIQUE INDEX idx_email ON users (email)", Expected: ddlCoverageFindingCovered},
	{Name: "CREATE FULLTEXT INDEX", SQL: "CREATE FULLTEXT INDEX idx_content ON posts (content)", Expected: ddlCoverageFindingCovered},
	{Name: "CREATE SPATIAL INDEX", SQL: "CREATE SPATIAL INDEX idx_location ON places (location)", Expected: ddlCoverageFindingCovered},
	{Name: "DROP INDEX", SQL: "DROP INDEX idx_email ON users", Expected: ddlCoverageFindingCovered},
	{Name: "ALTER TABLE RENAME INDEX", SQL: "ALTER TABLE users RENAME INDEX idx_email TO idx_user_email", Expected: ddlCoverageFindingCovered},

	// --- 4. Database/Schema lifecycle ---
	{Name: "CREATE DATABASE", SQL: "CREATE DATABASE app", Expected: ddlCoverageFindingCovered},
	{Name: "CREATE DATABASE IF NOT EXISTS", SQL: "CREATE DATABASE IF NOT EXISTS app", Expected: ddlCoverageFindingCovered},
	{Name: "CREATE SCHEMA", SQL: "CREATE SCHEMA app", Expected: ddlCoverageFindingCovered},
	{Name: "ALTER DATABASE", SQL: "ALTER DATABASE app CHARACTER SET utf8mb4", Expected: ddlCoverageFindingCovered},
	{Name: "DROP DATABASE", SQL: "DROP DATABASE app", Expected: ddlCoverageFindingCovered},
	{Name: "DROP DATABASE IF EXISTS", SQL: "DROP DATABASE IF EXISTS app", Expected: ddlCoverageFindingCovered},
	{Name: "DROP SCHEMA", SQL: "DROP SCHEMA app", Expected: ddlCoverageFindingCovered},

	// --- 5. View lifecycle ---
	{Name: "CREATE VIEW", SQL: "CREATE VIEW v_users AS SELECT id FROM users", Expected: ddlCoverageFindingCovered},
	{Name: "CREATE OR REPLACE VIEW", SQL: "CREATE OR REPLACE VIEW v_users AS SELECT id FROM users", Expected: ddlCoverageFindingCovered},
	{Name: "ALTER VIEW", SQL: "ALTER VIEW v_users AS SELECT id, name FROM users", Expected: ddlCoverageParserError},
	{Name: "DROP VIEW", SQL: "DROP VIEW v_users", Expected: ddlCoverageFindingCovered},

	// --- 6. Routine lifecycle ---
	{Name: "CREATE PROCEDURE", SQL: "CREATE PROCEDURE p_cleanup() SELECT 1", Expected: ddlCoverageFindingCovered},
	{Name: "ALTER PROCEDURE", SQL: "ALTER PROCEDURE p_cleanup SQL SECURITY INVOKER", Expected: ddlCoverageParserError},
	{Name: "DROP PROCEDURE", SQL: "DROP PROCEDURE p_cleanup", Expected: ddlCoverageFindingCovered},
	{Name: "CREATE FUNCTION", SQL: "CREATE FUNCTION hello() RETURNS VARCHAR(20) RETURN 'hello'", Expected: ddlCoverageParserError},
	{Name: "ALTER FUNCTION", SQL: "ALTER FUNCTION hello SQL SECURITY INVOKER", Expected: ddlCoverageParserError},
	{Name: "DROP FUNCTION", SQL: "DROP FUNCTION hello", Expected: ddlCoverageParserError},

	// --- 7. Trigger lifecycle ---
	{Name: "CREATE TRIGGER", SQL: "CREATE TRIGGER trg_users_bi BEFORE INSERT ON users FOR EACH ROW SET NEW.created_at = NOW()", Expected: ddlCoverageParserError},
	{Name: "DROP TRIGGER", SQL: "DROP TRIGGER trg_users_bi", Expected: ddlCoverageParserError},

	// --- 8. Event lifecycle ---
	{Name: "CREATE EVENT", SQL: "CREATE EVENT e_cleanup ON SCHEDULE EVERY 1 DAY DO CALL p_cleanup()", Expected: ddlCoverageParserError},
	{Name: "ALTER EVENT", SQL: "ALTER EVENT e_cleanup ON SCHEDULE EVERY 2 DAY", Expected: ddlCoverageParserError},
	{Name: "DROP EVENT", SQL: "DROP EVENT e_cleanup", Expected: ddlCoverageParserError},

	// --- 9. Privilege/User/Role lifecycle ---
	{Name: "CREATE USER", SQL: "CREATE USER 'admin'@'%' IDENTIFIED BY 'secret'", Expected: ddlCoverageFindingCovered},
	{Name: "ALTER USER", SQL: "ALTER USER 'admin'@'%' IDENTIFIED BY 'new_secret'", Expected: ddlCoverageFindingCovered},
	{Name: "DROP USER", SQL: "DROP USER 'admin'@'%'", Expected: ddlCoverageFindingCovered},
	{Name: "CREATE ROLE", SQL: "CREATE ROLE manager", Expected: ddlCoverageFindingCovered},
	{Name: "DROP ROLE", SQL: "DROP ROLE manager", Expected: ddlCoverageFindingCovered},
	{Name: "GRANT SELECT", SQL: "GRANT SELECT ON app.users TO 'reader'@'%'", Expected: ddlCoverageFindingCovered},
	{Name: "REVOKE SELECT", SQL: "REVOKE SELECT ON app.users FROM 'reader'@'%'", Expected: ddlCoverageFindingCovered},

	// --- 10. Tablespace/Resource lifecycle ---
	{Name: "CREATE TABLESPACE", SQL: "CREATE TABLESPACE ts1 ADD DATAFILE 'ts1.ibd'", Expected: ddlCoverageParserError},
	{Name: "ALTER TABLESPACE", SQL: "ALTER TABLESPACE ts1 ADD DATAFILE 'ts2.ibd'", Expected: ddlCoverageParserError},
	{Name: "DROP TABLESPACE", SQL: "DROP TABLESPACE ts1", Expected: ddlCoverageParserError},
	{Name: "CREATE RESOURCE GROUP", SQL: "CREATE RESOURCE GROUP rg1 TYPE = USER VCPU = 0-3", Expected: ddlCoverageParserError},
	{Name: "ALTER RESOURCE GROUP", SQL: "ALTER RESOURCE GROUP rg1 VCPU = 0-7", Expected: ddlCoverageParserError},
	{Name: "DROP RESOURCE GROUP", SQL: "DROP RESOURCE GROUP rg1", Expected: ddlCoverageFindingCovered},
}

var tidbDDLCensusCases = []struct {
	Name     string
	SQL      string
	Expected ddlCoverageClass
}{
	// --- 1. Table lifecycle ---
	{Name: "CREATE TABLE", SQL: "CREATE TABLE users (id bigint primary key)", Expected: ddlCoverageFindingCovered},
	{Name: "CREATE TABLE LIKE", SQL: "CREATE TABLE users_copy LIKE users", Expected: ddlCoverageFindingCovered},
	{Name: "CREATE TABLE AS SELECT", SQL: "CREATE TABLE users_backup AS SELECT * FROM users", Expected: ddlCoverageFindingCovered},
	{Name: "RENAME TABLE", SQL: "RENAME TABLE users TO users_old", Expected: ddlCoverageFindingCovered},
	{Name: "DROP TABLE", SQL: "DROP TABLE users", Expected: ddlCoverageFindingCovered},
	{Name: "TRUNCATE TABLE", SQL: "TRUNCATE TABLE users", Expected: ddlCoverageFindingCovered},

	// --- 2. ALTER TABLE basics ---
	{Name: "ALTER TABLE ADD COLUMN", SQL: "ALTER TABLE users ADD COLUMN email varchar(255)", Expected: ddlCoverageFindingCovered},
	{Name: "ALTER TABLE DROP COLUMN", SQL: "ALTER TABLE users DROP COLUMN email", Expected: ddlCoverageFindingCovered},
	{Name: "ALTER TABLE MODIFY COLUMN", SQL: "ALTER TABLE users MODIFY COLUMN email varchar(500)", Expected: ddlCoverageFindingCovered},
	{Name: "ALTER TABLE CHANGE COLUMN", SQL: "ALTER TABLE users CHANGE COLUMN email user_email varchar(255)", Expected: ddlCoverageFindingCovered},
	{Name: "ALTER TABLE RENAME COLUMN", SQL: "ALTER TABLE users RENAME COLUMN email TO user_email", Expected: ddlCoverageFindingCovered},
	{Name: "ALTER TABLE ADD PRIMARY KEY", SQL: "ALTER TABLE users ADD PRIMARY KEY (id)", Expected: ddlCoverageFindingCovered},
	{Name: "ALTER TABLE DROP PRIMARY KEY", SQL: "ALTER TABLE users DROP PRIMARY KEY", Expected: ddlCoverageFindingCovered},
	{Name: "ALTER TABLE ADD INDEX", SQL: "ALTER TABLE users ADD INDEX idx_email (email)", Expected: ddlCoverageFindingCovered},
	{Name: "ALTER TABLE DROP INDEX", SQL: "ALTER TABLE users DROP INDEX idx_email", Expected: ddlCoverageFindingCovered},
	{Name: "ALTER TABLE ADD UNIQUE", SQL: "ALTER TABLE users ADD UNIQUE idx_email (email)", Expected: ddlCoverageFindingCovered},
	{Name: "ALTER TABLE ADD FOREIGN KEY", SQL: "ALTER TABLE orders ADD FOREIGN KEY (user_id) REFERENCES users(id)", Expected: ddlCoverageFindingCovered},
	{Name: "ALTER TABLE DROP FOREIGN KEY", SQL: "ALTER TABLE orders DROP FOREIGN KEY fk_user", Expected: ddlCoverageFindingCovered},

	// --- 3. Index lifecycle ---
	{Name: "CREATE INDEX", SQL: "CREATE INDEX idx_email ON users (email)", Expected: ddlCoverageFindingCovered},
	{Name: "CREATE UNIQUE INDEX", SQL: "CREATE UNIQUE INDEX idx_email ON users (email)", Expected: ddlCoverageFindingCovered},
	{Name: "DROP INDEX", SQL: "DROP INDEX idx_email ON users", Expected: ddlCoverageFindingCovered},

	// --- 4. Database/Schema lifecycle ---
	{Name: "CREATE DATABASE", SQL: "CREATE DATABASE app", Expected: ddlCoverageFindingCovered},
	{Name: "CREATE DATABASE IF NOT EXISTS", SQL: "CREATE DATABASE IF NOT EXISTS app", Expected: ddlCoverageFindingCovered},
	{Name: "CREATE SCHEMA", SQL: "CREATE SCHEMA app", Expected: ddlCoverageFindingCovered},
	{Name: "ALTER DATABASE", SQL: "ALTER DATABASE app CHARACTER SET utf8mb4", Expected: ddlCoverageFindingCovered},
	{Name: "DROP DATABASE", SQL: "DROP DATABASE app", Expected: ddlCoverageFindingCovered},
	{Name: "DROP DATABASE IF EXISTS", SQL: "DROP DATABASE IF EXISTS app", Expected: ddlCoverageFindingCovered},
	{Name: "DROP SCHEMA", SQL: "DROP SCHEMA app", Expected: ddlCoverageFindingCovered},

	// --- 5. View lifecycle ---
	{Name: "CREATE VIEW", SQL: "CREATE VIEW v_users AS SELECT id FROM users", Expected: ddlCoverageFindingCovered},
	{Name: "CREATE OR REPLACE VIEW", SQL: "CREATE OR REPLACE VIEW v_users AS SELECT id FROM users", Expected: ddlCoverageFindingCovered},
	{Name: "ALTER VIEW", SQL: "ALTER VIEW v_users AS SELECT id, name FROM users", Expected: ddlCoverageParserError},
	{Name: "DROP VIEW", SQL: "DROP VIEW v_users", Expected: ddlCoverageFindingCovered},

	// --- 6. Placement/Policy ---
	{Name: "CREATE PLACEMENT POLICY", SQL: "CREATE PLACEMENT POLICY p1 PRIMARY_REGION='us-east-1' REGIONS='us-east-1'", Expected: ddlCoverageFindingCovered},
	{Name: "ALTER PLACEMENT POLICY", SQL: "ALTER PLACEMENT POLICY p1 PRIMARY_REGION='us-west-1' REGIONS='us-west-1'", Expected: ddlCoverageFindingCovered},
	{Name: "DROP PLACEMENT POLICY", SQL: "DROP PLACEMENT POLICY p1", Expected: ddlCoverageFindingCovered},

	// --- 7. Sequence lifecycle ---
	{Name: "CREATE SEQUENCE", SQL: "CREATE SEQUENCE seq1 START WITH 1 INCREMENT BY 1", Expected: ddlCoverageFindingCovered},
	{Name: "ALTER SEQUENCE", SQL: "ALTER SEQUENCE seq1 START WITH 100", Expected: ddlCoverageFindingCovered},
	{Name: "DROP SEQUENCE", SQL: "DROP SEQUENCE seq1", Expected: ddlCoverageFindingCovered},

	// --- 8. TTL/Locality/Placement table options ---
	{Name: "ALTER TABLE TTL", SQL: "ALTER TABLE users TTL = 'created_at + INTERVAL 30 DAY'", Expected: ddlCoverageParserError},
	{Name: "ALTER TABLE PLACEMENT POLICY", SQL: "ALTER TABLE users PLACEMENT POLICY p1", Expected: ddlCoverageNormalizedSilent},
	{Name: "ALTER TABLE LOCALITY", SQL: "ALTER TABLE users LOCALITY = 'us-east-1'", Expected: ddlCoverageParserError},

	// --- 9. Unsupported product areas / parser gaps ---
	{Name: "CREATE TRIGGER", SQL: "CREATE TRIGGER trg_users_bi BEFORE INSERT ON users FOR EACH ROW SET NEW.created_at = NOW()", Expected: ddlCoverageParserError},
	{Name: "DROP TRIGGER", SQL: "DROP TRIGGER trg_users_bi", Expected: ddlCoverageParserError},
	{Name: "CREATE PROCEDURE", SQL: "CREATE PROCEDURE p_cleanup() SELECT 1", Expected: ddlCoverageFindingCovered},
	{Name: "DROP PROCEDURE", SQL: "DROP PROCEDURE p_cleanup", Expected: ddlCoverageFindingCovered},
	{Name: "CREATE FUNCTION", SQL: "CREATE FUNCTION hello() RETURNS VARCHAR(20) RETURN 'hello'", Expected: ddlCoverageParserError},
	{Name: "DROP FUNCTION", SQL: "DROP FUNCTION hello", Expected: ddlCoverageParserError},
	{Name: "CREATE EVENT", SQL: "CREATE EVENT e_cleanup ON SCHEDULE EVERY 1 DAY DO CALL p_cleanup()", Expected: ddlCoverageParserError},
	{Name: "DROP EVENT", SQL: "DROP EVENT e_cleanup", Expected: ddlCoverageParserError},

	// --- 10. Privilege/User lifecycle ---
	{Name: "CREATE USER", SQL: "CREATE USER 'admin'@'%' IDENTIFIED BY 'secret'", Expected: ddlCoverageFindingCovered},
	{Name: "ALTER USER", SQL: "ALTER USER 'admin'@'%' IDENTIFIED BY 'new_secret'", Expected: ddlCoverageFindingCovered},
	{Name: "DROP USER", SQL: "DROP USER 'admin'@'%'", Expected: ddlCoverageFindingCovered},
	{Name: "GRANT SELECT", SQL: "GRANT SELECT ON app.users TO 'reader'@'%'", Expected: ddlCoverageFindingCovered},
	{Name: "REVOKE SELECT", SQL: "REVOKE SELECT ON app.users FROM 'reader'@'%'", Expected: ddlCoverageFindingCovered},
}

func TestCrossDialectDDLCoverageCensus(t *testing.T) {
	t.Parallel()

	type dialectCase struct {
		Dialect string
		Cases   []struct {
			Name     string
			SQL      string
			Expected ddlCoverageClass
		}
	}

	dialects := []dialectCase{
		{Dialect: "MySQL", Cases: mysqlDDLCensusCases},
		{Dialect: "TiDB", Cases: tidbDDLCensusCases},
	}

	for _, d := range dialects {
		t.Run(d.Dialect, func(t *testing.T) {
			t.Parallel()
			runDDLCensus(t, d.Dialect, d.Cases)
		})
	}
}

func runDDLCensus(t *testing.T, dialectName string, cases []struct {
	Name     string
	SQL      string
	Expected ddlCoverageClass
}) {
	t.Helper()

	var dialect spec.Dialect
	switch dialectName {
	case "MySQL":
		dialect = spec.DialectMySQL
	case "TiDB":
		dialect = spec.DialectTiDB
	case "PostgreSQL":
		dialect = spec.DialectPostgreSQL
	default:
		t.Fatalf("unknown dialect %q", dialectName)
	}

	var results []ddlCensusResult
	var mismatchCount int

	for _, tc := range cases {
		res := classifyDDLCensusResult(t, tc.Name, tc.SQL, dialectName, dialect)
		results = append(results, res)

		if res.Classification != tc.Expected {
			mismatchCount++
			t.Errorf("%s %s: expected %s, got %s", dialectName, tc.Name, tc.Expected, res.Classification)
		}
	}

	logDDLCensusTable(t, dialectName, results)

	summary := summarizeDDLCensusResults(dialectName, results)
	summary.assertArithmetic(t)

	t.Log("")
	t.Logf("=== %s DDL Census Summary ===", dialectName)
	t.Logf("Total:                %d", summary.Total)
	t.Logf("Finding covered:      %d", summary.FindingCovered)
	t.Logf("Normalized silent:    %d", summary.NormalizedSilent)
	t.Logf("Unsupported boundary: %d", summary.UnsupportedBoundary)
	t.Logf("Parser error:         %d", summary.ParserError)
	t.Logf("Classification mismatches: %d", mismatchCount)
}

func classifyDDLCensusResult(t *testing.T, name, sql, dialectName string, dialect spec.Dialect) ddlCensusResult {
	t.Helper()
	res := ddlCensusResult{Name: name, Dialect: dialectName}

	parsed, parseErr := Parse(context.Background(), sql, dialect)
	if parseErr != nil {
		res.Classification = ddlCoverageParserError
		t.Logf("ddl-census: %s %-40s -> parser_error: %v", dialectName, name, parseErr)
		return res
	}
	res.ParseOK = true

	statements, extractErr := Extract(context.Background(), parsed)
	if extractErr != nil {
		res.Classification = ddlCoverageParserError
		t.Logf("ddl-census: %s %-40s -> extract_error: %v", dialectName, name, extractErr)
		return res
	}

	var supported *spec.Statement
	for i := range statements {
		if statements[i].Unsupported == nil {
			s := statements[i]
			supported = &s
			break
		}
	}

	if supported == nil {
		res.Unsupported = true
		if len(statements) > 0 && statements[0].Unsupported != nil {
			res.UnsupportedFea = statements[0].Unsupported.Feature
		}
		res.Classification = ddlCoverageUnsupportedBoundary
		t.Logf("ddl-census: %s %-40s -> unsupported(%s)", dialectName, name, res.UnsupportedFea)
		return res
	}

	res.Kind = string(supported.Kind)
	if supported.DDL != nil {
		res.DDLOperation = string(supported.DDL.Operation)
	}

	result, auditErr := AuditSQL(context.Background(), Request{
		SQL:     sql,
		Dialect: dialect,
	})
	if auditErr != nil {
		t.Logf("ddl-census: %s %-40s -> audit_error: %v", dialectName, name, auditErr)
	}

	seen := make(map[string]struct{})
	if result.Statements != nil {
		for _, s := range result.Statements {
			for _, f := range s.Findings {
				seen[f.RuleID] = struct{}{}
			}
		}
	}
	for _, f := range result.GlobalFindings {
		seen[f.RuleID] = struct{}{}
	}
	ids := make([]string, 0, len(seen))
	for id := range seen {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	res.FindingRuleIDs = ids

	if len(res.FindingRuleIDs) > 0 {
		res.Classification = ddlCoverageFindingCovered
	} else {
		res.Classification = ddlCoverageNormalizedSilent
	}

	t.Logf("ddl-census: %s %-40s -> %s | op=%s findings=%v",
		dialectName, name, res.Classification, res.DDLOperation, res.FindingRuleIDs)
	return res
}

func logDDLCensusTable(t *testing.T, dialectName string, results []ddlCensusResult) {
	t.Helper()
	t.Log("")
	t.Logf("=== %s DDL Coverage Census ===", dialectName)
	t.Log("")
	t.Logf("%-40s | %-4s | %-22s | %-20s | %s",
		"Form", "OK?", "Classification", "DDL Op", "Detail")
	t.Log(strings.Repeat("-", 140))

	sorted := make([]ddlCensusResult, len(results))
	copy(sorted, results)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Name < sorted[j].Name
	})

	for _, r := range sorted {
		parseOK := "yes"
		if !r.ParseOK {
			parseOK = "no"
		}

		detail := strings.Join(r.FindingRuleIDs, ", ")
		if detail == "" {
			detail = "(none)"
		}
		if r.Unsupported {
			detail = fmt.Sprintf("unsupported: %s", r.UnsupportedFea)
		}

		t.Logf("%-40s | %-4s | %-22s | %-20s | %s",
			r.Name, parseOK, r.Classification, r.DDLOperation, detail)
	}
}
