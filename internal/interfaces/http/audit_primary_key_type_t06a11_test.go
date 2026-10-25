// Package httpapi verifies the T06-A11 single-member primary-key
// BIGINT/UNSIGNED contract at the HTTP seam.
package httpapi

import (
	"net/http"
	"testing"
)

// TestHandlerT06A11UnsignedPass posts the unsigned declaration under the
// isolated unsigned policy — a clean pass result at HTTP 200.
func TestHandlerT06A11UnsignedPass(t *testing.T) {
	policy := t06A8HTTPPolicy(t, map[string]string{
		"ddl.table.primary_key.unsigned.require": "      required: true\n",
	})
	code, payload := postT06A8Audit(t, policy,
		"CREATE TABLE t (id BIGINT UNSIGNED PRIMARY KEY);")
	if code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %#v", code, payload)
	}
	if payload["verdict"] != "pass" {
		t.Fatalf("verdict = %#v, want pass", payload["verdict"])
	}
}

// TestHandlerT06A11BigintReject posts the signed-INT declaration under the
// isolated bigint policy — the reject stays a populated 200 audit result
// carrying exactly the bigint blocker.
func TestHandlerT06A11BigintReject(t *testing.T) {
	policy := t06A8HTTPPolicy(t, map[string]string{
		"ddl.table.primary_key.bigint.require": "      required: true\n",
	})
	code, payload := postT06A8Audit(t, policy,
		"CREATE TABLE t (id INT PRIMARY KEY);")
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
	if finding["rule_id"] != "ddl.table.primary_key.bigint.require" ||
		finding["level"] != "blocker" ||
		finding["message"] != `primary key column "id" must use bigint` {
		t.Fatalf("finding = %#v, want the bigint blocker", finding)
	}
	metadata, _ := finding["metadata"].(map[string]any)
	if metadata["table"] != "t" || metadata["column"] != "id" ||
		metadata["type"] != "int(11)" {
		t.Fatalf("metadata = %#v, want table=t column=id type=int(11)", metadata)
	}
}
