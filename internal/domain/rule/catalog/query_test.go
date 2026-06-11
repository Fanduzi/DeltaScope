// Package catalog tests the structured query core for rule discoverability.
// input: catalog entries and query filters
// output: filtered results with validation behavior
// pos: v0.290.0 Task 3 query-core test coverage
// note: if this file changes, update this header and module README.md.
package catalog

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
)

// allEntries returns all catalog entries once for reuse across tests.
func allEntries() []Entry {
	return All()
}

// --- 1. Empty query returns all entries ---

func TestQueryEmptyReturnsAll(t *testing.T) {
	t.Parallel()
	entries := allEntries()
	res, err := QueryEntries(entries, Query{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Total != len(entries) {
		t.Errorf("Total=%d, want %d", res.Total, len(entries))
	}
	if len(res.Entries) != len(entries) {
		t.Errorf("len(Entries)=%d, want %d", len(res.Entries), len(entries))
	}
}

// --- 2. Empty result returns non-nil empty slice ---

func TestQueryEmptyResultSet(t *testing.T) {
	t.Parallel()
	res, err := QueryEntries(allEntries(), Query{Dialect: "mysql", Level: "notice"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// mysql + notice might match 0 entries (mysql dialect only has 1 entry at blocker/warning).
	// This tests that we get a non-nil empty slice.
	if res.Entries == nil {
		t.Fatal("Entries is nil, want non-nil empty slice")
	}
	if len(res.Entries) != res.Total {
		t.Errorf("len(Entries)=%d != Total=%d", len(res.Entries), res.Total)
	}
	// Verify we can safely iterate the result.
	for _, e := range res.Entries {
		_ = e.RuleID
	}
}

// --- 3. Dialect filter ---

func TestQueryDialectPostgreSQL(t *testing.T) {
	t.Parallel()
	res, err := QueryEntries(allEntries(), Query{Dialect: "postgresql"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Total == 0 {
		t.Fatal("expected at least one postgresql entry")
	}
	for _, e := range res.Entries {
		found := false
		for _, d := range e.Dialects {
			if d == "postgresql" {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("entry %q dialects=%v does not include postgresql", e.RuleID, e.Dialects)
		}
	}
}

func TestQueryDialectCommon(t *testing.T) {
	t.Parallel()
	res, err := QueryEntries(allEntries(), Query{Dialect: "common"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Total == 0 {
		t.Fatal("expected at least one common entry")
	}
	for _, e := range res.Entries {
		found := false
		for _, d := range e.Dialects {
			if d == "common" {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("entry %q dialects=%v does not include common", e.RuleID, e.Dialects)
		}
	}
}

func TestQueryDialectMySQL(t *testing.T) {
	t.Parallel()
	res, err := QueryEntries(allEntries(), Query{Dialect: "mysql"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, e := range res.Entries {
		found := false
		for _, d := range e.Dialects {
			if d == "mysql" {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("entry %q dialects=%v does not include mysql", e.RuleID, e.Dialects)
		}
	}
}

func TestQueryDialectTiDB(t *testing.T) {
	t.Parallel()
	res, err := QueryEntries(allEntries(), Query{Dialect: "tidb"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, e := range res.Entries {
		found := false
		for _, d := range e.Dialects {
			if d == "tidb" {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("entry %q dialects=%v does not include tidb", e.RuleID, e.Dialects)
		}
	}
}

// --- 4. Level filter ---

func TestQueryLevelBlocker(t *testing.T) {
	t.Parallel()
	res, err := QueryEntries(allEntries(), Query{Level: "blocker"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Total == 0 {
		t.Fatal("expected at least one blocker entry")
	}
	for _, e := range res.Entries {
		if e.DefaultLevel != rule.LevelBlocker {
			t.Errorf("entry %q level=%s, want blocker", e.RuleID, e.DefaultLevel)
		}
	}
}

func TestQueryLevelWarning(t *testing.T) {
	t.Parallel()
	res, err := QueryEntries(allEntries(), Query{Level: "warning"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Total == 0 {
		t.Fatal("expected at least one warning entry")
	}
	for _, e := range res.Entries {
		if e.DefaultLevel != rule.LevelWarning {
			t.Errorf("entry %q level=%s, want warning", e.RuleID, e.DefaultLevel)
		}
	}
}

func TestQueryLevelNotice(t *testing.T) {
	t.Parallel()
	res, err := QueryEntries(allEntries(), Query{Level: "notice"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Total == 0 {
		t.Fatal("expected at least one notice entry")
	}
	for _, e := range res.Entries {
		if e.DefaultLevel != rule.LevelNotice {
			t.Errorf("entry %q level=%s, want notice", e.RuleID, e.DefaultLevel)
		}
	}
}

// --- 5. Kind filter ---

func TestQueryKindDDL(t *testing.T) {
	t.Parallel()
	res, err := QueryEntries(allEntries(), Query{Kind: "ddl"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Total == 0 {
		t.Fatal("expected at least one ddl entry")
	}
	for _, e := range res.Entries {
		found := false
		for _, k := range e.StatementKinds {
			if k == "ddl" {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("entry %q kinds=%v does not include ddl", e.RuleID, e.StatementKinds)
		}
	}
}

func TestQueryKindDML(t *testing.T) {
	t.Parallel()
	res, err := QueryEntries(allEntries(), Query{Kind: "dml"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Total == 0 {
		t.Fatal("expected at least one dml entry")
	}
	for _, e := range res.Entries {
		found := false
		for _, k := range e.StatementKinds {
			if k == "dml" {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("entry %q kinds=%v does not include dml", e.RuleID, e.StatementKinds)
		}
	}
}

func TestQueryKindBatchRejected(t *testing.T) {
	t.Parallel()
	_, err := QueryEntries(allEntries(), Query{Kind: "batch"})
	if err == nil {
		t.Fatal("expected validation error for kind=batch")
	}
	if !strings.Contains(err.Error(), "batch") {
		t.Errorf("error %q should mention 'batch'", err.Error())
	}
}

// --- 6. Category filter is case-insensitive ---

func TestQueryCategoryCaseInsensitive(t *testing.T) {
	t.Parallel()
	resLower, err := QueryEntries(allEntries(), Query{Category: "alter_table"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resUpper, err := QueryEntries(allEntries(), Query{Category: "ALTER_TABLE"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resMixed, err := QueryEntries(allEntries(), Query{Category: "Alter_Table"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resLower.Total != resUpper.Total {
		t.Errorf("lower total=%d != upper total=%d", resLower.Total, resUpper.Total)
	}
	if resLower.Total != resMixed.Total {
		t.Errorf("lower total=%d != mixed total=%d", resLower.Total, resMixed.Total)
	}
	if resLower.Total == 0 {
		t.Fatal("expected at least one alter_table entry")
	}
}

// --- 7. Search is case-insensitive ---

func TestQuerySearchCaseInsensitive(t *testing.T) {
	t.Parallel()
	resLower, err := QueryEntries(allEntries(), Query{Search: "primary_key"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resUpper, err := QueryEntries(allEntries(), Query{Search: "PRIMARY_KEY"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resLower.Total != resUpper.Total {
		t.Errorf("lower total=%d != upper total=%d", resLower.Total, resUpper.Total)
	}
	if resLower.Total == 0 {
		t.Fatal("expected at least one match for 'primary_key'")
	}
}

// --- 8. Search matches rule_id ---

func TestQuerySearchMatchesRuleID(t *testing.T) {
	t.Parallel()
	needle := "dml.where.require"
	res, err := QueryEntries(allEntries(), Query{Search: needle})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Total == 0 {
		t.Fatal("expected to find dml.where.require by rule_id")
	}
	found := false
	for _, e := range res.Entries {
		if e.RuleID == needle {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("rule_id %q not found in results (total=%d)", needle, res.Total)
	}
}

// --- 9. Search matches config_key ---

func TestQuerySearchMatchesConfigKey(t *testing.T) {
	t.Parallel()
	// config_key equals rule_id for all entries, so this is equivalent to rule_id search.
	// Verify explicitly by searching for the exact config_key value.
	needle := "ddl.table.comment.require"
	res, err := QueryEntries(allEntries(), Query{Search: needle})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	found := false
	for _, e := range res.Entries {
		if e.ConfigKey == needle {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("config_key %q not matched by search", needle)
	}
}

// --- 10. Search matches summary/why/risk/suggestion ---

func TestQuerySearchMatchesSummary(t *testing.T) {
	t.Parallel()
	// "Require" appears in many summaries.
	res, err := QueryEntries(allEntries(), Query{Search: "require"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Total == 0 {
		t.Fatal("expected at least one entry matching 'require' in summary")
	}
}

func TestQuerySearchMatchesWhy(t *testing.T) {
	t.Parallel()
	// Why fields contain "shipped policy" for most rules.
	res, err := QueryEntries(allEntries(), Query{Search: "shipped policy"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Total == 0 {
		t.Fatal("expected at least one entry matching 'shipped policy' in why")
	}
}

func TestQuerySearchMatchesRisk(t *testing.T) {
	t.Parallel()
	// Risk fields contain "safety review" for DML rules.
	res, err := QueryEntries(allEntries(), Query{Search: "safety review"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Total == 0 {
		t.Fatal("expected at least one entry matching 'safety review' in risk")
	}
}

func TestQuerySearchMatchesSuggestion(t *testing.T) {
	t.Parallel()
	// Suggestion is the same as RemediationHint; contains "policy" for many.
	res, err := QueryEntries(allEntries(), Query{Search: "governance rule"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Total == 0 {
		t.Fatal("expected at least one entry matching 'governance rule' in suggestion")
	}
}

// --- 11. Search matches tags ---

func TestQuerySearchMatchesTags(t *testing.T) {
	t.Parallel()
	// Tags include "forbid" for .forbid rules.
	res, err := QueryEntries(allEntries(), Query{Search: "forbid"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Total == 0 {
		t.Fatal("expected at least one entry matching 'forbid' in tags")
	}
	// Verify at least one result actually has "forbid" in tags.
	tagFound := false
	for _, e := range res.Entries {
		for _, tag := range e.Tags {
			if strings.EqualFold(tag, "forbid") {
				tagFound = true
				break
			}
		}
		if tagFound {
			break
		}
	}
	if !tagFound {
		t.Error("no result has 'forbid' in tags; search may not be matching tags")
	}
}

// --- 12. Combined filters narrow results ---

func TestQueryCombinedFilters(t *testing.T) {
	t.Parallel()
	res, err := QueryEntries(allEntries(), Query{Dialect: "postgresql", Level: "warning"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Total == 0 {
		t.Fatal("expected at least one postgresql+warning entry")
	}
	for _, e := range res.Entries {
		// Check dialect.
		foundD := false
		for _, d := range e.Dialects {
			if d == "postgresql" {
				foundD = true
				break
			}
		}
		if !foundD {
			t.Errorf("entry %q is not postgresql", e.RuleID)
		}
		// Check level.
		if e.DefaultLevel != rule.LevelWarning {
			t.Errorf("entry %q level=%s, want warning", e.RuleID, e.DefaultLevel)
		}
	}

	// Verify combined result is smaller than either individual filter.
	resPG, _ := QueryEntries(allEntries(), Query{Dialect: "postgresql"})
	resWarn, _ := QueryEntries(allEntries(), Query{Level: "warning"})
	if res.Total > resPG.Total {
		t.Errorf("combined total=%d > postgresql total=%d", res.Total, resPG.Total)
	}
	if res.Total > resWarn.Total {
		t.Errorf("combined total=%d > warning total=%d", res.Total, resWarn.Total)
	}
}

func TestQueryCombinedDialectKindAndSearch(t *testing.T) {
	t.Parallel()
	res, err := QueryEntries(allEntries(), Query{Dialect: "common", Kind: "dml", Search: "require"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, e := range res.Entries {
		// Must have common dialect.
		foundD := false
		for _, d := range e.Dialects {
			if d == "common" {
				foundD = true
				break
			}
		}
		if !foundD {
			t.Errorf("entry %q is not common dialect", e.RuleID)
		}
		// Must be dml kind.
		foundK := false
		for _, k := range e.StatementKinds {
			if k == "dml" {
				foundK = true
				break
			}
		}
		if !foundK {
			t.Errorf("entry %q is not dml kind", e.RuleID)
		}
	}
}

// --- 13. Limit truncates deterministically ---

func TestQueryLimitTruncates(t *testing.T) {
	t.Parallel()
	res, err := QueryEntries(allEntries(), Query{Limit: 5})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(res.Entries) != 5 {
		t.Errorf("len(Entries)=%d, want 5", len(res.Entries))
	}
	// Total should reflect pre-limit count.
	if res.Total != len(allEntries()) {
		t.Errorf("Total=%d, want %d (total before limit)", res.Total, len(allEntries()))
	}
}

// --- 14. Limit 0 returns all matches ---

func TestQueryLimitZeroReturnsAll(t *testing.T) {
	t.Parallel()
	entries := allEntries()
	res, err := QueryEntries(entries, Query{Limit: 0})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Total != len(entries) {
		t.Errorf("Total=%d, want %d", res.Total, len(entries))
	}
	if len(res.Entries) != len(entries) {
		t.Errorf("len(Entries)=%d, want %d", len(res.Entries), len(entries))
	}
}

// --- 15. Repeated queries produce identical order ---

func TestQueryDeterministicOrdering(t *testing.T) {
	t.Parallel()
	q := Query{Dialect: "common", Level: "blocker"}
	res1, err := QueryEntries(allEntries(), q)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	res2, err := QueryEntries(allEntries(), q)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res1.Total != res2.Total {
		t.Fatalf("total changed between calls: %d vs %d", res1.Total, res2.Total)
	}
	for i := range res1.Entries {
		if res1.Entries[i].RuleID != res2.Entries[i].RuleID {
			t.Fatalf("order changed at index %d: %q vs %q", i, res1.Entries[i].RuleID, res2.Entries[i].RuleID)
		}
	}
}

// --- 16. Invalid dialect returns validation error ---

func TestQueryInvalidDialect(t *testing.T) {
	t.Parallel()
	_, err := QueryEntries(allEntries(), Query{Dialect: "oracle"})
	if err == nil {
		t.Fatal("expected validation error for dialect=oracle")
	}
	if !strings.Contains(err.Error(), "oracle") {
		t.Errorf("error %q should mention 'oracle'", err.Error())
	}
	if !strings.Contains(err.Error(), "dialect") {
		t.Errorf("error %q should mention 'dialect'", err.Error())
	}
}

// --- 17. Invalid level returns validation error ---

func TestQueryInvalidLevel(t *testing.T) {
	t.Parallel()
	_, err := QueryEntries(allEntries(), Query{Level: "critical"})
	if err == nil {
		t.Fatal("expected validation error for level=critical")
	}
	if !strings.Contains(err.Error(), "critical") {
		t.Errorf("error %q should mention 'critical'", err.Error())
	}
	if !strings.Contains(err.Error(), "level") {
		t.Errorf("error %q should mention 'level'", err.Error())
	}
}

// --- 18. Invalid kind returns validation error ---

func TestQueryInvalidKind(t *testing.T) {
	t.Parallel()
	_, err := QueryEntries(allEntries(), Query{Kind: "tcl"})
	if err == nil {
		t.Fatal("expected validation error for kind=tcl")
	}
	if !strings.Contains(err.Error(), "tcl") {
		t.Errorf("error %q should mention 'tcl'", err.Error())
	}
	if !strings.Contains(err.Error(), "kind") {
		t.Errorf("error %q should mention 'kind'", err.Error())
	}
}

// --- 19. Negative limit returns validation error ---

func TestQueryNegativeLimit(t *testing.T) {
	t.Parallel()
	_, err := QueryEntries(allEntries(), Query{Limit: -1})
	if err == nil {
		t.Fatal("expected validation error for limit=-1")
	}
	if !strings.Contains(err.Error(), "-1") {
		t.Errorf("error %q should mention '-1'", err.Error())
	}
	if !strings.Contains(err.Error(), "limit") {
		t.Errorf("error %q should mention 'limit'", err.Error())
	}
}

// --- 20. Query validation accepts all valid enum values ---

func TestQueryValidateAcceptsAllValid(t *testing.T) {
	t.Parallel()
	validQueries := []Query{
		{Dialect: "mysql"},
		{Dialect: "tidb"},
		{Dialect: "postgresql"},
		{Dialect: "common"},
		{Level: "blocker"},
		{Level: "warning"},
		{Level: "notice"},
		{Kind: "ddl"},
		{Kind: "dml"},
		{Limit: 0},
		{Limit: 100},
		{Dialect: "common", Level: "blocker", Kind: "ddl", Category: "table", Search: "primary", Limit: 10},
	}
	for i, q := range validQueries {
		if err := q.Validate(); err != nil {
			t.Errorf("query %d (%+v): unexpected error: %v", i, q, err)
		}
	}
}

// --- 21. No result metadata contains parser near, raw SQL, or severity ---

func TestQueryResultNoUnsafeContent(t *testing.T) {
	t.Parallel()
	res, err := QueryEntries(allEntries(), Query{Limit: 20})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, e := range res.Entries {
		for _, field := range []string{e.RuleID, e.ConfigKey, e.Category, e.Source} {
			if strings.Contains(strings.ToLower(field), "near ") {
				t.Errorf("entry %q field contains 'near ': %q", e.RuleID, field)
			}
			if strings.Contains(strings.ToLower(field), "select ") {
				t.Errorf("entry %q field contains 'select ': %q", e.RuleID, field)
			}
		}
		if strings.Contains(strings.ToLower(e.Summary), "severity") {
			t.Errorf("entry %q summary contains 'severity': %q", e.RuleID, e.Summary)
		}
	}
}

// --- 22. JSON marshaling confirms no severity field ---

func TestQueryResultJSONNoSeverity(t *testing.T) {
	t.Parallel()
	type queryResultJSON struct {
		Entries []json.RawMessage `json:"entries"`
		Total   int               `json:"total"`
	}

	res, err := QueryEntries(allEntries(), Query{Limit: 5})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Marshal a projection of each entry and check for "severity".
	for _, e := range res.Entries {
		proj := map[string]any{
			"rule_id":     e.RuleID,
			"config_key":  e.ConfigKey,
			"level":       string(e.DefaultLevel),
			"enabled":     e.DefaultEnabled,
			"kind":        e.StatementKinds[0],
			"dialects":    e.Dialects,
			"category":    e.Category,
			"summary":     e.Summary,
			"why":         e.Why,
			"risk":        e.Risk,
			"suggestion":  e.Suggestion,
			"tags":        e.Tags,
			"source":      e.Source,
		}
		data, err := json.Marshal(proj)
		if err != nil {
			t.Fatalf("failed to marshal entry %q: %v", e.RuleID, err)
		}
		raw := string(data)
		if strings.Contains(strings.ToLower(raw), "severity") {
			t.Errorf("entry %q JSON contains 'severity': %s", e.RuleID, raw)
		}
	}

	// Also marshal the full Result.
	fullResult := map[string]any{
		"total":   res.Total,
		"entries": res.Entries,
	}
	data, err := json.Marshal(fullResult)
	if err != nil {
		t.Fatalf("failed to marshal result: %v", err)
	}
	if strings.Contains(strings.ToLower(string(data)), "severity") {
		t.Fatalf("full result JSON contains 'severity': %s", string(data[:200]))
	}
}

// --- Additional: Category substring match ---

func TestQueryCategorySubstring(t *testing.T) {
	t.Parallel()
	// "alter" should match "alter_table" category.
	res, err := QueryEntries(allEntries(), Query{Category: "alter"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Total == 0 {
		t.Fatal("expected entries matching category substring 'alter'")
	}
	for _, e := range res.Entries {
		if !strings.Contains(strings.ToLower(e.Category), "alter") {
			t.Errorf("entry %q category=%q does not contain 'alter'", e.RuleID, e.Category)
		}
	}
}

// --- Additional: Validate method returns nil for empty query ---

func TestValidateEmptyQuery(t *testing.T) {
	t.Parallel()
	q := Query{}
	if err := q.Validate(); err != nil {
		t.Errorf("empty query should be valid: %v", err)
	}
}
