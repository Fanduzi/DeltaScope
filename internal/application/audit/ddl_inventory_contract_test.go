// Package audit verifies the official DDL acceptance inventory contract.
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

// ddlInventoryCell is one applicability cell: official documentation presence
// and product applicability for one anchor version, with its own parser/audit
// status and evidence refs.
type ddlInventoryCell struct {
	Anchor           string   `yaml:"anchor"`
	VersionRange     string   `yaml:"version_range"`
	OfficialPresence string   `yaml:"official_presence"`
	ProductScope     string   `yaml:"product_scope"`
	ParserStatus     string   `yaml:"parser_status"`
	AuditStatus      string   `yaml:"audit_status"`
	Evidence         []string `yaml:"evidence"`
}

// ddlInventorySubaction is an explicit per-subaction record used where one row
// carries subactions with meaningfully different parser/audit behavior.
type ddlInventorySubaction struct {
	SubactionID   string             `yaml:"subaction_id"`
	ParserStatus  string             `yaml:"parser_status"`
	AuditStatus   string             `yaml:"audit_status"`
	Applicability []ddlInventoryCell `yaml:"applicability"`
}

type ddlInventoryRow struct {
	ID              string                  `yaml:"id"`
	Product         string                  `yaml:"product"`
	Versions        []string                `yaml:"versions"`
	Sources         []string                `yaml:"sources"`
	Family          string                  `yaml:"family"`
	Subactions      []string                `yaml:"subactions"`
	SQLShape        string                  `yaml:"sql_shape"`
	Prerequisites   string                  `yaml:"prerequisites"`
	Status          string                  `yaml:"status"`
	ParserStatus    string                  `yaml:"parser_status"`
	AuditStatus     string                  `yaml:"audit_status"`
	Applicability   []ddlInventoryCell      `yaml:"applicability"`
	SubactionStatus []ddlInventorySubaction `yaml:"subaction_status"`
	StatusEvidence  string                  `yaml:"status_evidence"`
	Targets         string                  `yaml:"targets"`
	Acceptance      ddlInventoryAcceptance  `yaml:"acceptance"`
	Owner           string                  `yaml:"owner"`
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
	Version           int                           `yaml:"version"`
	Verified          string                        `yaml:"verified"`
	Purpose           string                        `yaml:"purpose"`
	Sources           map[string]ddlInventorySource `yaml:"sources"`
	Statuses          map[string]string             `yaml:"statuses"`
	ParserStatuses    map[string]string             `yaml:"parser_statuses"`
	AuditStatuses     map[string]string             `yaml:"audit_statuses"`
	ProductScopes     map[string]string             `yaml:"product_scopes"`
	OfficialPresences map[string]string             `yaml:"official_presences"`
	Owners            map[string]ddlInventoryOwner  `yaml:"owners"`
	RequiredRowIDs    []string                      `yaml:"required_row_ids"`
	Rows              []ddlInventoryRow             `yaml:"rows"`
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

// versionAnchors maps declared version strings to the anchor identities.
var versionAnchors = map[string]map[string]string{
	"mysql":      {"5.7": "mysql57", "8.0": "mysql80", "8.4": "mysql84"},
	"tidb":       {"8.5": "tidb85"},
	"postgresql": {"17": "postgresql17"},
}

var knownAnchors = map[string]bool{"mysql57": true, "mysql80": true, "mysql84": true, "tidb85": true, "postgresql17": true}

// anchorVersion maps each anchor identity back to the declared version string
// used inside probe-evidence section headers ("=== version <v> ...").
var anchorVersion = map[string]string{"mysql57": "5.7", "mysql80": "8.0", "mysql84": "8.4", "tidb85": "8.5", "postgresql17": "17"}

// ddlInventoryEvidenceDir is the committed per-row probe-evidence directory.
// Probe files there are row-scoped and version-sectioned, which lets the gate
// reject evidence copied from the wrong row or missing the cell's version.
const ddlInventoryEvidenceDir = "testdata/ddl-inventory/evidence/"

// validateCellEvidence enforces the R4/R6 evidence contract on one
// applicability cell: every cell needs at least one concrete ref (file: or
// gate:), missing: entries are notes, not evidence; file: refs must exist;
// probe files under testdata/ddl-inventory/evidence/ must belong to this row
// and must contain a probe section for the cell's own anchor version; gate:
// refs must resolve to a committed golden manifest (gate:T<NN>).
//
// R6 addition — subaction binding: when subID is non-empty the cell must also
// carry at least one subaction-bound ref, so one subaction cannot borrow
// another subaction's evidence and row-level probes cannot mask missing
// subaction evidence. Bound forms: an evidence/ probe file whose header
// declares the exact `subaction_id: <subID>`, or a `file:<path>#<subID>`
// selector whose literal is present in the referenced file.
func validateCellEvidence(root, rowID, subID string, cell ddlInventoryCell, anchorVer map[string]string) error {
	concrete := 0
	bound := 0
	for _, ref := range cell.Evidence {
		switch {
		case strings.HasPrefix(ref, "file:"):
			p := strings.TrimPrefix(ref, "file:")
			sel := ""
			if i := strings.LastIndex(p, "#"); i >= 0 {
				sel = p[i+1:]
				p = p[:i]
			}
			full := filepath.Join(root, p)
			info, err := os.Stat(full)
			if err != nil {
				return fmt.Errorf("cell evidence file does not exist: %s", p)
			}
			var text string
			if !info.IsDir() {
				content, err := os.ReadFile(full)
				if err != nil {
					return fmt.Errorf("cell evidence unreadable: %s", p)
				}
				text = string(content)
			}
			if strings.HasPrefix(p, ddlInventoryEvidenceDir) {
				base := filepath.Base(p)
				if !strings.HasPrefix(base, rowID+".") {
					return fmt.Errorf("cell evidence %s belongs to a different row (basename must start with %s.)", p, rowID)
				}
				want := anchorVer[cell.Anchor]
				if want == "" {
					return fmt.Errorf("cell anchor %s has no declared version", cell.Anchor)
				}
				if !strings.Contains(text, "=== version "+want+" ") {
					return fmt.Errorf("cell evidence %s has no probe section for version %s (cell anchor %s)", p, want, cell.Anchor)
				}
				rowHdr := evidenceHeaderValue(text, "row_id:")
				if rowHdr != "" && rowHdr != rowID {
					return fmt.Errorf("cell evidence %s is bound to row %q, not %q", p, rowHdr, rowID)
				}
				subHdr := evidenceHeaderValue(text, "subaction_id:")
				switch {
				case subID == "" && subHdr != "":
					return fmt.Errorf("row-level cell cannot cite subaction-bound probe %s (declares subaction_id %q)", p, subHdr)
				case subID != "" && subHdr == "":
					return fmt.Errorf("cell evidence %s is a row-level probe; subaction %q requires subaction-bound evidence", p, subID)
				case subID != "" && subHdr != subID:
					return fmt.Errorf("cell evidence %s is bound to subaction %q, not %q", p, subHdr, subID)
				case subID != "":
					bound++
				}
			}
			if sel != "" {
				if !strings.Contains(text, sel) {
					return fmt.Errorf("cell evidence selector %q not present in %s", sel, p)
				}
				if subID != "" && sel == subID {
					bound++
				}
			}
			concrete++
		case strings.HasPrefix(ref, "gate:"):
			g := strings.TrimPrefix(ref, "gate:")
			if matched, _ := filepath.Match("T[0-9][0-9]", g); !matched {
				return fmt.Errorf("cell evidence gate ref %q does not resolve to a declared gate (gate:T<NN> golden manifest)", ref)
			} else if _, err := os.Stat(filepath.Join(root, "testdata/ddl-golden", g+".json")); err != nil {
				return fmt.Errorf("cell evidence gate ref %s has no committed golden manifest testdata/ddl-golden/%s.json", ref, g)
			}
			concrete++
		case strings.HasPrefix(ref, "missing:"):
			// A note, not evidence. Tolerated alongside concrete refs.
		default:
			return fmt.Errorf("cell evidence %q lacks a kind prefix (file:/gate:/missing:)", ref)
		}
	}
	if concrete == 0 {
		return fmt.Errorf("cell has no concrete evidence ref (missing: notes are not evidence)")
	}
	if subID != "" && bound == 0 {
		return fmt.Errorf("subaction %q cell has no subaction-bound evidence (needs an evidence/ probe declaring subaction_id: %s or a file:<path>#%s selector)", subID, subID, subID)
	}
	return nil
}

// evidenceHeaderValue returns the value of a top-of-file `key: value` header
// line, or "" when the file does not declare it.
func evidenceHeaderValue(text, key string) string {
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(line, "===") {
			break
		}
		if v, ok := strings.CutPrefix(line, key); ok {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

// combinedRowCells returns the row's applicability cells plus every
// subaction cell. The aggregate must be derivable from the union so a
// silent or explicit_unsupported registered subaction cannot hide behind a
// fully-checked parent status.
func combinedRowCells(row ddlInventoryRow) []ddlInventoryCell {
	cells := append([]ddlInventoryCell{}, row.Applicability...)
	for _, sub := range row.SubactionStatus {
		cells = append(cells, sub.Applicability...)
	}
	return cells
}

// deriveAggregateStatus mirrors the reviewer-frozen derivation rules. Order
// matters: an all-vendor-boundary row is vendor_not_supported; documented forms
// the parser cannot read take precedence over mixed registered states
// (parser_unsupported); then the registered-domain audit states classify.
// Out-of-domain version_gated/vendor_boundary/not_applicable cells must not
// downgrade a completely audited registered domain.
func deriveAggregateStatus(cells []ddlInventoryCell) string {
	registered := 0
	vendor := 0
	documented := 0
	allDocUnrecognized := true
	states := map[string]bool{}
	for _, c := range cells {
		switch c.ProductScope {
		case "registered":
			registered++
			states[c.AuditStatus] = true
		case "vendor_boundary":
			vendor++
		}
		if c.OfficialPresence == "documented" {
			documented++
			if c.ParserStatus != "unrecognized" {
				allDocUnrecognized = false
			}
		}
	}
	if vendor > 0 && vendor == len(cells) {
		return "vendor_not_supported"
	}
	if documented > 0 && allDocUnrecognized {
		return "parser_unsupported"
	}
	if registered > 0 && len(states) == 1 {
		for s := range states {
			switch s {
			case "audited":
				return "semantically_checked"
			case "notice_only":
				return "generic_notice"
			case "explicit_unsupported":
				return "explicit_unsupported"
			case "silent":
				return "parse_only"
			}
		}
	}
	return "mixed_coverage"
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
	for name, vocab := range map[string]map[string]string{
		"parser_statuses":    inv.ParserStatuses,
		"audit_statuses":     inv.AuditStatuses,
		"product_scopes":     inv.ProductScopes,
		"official_presences": inv.OfficialPresences,
	} {
		if len(vocab) == 0 {
			t.Fatalf("%s vocabulary missing or empty", name)
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

		if row.Product != "mysql" && row.Product != "tidb" && row.Product != "postgresql" {
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
			switch row.Product {
			case "mysql":
				src, ok := mysqlVersions[v]
				if !ok {
					t.Fatalf("row %q invalid mysql version %q", row.ID, v)
				}
				if !sourceSet[src] {
					t.Fatalf("row %q version %s lacks its official source %s", row.ID, v, src)
				}
			case "tidb":
				if v != "8.5" {
					t.Fatalf("row %q invalid tidb version %q", row.ID, v)
				}
				if !sourceSet["tidb85"] {
					t.Fatalf("row %q lacks tidb85 source", row.ID)
				}
			default:
				if v != "17" {
					t.Fatalf("row %q invalid postgresql version %q", row.ID, v)
				}
				if !sourceSet["postgresql17"] {
					t.Fatalf("row %q lacks postgresql17 source", row.ID)
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
		if _, ok := inv.ParserStatuses[row.ParserStatus]; !ok {
			t.Fatalf("row %q missing or unsupported parser_status %q", row.ID, row.ParserStatus)
		}
		if _, ok := inv.AuditStatuses[row.AuditStatus]; !ok {
			t.Fatalf("row %q missing or unsupported audit_status %q", row.ID, row.AuditStatus)
		}
		checkInventoryCells := func(cells []ddlInventoryCell, label, subID string) map[string]bool {
			covered := map[string]bool{}
			if len(cells) == 0 {
				t.Fatalf("row %q %s has no applicability cells", row.ID, label)
			}
			for _, cell := range cells {
				if !knownAnchors[cell.Anchor] {
					t.Fatalf("row %q %s cell uses unknown anchor %q", row.ID, label, cell.Anchor)
				}
				covered[cell.Anchor] = true
				if _, ok := inv.OfficialPresences[cell.OfficialPresence]; !ok {
					t.Fatalf("row %q %s anchor %s unsupported official_presence %q", row.ID, label, cell.Anchor, cell.OfficialPresence)
				}
				if _, ok := inv.ProductScopes[cell.ProductScope]; !ok {
					t.Fatalf("row %q %s anchor %s unsupported product_scope %q", row.ID, label, cell.Anchor, cell.ProductScope)
				}
				if _, ok := inv.ParserStatuses[cell.ParserStatus]; !ok {
					t.Fatalf("row %q %s anchor %s unsupported cell parser_status %q", row.ID, label, cell.Anchor, cell.ParserStatus)
				}
				if _, ok := inv.AuditStatuses[cell.AuditStatus]; !ok {
					t.Fatalf("row %q %s anchor %s unsupported cell audit_status %q", row.ID, label, cell.Anchor, cell.AuditStatus)
				}
				if err := validateCellEvidence(root, row.ID, subID, cell, anchorVersion); err != nil {
					t.Fatalf("row %q %s anchor %s: %v", row.ID, label, cell.Anchor, err)
				}
			}
			return covered
		}
		covered := checkInventoryCells(row.Applicability, "row", "")
		for _, v := range row.Versions {
			anchor := versionAnchors[row.Product][v]
			if anchor == "" {
				t.Fatalf("row %q version %q has no anchor mapping", row.ID, v)
			}
			if !covered[anchor] {
				t.Fatalf("row %q declared version %s lacks an applicability cell for anchor %s", row.ID, v, anchor)
			}
		}
		if derived := deriveAggregateStatus(row.Applicability); derived != row.Status {
			t.Fatalf("row %q status %q does not match derived aggregate %q", row.ID, row.Status, derived)
		}
		if len(row.SubactionStatus) > 0 {
			if combined := deriveAggregateStatus(combinedRowCells(row)); combined != row.Status {
				t.Fatalf("row %q status %q masks subaction evidence: combined row+subaction derivation is %q", row.ID, row.Status, combined)
			}
		}
		for _, sub := range row.SubactionStatus {
			if sub.SubactionID == "" {
				t.Fatalf("row %q has a subaction record with no subaction_id", row.ID)
			}
			if _, ok := inv.ParserStatuses[sub.ParserStatus]; !ok {
				t.Fatalf("row %q subaction %q unsupported parser_status %q", row.ID, sub.SubactionID, sub.ParserStatus)
			}
			if _, ok := inv.AuditStatuses[sub.AuditStatus]; !ok {
				t.Fatalf("row %q subaction %q unsupported audit_status %q", row.ID, sub.SubactionID, sub.AuditStatus)
			}
			subCovered := checkInventoryCells(sub.Applicability, "subaction "+sub.SubactionID, sub.SubactionID)
			for _, v := range row.Versions {
				if !subCovered[versionAnchors[row.Product][v]] {
					t.Fatalf("row %q subaction %q declared version %s lacks an applicability cell", row.ID, sub.SubactionID, v)
				}
			}
		}
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

// TestDDLInventoryAggregateDerivation pins the frozen derivation rules on
// synthetic cells so a YAML edit cannot quietly change classification
// semantics.
func TestDDLInventoryAggregateDerivation(t *testing.T) {
	t.Parallel()
	cell := func(presence, scope, parser, audit string) ddlInventoryCell {
		return ddlInventoryCell{
			Anchor: "mysql80", VersionRange: "8.0",
			OfficialPresence: presence, ProductScope: scope,
			ParserStatus: parser, AuditStatus: audit,
		}
	}
	cases := []struct {
		name  string
		cells []ddlInventoryCell
		want  string
	}{
		{"all registered audited", []ddlInventoryCell{
			cell("documented", "registered", "recognized", "audited"),
			cell("documented", "registered", "recognized", "audited"),
		}, "semantically_checked"},
		{"audited registered plus gated cell", []ddlInventoryCell{
			cell("documented", "registered", "recognized", "audited"),
			cell("documented", "version_gated", "recognized", "explicit_unsupported"),
		}, "semantically_checked"},
		{"all registered notice only", []ddlInventoryCell{
			cell("documented", "registered", "recognized", "notice_only"),
		}, "generic_notice"},
		{"all registered silent", []ddlInventoryCell{
			cell("documented", "registered", "recognized", "silent"),
		}, "parse_only"},
		{"all registered explicit unsupported", []ddlInventoryCell{
			cell("documented", "registered", "recognized", "explicit_unsupported"),
			cell("documented", "version_gated", "recognized", "explicit_unsupported"),
		}, "explicit_unsupported"},
		{"mixed registered states", []ddlInventoryCell{
			cell("documented", "registered", "recognized", "audited"),
			cell("documented", "registered", "recognized", "silent"),
		}, "mixed_coverage"},
		{"documented unrecognized beats mixed", []ddlInventoryCell{
			cell("documented", "registered", "unrecognized", "not_applicable"),
			cell("documented", "registered", "recognized", "audited"),
		}, "mixed_coverage"},
		{"all documented unrecognized", []ddlInventoryCell{
			cell("documented", "registered", "unrecognized", "not_applicable"),
			cell("documented", "registered", "unrecognized", "not_applicable"),
		}, "parser_unsupported"},
		{"all vendor boundary", []ddlInventoryCell{
			cell("not_documented", "vendor_boundary", "unrecognized", "not_applicable"),
			cell("not_documented", "vendor_boundary", "unrecognized", "not_applicable"),
		}, "vendor_not_supported"},
	}
	for _, tc := range cases {
		if got := deriveAggregateStatus(tc.cells); got != tc.want {
			t.Fatalf("%s: derived %q, want %q", tc.name, got, tc.want)
		}
	}
}

// TestDDLInventoryEvidenceGate pins the ER1 negative controls: each rejection
// class must actually fire on synthetic cells, not just pass on the real
// inventory.
func TestDDLInventoryEvidenceGate(t *testing.T) {
	t.Parallel()
	root := repoRootFromInventory(t)
	cell := func(anchor string, evidence ...string) ddlInventoryCell {
		return ddlInventoryCell{
			Anchor: anchor, VersionRange: anchorVersion[anchor],
			OfficialPresence: "documented", ProductScope: "registered",
			ParserStatus: "recognized", AuditStatus: "audited",
			Evidence: evidence,
		}
	}
	probeRef := "file:testdata/ddl-inventory/evidence/mysql.drop-index.probes.txt"
	corpusRef := "file:testdata/sql-corpus/mysql/ddl/findings/lifecycle_notice_rules.expected.yaml"

	orderByRef := "file:testdata/ddl-inventory/evidence/mysql.alter-table.options.order-by.probes.txt"
	convertRef := "file:testdata/ddl-inventory/evidence/mysql.alter-table.options.convert-to-character-set.probes.txt"
	setDefRef := "file:testdata/ddl-inventory/evidence/mysql.alter-table.alter-column-default.alter-column-c-set-default.probes.txt"
	dropDefRef := "file:testdata/ddl-inventory/evidence/mysql.alter-table.alter-column-default.alter-column-c-drop-default.probes.txt"
	catalogRef := "file:internal/application/audit/catalogdata/ddl-coverage-catalog.json"
	tablespaceSel := "file:testdata/sql-corpus/mysql/ddl/findings/alter_table_tablespace_rules.sql#DISCARD TABLESPACE"

	cases := []struct {
		name    string
		rowID   string
		subID   string
		cell    ddlInventoryCell
		wantErr string
	}{
		{"concrete refs accepted", "mysql.drop-index", "", cell("mysql80", probeRef, corpusRef), ""},
		{"missing-only rejected", "mysql.drop-index", "", cell("mysql80", `missing: owner adds fixtures`), "no concrete evidence"},
		{"empty evidence rejected", "mysql.drop-index", "", cell("mysql80"), "no concrete evidence"},
		{"nonexistent file rejected", "mysql.drop-index", "", cell("mysql80", "file:does/not/exist.go"), "does not exist"},
		{"unknown gate rejected", "mysql.drop-index", "", cell("mysql80", "gate:no-such-gate"), "does not resolve"},
		{"missing gate manifest rejected", "mysql.drop-index", "", cell("mysql80", "gate:T99"), "no committed golden manifest"},
		{"valid gate ref accepted", "mysql.drop-index", "", cell("mysql80", "gate:T02"), ""},
		{"wrong-row probe rejected", "mysql.create-procedure", "", cell("mysql80", probeRef), "belongs to a different row"},
		{"wrong-version probe rejected", "mysql.drop-index", "", cell("tidb85", probeRef), "no probe section for version"},
		{"unprefixed ref rejected", "mysql.drop-index", "", cell("mysql80", "some/path.go"), "kind prefix"},
		{"audited cell with concrete ref accepted", "mysql.drop-index", "", cell("mysql80", corpusRef), ""},
		// R6 subaction binding: evidence must bind to the exact subaction, not
		// just the row.
		{"valid subaction probe accepted", "mysql.alter-table.options", "ORDER BY", cell("mysql80", orderByRef, catalogRef), ""},
		{"SET DEFAULT subaction probe accepted", "mysql.alter-table.alter-column-default", "ALTER COLUMN c SET DEFAULT", cell("mysql80", setDefRef), ""},
		{"subaction selector accepted", "mysql.alter-table.tablespace-ops", "DISCARD TABLESPACE", cell("mysql80", tablespaceSel), ""},
		{"wrong-subaction probe rejected", "mysql.alter-table.options", "ORDER BY", cell("mysql80", convertRef), "bound to subaction \"CONVERT TO CHARACTER SET\", not \"ORDER BY\""},
		{"SET DEFAULT borrowing DROP DEFAULT rejected", "mysql.alter-table.alter-column-default", "ALTER COLUMN c SET DEFAULT", cell("mysql80", dropDefRef), "bound to subaction \"ALTER COLUMN c DROP DEFAULT\", not \"ALTER COLUMN c SET DEFAULT\""},
		{"row-level probe cannot satisfy subaction", "mysql.alter-table.options", "ORDER BY", cell("mysql80", "file:testdata/ddl-inventory/evidence/mysql.alter-table.options.probes.txt"), "row-level probe"},
		{"subaction cell without bound ref rejected", "mysql.alter-table.options", "ORDER BY", cell("mysql80", catalogRef, corpusRef), "no subaction-bound evidence"},
		{"subaction wrong version rejected", "mysql.alter-table.options", "ORDER BY", cell("tidb85", orderByRef), "no probe section for version"},
		{"subaction probe on wrong row rejected", "mysql.logfile-group", "ADD UNDOFILE", cell("mysql80", orderByRef), "belongs to a different row"},
		{"row-level cell cannot cite subaction probe", "mysql.alter-table.options", "", cell("mysql80", orderByRef), "cannot cite subaction-bound probe"},
		// R7 NO ROLLBACK ON ERROR binding: the modifier variant and the
		// ordinary form must not borrow each other's evidence, and MySQL vs
		// TiDB evidence cannot be interchanged.
		{"NO ROLLBACK probe accepted", "mysql.alter-instance", "RELOAD TLS NO ROLLBACK ON ERROR", cell("mysql80", "file:testdata/ddl-inventory/evidence/mysql.alter-instance.reload-tls-no-rollback-on-error.probes.txt"), ""},
		{"NO ROLLBACK borrowing ordinary RELOAD TLS rejected", "mysql.alter-instance", "RELOAD TLS NO ROLLBACK ON ERROR", cell("mysql80", "file:testdata/ddl-inventory/evidence/mysql.alter-instance.reload-tls-8-0.probes.txt"), "bound to subaction \"RELOAD TLS (8.0+)\", not \"RELOAD TLS NO ROLLBACK ON ERROR\""},
		{"ordinary RELOAD TLS borrowing NO ROLLBACK rejected", "mysql.alter-instance", "RELOAD TLS (8.0+)", cell("mysql80", "file:testdata/ddl-inventory/evidence/mysql.alter-instance.reload-tls-no-rollback-on-error.probes.txt"), "bound to subaction \"RELOAD TLS NO ROLLBACK ON ERROR\", not \"RELOAD TLS (8.0+)\""},
		{"MySQL probe cannot satisfy TiDB subaction", "tidb.alter-instance", "ALTER INSTANCE RELOAD TLS NO ROLLBACK ON ERROR", cell("tidb85", "file:testdata/ddl-inventory/evidence/mysql.alter-instance.reload-tls-no-rollback-on-error.probes.txt"), "belongs to a different row"},
		{"TiDB ordinary RELOAD TLS accepted", "tidb.alter-instance", "ALTER INSTANCE RELOAD TLS", cell("tidb85", "file:testdata/ddl-inventory/evidence/tidb.alter-instance.reload-tls.probes.txt"), ""},
		{"TiDB NO ROLLBACK borrowing ordinary rejected", "tidb.alter-instance", "ALTER INSTANCE RELOAD TLS NO ROLLBACK ON ERROR", cell("tidb85", "file:testdata/ddl-inventory/evidence/tidb.alter-instance.reload-tls.probes.txt"), "bound to subaction \"ALTER INSTANCE RELOAD TLS\", not \"ALTER INSTANCE RELOAD TLS NO ROLLBACK ON ERROR\""},
	}
	for _, tc := range cases {
		err := validateCellEvidence(root, tc.rowID, tc.subID, tc.cell, anchorVersion)
		if tc.wantErr == "" && err != nil {
			t.Fatalf("%s: unexpected error: %v", tc.name, err)
		}
		if tc.wantErr != "" {
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("%s: want error containing %q, got %v", tc.name, tc.wantErr, err)
			}
		}
	}

	// Parent/subaction masking: a semantically_checked parent cannot hide a
	// registered silent subaction.
	row := ddlInventoryRow{
		ID: "synthetic.mask", Status: "semantically_checked",
		Applicability: []ddlInventoryCell{cell("mysql80", probeRef)},
		SubactionStatus: []ddlInventorySubaction{{
			SubactionID: "SUB", ParserStatus: "recognized", AuditStatus: "mixed",
			Applicability: []ddlInventoryCell{
				{Anchor: "mysql80", VersionRange: "8.0", OfficialPresence: "documented",
					ProductScope: "registered", ParserStatus: "recognized", AuditStatus: "silent",
					Evidence: []string{probeRef}},
			},
		}},
	}
	if derived := deriveAggregateStatus(combinedRowCells(row)); derived == row.Status {
		t.Fatalf("combined derivation must not equal parent status when a registered subaction is silent")
	}
}
