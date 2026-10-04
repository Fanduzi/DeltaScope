// Package httpapi verifies the T05-A6 ordered DROP COLUMN contract at the HTTP seam.
// input: offline six-rule HTTP audit requests for the first path and the
// removed-column index, with schema golden and no connection
// output: HTTP 200 review and reject payloads
// pos: HTTP transport contract tests for ordinary single-column DROP COLUMN post-state
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

func t05A6HTTPSixRulePolicy(t *testing.T) string {
	t.Helper()
	enabled := map[string]string{
		"ddl.table.exists.create.forbid":                "",
		"ddl.table.exists.alter.require":                "",
		"ddl.alter.drop_column.exists.require":          "",
		"ddl.alter.modify_column.exists.require":        "",
		"ddl.alter.modify_column.compatibility.require": "      required: true\n",
		"ddl.create_index.columns.exists.require":       "      required: true\n",
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
	path := filepath.Join(t.TempDir(), "t05-a6-isolated.yaml")
	if err := os.WriteFile(path, []byte(builder.String()), 0o600); err != nil {
		t.Fatalf("write isolated policy: %v", err)
	}
	return path
}

const t05A6DropSQL = "CREATE TABLE t (id INT PRIMARY KEY, obsolete INT, keep_c VARCHAR(10) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL);\n" +
	"ALTER TABLE t DROP COLUMN obsolete;\n" +
	"ALTER TABLE t MODIFY COLUMN keep_c VARCHAR(20) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL;\n" +
	"CREATE INDEX idx_keep ON t(keep_c);"

func postT05A6Audit(t *testing.T, dialect, sql string) (int, map[string]any) {
	t.Helper()
	handler, err := NewHandler(t05A6HTTPSixRulePolicy(t), "test-build")
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

func TestHandlerT05A6DropColumnOffline(t *testing.T) {
	code, payload := postT05A6Audit(t, "mysql", t05A6DropSQL)
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
	if len(statements) != 4 {
		t.Fatalf("expected 4 statements, got %#v", payload["statements"])
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
	for index := 1; index < 4; index++ {
		statement, _ := statements[index].(map[string]any)
		coverage, _ := statement["coverage"].(map[string]any)
		if coverage["status"] != "complete" {
			t.Fatalf("statement %d coverage = %#v", index, coverage)
		}
		if findings, ok := statement["findings"].([]any); ok && len(findings) != 0 {
			t.Fatalf("statement %d findings = %#v", index, findings)
		}
	}
}

func TestHandlerT05A6RemovedIndexOffline(t *testing.T) {
	sql := strings.Replace(t05A6DropSQL, "CREATE INDEX idx_keep ON t(keep_c);", "CREATE INDEX ix_removed ON t(obsolete);", 1)
	code, payload := postT05A6Audit(t, "tidb", sql)
	if code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %#v", code, payload)
	}
	if payload["verdict"] != "reject" {
		t.Fatalf("expected reject, got %#v", payload["verdict"])
	}
	statements, _ := payload["statements"].([]any)
	last, _ := statements[3].(map[string]any)
	findings, _ := last["findings"].([]any)
	if len(findings) != 1 {
		t.Fatalf("expected one finding, got %#v", last["findings"])
	}
	finding, _ := findings[0].(map[string]any)
	metadata, _ := finding["metadata"].(map[string]any)
	if finding["rule_id"] != "ddl.create_index.columns.exists.require" || metadata["schema"] != "golden" || metadata["table"] != "t" || metadata["index"] != "ix_removed" || metadata["column"] != "obsolete" || metadata["exists"] != false {
		t.Fatalf("finding = %#v", finding)
	}
}
