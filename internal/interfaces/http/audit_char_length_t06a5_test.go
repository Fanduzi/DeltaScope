// Package http verifies the T06-A5 declared-length policy contract at the
// HTTP seam — one under-limit and one over-limit representative through
// /v1/audit; the full matrix lives in the shared audit layer and Golden.
// input: CREATE TABLE statements with declared VARCHAR lengths through /v1/audit
// output: HTTP 200 with pass for the control; HTTP 200 with reject for the policy violation
// pos: HTTP transport contract test for issue #85 T06-A5
// note: if this file changes, update this header and module README.md.
package httpapi

import (
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/rule/catalog"
	"os"
)

// t06A5HTTPPolicy writes the isolated single-rule policy (limit=8, blocker).
func t06A5HTTPPolicy(t *testing.T, ruleID string) string {
	t.Helper()
	var text strings.Builder
	text.WriteString("rules:\n")
	for _, entry := range catalog.All() {
		if entry.RuleID == ruleID {
			fmt.Fprintf(&text, "  %q:\n    enabled: true\n    level: blocker\n    params:\n      limit: 8\n", entry.RuleID)
		} else {
			fmt.Fprintf(&text, "  %q:\n    enabled: false\n", entry.RuleID)
		}
	}
	path := filepath.Join(t.TempDir(), "policy.yaml")
	if err := os.WriteFile(path, []byte(text.String()), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// TestHandlerT06A5VarcharLengthUnderLimit: VARCHAR(8) at limit=8 → 200/pass.
func TestHandlerT06A5VarcharLengthUnderLimit(t *testing.T) {
	policy := t06A5HTTPPolicy(t, "ddl.column.varchar.max_length")
	code, payload := postT06A3Audit(t, policy, "mysql", "CREATE TABLE t (c VARCHAR(8));", nil)
	if code != http.StatusOK {
		t.Fatalf("http code = %d payload = %v, want 200", code, payload)
	}
	if payload["verdict"] != "pass" {
		t.Fatalf("verdict = %v, want pass", payload["verdict"])
	}
}

// TestHandlerT06A5VarcharLengthOverLimit: VARCHAR(9) at limit=8 → 200/reject
// with the one pinned blocker — a policy verdict is ordinary result data,
// never an HTTP error.
func TestHandlerT06A5VarcharLengthOverLimit(t *testing.T) {
	policy := t06A5HTTPPolicy(t, "ddl.column.varchar.max_length")
	code, payload := postT06A3Audit(t, policy, "mysql", "CREATE TABLE t (c VARCHAR(9));", nil)
	if code != http.StatusOK {
		t.Fatalf("http code = %d payload = %v, want 200", code, payload)
	}
	if payload["verdict"] != "reject" {
		t.Fatalf("verdict = %v, want reject", payload["verdict"])
	}
}
