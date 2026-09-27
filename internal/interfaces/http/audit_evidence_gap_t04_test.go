// Package httpapi verifies the T04-A/#83 evidence-gap HTTP contract.
// input: offline HTTP audit requests under the isolated modify-column compatibility policy
// output: HTTP 200 unverified results with evidence_gaps and preserved 400 unsupported semantics
// pos: HTTP transport contract tests for metadata evidence gaps (issue #83 T04-A slice)
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

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
)

const t04TargetRuleID = "ddl.alter.modify_column.compatibility.require"

func writeT04IsolatedPolicy(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "t04-isolated.yaml")
	ids := make([]string, 0, len(policy.Default().Rules))
	for id := range policy.Default().Rules {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	var builder strings.Builder
	builder.WriteString("rules:\n")
	for _, id := range ids {
		if id == t04TargetRuleID {
			builder.WriteString("  " + strconv.Quote(id) + ":\n    enabled: true\n    level: blocker\n    params:\n      required: true\n      requires_metadata: true\n")
			continue
		}
		fmt.Fprintf(&builder, "  %s:\n    enabled: false\n", strconv.Quote(id))
	}
	if err := os.WriteFile(path, []byte(builder.String()), 0o600); err != nil {
		t.Fatalf("write isolated policy: %v", err)
	}
	return path
}

// A gap-only audit is a normal successful response: HTTP 200 carrying
// coverage=unverified, verdict=review, and the bounded evidence gap — the
// transport must not manufacture an error for missing metadata facts.
func TestHandlerT04EvidenceGapReturnsOK(t *testing.T) {
	handler, err := NewHandler(writeT04IsolatedPolicy(t), "test-build")
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/audit", bytes.NewBufferString(`{"sql":"ALTER TABLE t MODIFY COLUMN c VARCHAR(20);","dialect":"mysql"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for evidence-gap-only audit, got %d: %s", rec.Code, rec.Body.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload["verdict"] != "review" {
		t.Fatalf("expected verdict review, got %#v", payload["verdict"])
	}
	if c, _ := payload["coverage"].(map[string]any); c["status"] != "unverified" {
		t.Fatalf("expected aggregate coverage unverified, got %#v", payload["coverage"])
	}
	stmts, _ := payload["statements"].([]any)
	if len(stmts) != 1 {
		t.Fatalf("expected 1 statement, got %#v", payload["statements"])
	}
	first, _ := stmts[0].(map[string]any)
	if c, _ := first["coverage"].(map[string]any); c["status"] != "unverified" {
		t.Fatalf("expected statement coverage unverified, got %#v", first["coverage"])
	}
	gaps, _ := first["evidence_gaps"].([]any)
	if len(gaps) != 1 {
		t.Fatalf("expected exactly one evidence gap, got %#v", gaps)
	}
	gap, _ := gaps[0].(map[string]any)
	if gap["rule_id"] != t04TargetRuleID || gap["reason_code"] != "missing_source_column" {
		t.Fatalf("gap = %#v, want %q/missing_source_column", gap, t04TargetRuleID)
	}
	if facts, _ := gap["required_facts"].([]any); len(facts) != 1 || facts[0] != "source_column.definition" {
		t.Fatalf("expected required_facts [source_column.definition], got %#v", gap["required_facts"])
	}
}

// A gap plus a recognized-but-unsupported statement keeps the existing 400
// contract — unverified coverage never promotes or hides unsupported evidence.
func TestHandlerT04GapWithUnsupportedKeepsBadRequest(t *testing.T) {
	handler, err := NewHandler(writeT04IsolatedPolicy(t), "test-build")
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/audit", bytes.NewBufferString(`{"sql":"ALTER TABLE t MODIFY COLUMN c VARCHAR(20); CREATE SEQUENCE golden_seq START WITH 1;","dialect":"mysql"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for unsupported mix, got %d: %s", rec.Code, rec.Body.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if c, _ := payload["coverage"].(map[string]any); c["status"] != "incomplete" {
		t.Fatalf("expected aggregate incomplete, got %#v", payload["coverage"])
	}
	stmts, _ := payload["statements"].([]any)
	if len(stmts) != 2 {
		t.Fatalf("expected 2 retained statements, got %#v", payload["statements"])
	}
	first, _ := stmts[0].(map[string]any)
	if c, _ := first["coverage"].(map[string]any); c["status"] != "unverified" {
		t.Fatalf("expected first statement unverified, got %#v", first["coverage"])
	}
	if gaps, _ := first["evidence_gaps"].([]any); len(gaps) != 1 {
		t.Fatalf("expected evidence gap retained on first statement, got %#v", gaps)
	}
	unsupported, _ := payload["unsupported"].([]any)
	if len(unsupported) != 1 {
		t.Fatalf("expected 1 unsupported detail, got %#v", payload["unsupported"])
	}
}
