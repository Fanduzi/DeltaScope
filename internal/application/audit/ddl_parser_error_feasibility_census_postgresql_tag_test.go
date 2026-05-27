//go:build postgresql

package audit

import (
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

var pgParserErrorFeasibilityCases = []ddlParserErrorFeasibilityCase{
	{Dialect: spec.DialectPostgreSQL, Family: "subscription", Name: "DROP SUBSCRIPTION WITH drop_slot",
		SQL:      "DROP SUBSCRIPTION sub WITH (drop_slot = true)",
		Expected: parserUpgradeCandidate,
		Reason:   "PG parser does not support WITH options on DROP SUBSCRIPTION; pg_query_go upgrade path"},
	{Dialect: spec.DialectPostgreSQL, Family: "constraints", Name: "NOT NULL NOT VALID",
		SQL:      "ALTER TABLE users ADD CONSTRAINT users_email_nn NOT NULL email NOT VALID",
		Expected: parserUpgradeCandidate,
		Reason:   "PG18 ALTER TABLE grammar form; pending pg_query_go v7 upgrade"},
	{Dialect: spec.DialectPostgreSQL, Family: "constraints", Name: "ALTER CONSTRAINT NOT ENFORCED",
		SQL:      "ALTER TABLE orders ALTER CONSTRAINT orders_user_id_fkey NOT ENFORCED",
		Expected: parserUpgradeCandidate,
		Reason:   "PG18 ALTER TABLE grammar form; pending pg_query_go v7 upgrade"},
	{Dialect: spec.DialectPostgreSQL, Family: "constraints", Name: "ALTER CONSTRAINT INHERIT",
		SQL:      "ALTER TABLE users ALTER CONSTRAINT users_email_nn INHERIT",
		Expected: parserUpgradeCandidate,
		Reason:   "PG18 ALTER TABLE grammar form; pending pg_query_go v7 upgrade"},
	{Dialect: spec.DialectPostgreSQL, Family: "constraints", Name: "ALTER CONSTRAINT NO INHERIT",
		SQL:      "ALTER TABLE users ALTER CONSTRAINT users_email_nn NO INHERIT",
		Expected: parserUpgradeCandidate,
		Reason:   "PG18 ALTER TABLE grammar form; pending pg_query_go v7 upgrade"},
}

func TestDDLParserErrorFeasibilityPostgreSQLCensus(t *testing.T) {
	t.Parallel()

	var results []ddlParserErrorFeasibilityResult
	for _, tc := range pgParserErrorFeasibilityCases {
		if tc.Expected == "" {
			t.Fatalf("%s %s/%s: feasibility bucket must not be empty", tc.Dialect, tc.Family, tc.Name)
		}
		if tc.Reason == "" {
			t.Fatalf("%s %s/%s: reason must not be empty", tc.Dialect, tc.Family, tc.Name)
		}
		results = append(results, assertParserErrorFeasibilityCase(t, tc))
	}

	t.Log("")
	t.Log("=== PostgreSQL DDL Parser-Error Feasibility Census Summary ===")

	bucketCounts := make(map[ddlParserErrorFeasibility]int)
	for _, r := range results {
		bucketCounts[r.Feasibility]++
	}
	for _, b := range []ddlParserErrorFeasibility{
		parserUpgradeCandidate, boundedFallbackCandidate,
		productUnsupportedOrInapplicable, unsafeFallbackDefer, needsResearch,
	} {
		if c := bucketCounts[b]; c > 0 {
			t.Logf("  %-40s %d", b, c)
		}
	}
	t.Logf("  %-40s %d", "total", len(results))

	if len(results) != 5 {
		t.Errorf("expected 5 PostgreSQL parser-error cases, got %d", len(results))
	}
	t.Logf("PostgreSQL parser-error cases: %d", len(results))
}

