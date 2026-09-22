// Package audit verifies the official DDL acceptance inventory contract.
// input: testdata/ddl-inventory/inventory.yaml rows, sources, owners, and proposed tasks
// output: gate evidence that every inventory row is classified, sourced, and assigned
// pos: application-level DDL inventory gate for the mysql-tidb-ddl-completion milestone
// note: if this file changes, update this header and module README.md.
package audit

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
)

var ddlInventoryPath = filepath.Join("..", "..", "..", "testdata", "ddl-inventory", "inventory.yaml")

var ddlRequiredRowsPath = filepath.Join("..", "..", "..", "testdata", "ddl-inventory", "required_rows.txt")

// Locked denominator minimums: required_rows.txt pins the exact row set, and
// these per-product minimums pin the count. Shrinking the denominator below
// them requires editing this test file (the gate itself), not just the YAML.
var minProductRows = map[string]int{"mysql": 63, "tidb": 50}

type ddlInventoryAcceptance struct {
	Dimensions string   `yaml:"dimensions"`
	Refs       []string `yaml:"refs"`
}

type ddlInventoryRow struct {
	ID             string                 `yaml:"id"`
	Product        string                 `yaml:"product"`
	Versions       []string               `yaml:"versions"`
	Sources        []string               `yaml:"sources"`
	Family         string                 `yaml:"family"`
	Subactions     []string               `yaml:"subactions"`
	SQLShape       string                 `yaml:"sql_shape"`
	Prerequisites  string                 `yaml:"prerequisites"`
	Status         string                 `yaml:"status"`
	StatusEvidence string                 `yaml:"status_evidence"`
	Targets        string                 `yaml:"targets"`
	Acceptance     ddlInventoryAcceptance `yaml:"acceptance"`
	Owner          string                 `yaml:"owner"`
}

type ddlInventorySource struct {
	URL      string `yaml:"url"`
	Verified string `yaml:"verified"`
	Note     string `yaml:"note"`
}

type ddlInventoryOwner struct {
	Issue int    `yaml:"issue"`
	Title string `yaml:"title"`
}

type ddlInventory struct {
	Version        int                           `yaml:"version"`
	Verified       string                        `yaml:"verified"`
	Purpose        string                        `yaml:"purpose"`
	Sources        map[string]ddlInventorySource `yaml:"sources"`
	Statuses       map[string]string             `yaml:"statuses"`
	Owners         map[string]ddlInventoryOwner  `yaml:"owners"`
	RequiredRowIDs []string                      `yaml:"required_row_ids"`
	Rows           []ddlInventoryRow             `yaml:"rows"`
}

func loadDDLInventory(t *testing.T) ddlInventory {
	t.Helper()
	data, err := os.ReadFile(ddlInventoryPath)
	if err != nil {
		t.Fatalf("read inventory: %v", err)
	}
	var inv ddlInventory
	if err := yaml.Unmarshal(data, &inv); err != nil {
		t.Fatalf("parse inventory yaml: %v", err)
	}
	return inv
}

func loadRequiredRowIDs(t *testing.T) map[string]bool {
	t.Helper()
	data, err := os.ReadFile(ddlRequiredRowsPath)
	if err != nil {
		t.Fatalf("read required_rows baseline: %v", err)
	}
	ids := map[string]bool{}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if ids[line] {
			t.Fatalf("required_rows baseline has duplicate id %q", line)
		}
		ids[line] = true
	}
	if len(ids) == 0 {
		t.Fatalf("required_rows baseline is empty")
	}
	return ids
}

// denominatorViolations is the locked denominator check. The required row set
// must be identical in three independent places: inventory rows, the YAML's
// required_row_ids, and the checked-in required_rows.txt baseline — plus the
// per-product minimums locked above. Shrinking any pair leaves the third
// (or the code constants) behind and fails.
func denominatorViolations(inv ddlInventory, baselineIDs map[string]bool) []string {
	var out []string
	if len(inv.RequiredRowIDs) == 0 {
		out = append(out, "required_row_ids missing or empty (denominator unprotected)")
	}
	requiredSet := map[string]bool{}
	for _, id := range inv.RequiredRowIDs {
		if requiredSet[id] {
			out = append(out, fmt.Sprintf("required_row_ids contains duplicate id %q", id))
		}
		requiredSet[id] = true
	}
	seen := map[string]bool{}
	productCounts := map[string]int{}
	for _, row := range inv.Rows {
		seen[row.ID] = true
		productCounts[row.Product]++
	}
	for id := range seen {
		if !requiredSet[id] {
			out = append(out, fmt.Sprintf("row %q exists but is absent from required_row_ids", id))
		}
		if !baselineIDs[id] {
			out = append(out, fmt.Sprintf("row %q absent from required_rows.txt baseline", id))
		}
	}
	for id := range requiredSet {
		if !seen[id] {
			out = append(out, fmt.Sprintf("required row %q missing from inventory rows (denominator shrunk)", id))
		}
	}
	for id := range baselineIDs {
		if !seen[id] {
			out = append(out, fmt.Sprintf("baseline row %q missing from inventory rows (denominator shrunk)", id))
		}
	}
	for product, min := range minProductRows {
		if productCounts[product] < min {
			out = append(out, fmt.Sprintf("product %s has %d rows, below locked minimum %d", product, productCounts[product], min))
		}
	}
	return out
}

func repoRootFromInventory(t *testing.T) string {
	t.Helper()
	abs, err := filepath.Abs(filepath.Join(filepath.Dir(ddlInventoryPath), "..", ".."))
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}
	return abs
}

func TestDDLInventoryContract(t *testing.T) {
	t.Parallel()
	inv := loadDDLInventory(t)
	baselineIDs := loadRequiredRowIDs(t)
	root := repoRootFromInventory(t)

	if inv.Version < 1 {
		t.Fatalf("inventory version missing")
	}
	if inv.Verified == "" {
		t.Fatalf("inventory verified date missing")
	}
	if len(inv.Rows) == 0 {
		t.Fatalf("inventory has zero rows")
	}

	requiredStatuses := []string{
		"semantically_checked",
		"generic_notice",
		"parse_only",
		"parser_unsupported",
		"vendor_not_supported",
	}
	for _, s := range requiredStatuses {
		if _, ok := inv.Statuses[s]; !ok {
			t.Fatalf("statuses missing required class %q", s)
		}
	}
	for key, src := range inv.Sources {
		if src.URL == "" || src.Verified == "" {
			t.Fatalf("source %q missing url or verified date", key)
		}
	}
	for id, o := range inv.Owners {
		if o.Issue <= 0 || o.Title == "" {
			t.Fatalf("owner %q missing issue number or title", id)
		}
	}
	mysqlVersions := map[string]string{"5.7": "mysql57", "8.0": "mysql80", "8.4": "mysql84"}
	seen := map[string]bool{}
	ownerCounts := map[string]int{}
	statusCounts := map[string]int{}

	for _, row := range inv.Rows {
		if row.ID == "" {
			t.Fatalf("row missing id")
		}
		if seen[row.ID] {
			t.Fatalf("duplicate inventory id %q", row.ID)
		}
		seen[row.ID] = true

		if row.Product != "mysql" && row.Product != "tidb" {
			t.Fatalf("row %q invalid product %q", row.ID, row.Product)
		}
		if len(row.Versions) == 0 {
			t.Fatalf("row %q missing versions", row.ID)
		}
		if len(row.Sources) == 0 {
			t.Fatalf("row %q missing sources", row.ID)
		}
		sourceSet := map[string]bool{}
		for _, s := range row.Sources {
			if _, ok := inv.Sources[s]; !ok {
				t.Fatalf("row %q references undeclared source %q", row.ID, s)
			}
			sourceSet[s] = true
		}
		for _, v := range row.Versions {
			if row.Product == "mysql" {
				src, ok := mysqlVersions[v]
				if !ok {
					t.Fatalf("row %q invalid mysql version %q", row.ID, v)
				}
				if !sourceSet[src] {
					t.Fatalf("row %q version %s lacks its official source %s", row.ID, v, src)
				}
			} else {
				if v != "8.5" {
					t.Fatalf("row %q invalid tidb version %q", row.ID, v)
				}
				if !sourceSet["tidb85"] {
					t.Fatalf("row %q lacks tidb85 source", row.ID)
				}
			}
		}

		if row.Family == "" || len(row.Subactions) == 0 || row.SQLShape == "" {
			t.Fatalf("row %q missing family/subactions/sql_shape", row.ID)
		}
		if row.Prerequisites == "" {
			t.Fatalf("row %q missing prerequisites (use 'none' explicitly)", row.ID)
		}
		if _, ok := inv.Statuses[row.Status]; !ok {
			t.Fatalf("row %q unclassified status %q", row.ID, row.Status)
		}
		statusCounts[row.Status]++
		if row.StatusEvidence == "" {
			t.Fatalf("row %q missing status_evidence (write 'unchecked' facts honestly)", row.ID)
		}
		if row.Targets == "" {
			t.Fatalf("row %q missing targets", row.ID)
		}
		if row.Acceptance.Dimensions == "" {
			t.Fatalf("row %q missing acceptance dimensions", row.ID)
		}

		hasConcreteRef := false
		for _, ref := range row.Acceptance.Refs {
			switch {
			case strings.HasPrefix(ref, "file:"):
				p := strings.TrimPrefix(ref, "file:")
				if _, err := os.Stat(filepath.Join(root, p)); err != nil {
					t.Fatalf("row %q acceptance file ref missing: %s", row.ID, p)
				}
				hasConcreteRef = true
			case strings.HasPrefix(ref, "gate:"):
				hasConcreteRef = true
			case strings.HasPrefix(ref, "missing:"):
				if len(strings.TrimPrefix(ref, "missing:")) < 8 {
					t.Fatalf("row %q has a missing-ref with no useful description", row.ID)
				}
			default:
				t.Fatalf("row %q acceptance ref %q lacks a kind prefix (file:/gate:/missing:)", row.ID, ref)
			}
		}
		if row.Status == "semantically_checked" && !hasConcreteRef {
			t.Fatalf("row %q claims semantically_checked without concrete evidence ref", row.ID)
		}

		if row.Owner == "" {
			t.Fatalf("row %q unassigned", row.ID)
		}
		if _, isOwner := inv.Owners[row.Owner]; !isOwner {
			t.Fatalf("row %q owner %q is not a declared milestone task", row.ID, row.Owner)
		}
		ownerCounts[row.Owner]++
	}

	for status := range statusCounts {
		if _, ok := inv.Statuses[status]; !ok {
			t.Fatalf("rows use undeclared status %q", status)
		}
	}
	for _, v := range denominatorViolations(inv, baselineIDs) {
		t.Fatalf("denominator violation: %s", v)
	}
	t.Logf("inventory rows=%d baseline=%d statuses=%v owners=%v", len(inv.Rows), len(baselineIDs), statusCounts, ownerCounts)
}

func TestDDLInventoryDenominatorLocked(t *testing.T) {
	t.Parallel()
	inv := loadDDLInventory(t)
	baselineIDs := loadRequiredRowIDs(t)

	if got := denominatorViolations(inv, baselineIDs); len(got) != 0 {
		t.Fatalf("real inventory should satisfy the locked denominator, got: %v", got)
	}

	// Shrinking rows and required_row_ids together inside the same YAML file
	// must still fail: the required_rows.txt baseline and the per-product
	// minimums are independent of the editable inventory.
	shrunk := inv
	shrunk.Rows = inv.Rows[:1]
	shrunk.RequiredRowIDs = []string{inv.Rows[0].ID}
	got := denominatorViolations(shrunk, baselineIDs)
	if len(got) == 0 {
		t.Fatalf("shrinking rows+required_row_ids to one entry must violate the denominator")
	}
	for _, v := range got {
		t.Logf("expected violation: %s", v)
	}
}
