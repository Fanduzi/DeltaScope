//go:build postgresql

// Package audit verifies the generated DDL coverage catalog against census baselines.
// input: census fixtures and the checked-in docs/reference plus catalogdata JSON copies
// output: drift detection for generated catalog JSON used by docs and the embedded binary copy
// pos: catalog generator and release drift gate
// note: if this file changes, update this header and module README.md.
package audit

import (
	"encoding/json"
	"os"
	"sort"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// --- Catalog JSON types ---

type catalogEntryJSON struct {
	Dialect        string   `json:"dialect"`
	Family         string   `json:"family"`
	Form           string   `json:"form"`
	Classification string   `json:"classification"`
	FindingRuleIDs []string `json:"finding_rule_ids"`
	GuidanceCode   string   `json:"guidance_code,omitempty"`
	EvidenceRef    string   `json:"evidence_ref,omitempty"`
	Notes          string   `json:"notes"`
}

type catalogSummaryJSON struct {
	Total               int `json:"total"`
	FindingCovered      int `json:"finding_covered"`
	NormalizedSilent    int `json:"normalized_silent"`
	UnsupportedBoundary int `json:"unsupported_boundary"`
	ParserError         int `json:"parser_error"`
	Unclassified        int `json:"unclassified"`
}

type catalogJSON struct {
	Version       string                         `json:"version"`
	GeneratedFrom []string                       `json:"generated_from"`
	Summary       map[string]*catalogSummaryJSON `json:"summary"`
	Entries       []catalogEntryJSON             `json:"entries"`
}

// --- Constants ---

const (
	catalogVersion      = "v0.270.0"
	catalogJSONRelPath  = "../../../docs/reference/ddl-coverage-catalog.json"
	catalogEmbedRelPath = "catalogdata/ddl-coverage-catalog.json"
	catalogProjectRoot  = "../../../"
)

var catalogGeneratedFrom = []string{
	"internal/application/audit/cross_dialect_ddl_coverage_census_test.go",
	"internal/application/audit/postgresql_ddl_coverage_census_postgresql_tag_test.go",
	"internal/application/audit/postgresql_ddl_completion_census_postgresql_tag_test.go",
	"internal/application/audit/postgresql_ddl_deep_coverage_census_postgresql_tag_test.go",
	"internal/application/audit/postgresql_ddl_long_tail_census_postgresql_tag_test.go",
	"internal/application/audit/postgresql_alter_table_residual_census_postgresql_tag_test.go",
	"internal/application/audit/ddl_parser_error_feasibility_census_test.go",
	"internal/application/audit/ddl_parser_error_feasibility_census_postgresql_tag_test.go",
}

var allowedDialects = map[string]bool{
	"mysql": true, "tidb": true, "postgresql": true,
}

var allowedClassifications = map[string]bool{
	"finding_covered":      true,
	"normalized_silent":    true,
	"unsupported_boundary": true,
	"parser_error":         true,
	"unclassified":         true,
}

var requiredParserUpgradeCandidates = []struct {
	dialect string
	form    string
}{
	{"mysql", "ALTER VIEW"},
	{"mysql", "ALTER PROCEDURE"},
	{"mysql", "CREATE FUNCTION"},
	{"mysql", "ALTER FUNCTION"},
	{"mysql", "DROP FUNCTION"},
	{"postgresql", "DROP SUBSCRIPTION WITH drop_slot"},
	{"postgresql", "NOT NULL NOT VALID"},
	{"postgresql", "ALTER CONSTRAINT NOT ENFORCED"},
	{"postgresql", "ALTER CONSTRAINT INHERIT"},
	{"postgresql", "ALTER CONSTRAINT NO INHERIT"},
}

// --- Family derivation ---

type familyRule struct {
	prefix string
	family string
}

var familyRules = []familyRule{
	// Order: most specific prefix first.
	{"CREATE TEMPORARY TABLE", "table_lifecycle"},
	{"CREATE TEMP TABLE", "table_lifecycle"},
	{"CREATE TABLE", "table_lifecycle"},
	{"DROP TABLE", "table_lifecycle"},
	{"TRUNCATE TABLE", "table_lifecycle"},
	{"RENAME TABLE", "table_lifecycle"},
	{"CREATE TABLE AS SELECT", "table_lifecycle"},

	{"ALTER TABLE", "alter_table"},

	{"CREATE UNIQUE INDEX", "index_lifecycle"},
	{"CREATE FULLTEXT INDEX", "index_lifecycle"},
	{"CREATE SPATIAL INDEX", "index_lifecycle"},
	{"CREATE INDEX", "index_lifecycle"},
	{"DROP INDEX", "index_lifecycle"},

	{"CREATE DATABASE", "schema_lifecycle"},
	{"CREATE SCHEMA", "schema_lifecycle"},
	{"ALTER DATABASE", "schema_lifecycle"},
	{"DROP DATABASE", "schema_lifecycle"},
	{"DROP SCHEMA", "schema_lifecycle"},
	{"ALTER SCHEMA", "schema_lifecycle"},

	{"CREATE OR REPLACE VIEW", "view_lifecycle"},
	{"CREATE VIEW", "view_lifecycle"},
	{"CREATE TEMP VIEW", "view_lifecycle"},
	{"ALTER VIEW", "view_lifecycle"},
	{"DROP VIEW", "view_lifecycle"},

	{"CREATE OR REPLACE FUNCTION", "routine_lifecycle"},
	{"CREATE FUNCTION", "routine_lifecycle"},
	{"CREATE FUNCTION SECURITY DEFINER", "routine_lifecycle"},
	{"ALTER FUNCTION", "routine_lifecycle"},
	{"DROP FUNCTION", "routine_lifecycle"},
	{"CREATE PROCEDURE", "routine_lifecycle"},
	{"ALTER PROCEDURE", "routine_lifecycle"},
	{"DROP PROCEDURE", "routine_lifecycle"},

	{"CREATE TRIGGER", "trigger_lifecycle"},
	{"CREATE CONSTRAINT TRIGGER", "trigger_lifecycle"},
	{"DROP TRIGGER", "trigger_lifecycle"},

	{"CREATE EVENT", "event_lifecycle"},
	{"ALTER EVENT", "event_lifecycle"},
	{"DROP EVENT", "event_lifecycle"},

	{"CREATE USER", "privilege_lifecycle"},
	{"ALTER USER", "privilege_lifecycle"},
	{"DROP USER", "privilege_lifecycle"},
	{"CREATE ROLE", "privilege_lifecycle"},
	{"DROP ROLE", "privilege_lifecycle"},
	{"GRANT", "privilege_lifecycle"},
	{"REVOKE", "privilege_lifecycle"},

	{"CREATE TABLESPACE", "tablespace_lifecycle"},
	{"ALTER TABLESPACE", "tablespace_lifecycle"},
	{"DROP TABLESPACE", "tablespace_lifecycle"},

	{"CREATE RESOURCE GROUP", "resource_group_lifecycle"},
	{"ALTER RESOURCE GROUP", "resource_group_lifecycle"},
	{"DROP RESOURCE GROUP", "resource_group_lifecycle"},

	{"CREATE PLACEMENT POLICY", "placement_policy"},
	{"ALTER PLACEMENT POLICY", "placement_policy"},
	{"DROP PLACEMENT POLICY", "placement_policy"},

	{"CREATE SEQUENCE", "sequence_lifecycle"},
	{"ALTER SEQUENCE", "sequence_lifecycle"},
	{"DROP SEQUENCE", "sequence_lifecycle"},

	{"CREATE TYPE", "type_lifecycle"},
	{"ALTER TYPE", "type_lifecycle"},
	{"DROP TYPE", "type_lifecycle"},

	{"CREATE DOMAIN", "domain_lifecycle"},
	{"ALTER DOMAIN", "domain_lifecycle"},
	{"DROP DOMAIN", "domain_lifecycle"},

	{"CREATE MATERIALIZED VIEW", "materialized_view_lifecycle"},
	{"DROP MATERIALIZED VIEW", "materialized_view_lifecycle"},
	{"REFRESH MATERIALIZED VIEW", "materialized_view_lifecycle"},
	{"ALTER MATERIALIZED VIEW", "materialized_view_lifecycle"},

	{"COMMENT ON", "annotation"},
	{"SECURITY LABEL", "annotation"},

	{"CREATE EXTENSION", "extension_lifecycle"},
	{"ALTER EXTENSION", "extension_lifecycle"},
	{"DROP EXTENSION", "extension_lifecycle"},

	{"CREATE POLICY", "rls_lifecycle"},
	{"ALTER POLICY", "rls_lifecycle"},
	{"DROP POLICY", "rls_lifecycle"},

	{"CREATE PUBLICATION", "publication_lifecycle"},
	{"ALTER PUBLICATION", "publication_lifecycle"},
	{"DROP PUBLICATION", "publication_lifecycle"},

	{"CREATE SUBSCRIPTION", "subscription_lifecycle"},
	{"ALTER SUBSCRIPTION", "subscription_lifecycle"},
	{"DROP SUBSCRIPTION", "subscription_lifecycle"},

	{"CREATE FOREIGN TABLE", "foreign_table_lifecycle"},
	{"ALTER FOREIGN TABLE", "foreign_table_lifecycle"},
	{"DROP FOREIGN TABLE", "foreign_table_lifecycle"},

	{"CREATE SERVER", "foreign_server_lifecycle"},
	{"ALTER SERVER", "foreign_server_lifecycle"},
	{"DROP SERVER", "foreign_server_lifecycle"},

	{"CREATE USER MAPPING", "user_mapping_lifecycle"},
	{"ALTER USER MAPPING", "user_mapping_lifecycle"},
	{"DROP USER MAPPING", "user_mapping_lifecycle"},

	{"CREATE FOREIGN DATA WRAPPER", "fdw_lifecycle"},
	{"ALTER FOREIGN DATA WRAPPER", "fdw_lifecycle"},
	{"DROP FOREIGN DATA WRAPPER", "fdw_lifecycle"},

	{"CREATE EVENT TRIGGER", "event_trigger_lifecycle"},
	{"ALTER EVENT TRIGGER", "event_trigger_lifecycle"},
	{"DROP EVENT TRIGGER", "event_trigger_lifecycle"},

	{"CREATE RULE", "rule_lifecycle"},
	{"ALTER RULE", "rule_lifecycle"},
	{"DROP RULE", "rule_lifecycle"},

	{"CREATE TEXT SEARCH", "text_search_lifecycle"},
	{"ALTER TEXT SEARCH", "text_search_lifecycle"},
	{"DROP TEXT SEARCH", "text_search_lifecycle"},

	{"CREATE COLLATION", "collation_lifecycle"},
	{"ALTER COLLATION", "collation_lifecycle"},
	{"DROP COLLATION", "collation_lifecycle"},

	{"CREATE STATISTICS", "statistics_lifecycle"},
	{"ALTER STATISTICS", "statistics_lifecycle"},
	{"DROP STATISTICS", "statistics_lifecycle"},

	{"CREATE AGGREGATE", "aggregate_lifecycle"},
	{"ALTER AGGREGATE", "aggregate_lifecycle"},
	{"DROP AGGREGATE", "aggregate_lifecycle"},

	{"CREATE OPERATOR CLASS", "operator_lifecycle"},
	{"ALTER OPERATOR CLASS", "operator_lifecycle"},
	{"DROP OPERATOR CLASS", "operator_lifecycle"},
	{"CREATE OPERATOR FAMILY", "operator_lifecycle"},
	{"ALTER OPERATOR FAMILY", "operator_lifecycle"},
	{"DROP OPERATOR FAMILY", "operator_lifecycle"},
	{"CREATE OPERATOR", "operator_lifecycle"},
	{"ALTER OPERATOR", "operator_lifecycle"},
	{"DROP OPERATOR", "operator_lifecycle"},

	{"CREATE CONVERSION", "conversion_lifecycle"},
	{"ALTER CONVERSION", "conversion_lifecycle"},
	{"DROP CONVERSION", "conversion_lifecycle"},

	{"CREATE TRANSFORM", "transform_lifecycle"},
	{"DROP TRANSFORM", "transform_lifecycle"},
	{"CREATE ACCESS METHOD", "access_method_lifecycle"},
	{"DROP ACCESS METHOD", "access_method_lifecycle"},
	{"ALTER LARGE OBJECT", "large_object_lifecycle"},

	{"ALTER INDEX", "index_lifecycle"},
}

func deriveCatalogFamily(form string) string {
	upper := strings.ToUpper(form)
	for _, r := range familyRules {
		if strings.HasPrefix(upper, r.prefix) {
			return r.family
		}
	}
	return "other"
}

// --- Intermediate entry type ---

type intermediateCatalogEntry struct {
	dialect        string
	family         string
	form           string
	sql            string
	dialectSpec    spec.Dialect
	classification string
	findingRuleIDs []string
}

func (e intermediateCatalogEntry) toCatalogEntry() catalogEntryJSON {
	notes := ""
	switch e.classification {
	case "normalized_silent":
		notes = "normalized, no finding under default policy"
	case "unsupported_boundary":
		notes = "recognized product boundary"
	case "parser_error":
		notes = "parser error"
	case "unclassified":
		notes = "unclassified"
	}

	gc, ref := "", ""
	if e.classification == "parser_error" {
		gc, ref = classifyParserErrorGuidance(e.sql, e.dialectSpec)
	}

	rids := e.findingRuleIDs
	if rids == nil {
		rids = []string{}
	}

	return catalogEntryJSON{
		Dialect:        e.dialect,
		Family:         e.family,
		Form:           e.form,
		Classification: e.classification,
		FindingRuleIDs: rids,
		GuidanceCode:   gc,
		EvidenceRef:    ref,
		Notes:          notes,
	}
}

// --- Builder functions ---

func buildCensusCatalogEntries(t *testing.T, dialectName string, dialect spec.Dialect, cases []struct {
	Name     string
	SQL      string
	Expected ddlCoverageClass
}) []intermediateCatalogEntry {
	t.Helper()
	entries := make([]intermediateCatalogEntry, 0, len(cases))
	for _, tc := range cases {
		res := classifyDDLCensusResult(t, tc.Name, tc.SQL, dialectName, dialect)
		entries = append(entries, intermediateCatalogEntry{
			dialect:        strings.ToLower(dialectName),
			family:         deriveCatalogFamily(tc.Name),
			form:           tc.Name,
			sql:            tc.SQL,
			dialectSpec:    dialect,
			classification: string(res.Classification),
			findingRuleIDs: res.FindingRuleIDs,
		})
	}
	return entries
}

func buildPGRepresentativeCatalogEntries(t *testing.T) []intermediateCatalogEntry {
	t.Helper()
	entries := make([]intermediateCatalogEntry, 0, len(pgDDLCensusCases))
	for _, tc := range pgDDLCensusCases {
		if tc.DeferReason != "" {
			continue
		}
		res := runCensusCase(t, tc)
		classification := "unclassified"
		switch res.Status {
		case statusFindingCovered:
			classification = "finding_covered"
		case statusNormalizedSilent:
			classification = "normalized_silent"
		case statusUnsupportedExp:
			classification = "unsupported_boundary"
		case statusParserError:
			classification = "parser_error"
		}
		rids := append([]string{}, res.StmtFindings...)
		rids = append(rids, res.GlobalFindings...)
		sort.Strings(rids)
		entries = append(entries, intermediateCatalogEntry{
			dialect:        "postgresql",
			family:         deriveCatalogFamily(tc.Name),
			form:           tc.Name,
			sql:            tc.SQL,
			dialectSpec:    spec.DialectPostgreSQL,
			classification: classification,
			findingRuleIDs: rids,
		})
	}
	return entries
}

func buildPGResidualCatalogEntries(t *testing.T) []intermediateCatalogEntry {
	t.Helper()
	entries := make([]intermediateCatalogEntry, 0, len(pgAlterTableResidualCases))
	for _, tc := range pgAlterTableResidualCases {
		res := classifyAlterTableResidualCase(t, tc)
		classification := "unclassified"
		switch res.Classification {
		case residualFindingCovered:
			classification = "finding_covered"
		case residualNormalizedSilent:
			classification = "normalized_silent"
		case residualUnsupportedBound:
			classification = "unsupported_boundary"
		case residualParserError:
			classification = "parser_error"
		}
		entries = append(entries, intermediateCatalogEntry{
			dialect:        "postgresql",
			family:         tc.Family,
			form:           tc.Name,
			sql:            tc.SQL,
			dialectSpec:    spec.DialectPostgreSQL,
			classification: classification,
			findingRuleIDs: res.FindingRuleIDs,
		})
	}
	return entries
}

// --- Summary computation ---

func computeCatalogSummary(entries []intermediateCatalogEntry) *catalogSummaryJSON {
	s := &catalogSummaryJSON{Total: len(entries)}
	for _, e := range entries {
		switch e.classification {
		case "finding_covered":
			s.FindingCovered++
		case "normalized_silent":
			s.NormalizedSilent++
		case "unsupported_boundary":
			s.UnsupportedBoundary++
		case "parser_error":
			s.ParserError++
		default:
			s.Unclassified++
		}
	}
	return s
}

func assertCatalogSummary(t *testing.T, name string, got *catalogSummaryJSON, total, finding, silent, unsupported, parserErr, unclassified int) {
	t.Helper()
	if got.Total != total {
		t.Errorf("catalog summary %s: expected total=%d, got %d", name, total, got.Total)
	}
	if got.FindingCovered != finding {
		t.Errorf("catalog summary %s: expected finding_covered=%d, got %d", name, finding, got.FindingCovered)
	}
	if got.NormalizedSilent != silent {
		t.Errorf("catalog summary %s: expected normalized_silent=%d, got %d", name, silent, got.NormalizedSilent)
	}
	if got.UnsupportedBoundary != unsupported {
		t.Errorf("catalog summary %s: expected unsupported_boundary=%d, got %d", name, unsupported, got.UnsupportedBoundary)
	}
	if got.ParserError != parserErr {
		t.Errorf("catalog summary %s: expected parser_error=%d, got %d", name, parserErr, got.ParserError)
	}
	if got.Unclassified != unclassified {
		t.Errorf("catalog summary %s: expected unclassified=%d, got %d", name, unclassified, got.Unclassified)
	}
}

// --- Validation functions ---

func validateCatalogEntries(t *testing.T, cat catalogJSON) {
	t.Helper()
	for i, e := range cat.Entries {
		if !allowedDialects[e.Dialect] {
			t.Errorf("entry %d: invalid dialect %q", i, e.Dialect)
		}
		if !allowedClassifications[e.Classification] {
			t.Errorf("entry %d: invalid classification %q", i, e.Classification)
		}
		if e.Classification == "unclassified" {
			t.Errorf("entry %d (%s/%s): unclassified entries not allowed in release catalog", i, e.Dialect, e.Form)
		}
		if e.Form == "" {
			t.Errorf("entry %d: empty form", i)
		}
		if e.Family == "" {
			t.Errorf("entry %d: empty family", i)
		}
	}
}

func validateRequiredParserUpgradeCandidates(t *testing.T, cat catalogJSON) {
	t.Helper()
	for _, req := range requiredParserUpgradeCandidates {
		found := false
		for _, e := range cat.Entries {
			if e.Dialect == req.dialect && e.Form == req.form {
				if e.GuidanceCode != "parser_upgrade_candidate" {
					t.Errorf("required candidate %s/%s: expected guidance_code=parser_upgrade_candidate, got %q",
						req.dialect, req.form, e.GuidanceCode)
				}
				if e.EvidenceRef == "" {
					t.Errorf("required candidate %s/%s: missing evidence_ref", req.dialect, req.form)
				}
				found = true
				break
			}
		}
		if !found {
			t.Errorf("required parser-upgrade candidate not found in catalog: %s/%s", req.dialect, req.form)
		}
	}
}

func validateNoLeak(t *testing.T, cat catalogJSON) {
	t.Helper()
	jsonBytes, err := json.Marshal(cat)
	if err != nil {
		t.Fatalf("no-leak check: failed to marshal catalog: %v", err)
	}
	jsonStr := string(jsonBytes)

	forbidden := []string{
		"near \"",
		"near `",
		"Syntax error",
		"syntax error",
		"$$",
		"BEGIN NULL",
		"RETURN '",
	}
	for _, f := range forbidden {
		if strings.Contains(jsonStr, f) {
			t.Errorf("no-leak violation: catalog JSON contains forbidden pattern %q", f)
		}
	}
}

func validateGeneratedFromPaths(t *testing.T) {
	t.Helper()
	for _, path := range catalogGeneratedFrom {
		full := catalogProjectRoot + path
		if _, err := os.Stat(full); os.IsNotExist(err) {
			t.Errorf("generated_from path does not exist: %s", path)
		}
	}
}

// --- Main test ---

func TestDDLCoverageCatalog(t *testing.T) {
	// Build entries from all census sources.
	mysqlEntries := buildCensusCatalogEntries(t, "MySQL", spec.DialectMySQL, mysqlDDLCensusCases)
	tidbEntries := buildCensusCatalogEntries(t, "TiDB", spec.DialectTiDB, tidbDDLCensusCases)
	pgRepEntries := buildPGRepresentativeCatalogEntries(t)
	pgCompEntries := buildCensusCatalogEntries(t, "PostgreSQL", spec.DialectPostgreSQL, pgDDLCompletionCensusCases)
	pgDeepEntries := buildCensusCatalogEntries(t, "PostgreSQL", spec.DialectPostgreSQL, pgDDLDeepCoverageCensusCases)
	pgLTEntries := buildCensusCatalogEntries(t, "PostgreSQL", spec.DialectPostgreSQL, pgDDLLongTailCensusCases)
	pgResEntries := buildPGResidualCatalogEntries(t)

	// PG all = representative + completion + deep + long-tail + residual.
	var pgAllEntries []intermediateCatalogEntry
	pgAllEntries = append(pgAllEntries, pgRepEntries...)
	pgAllEntries = append(pgAllEntries, pgCompEntries...)
	pgAllEntries = append(pgAllEntries, pgDeepEntries...)
	pgAllEntries = append(pgAllEntries, pgLTEntries...)
	pgAllEntries = append(pgAllEntries, pgResEntries...)

	// Compute summaries.
	mysqlSummary := computeCatalogSummary(mysqlEntries)
	tidbSummary := computeCatalogSummary(tidbEntries)
	pgSummary := computeCatalogSummary(pgAllEntries)
	pgResSummary := computeCatalogSummary(pgResEntries)

	// Validate baselines.
	assertCatalogSummary(t, "mysql", mysqlSummary, 61, 46, 0, 0, 15, 0)
	assertCatalogSummary(t, "tidb", tidbSummary, 54, 45, 0, 0, 9, 0)
	assertCatalogSummary(t, "postgresql", pgSummary, 285, 274, 6, 0, 5, 0)
	assertCatalogSummary(t, "postgresql_alter_table_residual", pgResSummary, 66, 60, 2, 0, 4, 0)

	// Convert to JSON entries.
	var allIntermediate []intermediateCatalogEntry
	allIntermediate = append(allIntermediate, mysqlEntries...)
	allIntermediate = append(allIntermediate, tidbEntries...)
	allIntermediate = append(allIntermediate, pgAllEntries...)

	jsonEntries := make([]catalogEntryJSON, 0, len(allIntermediate))
	for _, e := range allIntermediate {
		jsonEntries = append(jsonEntries, e.toCatalogEntry())
	}

	// Deterministic sort: dialect → family → form.
	sort.Slice(jsonEntries, func(i, j int) bool {
		if jsonEntries[i].Dialect != jsonEntries[j].Dialect {
			return jsonEntries[i].Dialect < jsonEntries[j].Dialect
		}
		if jsonEntries[i].Family != jsonEntries[j].Family {
			return jsonEntries[i].Family < jsonEntries[j].Family
		}
		return jsonEntries[i].Form < jsonEntries[j].Form
	})

	// Build catalog.
	cat := catalogJSON{
		Version:       catalogVersion,
		GeneratedFrom: catalogGeneratedFrom,
		Summary: map[string]*catalogSummaryJSON{
			"mysql":                           mysqlSummary,
			"tidb":                            tidbSummary,
			"postgresql":                      pgSummary,
			"postgresql_alter_table_residual": pgResSummary,
		},
		Entries: jsonEntries,
	}

	// Validate entries.
	validateCatalogEntries(t, cat)

	// Validate required parser-upgrade candidates.
	validateRequiredParserUpgradeCandidates(t, cat)

	// Validate no-leak.
	validateNoLeak(t, cat)

	// Validate generated_from paths exist.
	validateGeneratedFromPaths(t)

	// Generate deterministic JSON.
	jsonBytes, err := json.MarshalIndent(cat, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal catalog: %v", err)
	}
	jsonStr := string(jsonBytes) + "\n"

	// Update mode: write the JSON file.
	if os.Getenv("UPDATE_DDL_COVERAGE_CATALOG") == "1" {
		if err := os.MkdirAll("../../../docs/reference", 0o755); err != nil {
			t.Fatalf("failed to create docs/reference: %v", err)
		}
		if err := os.MkdirAll("catalogdata", 0o755); err != nil {
			t.Fatalf("failed to create catalogdata: %v", err)
		}
		if err := os.WriteFile(catalogJSONRelPath, []byte(jsonStr), 0o644); err != nil {
			t.Fatalf("failed to write catalog: %v", err)
		}
		if err := os.WriteFile(catalogEmbedRelPath, []byte(jsonStr), 0o644); err != nil {
			t.Fatalf("failed to write embedded catalog: %v", err)
		}
		t.Logf("catalog written to %s and %s", catalogJSONRelPath, catalogEmbedRelPath)
		return
	}

	// Normal mode: compare against checked-in docs JSON and the embed copy.
	existing, err := os.ReadFile(catalogJSONRelPath)
	if err != nil {
		t.Fatalf("catalog file not found (run with UPDATE_DDL_COVERAGE_CATALOG=1 to generate): %v", err)
	}
	if string(existing) != jsonStr {
		t.Errorf("catalog JSON is stale; run:\n  UPDATE_DDL_COVERAGE_CATALOG=1 go test ./internal/application/audit -tags postgresql -run TestDDLCoverageCatalog")
	}
	embedded, err := os.ReadFile(catalogEmbedRelPath)
	if err != nil {
		t.Fatalf("embedded catalog file not found (run with UPDATE_DDL_COVERAGE_CATALOG=1 to generate): %v", err)
	}
	if string(embedded) != jsonStr {
		t.Errorf("embedded catalog JSON is stale; run:\n  UPDATE_DDL_COVERAGE_CATALOG=1 go test ./internal/application/audit -tags postgresql -run TestDDLCoverageCatalog")
	}
}
