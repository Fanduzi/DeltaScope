// Package httpapi verifies the T06-A12 ordinary CREATE TABLE non-PK-column
// NOT NULL contract at the HTTP seam.
package httpapi

import (
	"net/http"
	"testing"
)

// TestHandlerT06A12TimeExempt posts the DATETIME NULL declaration under the
// isolated time-allow policy — a clean pass result at HTTP 200.
func TestHandlerT06A12TimeExempt(t *testing.T) {
	policy := t06A8HTTPPolicy(t, map[string]string{
		"ddl.column.not_null.require": "      required: true\n      allow_time_null: true\n",
	})
	code, payload := postT06A8Audit(t, policy,
		"CREATE TABLE t (c DATETIME NULL);")
	if code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %#v", code, payload)
	}
	if payload["verdict"] != "pass" {
		t.Fatalf("verdict = %#v, want pass", payload["verdict"])
	}
}

// TestHandlerT06A12MixedReject posts the three-column declaration under the
// isolated time-allow policy — the reject stays a populated 200 audit result
// carrying exactly the not_null blocker bound to the ordinary column.
func TestHandlerT06A12MixedReject(t *testing.T) {
	policy := t06A8HTTPPolicy(t, map[string]string{
		"ddl.column.not_null.require": "      required: true\n      allow_time_null: true\n",
	})
	code, payload := postT06A8Audit(t, policy,
		"CREATE TABLE t (ordinary INT, time_c DATETIME NULL, payload TEXT);")
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
	if finding["rule_id"] != "ddl.column.not_null.require" ||
		finding["level"] != "blocker" ||
		finding["message"] != `column "ordinary" should be declared NOT NULL` ||
		metadata["column"] != "ordinary" {
		t.Fatalf("finding = %#v, want the not_null blocker bound to ordinary", finding)
	}
}
