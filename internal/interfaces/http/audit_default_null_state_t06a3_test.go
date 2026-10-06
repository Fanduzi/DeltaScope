// Package httpapi verifies the T06-A3 typed DEFAULT NULL contract at the HTTP seam.
// input: MySQL/TiDB CREATE+DROP+INDEX /v1/audit requests with the isolated four-rule drop-state policy
// output: HTTP 200 pass for the three-statement path with a known initial state, HTTP 200 reject for a missing DEFAULT
// pos: HTTP transport contract test for the frozen T06-A3 oracle (issue #85)
// note: if this file changes, update this header and module README.md.
package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/rule/catalog"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
	"github.com/Fanduzi/DeltaScope/pkg/deltascope"
)

const t06A3HTTPSQL = "CREATE TABLE t (id INT PRIMARY KEY, obsolete INT, keep_c VARCHAR(8) DEFAULT NULL);\n" +
	"ALTER TABLE t DROP COLUMN obsolete;\n" +
	"CREATE INDEX idx_keep ON t(keep_c);"

type t06A3HTTPAbsentProvider struct{}

func (t06A3HTTPAbsentProvider) LoadInstanceFacts(context.Context, deltascope.Dialect, string) (*deltascope.InstanceFacts, error) {
	return &deltascope.InstanceFacts{}, nil
}

func (t06A3HTTPAbsentProvider) LoadTableSnapshot(_ context.Context, _ deltascope.Dialect, schema, table string) (*deltascope.TableSnapshot, error) {
	return &spec.TableSnapshot{Exists: false, Table: &spec.Table{Schema: schema, Name: table}}, nil
}

func t06A3HTTPPolicy(t *testing.T, enabled map[string]bool) string {
	t.Helper()
	required := map[string]bool{
		"ddl.create_index.columns.exists.require": true,
		"ddl.column.default.require":              true,
	}
	var text strings.Builder
	text.WriteString("rules:\n")
	for _, entry := range catalog.All() {
		if enabled[entry.RuleID] {
			fmt.Fprintf(&text, "  %q:\n    enabled: true\n    level: blocker\n", entry.RuleID)
			if required[entry.RuleID] {
				text.WriteString("    params:\n      required: true\n")
			}
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

func postT06A3Audit(t *testing.T, policy, dialect, sql string, provider deltascope.MetadataProvider) (int, map[string]any) {
	t.Helper()
	auditFn := func(ctx context.Context, request deltascope.Request) (deltascope.Result, error) {
		request.MetadataProvider = provider
		return deltascope.Audit(ctx, request)
	}
	handler, err := NewHandler(policy, "test-build", WithAuditFunc(auditFn))
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
		t.Fatalf("decode response: %v", err)
	}
	return rec.Code, payload
}

func TestHandlerT06A3NullDropState(t *testing.T) {
	policy := t06A3HTTPPolicy(t, map[string]bool{
		"ddl.table.exists.create.forbid":          true,
		"ddl.table.exists.alter.require":          true,
		"ddl.alter.drop_column.exists.require":    true,
		"ddl.create_index.columns.exists.require": true,
	})
	for _, dialect := range []string{"mysql", "tidb"} {
		t.Run(dialect, func(t *testing.T) {
			code, payload := postT06A3Audit(t, policy, dialect, t06A3HTTPSQL, t06A3HTTPAbsentProvider{})
			if code != http.StatusOK {
				t.Fatalf("status = %d, want 200: %#v", code, payload)
			}
			if payload["verdict"] != "pass" || payload["error"] != nil {
				t.Fatalf("verdict = %#v error = %#v, want pass", payload["verdict"], payload["error"])
			}
			if coverage, _ := payload["coverage"].(map[string]any); coverage["status"] != "complete" {
				t.Fatalf("coverage = %#v, want complete", payload["coverage"])
			}
			statements, _ := payload["statements"].([]any)
			if len(statements) != 3 {
				t.Fatalf("statements = %d, want 3", len(statements))
			}
			for i, raw := range statements {
				statement, _ := raw.(map[string]any)
				if findings, _ := statement["findings"].([]any); len(findings) != 0 {
					t.Fatalf("statement %d findings = %#v, want none", i, findings)
				}
				if gaps := statement["evidence_gaps"]; gaps != nil {
					t.Fatalf("statement %d gaps = %#v, want none", i, gaps)
				}
			}
		})
	}
}

func TestHandlerT06A3DefaultRequire(t *testing.T) {
	policy := t06A3HTTPPolicy(t, map[string]bool{"ddl.column.default.require": true})
	cases := []struct {
		name    string
		sql     string
		verdict string
	}{
		{"mysql_no_default", "CREATE TABLE t (c VARCHAR(8));", "reject"},
		{"mysql_default_null", "CREATE TABLE t (c VARCHAR(8) DEFAULT NULL);", "pass"},
		{"tidb_no_default", "CREATE TABLE t (c VARCHAR(8));", "reject"},
		{"tidb_default_null", "CREATE TABLE t (c VARCHAR(8) DEFAULT NULL);", "pass"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dialect := "mysql"
			if strings.HasPrefix(tc.name, "tidb") {
				dialect = "tidb"
			}
			code, payload := postT06A3Audit(t, policy, dialect, tc.sql, nil)
			if code != http.StatusOK {
				t.Fatalf("status = %d, want 200: %#v", code, payload)
			}
			if payload["verdict"] != tc.verdict || payload["error"] != nil {
				t.Fatalf("verdict = %#v error = %#v, want %s", payload["verdict"], payload["error"], tc.verdict)
			}
		})
	}
}
