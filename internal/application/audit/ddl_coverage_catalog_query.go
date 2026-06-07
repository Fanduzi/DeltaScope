package audit

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// CatalogEntry represents a single DDL coverage catalog entry returned by
// QueryCatalog. Fields mirror the v0.270.0 catalog JSON schema.
type CatalogEntry struct {
	Dialect        string   `json:"dialect"`
	Family         string   `json:"family"`
	Form           string   `json:"form"`
	Classification string   `json:"classification"`
	FindingRuleIDs []string `json:"finding_rule_ids"`
	GuidanceCode   string   `json:"guidance_code,omitempty"`
	EvidenceRef    string   `json:"evidence_ref,omitempty"`
	Notes          string   `json:"notes"`
}

// CatalogQuery holds filter parameters for querying the DDL coverage catalog.
// All string fields are optional; zero values mean "no filter".
type CatalogQuery struct {
	Dialect        string
	Classification string
	GuidanceCode   string
	Family         string
	Form           string
	Search         string
	Limit          int
}

// CatalogResult holds the query output: a filtered slice of entries plus
// summary metadata.
type CatalogResult struct {
	Entries []CatalogEntry `json:"entries"`
	Total   int            `json:"total"`
}

// catalogQueryDialects lists the dialect values accepted by CatalogQuery.
var catalogQueryDialects = map[string]bool{
	"mysql": true, "tidb": true, "postgresql": true,
}

// catalogQueryClassifications lists the classification values accepted by
// CatalogQuery.
var catalogQueryClassifications = map[string]bool{
	"finding_covered": true, "normalized_silent": true,
	"unsupported_boundary": true, "parser_error": true,
	"unclassified": true,
}

// catalogQueryGuidanceCodes lists the guidance_code values accepted by
// CatalogQuery.
var catalogQueryGuidanceCodes = map[string]bool{
	"parser_upgrade_candidate": true,
}

// Validate checks that enum filter values are recognized. Returns an error
// describing the first invalid field, or nil.
func (q CatalogQuery) Validate() error {
	if q.Dialect != "" && !catalogQueryDialects[q.Dialect] {
		return fmt.Errorf("invalid dialect %q: must be one of mysql, tidb, postgresql", q.Dialect)
	}
	if q.Classification != "" && !catalogQueryClassifications[q.Classification] {
		return fmt.Errorf("invalid classification %q: must be one of finding_covered, normalized_silent, unsupported_boundary, parser_error, unclassified", q.Classification)
	}
	if q.GuidanceCode != "" && !catalogQueryGuidanceCodes[q.GuidanceCode] {
		return fmt.Errorf("invalid guidance_code %q: not a known catalog guidance code", q.GuidanceCode)
	}
	return nil
}

// catalogFile is the top-level JSON structure of the checked-in catalog.
type catalogFile struct {
	Entries []CatalogEntry `json:"entries"`
}

// LoadCatalog reads the DDL coverage catalog from the given file path and
// returns entries in their canonical (deterministic) order.
func LoadCatalog(path string) ([]CatalogEntry, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read catalog: %w", err)
	}
	var f catalogFile
	if err := json.Unmarshal(raw, &f); err != nil {
		return nil, fmt.Errorf("parse catalog: %w", err)
	}
	return f.Entries, nil
}

// QueryCatalog filters entries according to the query parameters. It returns
// a CatalogResult with the matching entries preserving their original
// deterministic order, and a total count. Empty results are a success, not an
// error.
func QueryCatalog(entries []CatalogEntry, q CatalogQuery) CatalogResult {
	var out []CatalogEntry
	familyLower := strings.ToLower(q.Family)
	formLower := strings.ToLower(q.Form)
	searchLower := strings.ToLower(q.Search)

	for _, e := range entries {
		if !entryMatches(e, q, familyLower, formLower, searchLower) {
			continue
		}
		out = append(out, e)
	}
	if out == nil {
		out = []CatalogEntry{}
	}
	total := len(out)
	if q.Limit > 0 && total > q.Limit {
		out = out[:q.Limit]
	}
	return CatalogResult{Entries: out, Total: total}
}

func entryMatches(e CatalogEntry, q CatalogQuery, familyLower, formLower, searchLower string) bool {
	if q.Dialect != "" && e.Dialect != q.Dialect {
		return false
	}
	if q.Classification != "" && e.Classification != q.Classification {
		return false
	}
	if q.GuidanceCode != "" && e.GuidanceCode != q.GuidanceCode {
		return false
	}
	if familyLower != "" && !strings.Contains(strings.ToLower(e.Family), familyLower) {
		return false
	}
	if formLower != "" && !strings.Contains(strings.ToLower(e.Form), formLower) {
		return false
	}
	if searchLower != "" && !entryContainsSearch(e, searchLower) {
		return false
	}
	return true
}

func entryContainsSearch(e CatalogEntry, lower string) bool {
	if strings.Contains(strings.ToLower(e.Family), lower) {
		return true
	}
	if strings.Contains(strings.ToLower(e.Form), lower) {
		return true
	}
	if strings.Contains(strings.ToLower(e.Notes), lower) {
		return true
	}
	if strings.Contains(strings.ToLower(e.GuidanceCode), lower) {
		return true
	}
	if strings.Contains(strings.ToLower(e.EvidenceRef), lower) {
		return true
	}
	for _, rid := range e.FindingRuleIDs {
		if strings.Contains(strings.ToLower(rid), lower) {
			return true
		}
	}
	return false
}
