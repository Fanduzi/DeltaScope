// Package httpapi verifies the T05-A5 column-identity contract at the HTTP seam.
// input: offline eleven-rule HTTP audit requests for a published CHANGE, a
// MySQL 5.7 RENAME rejection, a missing-version RENAME, and an empty SQL request
// output: HTTP 200 pass-shaped review, reject, and gap payloads, plus one HTTP 400
// pos: HTTP transport contract tests for CHANGE and RENAME COLUMN post-state
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

func t05A5HTTPIdentityPolicy(t *testing.T) string {
	t.Helper()
	enabled := map[string]string{
		"ddl.table.exists.create.forbid":                "",
		"ddl.table.exists.alter.require":                "",
		"ddl.alter.change_column.exists.require":        "",
		"ddl.alter.change_column.compatibility.require": "      required: true\n",
		"ddl.alter.rename_column.exists.require":        "",
		"ddl.alter.modify_column.exists.require":        "",
		"ddl.alter.modify_column.compatibility.require": "      required: true\n",
		"ddl.create_index.columns.exists.require":       "      required: true\n",
		"ddl.alter.change_column.target.exists.forbid":  "",
		"ddl.alter.rename_column.target.exists.forbid":  "",
		"ddl.alter.rename_column.version.require":       "      required: true\n",
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
	path := filepath.Join(t.TempDir(), "t05-a5-isolated.yaml")
	if err := os.WriteFile(path, []byte(builder.String()), 0o600); err != nil {
		t.Fatalf("write isolated policy: %v", err)
	}
	return path
}

const t05A5ChangeSQL = "CREATE TABLE t (id INT PRIMARY KEY, c VARCHAR(10) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL);\n" +
	"ALTER TABLE t CHANGE COLUMN c c2 VARCHAR(10) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL;\n" +
	"ALTER TABLE t MODIFY COLUMN c2 VARCHAR(20) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL;\n" +
	"CREATE INDEX idx_c2 ON t(c2);"

func t05A5RenameSQL() string {
	return strings.Replace(t05A5ChangeSQL, "CHANGE COLUMN c c2 VARCHAR(10) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL", "RENAME COLUMN c TO c2", 1)
}

func postT05A5Audit(t *testing.T, dialect, sql, targetVersion string) (int, map[string]any) {
	t.Helper()
	handler, err := NewHandler(t05A5HTTPIdentityPolicy(t), "test-build")
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}
	body := map[string]any{"sql": sql, "dialect": dialect, "schema": "golden"}
	if targetVersion != "" {
		body["target_version"] = targetVersion
	}
	payload, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/v1/audit", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	var decoded map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &decoded); err != nil {
		t.Fatalf("decode response: %v\nbody=%s", err, rec.Body.String())
	}
	return rec.Code, decoded
}

func TestHandlerT05A5ChangePublishes(t *testing.T) {
	code, payload := postT05A5Audit(t, "mysql", t05A5ChangeSQL, "")
	if code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %#v", code, payload)
	}
	if payload["verdict"] != "review" {
		t.Fatalf("expected review, got %#v", payload["verdict"])
	}
	statements, _ := payload["statements"].([]any)
	if len(statements) != 4 {
		t.Fatalf("expected 4 statements, got %#v", payload["statements"])
	}
	first, _ := statements[0].(map[string]any)
	gaps, _ := first["evidence_gaps"].([]any)
	if len(gaps) != 1 {
		t.Fatalf("create gaps = %#v", gaps)
	}
	gap, _ := gaps[0].(map[string]any)
	if gap["rule_id"] != "ddl.table.exists.create.forbid" || gap["reason_code"] != "unknown_table_state" {
		t.Fatalf("create gap = %#v", gap)
	}
	for _, index := range []int{1, 2, 3} {
		statement, _ := statements[index].(map[string]any)
		if coverage, _ := statement["coverage"].(map[string]any); coverage["status"] != "complete" {
			t.Fatalf("statement %d coverage = %#v", index, statement["coverage"])
		}
		if findings, ok := statement["findings"].([]any); ok && len(findings) != 0 {
			t.Fatalf("statement %d findings = %#v", index, findings)
		}
		if gaps, ok := statement["evidence_gaps"].([]any); ok && len(gaps) != 0 {
			t.Fatalf("statement %d gaps = %#v", index, gaps)
		}
	}
}

func TestHandlerT05A5RenameVersionReject(t *testing.T) {
	code, payload := postT05A5Audit(t, "mysql", t05A5RenameSQL(), "5.7.44")
	if code != http.StatusOK || payload["verdict"] != "reject" {
		t.Fatalf("status %d verdict %#v", code, payload["verdict"])
	}
	statements, _ := payload["statements"].([]any)
	rename, _ := statements[1].(map[string]any)
	findings, _ := rename["findings"].([]any)
	if len(findings) != 1 {
		t.Fatalf("findings = %#v", findings)
	}
	finding, _ := findings[0].(map[string]any)
	metadata, _ := finding["metadata"].(map[string]any)
	if finding["rule_id"] != "ddl.alter.rename_column.version.require" || metadata["target_version"] != "5.7.44" || metadata["minimum_supported_version"] != "8.0.3" {
		t.Fatalf("finding = %#v", finding)
	}
	next, _ := statements[2].(map[string]any)
	if gaps, _ := next["evidence_gaps"].([]any); len(gaps) == 0 {
		t.Fatal("incompatible RENAME published c2")
	}
}

func TestHandlerT05A5RenameVersionGap(t *testing.T) {
	code, payload := postT05A5Audit(t, "mysql", "ALTER TABLE t RENAME COLUMN c TO c2;", "")
	if code != http.StatusOK || payload["verdict"] != "review" {
		t.Fatalf("status %d verdict %#v", code, payload["verdict"])
	}
	statements, _ := payload["statements"].([]any)
	statement, _ := statements[0].(map[string]any)
	if findings, ok := statement["findings"].([]any); ok && len(findings) != 0 {
		t.Fatalf("missing version must not become a finding: %#v", findings)
	}
	gaps, _ := statement["evidence_gaps"].([]any)
	found := false
	for _, item := range gaps {
		gap, _ := item.(map[string]any)
		if gap["rule_id"] == "ddl.alter.rename_column.version.require" && gap["reason_code"] == "missing_target_version" {
			found = true
		}
	}
	if !found {
		t.Fatalf("gaps = %#v", gaps)
	}
}

func TestHandlerT05A5EmptySQLBadRequest(t *testing.T) {
	code, payload := postT05A5Audit(t, "mysql", "", "")
	if code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %#v", code, payload)
	}
	errBody, _ := payload["error"].(map[string]any)
	if errBody["code"] != "bad_request" {
		t.Fatalf("expected bad_request, got %#v", payload["error"])
	}
}
