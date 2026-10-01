// Package httpapi verifies HTTP offline existence caveats on audit context.
// input: offline ALTER DROP COLUMN HTTP audit requests
// output: review verdict plus JSON evidence-gap contract matching the CLI caveat (context.unproven, unknown_table_state)
// pos: HTTP contract coverage for issue #28 and the T05 offline existence-gap behavior
// note: if this file changes, update this header and module README.md.
package httpapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	domainpolicy "github.com/Fanduzi/DeltaScope/internal/domain/policy"
)

func TestHandlerOfflineDropColumnStatesExistenceNotChecked(t *testing.T) {
	handler, err := NewHandler("", "test-build")
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/audit", bytes.NewBufferString(`{"sql":"alter table users drop column not_a_col"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload["verdict"] != "review" {
		t.Fatalf("expected verdict review, got %#v", payload["verdict"])
	}
	assertJSONContextExistenceCaveat(t, payload)
	if strings.Contains(rec.Body.String(), "existing column") {
		t.Fatalf("HTTP notice must not claim the column exists, got %s", rec.Body.String())
	}
}

func TestHandlerCapabilitiesListsOfflineExistenceContextFields(t *testing.T) {
	handler, err := NewHandler("", "test-build")
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/capabilities", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	fields, ok := payload["context_fields"].([]any)
	if !ok {
		t.Fatalf("expected context_fields array, got %#v", payload["context_fields"])
	}
	got := make([]string, 0, len(fields))
	for _, item := range fields {
		text, _ := item.(string)
		got = append(got, text)
	}
	joined := strings.Join(got, ",")
	if !strings.Contains(joined, "note") || !strings.Contains(joined, "unproven") {
		t.Fatalf("expected context_fields to advertise note and unproven, got %#v", fields)
	}
}

func assertJSONContextExistenceCaveat(t *testing.T, payload map[string]any) {
	t.Helper()
	contextValue, ok := payload["context"].(map[string]any)
	if !ok {
		t.Fatalf("expected context object, got %#v", payload["context"])
	}
	if contextValue["mode"] != "offline" {
		t.Fatalf("expected offline mode, got %#v", contextValue)
	}
	if contextValue["note"] != "existence not checked (no database connection)" {
		t.Fatalf("expected context.note existence caveat, got %#v", contextValue["note"])
	}
	unproven, ok := contextValue["unproven"].([]any)
	if !ok {
		t.Fatalf("expected context.unproven array, got %#v", contextValue["unproven"])
	}
	got := make([]string, 0, len(unproven))
	for _, item := range unproven {
		text, _ := item.(string)
		got = append(got, text)
	}
	if strings.Join(got, ",") != "column_exists,table_exists" {
		t.Fatalf("expected unproven [column_exists table_exists], got %#v", unproven)
	}
}

// T05-A1 ordered-state first path at the HTTP seam under the frozen four-rule
// profile: HTTP 200, review verdict — the first statement's offline
// existence gap rides a real handler request while the derived statements
// stay complete.
func TestHandlerOfflineOrderedFirstPathPass(t *testing.T) {
	handler, err := NewHandler(t05HTTPFourRulePolicy(t), "test-build")
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/audit", bytes.NewBufferString(
		`{"sql":"CREATE TABLE t (id INT PRIMARY KEY); ALTER TABLE t ADD COLUMN c INT; CREATE INDEX idx_c ON t (c);","dialect":"mysql"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload["verdict"] != "review" {
		t.Fatalf("expected verdict review, got %#v", payload["verdict"])
	}
	statements, ok := payload["statements"].([]any)
	if !ok || len(statements) != 3 {
		t.Fatalf("expected 3 statements, got %#v", payload["statements"])
	}
	// Statement 0 carries the offline unknown-existence gap; statements 1-2
	// run on the derived post-CREATE shape and stay complete.
	stmt0 := statements[0].(map[string]any)
	if c, _ := stmt0["coverage"].(map[string]any); c["status"] != "unverified" {
		t.Fatalf("statement 0 coverage must be unverified, got %#v", stmt0["coverage"])
	}
	gaps, ok := stmt0["evidence_gaps"].([]any)
	if !ok || len(gaps) != 1 || gaps[0].(map[string]any)["reason_code"] != "unknown_table_state" {
		t.Fatalf("statement 0 must carry one unknown_table_state gap, got %#v", stmt0["evidence_gaps"])
	}
	for i := 1; i <= 2; i++ {
		stmt := statements[i].(map[string]any)
		if c, _ := stmt["coverage"].(map[string]any); c["status"] != "complete" {
			t.Fatalf("statement %d coverage must be complete via derived state, got %#v", i, stmt["coverage"])
		}
		if gaps, ok := stmt["evidence_gaps"].([]any); ok && len(gaps) != 0 {
			t.Fatalf("statement %d must have no gaps, got %#v", i, gaps)
		}
		if findings, ok := stmt["findings"].([]any); ok && len(findings) != 0 {
			t.Fatalf("statement %d must have no findings, got %#v", i, findings)
		}
	}
}

// t05HTTPFourRulePolicy writes the frozen T05-A1 first-path profile: only the
// four existence rules enabled at blocker, everything else disabled.
func t05HTTPFourRulePolicy(t *testing.T) string {
	t.Helper()
	var builder strings.Builder
	builder.WriteString("rules:\n")
	enabled := map[string]string{
		"ddl.table.exists.create.forbid":          "",
		"ddl.table.exists.alter.require":          "",
		"ddl.alter.add_column.exists.forbid":      "",
		"ddl.create_index.columns.exists.require": "      required: true\n",
	}
	ruleIDs := make([]string, 0, len(domainpolicy.Default().Rules))
	for id := range domainpolicy.Default().Rules {
		ruleIDs = append(ruleIDs, id)
	}
	sort.Strings(ruleIDs)
	for _, id := range ruleIDs {
		if _, keep := enabled[id]; keep {
			continue
		}
		fmt.Fprintf(&builder, "  %s:\n    enabled: false\n", strconv.Quote(id))
	}
	for id, params := range enabled {
		fmt.Fprintf(&builder, "  %s:\n    enabled: true\n    level: blocker\n", strconv.Quote(id))
		if params != "" {
			builder.WriteString("    params:\n")
			builder.WriteString(params)
		}
	}
	path := filepath.Join(t.TempDir(), "policy.yaml")
	if err := os.WriteFile(path, []byte(builder.String()), 0o600); err != nil {
		t.Fatalf("write policy: %v", err)
	}
	return path
}
