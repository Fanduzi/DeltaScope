// Package httpapi verifies the T06-A9 audit-column role contract at the HTTP
// seam — the complete pair and the two-blocker missing-both reject are both
// ordinary audit payloads at HTTP 200, never transport errors.
// input: POST /v1/audit bodies carrying explicit DATETIME audit columns under the isolated policy
// output: HTTP 200 with pass or two-finding reject audit payloads
// pos: HTTP transport contract test for issue #85 T06-A9
// note: if this file changes, update this header and module README.md.
package httpapi

import (
	"net/http"
	"testing"
)

// TestHandlerT06A9AuditColumnPair posts the complete declared pair — a clean
// pass result, no findings.
func TestHandlerT06A9AuditColumnPair(t *testing.T) {
	policy := t06A8HTTPPolicy(t, map[string]string{
		"ddl.table.audit_columns.require": "      required: true\n",
	})
	code, payload := postT06A8Audit(t, policy,
		"CREATE TABLE t (id INT PRIMARY KEY, created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP, updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP);")
	if code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %#v", code, payload)
	}
	if payload["verdict"] != "pass" {
		t.Fatalf("verdict = %#v, want pass", payload["verdict"])
	}
}

// TestHandlerT06A9AuditColumnMissingBoth posts a table without audit columns
// — a policy reject stays a populated 200 audit result carrying exactly the
// created+updated blocker pair.
func TestHandlerT06A9AuditColumnMissingBoth(t *testing.T) {
	policy := t06A8HTTPPolicy(t, map[string]string{
		"ddl.table.audit_columns.require": "      required: true\n",
	})
	code, payload := postT06A8Audit(t, policy, "CREATE TABLE t (id INT PRIMARY KEY);")
	if code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %#v", code, payload)
	}
	if payload["verdict"] != "reject" {
		t.Fatalf("verdict = %#v, want reject", payload["verdict"])
	}
	statements, _ := payload["statements"].([]any)
	if len(statements) != 1 {
		t.Fatalf("statements = %#v, want 1", statements)
	}
	findings, _ := statements[0].(map[string]any)["findings"].([]any)
	if len(findings) != 2 {
		t.Fatalf("findings = %#v, want exactly 2", findings)
	}
	kinds := map[string]bool{}
	for _, raw := range findings {
		f, _ := raw.(map[string]any)
		if f["rule_id"] != "ddl.table.audit_columns.require" || f["level"] != "blocker" {
			t.Fatalf("finding = %#v, want audit-columns blocker", f)
		}
		metadata, _ := f["metadata"].(map[string]any)
		kind, _ := metadata["kind"].(string)
		if kinds[kind] {
			t.Fatalf("duplicate role %q in %#v", kind, findings)
		}
		kinds[kind] = true
	}
	if !kinds["created"] || !kinds["updated"] {
		t.Fatalf("kinds = %#v, want created+updated pair", findings)
	}
}
