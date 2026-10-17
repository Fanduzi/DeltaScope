// Package httpapi verifies the T06-A14 table ROW_FORMAT allowlist policy at
// the HTTP seam.
// input: POST /v1/audit bodies carrying the omitted ROW_FORMAT declaration
// under the isolated strict and optional profiles
// output: HTTP 200 with a one-blocker reject payload under strict and a
// pass payload under optional — policy rejects stay ordinary audit
// results, never transport errors
// pos: HTTP transport contract test for issue #85 T06-A14
// note: if this file changes, update this header and module README.md.
package httpapi

import (
	"net/http"
	"testing"
)

// TestHandlerT06A14RowFormatOmittedStrictReject posts the option-less
// declaration under the isolated strict profile — require_explicit=true
// turns omission into the single row_format blocker with actual="".
func TestHandlerT06A14RowFormatOmittedStrictReject(t *testing.T) {
	policy := t06A8HTTPPolicy(t, map[string]string{
		"ddl.table.row_format.allowlist": "      values: [\"DYNAMIC\"]\n      require_explicit: true\n",
	})
	code, payload := postT06A8Audit(t, policy, "CREATE TABLE t (c INT);")
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
	if len(findings) != 1 {
		t.Fatalf("findings = %#v, want exactly one", findings)
	}
	finding, _ := findings[0].(map[string]any)
	metadata, _ := finding["metadata"].(map[string]any)
	if finding["rule_id"] != "ddl.table.row_format.allowlist" ||
		finding["level"] != "blocker" ||
		finding["message"] != "table row format must be one of [DYNAMIC]" ||
		metadata["option"] != "row_format" ||
		metadata["actual"] != "" {
		t.Fatalf("finding = %#v, want the row_format blocker with actual=\"\"", finding)
	}
}

// TestHandlerT06A14RowFormatOmittedOptionalPass posts the same option-less
// declaration under the isolated optional profile — the omitted declaration
// stays applicable but produces no finding at HTTP 200.
func TestHandlerT06A14RowFormatOmittedOptionalPass(t *testing.T) {
	policy := t06A8HTTPPolicy(t, map[string]string{
		"ddl.table.row_format.allowlist": "      values: [\"DYNAMIC\"]\n      require_explicit: false\n",
	})
	code, payload := postT06A8Audit(t, policy, "CREATE TABLE t (c INT);")
	if code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %#v", code, payload)
	}
	if payload["verdict"] != "pass" {
		t.Fatalf("verdict = %#v, want pass", payload["verdict"])
	}
	statements, _ := payload["statements"].([]any)
	if len(statements) != 1 {
		t.Fatalf("statements = %#v, want 1", statements)
	}
	if findings, _ := statements[0].(map[string]any)["findings"].([]any); len(findings) != 0 {
		t.Fatalf("findings = %#v, want none", findings)
	}
}
