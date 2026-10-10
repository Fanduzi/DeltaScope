// Package httpapi verifies the T06-A10 AUTO_INCREMENT contract at the HTTP seam.
// input: POST /v1/audit bodies carrying the frozen pk-auto and init-mismatch
// inputs under their isolated policies
// output: HTTP 200 with a pass payload and a one-blocker reject payload —
// policy rejects stay ordinary audit results, never transport errors
// pos: HTTP transport contract test for issue #85 T06-A10
// note: if this file changes, update this header and module README.md.
package httpapi

import (
	"net/http"
	"testing"
)

// TestHandlerT06A10PrimaryKeyAutoIncrement posts the pk-auto declaration — a
// clean pass result at HTTP 200.
func TestHandlerT06A10PrimaryKeyAutoIncrement(t *testing.T) {
	policy := t06A8HTTPPolicy(t, map[string]string{
		"ddl.table.primary_key.auto_increment.require": "      required: true\n",
	})
	code, payload := postT06A8Audit(t, policy,
		"CREATE TABLE t (id BIGINT AUTO_INCREMENT PRIMARY KEY);")
	if code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %#v", code, payload)
	}
	if payload["verdict"] != "pass" {
		t.Fatalf("verdict = %#v, want pass", payload["verdict"])
	}
}

// TestHandlerT06A10InitValueMismatch posts the AUTO_INCREMENT=9 declaration
// under the value=8 init policy — the reject stays a populated 200 audit
// result carrying exactly the init-value blocker.
func TestHandlerT06A10InitValueMismatch(t *testing.T) {
	policy := t06A8HTTPPolicy(t, map[string]string{
		"ddl.table.auto_increment.init_value.require": "      value: 8\n",
	})
	code, payload := postT06A8Audit(t, policy,
		"CREATE TABLE t (id BIGINT AUTO_INCREMENT PRIMARY KEY) AUTO_INCREMENT=9;")
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
	if finding["rule_id"] != "ddl.table.auto_increment.init_value.require" ||
		finding["level"] != "blocker" ||
		finding["message"] != "table auto_increment init value must be 8" {
		t.Fatalf("finding = %#v, want the init-value blocker", finding)
	}
	metadata, _ := finding["metadata"].(map[string]any)
	if metadata["table"] != "t" || metadata["required_value"] != float64(8) || metadata["actual_value"] != float64(9) {
		t.Fatalf("metadata = %#v, want table=t required=8 actual=9", metadata)
	}
}
