// Package httpapi verifies the T06-A13 BLOB/TEXT forbid policy at the HTTP
// seam.
package httpapi

import (
	"net/http"
	"testing"
)

// TestHandlerT06A13BlobTextReject posts the mixed declaration under the
// isolated blob-text blocker policy — the reject stays a populated 200
// audit result carrying exactly the two column-bound findings (c_json is
// not swept in).
func TestHandlerT06A13BlobTextReject(t *testing.T) {
	policy := t06A8HTTPPolicy(t, map[string]string{
		"ddl.column.blob_text.forbid": "      forbid: true\n",
	})
	code, payload := postT06A8Audit(t, policy,
		"CREATE TABLE t (c_blob BLOB, c_text TEXT, c_json JSON, c_bit BIT(1), c_float FLOAT, c_double DOUBLE, c_timestamp TIMESTAMP NULL, c_datetime DATETIME NULL, c_int INT);")
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
		t.Fatalf("findings = %#v, want exactly two", findings)
	}
	for i, want := range []string{"c_blob", "c_text"} {
		finding, _ := findings[i].(map[string]any)
		metadata, _ := finding["metadata"].(map[string]any)
		if finding["rule_id"] != "ddl.column.blob_text.forbid" ||
			finding["level"] != "blocker" ||
			finding["message"] != `column "`+want+`" must not use the blob/text type family` ||
			metadata["column"] != want {
			t.Fatalf("finding[%d] = %#v, want the blob/text blocker bound to %s", i, finding, want)
		}
	}
}

// TestHandlerT06A13BlobTextSafe posts the non-matching declaration under
// the same isolated policy — a clean pass at HTTP 200.
func TestHandlerT06A13BlobTextSafe(t *testing.T) {
	policy := t06A8HTTPPolicy(t, map[string]string{
		"ddl.column.blob_text.forbid": "      forbid: true\n",
	})
	code, payload := postT06A8Audit(t, policy,
		"CREATE TABLE t (c_int INT, c_string VARCHAR(8), c_datetime DATETIME NULL);")
	if code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %#v", code, payload)
	}
	if payload["verdict"] != "pass" {
		t.Fatalf("verdict = %#v, want pass", payload["verdict"])
	}
}
