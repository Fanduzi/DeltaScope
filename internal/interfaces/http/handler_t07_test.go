// Package httpapi verifies the T07 advanced-column transport contract.
// input: /v1/audit JSON requests carrying generated/expression-default/
// visibility SQL for both dialects under the default policy
// output: success, version-reject, vendor-boundary diagnostic, and
// privacy-bounded unsupported shapes through the shared audit seam
// pos: HTTP transport coverage for issue #86 advanced columns
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

// t07HTTPPolicy writes the frozen t07-all profile: the two T07 rules plus the
// index-existence and rename-version rules enabled, everything else disabled.
func t07HTTPPolicy(t *testing.T) string {
	t.Helper()
	enabled := map[string]string{
		"ddl.alter.column.dependency.require":       "      required: true\n",
		"ddl.column.advanced.compatibility.require": "      required: true\n",
		"ddl.create_index.columns.exists.require":   "",
		"ddl.alter.rename_column.version.require":   "      required: true\n",
	}
	var builder strings.Builder
	builder.WriteString("rules:\n")
	ids := make([]string, 0, len(policy.Default().Rules))
	for id := range policy.Default().Rules {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		if params, keep := enabled[id]; keep {
			fmt.Fprintf(&builder, "  %s:\n    enabled: true\n    level: blocker\n", strconv.Quote(id))
			if params != "" {
				builder.WriteString("    params:\n")
				builder.WriteString(params)
			}
			continue
		}
		fmt.Fprintf(&builder, "  %s:\n    enabled: false\n", strconv.Quote(id))
	}
	path := filepath.Join(t.TempDir(), "t07-http-policy.yaml")
	if err := os.WriteFile(path, []byte(builder.String()), 0o600); err != nil {
		t.Fatalf("write policy: %v", err)
	}
	return path
}

func postAuditT07(t *testing.T, handler http.Handler, body string) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/v1/audit", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response (status %d): %v\n%s", rec.Code, err, rec.Body.String())
	}
	return rec, payload
}

// TestHandlerAuditT07Visibility proves the shared seam through HTTP: a
// supported MySQL 8.0.23+ visibility declaration passes, the MySQL 5.7 form
// rejects at blocker level, and the same syntax under TiDB returns the
// vendor-boundary unsupported envelope with diagnostics.
func TestHandlerAuditT07Visibility(t *testing.T) {
	handler, err := NewHandler(t07HTTPPolicy(t), "test-build")
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}
	sql := "CREATE TABLE t (id INT PRIMARY KEY, hidden_c INT INVISIBLE, visible_c INT)"

	t.Run("mysql80_pass", func(t *testing.T) {
		rec, payload := postAuditT07(t, handler,
			`{"sql":`+strconvQuote(sql)+`,"dialect":"mysql","target_version":"8.0.46"}`)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
		}
		if payload["verdict"] != "pass" {
			t.Fatalf("verdict = %#v, want pass: %+v", payload["verdict"], payload)
		}
		if c, _ := payload["coverage"].(map[string]any); c["status"] != "complete" {
			t.Fatalf("coverage = %#v, want complete", payload["coverage"])
		}
	})

	t.Run("mysql57_reject", func(t *testing.T) {
		rec, payload := postAuditT07(t, handler,
			`{"sql":`+strconvQuote(sql)+`,"dialect":"mysql","target_version":"5.7.44"}`)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
		}
		if payload["verdict"] != "reject" {
			t.Fatalf("verdict = %#v, want reject", payload["verdict"])
		}
		stmts, _ := payload["statements"].([]any)
		first, _ := stmts[0].(map[string]any)
		findings, _ := first["findings"].([]any)
		if len(findings) != 1 {
			t.Fatalf("findings = %#v, want one version blocker", findings)
		}
		finding, _ := findings[0].(map[string]any)
		meta, _ := finding["metadata"].(map[string]any)
		if meta["reason_code"] != "version_unsupported" || meta["feature"] != "visibility" ||
			meta["minimum_version"] != "8.0.23" {
			t.Fatalf("finding metadata = %#v", meta)
		}
	})

	t.Run("tidb_vendor_boundary", func(t *testing.T) {
		rec, payload := postAuditT07(t, handler,
			`{"sql":`+strconvQuote(sql)+`,"dialect":"tidb","target_version":"8.5.0"}`)
		if rec.Code == http.StatusOK {
			t.Fatalf("status = 200, want a diagnostic error: %s", rec.Body.String())
		}
		unsupported, _ := payload["unsupported"].([]any)
		if len(unsupported) != 1 {
			t.Fatalf("unsupported = %#v, want one vendor boundary entry", unsupported)
		}
		entry, _ := unsupported[0].(map[string]any)
		if entry["feature"] != "create_table.column.visibility" {
			t.Fatalf("unsupported feature = %#v", entry["feature"])
		}
		if diagnostics, _ := payload["diagnostics"].([]any); len(diagnostics) != 1 {
			t.Fatalf("diagnostics = %#v, want one", payload["diagnostics"])
		}
	})
}

// TestHandlerAuditT07DependencyBlocker proves the dependency rule's finding
// reaches the HTTP body untouched — the adapter does no analysis of its own.
func TestHandlerAuditT07DependencyBlocker(t *testing.T) {
	handler, err := NewHandler(t07HTTPPolicy(t), "test-build")
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}
	sql := "CREATE TABLE t (a INT, b INT GENERATED ALWAYS AS (a + 1) STORED); ALTER TABLE t DROP COLUMN a;"
	rec, payload := postAuditT07(t, handler,
		`{"sql":`+strconvQuote(sql)+`,"dialect":"mysql","target_version":"8.0.46"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if payload["verdict"] != "reject" {
		t.Fatalf("verdict = %#v, want reject", payload["verdict"])
	}
	stmts, _ := payload["statements"].([]any)
	second, _ := stmts[1].(map[string]any)
	findings, _ := second["findings"].([]any)
	if len(findings) != 1 {
		t.Fatalf("findings = %#v, want one dependency blocker", findings)
	}
	finding, _ := findings[0].(map[string]any)
	if finding["rule_id"] != "ddl.alter.column.dependency.require" {
		t.Fatalf("rule_id = %#v", finding["rule_id"])
	}
	meta, _ := finding["metadata"].(map[string]any)
	if meta["dependent_column"] != "b" || meta["dependency_kind"] != "generated" {
		t.Fatalf("finding metadata = %#v", meta)
	}
}

// TestHandlerAuditT07UnknownFunctionPrivacy pins the C11 boundary over HTTP:
// an unprovable expression emits the bounded unsupported marker whose
// metadata carries only the fixed reason — the authored literal survives only
// inside the raw-sql echo fields the contract allows.
func TestHandlerAuditT07UnknownFunctionPrivacy(t *testing.T) {
	handler, err := NewHandler(t07HTTPPolicy(t), "test-build")
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}
	sql := "CREATE TABLE t (a INT, b VARCHAR(64) AS (T07_UNKNOWN_FN('T07_PRIVATE_LITERAL_6b4d1e')) VIRTUAL)"
	rec, payload := postAuditT07(t, handler,
		`{"sql":`+strconvQuote(sql)+`,"dialect":"mysql","target_version":"8.0.46"}`)
	if rec.Code == http.StatusOK {
		t.Fatalf("status = 200, want a diagnostic error: %s", rec.Body.String())
	}
	unsupported, _ := payload["unsupported"].([]any)
	if len(unsupported) != 1 {
		t.Fatalf("unsupported = %#v, want one entry", unsupported)
	}
	entry, _ := unsupported[0].(map[string]any)
	if entry["feature"] != "create_table.column.expression_dependencies" {
		t.Fatalf("unsupported feature = %#v", entry["feature"])
	}
	meta, _ := entry["metadata"].(map[string]any)
	if meta["reason"] != "unknown_function" || meta["aspect"] != "column" {
		t.Fatalf("unsupported metadata = %#v", meta)
	}
	// Privacy: the literal may only echo inside the raw sql field, never in
	// the feature/reason/metadata envelope.
	for _, key := range []string{"feature", "reason"} {
		if s, _ := entry[key].(string); strings.Contains(s, "T07_PRIVATE_LITERAL") {
			t.Fatalf("unsupported %s leaked the literal: %#v", key, entry[key])
		}
	}
}

func strconvQuote(s string) string {
	raw, _ := json.Marshal(s)
	return string(raw)
}
