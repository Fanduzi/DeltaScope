package audit

import (
	"context"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

type ddlParserErrorFeasibility string

const (
	parserUpgradeCandidate           ddlParserErrorFeasibility = "parser_upgrade_candidate"
	boundedFallbackCandidate         ddlParserErrorFeasibility = "bounded_fallback_candidate"
	productUnsupportedOrInapplicable ddlParserErrorFeasibility = "product_unsupported_or_inapplicable"
	unsafeFallbackDefer              ddlParserErrorFeasibility = "unsafe_fallback_defer"
	needsResearch                    ddlParserErrorFeasibility = "needs_research"
)

type ddlParserErrorFeasibilityCase struct {
	Dialect  spec.Dialect
	Family   string
	Name     string
	SQL      string
	Expected ddlParserErrorFeasibility
	Reason   string
}

type ddlParserErrorFeasibilityResult struct {
	Dialect     string
	Family      string
	Name        string
	Feasibility ddlParserErrorFeasibility
	ParseError  string
	Reason      string
}

func assertParserErrorFeasibilityCase(t *testing.T, tc ddlParserErrorFeasibilityCase) ddlParserErrorFeasibilityResult {
	t.Helper()

	parsed, parseErr := Parse(context.Background(), tc.SQL, tc.Dialect)
	if parseErr != nil {
		t.Logf("%s %s/%s: parser_error confirmed: %v", tc.Dialect, tc.Family, tc.Name, parseErr)
		return ddlParserErrorFeasibilityResult{
			Dialect:     string(tc.Dialect),
			Family:      tc.Family,
			Name:        tc.Name,
			Feasibility: tc.Expected,
			ParseError:  parseErr.Error(),
			Reason:      tc.Reason,
		}
	}

	statements, extractErr := Extract(context.Background(), parsed)
	if extractErr != nil {
		t.Logf("%s %s/%s: extract_error (treated as parser_error): %v", tc.Dialect, tc.Family, tc.Name, extractErr)
		return ddlParserErrorFeasibilityResult{
			Dialect:     string(tc.Dialect),
			Family:      tc.Family,
			Name:        tc.Name,
			Feasibility: tc.Expected,
			ParseError:  extractErr.Error(),
			Reason:      tc.Reason,
		}
	}

	for i := range statements {
		if statements[i].Unsupported == nil {
			t.Fatalf("%s %s/%s: case no longer produces parser_error — supported DDL found; revisit feasibility decision",
				tc.Dialect, tc.Family, tc.Name)
		}
	}

	t.Logf("%s %s/%s: parse succeeded but only unsupported statements (still parser_error)", tc.Dialect, tc.Family, tc.Name)
	return ddlParserErrorFeasibilityResult{
		Dialect:     string(tc.Dialect),
		Family:      tc.Family,
		Name:        tc.Name,
		Feasibility: tc.Expected,
		ParseError:  "parsed-ok-but-no-supported-statement",
		Reason:      tc.Reason,
	}
}

// --- MySQL parser-error feasibility cases (15) ---

var mysqlParserErrorFeasibilityCases = []ddlParserErrorFeasibilityCase{
	{Dialect: spec.DialectMySQL, Family: "view", Name: "ALTER VIEW",
		SQL:      "ALTER VIEW v_users AS SELECT id, name FROM users",
		Expected: boundedFallbackCandidate,
		Reason:   "View name and ALTER action are bounded; SELECT body not needed for identity extraction"},
	{Dialect: spec.DialectMySQL, Family: "routine", Name: "ALTER PROCEDURE",
		SQL:      "ALTER PROCEDURE p_cleanup SQL SECURITY INVOKER",
		Expected: unsafeFallbackDefer,
		Reason:   "Routine option parsing (SQL SECURITY, COMMENT, etc.) requires understanding option semantics"},
	{Dialect: spec.DialectMySQL, Family: "routine", Name: "CREATE FUNCTION",
		SQL:      "CREATE FUNCTION hello() RETURNS VARCHAR(20) RETURN 'hello'",
		Expected: unsafeFallbackDefer,
		Reason:   "Function body (RETURN expression) is unbounded; fallback would leak routine logic"},
	{Dialect: spec.DialectMySQL, Family: "routine", Name: "ALTER FUNCTION",
		SQL:      "ALTER FUNCTION hello SQL SECURITY INVOKER",
		Expected: unsafeFallbackDefer,
		Reason:   "Routine option parsing requires understanding option semantics"},
	{Dialect: spec.DialectMySQL, Family: "routine", Name: "DROP FUNCTION",
		SQL:      "DROP FUNCTION hello",
		Expected: unsafeFallbackDefer,
		Reason:   "Consistent with routine lifecycle; parser upgrade path preferred over per-statement fallback"},
	{Dialect: spec.DialectMySQL, Family: "trigger", Name: "CREATE TRIGGER",
		SQL:      "CREATE TRIGGER trg_users_bi BEFORE INSERT ON users FOR EACH ROW SET NEW.created_at = NOW()",
		Expected: unsafeFallbackDefer,
		Reason:   "Trigger body and timing/event/row clause parsing is unbounded"},
	{Dialect: spec.DialectMySQL, Family: "trigger", Name: "DROP TRIGGER",
		SQL:      "DROP TRIGGER trg_users_bi",
		Expected: unsafeFallbackDefer,
		Reason:   "Consistent with trigger lifecycle; parser upgrade path preferred"},
	{Dialect: spec.DialectMySQL, Family: "event", Name: "CREATE EVENT",
		SQL:      "CREATE EVENT e_cleanup ON SCHEDULE EVERY 1 DAY DO CALL p_cleanup()",
		Expected: unsafeFallbackDefer,
		Reason:   "Event schedule and body parsing are unbounded"},
	{Dialect: spec.DialectMySQL, Family: "event", Name: "ALTER EVENT",
		SQL:      "ALTER EVENT e_cleanup ON SCHEDULE EVERY 2 DAY",
		Expected: unsafeFallbackDefer,
		Reason:   "Event schedule and option parsing are unbounded"},
	{Dialect: spec.DialectMySQL, Family: "event", Name: "DROP EVENT",
		SQL:      "DROP EVENT e_cleanup",
		Expected: unsafeFallbackDefer,
		Reason:   "Consistent with event lifecycle; parser upgrade path preferred"},
	{Dialect: spec.DialectMySQL, Family: "tablespace", Name: "CREATE TABLESPACE",
		SQL:      "CREATE TABLESPACE ts1 ADD DATAFILE 'ts1.ibd'",
		Expected: parserUpgradeCandidate,
		Reason:   "Standard MySQL 8.0 DDL with bounded structure; TiDB parser may add support in future version"},
	{Dialect: spec.DialectMySQL, Family: "tablespace", Name: "ALTER TABLESPACE",
		SQL:      "ALTER TABLESPACE ts1 ADD DATAFILE 'ts2.ibd'",
		Expected: parserUpgradeCandidate,
		Reason:   "Standard MySQL 8.0 DDL with bounded structure; TiDB parser may add support"},
	{Dialect: spec.DialectMySQL, Family: "tablespace", Name: "DROP TABLESPACE",
		SQL:      "DROP TABLESPACE ts1",
		Expected: parserUpgradeCandidate,
		Reason:   "Standard MySQL 8.0 DDL with bounded structure; TiDB parser may add support"},
	{Dialect: spec.DialectMySQL, Family: "resource_group", Name: "CREATE RESOURCE GROUP",
		SQL:      "CREATE RESOURCE GROUP rg1 TYPE = USER VCPU = 0-3",
		Expected: parserUpgradeCandidate,
		Reason:   "MySQL 8.0 resource group DDL; bounded key=value structure; TiDB parser may add support"},
	{Dialect: spec.DialectMySQL, Family: "resource_group", Name: "ALTER RESOURCE GROUP",
		SQL:      "ALTER RESOURCE GROUP rg1 VCPU = 0-7",
		Expected: parserUpgradeCandidate,
		Reason:   "MySQL 8.0 resource group DDL; bounded key=value structure; TiDB parser may add support"},
}

// --- TiDB parser-error feasibility cases (9) ---

var tidbParserErrorFeasibilityCases = []ddlParserErrorFeasibilityCase{
	{Dialect: spec.DialectTiDB, Family: "view", Name: "ALTER VIEW",
		SQL:      "ALTER VIEW v_users AS SELECT id, name FROM users",
		Expected: boundedFallbackCandidate,
		Reason:   "View name and ALTER action are bounded; SELECT body not needed for identity extraction"},
	{Dialect: spec.DialectTiDB, Family: "table_option", Name: "ALTER TABLE TTL",
		SQL:      "ALTER TABLE users TTL = 'created_at + INTERVAL 30 DAY'",
		Expected: boundedFallbackCandidate,
		Reason:   "TiDB-specific table option with bounded key=value structure; no body parsing required"},
	{Dialect: spec.DialectTiDB, Family: "table_option", Name: "ALTER TABLE LOCALITY",
		SQL:      "ALTER TABLE users LOCALITY = 'us-east-1'",
		Expected: boundedFallbackCandidate,
		Reason:   "TiDB-specific table option with bounded key=value structure; no body parsing required"},
	{Dialect: spec.DialectTiDB, Family: "trigger", Name: "CREATE TRIGGER",
		SQL:      "CREATE TRIGGER trg_users_bi BEFORE INSERT ON users FOR EACH ROW SET NEW.created_at = NOW()",
		Expected: productUnsupportedOrInapplicable,
		Reason:   "TiDB does not support triggers; parsing is not applicable"},
	{Dialect: spec.DialectTiDB, Family: "trigger", Name: "DROP TRIGGER",
		SQL:      "DROP TRIGGER trg_users_bi",
		Expected: productUnsupportedOrInapplicable,
		Reason:   "TiDB does not support triggers; parsing is not applicable"},
	{Dialect: spec.DialectTiDB, Family: "routine", Name: "CREATE FUNCTION",
		SQL:      "CREATE FUNCTION hello() RETURNS VARCHAR(20) RETURN 'hello'",
		Expected: productUnsupportedOrInapplicable,
		Reason:   "TiDB does not support user-defined SQL functions; parsing is not applicable"},
	{Dialect: spec.DialectTiDB, Family: "routine", Name: "DROP FUNCTION",
		SQL:      "DROP FUNCTION hello",
		Expected: productUnsupportedOrInapplicable,
		Reason:   "TiDB does not support user-defined SQL functions; parsing is not applicable"},
	{Dialect: spec.DialectTiDB, Family: "event", Name: "CREATE EVENT",
		SQL:      "CREATE EVENT e_cleanup ON SCHEDULE EVERY 1 DAY DO CALL p_cleanup()",
		Expected: productUnsupportedOrInapplicable,
		Reason:   "TiDB does not support events; parsing is not applicable"},
	{Dialect: spec.DialectTiDB, Family: "event", Name: "DROP EVENT",
		SQL:      "DROP EVENT e_cleanup",
		Expected: productUnsupportedOrInapplicable,
		Reason:   "TiDB does not support events; parsing is not applicable"},
}

func TestDDLParserErrorFeasibilityCensus(t *testing.T) {
	t.Parallel()

	allCases := append(mysqlParserErrorFeasibilityCases, tidbParserErrorFeasibilityCases...)

	var results []ddlParserErrorFeasibilityResult
	for _, tc := range allCases {
		if tc.Expected == "" {
			t.Fatalf("%s %s/%s: feasibility bucket must not be empty", tc.Dialect, tc.Family, tc.Name)
		}
		if tc.Reason == "" {
			t.Fatalf("%s %s/%s: reason must not be empty", tc.Dialect, tc.Family, tc.Name)
		}
		results = append(results, assertParserErrorFeasibilityCase(t, tc))
	}

	t.Log("")
	t.Log("=== MySQL/TiDB DDL Parser-Error Feasibility Census Summary ===")

	bucketCounts := make(map[ddlParserErrorFeasibility]map[string]int)
	for _, b := range []ddlParserErrorFeasibility{
		parserUpgradeCandidate, boundedFallbackCandidate,
		productUnsupportedOrInapplicable, unsafeFallbackDefer, needsResearch,
	} {
		bucketCounts[b] = make(map[string]int)
	}
	for _, r := range results {
		bucketCounts[r.Feasibility][r.Dialect]++
	}

	dialects := []string{string(spec.DialectMySQL), string(spec.DialectTiDB)}
	buckets := []ddlParserErrorFeasibility{
		parserUpgradeCandidate, boundedFallbackCandidate,
		productUnsupportedOrInapplicable, unsafeFallbackDefer, needsResearch,
	}
	for _, d := range dialects {
		total := 0
		for _, b := range buckets {
			c := bucketCounts[b][d]
			total += c
			if c > 0 {
				t.Logf("  %-40s %s: %d", b, d, c)
			}
		}
		t.Logf("  %-40s %s total: %d", "", d, total)
		t.Log("")
	}

	var mysqlCount, tidbCount int
	for _, r := range results {
		switch r.Dialect {
		case string(spec.DialectMySQL):
			mysqlCount++
		case string(spec.DialectTiDB):
			tidbCount++
		}
	}
	if mysqlCount != 15 {
		t.Errorf("expected 15 MySQL parser-error cases, got %d", mysqlCount)
	}
	if tidbCount != 9 {
		t.Errorf("expected 9 TiDB parser-error cases, got %d", tidbCount)
	}
	t.Logf("MySQL parser-error cases: %d", mysqlCount)
	t.Logf("TiDB parser-error cases:  %d", tidbCount)
}
