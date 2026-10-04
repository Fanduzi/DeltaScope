// Package httpapi verifies the T05-A4 ordered MODIFY contract at the HTTP seam.
// input: offline four-rule HTTP audit requests for the VARCHAR narrow and wide
// sequences, an isolated unknown MODIFY, and an empty SQL request
// output: HTTP 200 reject/review/gap payloads and one HTTP 400 bad_request
// pos: HTTP transport contract tests for ordinary single-column MODIFY post-state
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

func t05A4HTTPFourRulePolicy(t *testing.T) string {
	t.Helper()
	enabled := map[string]string{
		"ddl.table.exists.create.forbid":                "",
		"ddl.table.exists.alter.require":                "",
		"ddl.alter.modify_column.exists.require":        "",
		"ddl.alter.modify_column.compatibility.require": "      required: true\n",
	}
	ruleIDs := make([]string, 0, len(domainpolicy.Default().Rules))
	for id := range domainpolicy.Default().Rules {
		ruleIDs = append(ruleIDs, id)
	}
	sort.Strings(ruleIDs)
	var builder strings.Builder
	builder.WriteString("rules:\n")
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
	path := filepath.Join(t.TempDir(), "t05-a4-isolated.yaml")
	if err := os.WriteFile(path, []byte(builder.String()), 0o600); err != nil {
		t.Fatalf("write isolated policy: %v", err)
	}
	return path
}

const t05A4NarrowSQL = "CREATE TABLE t (id INT PRIMARY KEY, c VARCHAR(10) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL);\n" +
	"ALTER TABLE t MODIFY COLUMN c VARCHAR(20) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL;\n" +
	"ALTER TABLE t MODIFY COLUMN c VARCHAR(15) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL;"

func postT05A4Audit(t *testing.T, dialect, sql string) (int, map[string]any) {
	t.Helper()
	handler, err := NewHandler(t05A4HTTPFourRulePolicy(t), "test-build")
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}
	body, err := json.Marshal(map[string]any{"sql": sql, "dialect": dialect, "schema": "golden"})
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/v1/audit", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v\nbody=%s", err, rec.Body.String())
	}
	return rec.Code, payload
}

func TestHandlerT05A4ModifyNarrowReject(t *testing.T) {
	code, payload := postT05A4Audit(t, "mysql", t05A4NarrowSQL)
	if code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %#v", code, payload)
	}
	if payload["verdict"] != "reject" {
		t.Fatalf("expected reject, got %#v", payload["verdict"])
	}
	if c, _ := payload["coverage"].(map[string]any); c["status"] != "unverified" {
		t.Fatalf("expected unverified, got %#v", payload["coverage"])
	}
	statements, _ := payload["statements"].([]any)
	if len(statements) != 3 {
		t.Fatalf("expected 3 statements, got %#v", payload["statements"])
	}
	first, _ := statements[0].(map[string]any)
	gaps, _ := first["evidence_gaps"].([]any)
	if len(gaps) != 1 {
		t.Fatalf("expected the create gap, got %#v", first["evidence_gaps"])
	}
	gap, _ := gaps[0].(map[string]any)
	if gap["rule_id"] != "ddl.table.exists.create.forbid" || gap["reason_code"] != "unknown_table_state" {
		t.Fatalf("create gap = %#v", gap)
	}
	third, _ := statements[2].(map[string]any)
	findings, _ := third["findings"].([]any)
	if len(findings) != 1 {
		t.Fatalf("expected one shrink finding, got %#v", third["findings"])
	}
	finding, _ := findings[0].(map[string]any)
	metadata, _ := finding["metadata"].(map[string]any)
	if finding["rule_id"] != "ddl.alter.modify_column.compatibility.require" || metadata["source_length"] != float64(20) || metadata["target_length"] != float64(15) {
		t.Fatalf("finding = %#v, want 20 to 15", finding)
	}
}

func TestHandlerT05A4ModifyWideReview(t *testing.T) {
	sql := strings.Replace(t05A4NarrowSQL, "VARCHAR(15)", "VARCHAR(30)", 1)
	code, payload := postT05A4Audit(t, "tidb", sql)
	if code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %#v", code, payload)
	}
	if payload["verdict"] != "review" {
		t.Fatalf("expected review, got %#v", payload["verdict"])
	}
	if c, _ := payload["coverage"].(map[string]any); c["status"] != "unverified" {
		t.Fatalf("expected unverified, got %#v", payload["coverage"])
	}
	statements, _ := payload["statements"].([]any)
	for _, index := range []int{1, 2} {
		statement, _ := statements[index].(map[string]any)
		if coverage, _ := statement["coverage"].(map[string]any); coverage["status"] != "complete" {
			t.Fatalf("statement %d coverage = %#v, want complete", index, statement["coverage"])
		}
		if findings, ok := statement["findings"].([]any); ok && len(findings) != 0 {
			t.Fatalf("statement %d findings = %#v", index, findings)
		}
	}
}

func TestHandlerT05A4IsolatedModifyGap(t *testing.T) {
	code, payload := postT05A4Audit(t, "mysql", "ALTER TABLE t MODIFY COLUMN c VARCHAR(20) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL;")
	if code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %#v", code, payload)
	}
	if payload["verdict"] != "review" {
		t.Fatalf("expected review, got %#v", payload["verdict"])
	}
	statements, _ := payload["statements"].([]any)
	statement, _ := statements[0].(map[string]any)
	if findings, ok := statement["findings"].([]any); ok && len(findings) != 0 {
		t.Fatalf("isolated modify must have no findings, got %#v", findings)
	}
	gaps, _ := statement["evidence_gaps"].([]any)
	got := map[string]string{}
	for _, item := range gaps {
		gap, _ := item.(map[string]any)
		ruleID, _ := gap["rule_id"].(string)
		reason, _ := gap["reason_code"].(string)
		got[ruleID] = reason
	}
	want := map[string]string{
		"ddl.table.exists.alter.require":                "unknown_table_state",
		"ddl.alter.modify_column.exists.require":        "unknown_table_state",
		"ddl.alter.modify_column.compatibility.require": "missing_source_column",
	}
	if len(got) != len(want) {
		t.Fatalf("gaps = %#v, want %#v", got, want)
	}
	for ruleID, reason := range want {
		if got[ruleID] != reason {
			t.Fatalf("gap %s = %q, want %q (all %#v)", ruleID, got[ruleID], reason, got)
		}
	}
}

func TestHandlerT05A4EmptySQLBadRequest(t *testing.T) {
	code, payload := postT05A4Audit(t, "mysql", "")
	if code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %#v", code, payload)
	}
	errBody, _ := payload["error"].(map[string]any)
	if errBody["code"] != "bad_request" {
		t.Fatalf("expected bad_request, got %#v", payload["error"])
	}
	message, _ := errBody["message"].(string)
	if !strings.Contains(message, "sql must not be empty") {
		t.Fatalf("message = %q", message)
	}
}
