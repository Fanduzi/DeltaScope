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

var mysqlDDLCensusCases = []struct {
	Name     string
	SQL      string
	Expected ddlCoverageClass
}{
	// Table lifecycle
	{Name: "CREATE TABLE", SQL: "CREATE TABLE users (id bigint primary key)", Expected: ddlCoverageFindingCovered},
	{Name: "ALTER TABLE ADD COLUMN", SQL: "ALTER TABLE users ADD COLUMN email varchar(255)", Expected: ddlCoverageNormalizedSilent},
	{Name: "DROP TABLE", SQL: "DROP TABLE users", Expected: ddlCoverageFindingCovered},
	{Name: "TRUNCATE TABLE", SQL: "TRUNCATE TABLE users", Expected: ddlCoverageFindingCovered},

	// View lifecycle
	{Name: "CREATE VIEW", SQL: "CREATE VIEW v_users AS SELECT id FROM users", Expected: ddlCoverageFindingCovered},
	{Name: "DROP VIEW", SQL: "DROP VIEW v_users", Expected: ddlCoverageFindingCovered},

	// Database/Schema lifecycle
	{Name: "CREATE DATABASE", SQL: "CREATE DATABASE app", Expected: ddlCoverageFindingCovered},
	{Name: "CREATE DATABASE IF NOT EXISTS", SQL: "CREATE DATABASE IF NOT EXISTS app", Expected: ddlCoverageFindingCovered},
	{Name: "CREATE SCHEMA", SQL: "CREATE SCHEMA app", Expected: ddlCoverageFindingCovered},
	{Name: "DROP DATABASE", SQL: "DROP DATABASE app", Expected: ddlCoverageFindingCovered},
	{Name: "DROP DATABASE IF EXISTS", SQL: "DROP DATABASE IF EXISTS app", Expected: ddlCoverageFindingCovered},
	{Name: "DROP SCHEMA", SQL: "DROP SCHEMA app", Expected: ddlCoverageFindingCovered},

	// Trigger lifecycle — TiDB parser cannot parse triggers
	{Name: "CREATE TRIGGER", SQL: "CREATE TRIGGER trg_users_bi BEFORE INSERT ON users FOR EACH ROW SET NEW.created_at = NOW()", Expected: ddlCoverageParserError},
	{Name: "DROP TRIGGER", SQL: "DROP TRIGGER trg_users_bi", Expected: ddlCoverageParserError},

	// Routine lifecycle — parsed and normalized but no rules
	{Name: "CREATE PROCEDURE", SQL: "CREATE PROCEDURE p_cleanup() SELECT 1", Expected: ddlCoverageNormalizedSilent},
	{Name: "DROP PROCEDURE", SQL: "DROP PROCEDURE p_cleanup", Expected: ddlCoverageNormalizedSilent},

	// Privilege/DCL — parsed and normalized but no rules
	{Name: "GRANT SELECT", SQL: "GRANT SELECT ON app.users TO 'reader'@'%'", Expected: ddlCoverageNormalizedSilent},
	{Name: "REVOKE SELECT", SQL: "REVOKE SELECT ON app.users FROM 'reader'@'%'", Expected: ddlCoverageNormalizedSilent},
}

var tidbDDLCensusCases = []struct {
	Name     string
	SQL      string
	Expected ddlCoverageClass
}{
	// Table lifecycle
	{Name: "CREATE TABLE", SQL: "CREATE TABLE users (id bigint primary key)", Expected: ddlCoverageFindingCovered},
	{Name: "ALTER TABLE ADD COLUMN", SQL: "ALTER TABLE users ADD COLUMN email varchar(255)", Expected: ddlCoverageNormalizedSilent},
	{Name: "DROP TABLE", SQL: "DROP TABLE users", Expected: ddlCoverageFindingCovered},
	{Name: "TRUNCATE TABLE", SQL: "TRUNCATE TABLE users", Expected: ddlCoverageFindingCovered},

	// View lifecycle
	{Name: "CREATE VIEW", SQL: "CREATE VIEW v_users AS SELECT id FROM users", Expected: ddlCoverageFindingCovered},
	{Name: "DROP VIEW", SQL: "DROP VIEW v_users", Expected: ddlCoverageFindingCovered},

	// Database/Schema lifecycle
	{Name: "CREATE DATABASE", SQL: "CREATE DATABASE app", Expected: ddlCoverageFindingCovered},
	{Name: "CREATE DATABASE IF NOT EXISTS", SQL: "CREATE DATABASE IF NOT EXISTS app", Expected: ddlCoverageFindingCovered},
	{Name: "CREATE SCHEMA", SQL: "CREATE SCHEMA app", Expected: ddlCoverageFindingCovered},
	{Name: "DROP DATABASE", SQL: "DROP DATABASE app", Expected: ddlCoverageFindingCovered},
	{Name: "DROP DATABASE IF EXISTS", SQL: "DROP DATABASE IF EXISTS app", Expected: ddlCoverageFindingCovered},
	{Name: "DROP SCHEMA", SQL: "DROP SCHEMA app", Expected: ddlCoverageFindingCovered},

	// Trigger lifecycle — TiDB parser cannot parse triggers
	{Name: "CREATE TRIGGER", SQL: "CREATE TRIGGER trg_users_bi BEFORE INSERT ON users FOR EACH ROW SET NEW.created_at = NOW()", Expected: ddlCoverageParserError},
	{Name: "DROP TRIGGER", SQL: "DROP TRIGGER trg_users_bi", Expected: ddlCoverageParserError},

	// Routine lifecycle — parsed and normalized but no rules
	{Name: "CREATE PROCEDURE", SQL: "CREATE PROCEDURE p_cleanup() SELECT 1", Expected: ddlCoverageNormalizedSilent},
	{Name: "DROP PROCEDURE", SQL: "DROP PROCEDURE p_cleanup", Expected: ddlCoverageNormalizedSilent},

	// Privilege/DCL — parsed and normalized but no rules
	{Name: "GRANT SELECT", SQL: "GRANT SELECT ON app.users TO 'reader'@'%'", Expected: ddlCoverageNormalizedSilent},
	{Name: "REVOKE SELECT", SQL: "REVOKE SELECT ON app.users FROM 'reader'@'%'", Expected: ddlCoverageNormalizedSilent},
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

	var parseErrors, unsupported, silent, findingCovered int
	for _, r := range results {
		switch r.Classification {
		case ddlCoverageParserError:
			parseErrors++
		case ddlCoverageUnsupportedBoundary:
			unsupported++
		case ddlCoverageNormalizedSilent:
			silent++
		case ddlCoverageFindingCovered:
			findingCovered++
		}
	}

	t.Log("")
	t.Logf("=== %s DDL Census Summary ===", dialectName)
	t.Logf("Total:                %d", len(results))
	t.Logf("Finding covered:      %d", findingCovered)
	t.Logf("Normalized silent:    %d", silent)
	t.Logf("Unsupported boundary: %d", unsupported)
	t.Logf("Parser error:         %d", parseErrors)
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
