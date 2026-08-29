// Package catalog verifies rule catalog discoverability metadata.
// input: built-in policy defaults plus catalog entries with dialects, category, tags, and source
// output: regression coverage for catalog completeness, field validity, deterministic ordering, and discoverability contract
// pos: shipped rule discoverability catalog test coverage
// note: if this file changes, update this header and module README.md.
package catalog

import (
	"encoding/json"
	"sort"
	"testing"

	domainpolicy "github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
)

// TestCatalogNonEmpty verifies the catalog contains entries.
func TestCatalogNonEmpty(t *testing.T) {
	t.Parallel()
	items := All()
	if len(items) == 0 {
		t.Fatal("expected non-empty catalog")
	}
}

// TestEveryRegisteredRuleRepresented verifies every rule in the default policy
// has exactly one catalog entry.
func TestEveryRegisteredRuleRepresented(t *testing.T) {
	t.Parallel()
	defaults := domainpolicy.Default()
	items := All()

	seen := make(map[string]int, len(items))
	for _, entry := range items {
		seen[entry.RuleID]++
	}

	for ruleID := range defaults.Rules {
		count, ok := seen[ruleID]
		if !ok {
			t.Errorf("registered rule %q has no catalog entry", ruleID)
			continue
		}
		if count != 1 {
			t.Errorf("registered rule %q has %d catalog entries, want 1", ruleID, count)
		}
	}
}

// TestEveryCatalogEntryCorrespondsToRealRule verifies no catalog entry is orphaned.
func TestEveryCatalogEntryCorrespondsToRealRule(t *testing.T) {
	t.Parallel()
	defaults := domainpolicy.Default()
	items := All()

	for _, entry := range items {
		if _, ok := defaults.Rules[entry.RuleID]; !ok {
			t.Errorf("catalog entry %q has no corresponding registered rule", entry.RuleID)
		}
	}
}

// TestNoDuplicateRuleID verifies every entry has a unique rule_id.
func TestNoDuplicateRuleID(t *testing.T) {
	t.Parallel()
	items := All()
	seen := make(map[string]int, len(items))
	for _, entry := range items {
		seen[entry.RuleID]++
	}
	for id, count := range seen {
		if count > 1 {
			t.Errorf("rule_id %q appears %d times, want 1", id, count)
		}
	}
}

// TestRequiredFieldsNonEmpty verifies every entry has non-empty required fields.
func TestRequiredFieldsNonEmpty(t *testing.T) {
	t.Parallel()
	for _, entry := range All() {
		if entry.RuleID == "" {
			t.Fatal("found entry with empty rule_id")
		}
		if entry.ConfigKey == "" {
			t.Fatalf("entry %q has empty config_key", entry.RuleID)
		}
		if len(entry.StatementKinds) == 0 {
			t.Fatalf("entry %q has empty statement_kinds (kind)", entry.RuleID)
		}
		if entry.Category == "" {
			t.Fatalf("entry %q has empty category", entry.RuleID)
		}
		if entry.Summary == "" {
			t.Fatalf("entry %q has empty summary", entry.RuleID)
		}
		if entry.Source == "" {
			t.Fatalf("entry %q has empty source", entry.RuleID)
		}
	}
}

// TestEveryEntryHasDialectScope verifies every entry has at least one dialect.
func TestEveryEntryHasDialectScope(t *testing.T) {
	t.Parallel()
	for _, entry := range All() {
		if len(entry.Dialects) == 0 {
			t.Fatalf("entry %q has no dialects", entry.RuleID)
		}
		for _, d := range entry.Dialects {
			switch d {
			case "common", "mysql", "tidb", "postgresql":
				// valid
			default:
				t.Fatalf("entry %q has unknown dialect %q", entry.RuleID, d)
			}
		}
	}
}

// TestLevelVocabulary verifies every entry uses only blocker/warning/notice.
func TestLevelVocabulary(t *testing.T) {
	t.Parallel()
	for _, entry := range All() {
		switch entry.DefaultLevel {
		case rule.LevelBlocker, rule.LevelWarning, rule.LevelNotice:
			// valid
		default:
			t.Fatalf("entry %q has invalid level %q", entry.RuleID, entry.DefaultLevel)
		}
	}
}

// TestEnabledAndLevelMatchDefaultPolicy verifies representative rules match
// their default policy enabled state and level.
func TestEnabledAndLevelMatchDefaultPolicy(t *testing.T) {
	t.Parallel()
	defaults := domainpolicy.Default()

	// Spot-check representative rules.
	representative := []struct {
		ruleID  string
		enabled bool
		level   rule.Level
	}{
		{"ddl.table.primary_key.require", true, rule.LevelBlocker},
		{"ddl.table.comment.require", true, rule.LevelWarning},
		{"ddl.pg.alter.drop_column.advisory", true, rule.LevelWarning},
		{"ddl.pg.table.foreign_key.cross_schema.advisory", true, rule.LevelNotice},
		{"dml.where.require", true, rule.LevelBlocker},
		{"dml.limit.forbid", true, rule.LevelWarning},
		{"ddl.database.create.notice", true, rule.LevelNotice},
	}

	for _, r := range representative {
		policy, ok := defaults.Rules[r.ruleID]
		if !ok {
			t.Fatalf("representative rule %q not found in defaults", r.ruleID)
		}
		entry, ok := Lookup(r.ruleID)
		if !ok {
			t.Fatalf("representative rule %q not found in catalog", r.ruleID)
		}
		if entry.DefaultEnabled != policy.Enabled {
			t.Errorf("rule %q enabled=%t, policy enabled=%t", r.ruleID, entry.DefaultEnabled, policy.Enabled)
		}
		if entry.DefaultEnabled != r.enabled {
			t.Errorf("rule %q enabled=%t, want %t", r.ruleID, entry.DefaultEnabled, r.enabled)
		}
		if entry.DefaultLevel != policy.Level {
			t.Errorf("rule %q level=%s, policy level=%s", r.ruleID, entry.DefaultLevel, policy.Level)
		}
		if entry.DefaultLevel != r.level {
			t.Errorf("rule %q level=%s, want %s", r.ruleID, entry.DefaultLevel, r.level)
		}
	}

	// Also verify ALL entries match their policy.
	items := All()
	for _, entry := range items {
		policy, ok := defaults.Rules[entry.RuleID]
		if !ok {
			continue
		}
		if entry.DefaultEnabled != policy.Enabled {
			t.Errorf("rule %q catalog enabled=%t, policy enabled=%t", entry.RuleID, entry.DefaultEnabled, policy.Enabled)
		}
		if entry.DefaultLevel != policy.Level {
			t.Errorf("rule %q catalog level=%s, policy level=%s", entry.RuleID, entry.DefaultLevel, policy.Level)
		}
	}
}

// TestDeterministicOrdering verifies the catalog has stable sorted ordering.
func TestDeterministicOrdering(t *testing.T) {
	t.Parallel()
	first := All()
	second := All()

	if len(first) != len(second) {
		t.Fatalf("catalog length changed between calls: %d vs %d", len(first), len(second))
	}
	for i := range first {
		if first[i].RuleID != second[i].RuleID {
			t.Fatalf("catalog ordering changed at index %d: %q vs %q", i, first[i].RuleID, second[i].RuleID)
		}
	}

	// Verify sorted by rule_id.
	ids := make([]string, len(first))
	for i, entry := range first {
		ids[i] = entry.RuleID
	}
	if !sort.StringsAreSorted(ids) {
		t.Fatal("catalog entries are not sorted by rule_id")
	}
}

// TestRepresentativeRulesExist verifies known representative rules exist
// with correct dialect/kind/level attributes.
func TestRepresentativeRulesExist(t *testing.T) {
	t.Parallel()
	cases := []struct {
		ruleID   string
		dialect  string
		kind     string
		level    rule.Level
		category string
	}{
		// Common DDL blocker.
		{"ddl.table.primary_key.require", "common", "ddl", rule.LevelBlocker, "table"},
		// Common DDL warning.
		{"ddl.table.comment.require", "common", "ddl", rule.LevelWarning, "table"},
		// PostgreSQL notice.
		{"ddl.pg.table.foreign_key.cross_schema.advisory", "postgresql", "ddl", rule.LevelNotice, "table"},
		// PostgreSQL warning.
		{"ddl.pg.alter.drop_column.advisory", "postgresql", "ddl", rule.LevelWarning, "alter_table"},
		// DML blocker.
		{"dml.where.require", "common", "dml", rule.LevelBlocker, "dml_safety"},
		// DML warning.
		{"dml.limit.forbid", "common", "dml", rule.LevelWarning, "dml_safety"},
		// Common DDL notice.
		{"ddl.database.create.notice", "common", "ddl", rule.LevelNotice, "database"},
		// Alter table category.
		{"ddl.alter.drop_column.forbid", "common", "ddl", rule.LevelWarning, "alter_table"},
	}

	for _, tc := range cases {
		entry, ok := Lookup(tc.ruleID)
		if !ok {
			t.Fatalf("representative rule %q not found", tc.ruleID)
		}
		foundDialect := false
		for _, d := range entry.Dialects {
			if d == tc.dialect {
				foundDialect = true
				break
			}
		}
		if !foundDialect {
			t.Errorf("rule %q: dialects=%v, want %q", tc.ruleID, entry.Dialects, tc.dialect)
		}
		foundKind := false
		for _, k := range entry.StatementKinds {
			if k == tc.kind {
				foundKind = true
				break
			}
		}
		if !foundKind {
			t.Errorf("rule %q: kinds=%v, want %q", tc.ruleID, entry.StatementKinds, tc.kind)
		}
		if entry.DefaultLevel != tc.level {
			t.Errorf("rule %q: level=%s, want %s", tc.ruleID, entry.DefaultLevel, tc.level)
		}
		if entry.Category != tc.category {
			t.Errorf("rule %q: category=%s, want %s", tc.ruleID, entry.Category, tc.category)
		}
	}
}

// TestNoSeverityFieldInJSON verifies no "severity" field appears when entries
// are marshaled to JSON through a discoverability-oriented projection.
func TestNoSeverityFieldInJSON(t *testing.T) {
	t.Parallel()
	// Marshal the first entry as a discoverability projection.
	type discoverabilityEntry struct {
		RuleID     string   `json:"rule_id"`
		ConfigKey  string   `json:"config_key"`
		Level      string   `json:"level"`
		Enabled    bool     `json:"enabled"`
		Kind       string   `json:"kind"`
		Dialects   []string `json:"dialects"`
		Category   string   `json:"category"`
		Summary    string   `json:"summary"`
		Why        string   `json:"why,omitempty"`
		Risk       string   `json:"risk,omitempty"`
		Suggestion string   `json:"suggestion,omitempty"`
		Tags       []string `json:"tags"`
		Source     string   `json:"source"`
	}

	items := All()
	proj := discoverabilityEntry{
		RuleID:     items[0].RuleID,
		ConfigKey:  items[0].ConfigKey,
		Level:      string(items[0].DefaultLevel),
		Enabled:    items[0].DefaultEnabled,
		Kind:       items[0].StatementKinds[0],
		Dialects:   items[0].Dialects,
		Category:   items[0].Category,
		Summary:    items[0].Summary,
		Why:        items[0].Why,
		Risk:       items[0].Risk,
		Suggestion: items[0].Suggestion,
		Tags:       items[0].Tags,
		Source:     items[0].Source,
	}

	data, err := json.Marshal(proj)
	if err != nil {
		t.Fatalf("failed to marshal entry: %v", err)
	}
	raw := string(data)
	if containsInsensitive(raw, "severity") {
		t.Fatalf("JSON contains 'severity': %s", raw)
	}
}

// TestNoUserPayloadInMetadata verifies catalog entries contain no SQL,
// parser near-text, or user-derived content.
func TestNoUserPayloadInMetadata(t *testing.T) {
	t.Parallel()
	for _, entry := range All() {
		// RuleID, ConfigKey, Category, Source should not contain SQL keywords
		// in a way that suggests user payload leakage.
		for _, field := range []string{entry.RuleID, entry.ConfigKey, entry.Category, entry.Source} {
			if containsInsensitive(field, "SELECT ") ||
				containsInsensitive(field, "INSERT ") ||
				containsInsensitive(field, "UPDATE ") ||
				containsInsensitive(field, "DELETE ") ||
				containsInsensitive(field, "near ") {
				t.Fatalf("entry %q field contains user payload: %q", entry.RuleID, field)
			}
		}
	}
}

// TestSupplementalEnrichmentMapsToRealRules verifies that all enrichment
// derivation functions produce valid results for every registered rule.
// This prevents silent drift between enrichment metadata and the rule registry.
func TestSupplementalEnrichmentMapsToRealRules(t *testing.T) {
	t.Parallel()
	defaults := domainpolicy.Default()
	items := All()

	enrichedIDs := make(map[string]bool, len(items))
	for _, entry := range items {
		enrichedIDs[entry.RuleID] = true

		// Verify enrichment functions produce non-empty results.
		dialects := dialectsForRule(entry.RuleID)
		if len(dialects) == 0 {
			t.Errorf("dialectsForRule(%q) returned empty", entry.RuleID)
		}
		category := categoryForRule(entry.RuleID)
		if category == "" {
			t.Errorf("categoryForRule(%q) returned empty", entry.RuleID)
		}
		tags := tagsForRule(entry.RuleID)
		if len(tags) == 0 {
			t.Errorf("tagsForRule(%q) returned empty", entry.RuleID)
		}
	}

	// Every enrichment must correspond to a real rule.
	for ruleID := range defaults.Rules {
		if !enrichedIDs[ruleID] {
			t.Errorf("rule %q has no enrichment entry", ruleID)
		}
	}
}

// TestDialectsForRuleKnownPatterns verifies dialect derivation for representative rules.
func TestDialectsForRuleKnownPatterns(t *testing.T) {
	t.Parallel()
	cases := []struct {
		ruleID  string
		dialect string
	}{
		{"ddl.table.primary_key.require", "common"},
		{"dml.where.require", "common"},
		{"ddl.pg.alter.drop_column.advisory", "postgresql"},
		{"ddl.pg.create_index.concurrently.require", "postgresql"},
		{"ddl.tidb.alter_table.placement_policy.notice", "tidb"},
		{"ddl.alter.merge.mysql.require", "mysql"},
		{"ddl.alter.merge.tidb.require", "tidb"},
	}
	for _, tc := range cases {
		dialects := dialectsForRule(tc.ruleID)
		if len(dialects) != 1 || dialects[0] != tc.dialect {
			t.Errorf("dialectsForRule(%q) = %v, want [%q]", tc.ruleID, dialects, tc.dialect)
		}
	}
	if got := dialectsForRule("dml.table.exists.require"); len(got) != 2 || got[0] != "mysql" || got[1] != "tidb" {
		t.Fatalf("dialectsForRule(dml.table.exists.require) = %v, want [mysql tidb]", got)
	}
}

// TestCategoryForRuleCoversAllRules verifies every registered rule gets a
// non-empty, non-"unknown", non-"other" category.
func TestCategoryForRuleCoversAllRules(t *testing.T) {
	t.Parallel()
	defaults := domainpolicy.Default()
	for ruleID := range defaults.Rules {
		cat := categoryForRule(ruleID)
		if cat == "" {
			t.Errorf("rule %q has empty category", ruleID)
		}
		if cat == "unknown" || cat == "other" {
			t.Errorf("rule %q has fallback category %q — add a mapping", ruleID, cat)
		}
	}
}

// TestSourceAlwaysPolicy verifies all entries derive from policy.
func TestSourceAlwaysPolicy(t *testing.T) {
	t.Parallel()
	for _, entry := range All() {
		if entry.Source != "policy" {
			t.Errorf("entry %q source=%q, want 'policy'", entry.RuleID, entry.Source)
		}
	}
}

// TestConfigKeyMatchesRuleID verifies config_key equals rule_id.
func TestConfigKeyMatchesRuleID(t *testing.T) {
	t.Parallel()
	for _, entry := range All() {
		if entry.ConfigKey != entry.RuleID {
			t.Errorf("entry %q config_key=%q, want %q", entry.RuleID, entry.ConfigKey, entry.RuleID)
		}
	}
}

// TestTagsForRuleContainDialectAndCategory verifies tags include dialect and category.
func TestTagsForRuleContainDialectAndCategory(t *testing.T) {
	t.Parallel()
	entry, ok := Lookup("ddl.table.primary_key.require")
	if !ok {
		t.Fatal("expected ddl.table.primary_key.require to exist")
	}
	tags := entry.Tags

	hasDDL := false
	hasCommon := false
	hasTable := false
	for _, tag := range tags {
		switch tag {
		case "ddl":
			hasDDL = true
		case "common":
			hasCommon = true
		case "table":
			hasTable = true
		}
	}
	if !hasDDL {
		t.Error("expected tags to contain 'ddl'")
	}
	if !hasCommon {
		t.Error("expected tags to contain 'common'")
	}
	if !hasTable {
		t.Error("expected tags to contain 'table' (category)")
	}
}

// containsInsensitive reports whether s contains substr, case-insensitively.
func containsInsensitive(s, substr string) bool {
	sl := len(s)
	subl := len(substr)
	for i := 0; i <= sl-subl; i++ {
		if equalFold(s[i:i+subl], substr) {
			return true
		}
	}
	return false
}

func equalFold(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		ca, cb := a[i], b[i]
		if ca == cb {
			continue
		}
		if ca >= 'A' && ca <= 'Z' {
			ca += 0x20
		}
		if cb >= 'A' && cb <= 'Z' {
			cb += 0x20
		}
		if ca != cb {
			return false
		}
	}
	return true
}
