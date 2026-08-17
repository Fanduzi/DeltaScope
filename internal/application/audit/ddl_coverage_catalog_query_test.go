// Package audit verifies DDL coverage catalog query behavior.
// input: the embedded generated catalog and the checked-in docs/reference catalog JSON
// output: coverage for LoadEmbeddedCatalog, LoadCatalog, QueryCatalog, and Validate
// pos: application catalog query tests
// note: if this file changes, update this header and module README.md.
package audit

import (
	"reflect"
	"strings"
	"testing"
)

const testCatalogPath = "../../../docs/reference/ddl-coverage-catalog.json"

func TestLoadEmbeddedCatalogMatchesCheckedInCatalog(t *testing.T) {
	version, entries, err := LoadEmbeddedCatalog()
	if err != nil {
		t.Fatalf("LoadEmbeddedCatalog: %v", err)
	}
	if version != "v0.270.0" {
		t.Errorf("embedded catalog version = %q, want v0.270.0", version)
	}
	if len(entries) != 400 {
		t.Errorf("embedded catalog entries = %d, want 400", len(entries))
	}

	disk, err := LoadCatalog(testCatalogPath)
	if err != nil {
		t.Fatalf("LoadCatalog(%s): %v", testCatalogPath, err)
	}
	if !reflect.DeepEqual(entries, disk) {
		t.Error("embedded catalog entries differ from the generated docs/reference catalog")
	}
}

func loadTestCatalog(t *testing.T) []CatalogEntry {
	t.Helper()
	entries, err := LoadCatalog(testCatalogPath)
	if err != nil {
		t.Fatalf("load catalog: %v", err)
	}
	return entries
}

func TestQueryCatalog_MySQLAlterView(t *testing.T) {
	entries := loadTestCatalog(t)

	tests := []struct {
		name string
		q    CatalogQuery
	}{
		{"dialect=mysql, classification=parser_error", CatalogQuery{Dialect: "mysql", Classification: "parser_error"}},
		{"dialect=mysql, guidance_code=parser_upgrade_candidate", CatalogQuery{Dialect: "mysql", GuidanceCode: "parser_upgrade_candidate"}},
		{"search=alter view", CatalogQuery{Dialect: "mysql", Search: "alter view"}},
		{"form=ALTER VIEW", CatalogQuery{Dialect: "mysql", Form: "ALTER VIEW"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			res := QueryCatalog(entries, tc.q)
			found := false
			for _, e := range res.Entries {
				if e.Dialect == "mysql" && e.Form == "ALTER VIEW" {
					found = true
					if e.Classification != "parser_error" {
						t.Errorf("ALTER VIEW classification = %q, want parser_error", e.Classification)
					}
					if e.GuidanceCode != "parser_upgrade_candidate" {
						t.Errorf("ALTER VIEW guidance_code = %q, want parser_upgrade_candidate", e.GuidanceCode)
					}
					break
				}
			}
			if !found {
				t.Error("ALTER VIEW not found in results")
			}
		})
	}
}

func TestQueryCatalog_PostgreSQLDropSubscription(t *testing.T) {
	entries := loadTestCatalog(t)

	res := QueryCatalog(entries, CatalogQuery{Dialect: "postgresql", Search: "subscription"})
	if len(res.Entries) == 0 {
		t.Fatal("expected at least one PostgreSQL subscription entry")
	}
	found := false
	for _, e := range res.Entries {
		if e.Form == "DROP SUBSCRIPTION" {
			found = true
			if e.Classification != "finding_covered" {
				t.Errorf("DROP SUBSCRIPTION classification = %q, want finding_covered", e.Classification)
			}
			break
		}
	}
	if !found {
		t.Error("DROP SUBSCRIPTION not found in results")
	}
}

func TestQueryCatalog_TiDBEntries(t *testing.T) {
	entries := loadTestCatalog(t)

	res := QueryCatalog(entries, CatalogQuery{Dialect: "tidb"})
	if len(res.Entries) == 0 {
		t.Fatal("expected TiDB entries")
	}
	for _, e := range res.Entries {
		if e.Dialect != "tidb" {
			t.Errorf("got dialect %q, want tidb", e.Dialect)
		}
	}
	if res.Total != 54 {
		t.Errorf("TiDB total = %d, want 54", res.Total)
	}
}

func TestQueryCatalog_ClassificationFilter(t *testing.T) {
	entries := loadTestCatalog(t)

	res := QueryCatalog(entries, CatalogQuery{Classification: "parser_error"})
	if len(res.Entries) == 0 {
		t.Fatal("expected parser_error entries")
	}
	for _, e := range res.Entries {
		if e.Classification != "parser_error" {
			t.Errorf("got classification %q, want parser_error", e.Classification)
		}
	}
	// All dialects should have parser_error entries.
	seenDialects := map[string]bool{}
	for _, e := range res.Entries {
		seenDialects[e.Dialect] = true
	}
	for _, d := range []string{"mysql", "tidb", "postgresql"} {
		if !seenDialects[d] {
			t.Errorf("no parser_error entries for dialect %q", d)
		}
	}
}

func TestQueryCatalog_GuidanceCodeFilter(t *testing.T) {
	entries := loadTestCatalog(t)

	res := QueryCatalog(entries, CatalogQuery{GuidanceCode: "parser_upgrade_candidate"})
	if len(res.Entries) == 0 {
		t.Fatal("expected parser_upgrade_candidate entries")
	}
	for _, e := range res.Entries {
		if e.GuidanceCode != "parser_upgrade_candidate" {
			t.Errorf("got guidance_code %q, want parser_upgrade_candidate", e.GuidanceCode)
		}
	}
}

func TestQueryCatalog_FamilySubstringCaseInsensitive(t *testing.T) {
	entries := loadTestCatalog(t)

	lower := QueryCatalog(entries, CatalogQuery{Family: "view_lifecycle"})
	upper := QueryCatalog(entries, CatalogQuery{Family: "VIEW_LIFECYCLE"})
	mixed := QueryCatalog(entries, CatalogQuery{Family: "View_Lifecycle"})

	if lower.Total == 0 {
		t.Fatal("expected entries for family=view_lifecycle")
	}
	if lower.Total != upper.Total || lower.Total != mixed.Total {
		t.Errorf("case-insensitive mismatch: lower=%d upper=%d mixed=%d", lower.Total, upper.Total, mixed.Total)
	}
}

func TestQueryCatalog_FormSubstringCaseInsensitive(t *testing.T) {
	entries := loadTestCatalog(t)

	lower := QueryCatalog(entries, CatalogQuery{Form: "alter table"})
	upper := QueryCatalog(entries, CatalogQuery{Form: "ALTER TABLE"})

	if lower.Total == 0 {
		t.Fatal("expected entries for form=alter table")
	}
	if lower.Total != upper.Total {
		t.Errorf("case-insensitive form mismatch: lower=%d upper=%d", lower.Total, upper.Total)
	}
}

func TestQueryCatalog_SearchCaseInsensitive(t *testing.T) {
	entries := loadTestCatalog(t)

	lower := QueryCatalog(entries, CatalogQuery{Search: "subscription"})
	upper := QueryCatalog(entries, CatalogQuery{Search: "SUBSCRIPTION"})

	if lower.Total == 0 {
		t.Fatal("expected entries for search=subscription")
	}
	if lower.Total != upper.Total {
		t.Errorf("case-insensitive search mismatch: lower=%d upper=%d", lower.Total, upper.Total)
	}
}

func TestQueryCatalog_CombinedFilters(t *testing.T) {
	entries := loadTestCatalog(t)

	// dialect=mysql + classification=parser_error should be a subset of dialect=mysql alone.
	mysqlAll := QueryCatalog(entries, CatalogQuery{Dialect: "mysql"})
	mysqlPE := QueryCatalog(entries, CatalogQuery{Dialect: "mysql", Classification: "parser_error"})

	if mysqlPE.Total > mysqlAll.Total {
		t.Errorf("combined filter returned more (%d) than dialect-only (%d)", mysqlPE.Total, mysqlAll.Total)
	}
	if mysqlPE.Total == 0 {
		t.Error("expected mysql parser_error entries")
	}
	for _, e := range mysqlPE.Entries {
		if e.Dialect != "mysql" {
			t.Errorf("got dialect %q, want mysql", e.Dialect)
		}
		if e.Classification != "parser_error" {
			t.Errorf("got classification %q, want parser_error", e.Classification)
		}
	}
}

func TestQueryCatalog_Limit(t *testing.T) {
	entries := loadTestCatalog(t)

	res := QueryCatalog(entries, CatalogQuery{Limit: 5})
	if len(res.Entries) != 5 {
		t.Errorf("got %d entries, want 5", len(res.Entries))
	}
	if res.Total < 5 {
		t.Errorf("total %d < 5", res.Total)
	}

	// limit > total returns all.
	resAll := QueryCatalog(entries, CatalogQuery{Limit: 99999})
	if resAll.Total != len(resAll.Entries) {
		t.Errorf("total=%d but entries=%d", resAll.Total, len(resAll.Entries))
	}

	// limit=0 means no limit.
	resZero := QueryCatalog(entries, CatalogQuery{Limit: 0})
	if resZero.Total != len(entries) {
		t.Errorf("limit=0: got %d entries, want %d", resZero.Total, len(entries))
	}
}

func TestQueryCatalog_LimitDeterministic(t *testing.T) {
	entries := loadTestCatalog(t)

	res1 := QueryCatalog(entries, CatalogQuery{Limit: 10})
	res2 := QueryCatalog(entries, CatalogQuery{Limit: 10})

	if len(res1.Entries) != len(res2.Entries) {
		t.Fatalf("different lengths: %d vs %d", len(res1.Entries), len(res2.Entries))
	}
	for i := range res1.Entries {
		if !reflect.DeepEqual(res1.Entries[i], res2.Entries[i]) {
			t.Errorf("entry %d mismatch: %+v vs %+v", i, res1.Entries[i], res2.Entries[i])
		}
	}
}

func TestQueryCatalog_EmptyResult(t *testing.T) {
	entries := loadTestCatalog(t)

	res := QueryCatalog(entries, CatalogQuery{Dialect: "mysql", Search: "xyzzy_no_match"})
	if res.Total != 0 {
		t.Errorf("expected 0 results, got %d", res.Total)
	}
	if res.Entries == nil {
		t.Error("Entries should be empty slice, not nil")
	}
}

func TestQueryCatalog_InvalidDialect(t *testing.T) {
	q := CatalogQuery{Dialect: "oracle"}
	err := q.Validate()
	if err == nil {
		t.Fatal("expected validation error for invalid dialect")
	}
	if !strings.Contains(err.Error(), "oracle") {
		t.Errorf("error should mention 'oracle': %v", err)
	}
}

func TestQueryCatalog_InvalidClassification(t *testing.T) {
	q := CatalogQuery{Classification: "bogus"}
	err := q.Validate()
	if err == nil {
		t.Fatal("expected validation error for invalid classification")
	}
	if !strings.Contains(err.Error(), "bogus") {
		t.Errorf("error should mention 'bogus': %v", err)
	}
}

func TestQueryCatalog_InvalidGuidanceCode(t *testing.T) {
	q := CatalogQuery{GuidanceCode: "nonexistent"}
	err := q.Validate()
	if err == nil {
		t.Fatal("expected validation error for invalid guidance_code")
	}
	if !strings.Contains(err.Error(), "nonexistent") {
		t.Errorf("error should mention 'nonexistent': %v", err)
	}
}

func TestQueryCatalog_ValidFiltersPass(t *testing.T) {
	for _, q := range []CatalogQuery{
		{},
		{Dialect: "mysql"},
		{Dialect: "tidb"},
		{Dialect: "postgresql"},
		{Classification: "finding_covered"},
		{Classification: "normalized_silent"},
		{Classification: "unsupported_boundary"},
		{Classification: "parser_error"},
		{Classification: "unclassified"},
		{GuidanceCode: "parser_upgrade_candidate"},
	} {
		if err := q.Validate(); err != nil {
			t.Errorf("Validate(%+v) = %v, want nil", q, err)
		}
	}
}

func TestQueryCatalog_SearchAcrossFields(t *testing.T) {
	entries := loadTestCatalog(t)

	// Search should match against finding_rule_ids.
	res := QueryCatalog(entries, CatalogQuery{Search: "ddl.view.create.forbid"})
	if res.Total == 0 {
		t.Error("expected match against finding_rule_id")
	}
	found := false
	for _, e := range res.Entries {
		for _, rid := range e.FindingRuleIDs {
			if rid == "ddl.view.create.forbid" {
				found = true
				break
			}
		}
	}
	if !found {
		t.Error("search did not find entry with ddl.view.create.forbid rule ID")
	}
}

func TestQueryCatalog_NoLeakSanity(t *testing.T) {
	entries := loadTestCatalog(t)

	res := QueryCatalog(entries, CatalogQuery{})
	forbidden := []string{
		"near \"",
		"near `",
		"Syntax error",
		"syntax error",
		"$$",
		"BEGIN NULL",
		"RETURN '",
	}
	for _, e := range res.Entries {
		for _, f := range forbidden {
			if strings.Contains(e.Form, f) {
				t.Errorf("no-leak violation in form: %q contains %q", e.Form, f)
			}
			if strings.Contains(e.Notes, f) {
				t.Errorf("no-leak violation in notes: %q contains %q", e.Notes, f)
			}
			if strings.Contains(e.GuidanceCode, f) {
				t.Errorf("no-leak violation in guidance_code: %q contains %q", e.GuidanceCode, f)
			}
		}
	}
}

func TestQueryCatalog_EmptyQueryReturnsAll(t *testing.T) {
	entries := loadTestCatalog(t)

	res := QueryCatalog(entries, CatalogQuery{})
	if res.Total != len(entries) {
		t.Errorf("empty query: got %d, want %d", res.Total, len(entries))
	}
}

func TestLoadCatalog_FileNotFound(t *testing.T) {
	_, err := LoadCatalog("/nonexistent/path/catalog.json")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestQueryCatalog_OrdersMatchCatalog(t *testing.T) {
	entries := loadTestCatalog(t)

	res := QueryCatalog(entries, CatalogQuery{Dialect: "mysql"})
	if res.Total == 0 {
		t.Fatal("expected mysql entries")
	}
	// Verify that result order matches the original catalog order.
	j := 0
	for _, e := range entries {
		if e.Dialect == "mysql" {
			if j >= len(res.Entries) {
				t.Fatalf("ran out of filtered entries at index %d", j)
			}
			if !reflect.DeepEqual(res.Entries[j], e) {
				t.Errorf("entry %d mismatch:\ngot  %+v\nwant %+v", j, res.Entries[j], e)
			}
			j++
		}
	}
}
