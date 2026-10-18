// Package httpapi verifies the T06-A15 row-size field-level fact contract at
// the HTTP seam.
// input: POST /v1/audit bodies carrying the frozen over-limit,
// missing-charset, and determined-finding-plus-gap inputs under the
// isolated row-size policy — all without a metadata provider
// output: HTTP 200 with an ordinary reject payload, an ordinary
// review/unverified payload carrying the field-scoped gap, and a reject
// payload carrying both — policy outcomes stay ordinary audit results,
// never transport errors
// pos: HTTP transport contract test for issue #85 T06-A15
// note: if this file changes, update this header and module README.md.
package httpapi

import (
	"net/http"
	"testing"
)

// TestHandlerT06A15DeclaredOverLimitReject posts the declaration-complete
// over-limit input — the estimate needs no instance object, so the blocker
// returns as a populated 200 audit result.
func TestHandlerT06A15DeclaredOverLimitReject(t *testing.T) {
	policy := t06A8HTTPPolicy(t, map[string]string{
		"ddl.table.row_size.max_bytes.require": "      required: true\n",
	})
	code, payload := postT06A8Audit(t, policy,
		"CREATE TABLE t (c VARCHAR(65534)) ENGINE=InnoDB DEFAULT CHARSET=latin1 ROW_FORMAT=DYNAMIC;")
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
	if finding["rule_id"] != "ddl.table.row_size.max_bytes.require" ||
		finding["level"] != "blocker" ||
		finding["message"] != "estimated row size exceeds 65535 bytes (65536)" ||
		metadata["charset"] != "latin1" ||
		metadata["row_format"] != "DYNAMIC" ||
		metadata["estimated"] != float64(65536) {
		t.Fatalf("finding = %#v, want the over-limit blocker with declared metadata", finding)
	}
}

// TestHandlerT06A15MissingCharsetReview posts the missing-charset input —
// the unresolvable width reports a field-scoped evidence gap inside an
// ordinary review payload, never an unsupported error.
func TestHandlerT06A15MissingCharsetReview(t *testing.T) {
	policy := t06A8HTTPPolicy(t, map[string]string{
		"ddl.table.row_size.max_bytes.require": "      required: true\n",
	})
	code, payload := postT06A8Audit(t, policy,
		"CREATE TABLE t (c VARCHAR(65534)) ENGINE=InnoDB ROW_FORMAT=DYNAMIC;")
	if code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %#v", code, payload)
	}
	if payload["verdict"] != "review" {
		t.Fatalf("verdict = %#v, want review", payload["verdict"])
	}
	coverage, _ := payload["coverage"].(map[string]any)
	if coverage["status"] != "unverified" {
		t.Fatalf("coverage = %#v, want unverified", coverage)
	}
	statements, _ := payload["statements"].([]any)
	if len(statements) != 1 {
		t.Fatalf("statements = %#v, want 1", statements)
	}
	statement, _ := statements[0].(map[string]any)
	if findings, _ := statement["findings"].([]any); len(findings) != 0 {
		t.Fatalf("findings = %#v, want none", findings)
	}
	gaps, _ := statement["evidence_gaps"].([]any)
	if len(gaps) != 1 {
		t.Fatalf("evidence_gaps = %#v, want exactly one", gaps)
	}
	gap, _ := gaps[0].(map[string]any)
	facts, _ := gap["required_facts"].([]any)
	if gap["rule_id"] != "ddl.table.row_size.max_bytes.require" ||
		gap["reason_code"] != "missing_instance_fact" ||
		len(facts) != 1 || facts[0] != "instance.default_charset" {
		t.Fatalf("gap = %#v, want the default_charset missing_instance_fact", gap)
	}
}

// TestHandlerT06A15MissingFormatOverReject posts the missing-row-format
// input that still proves the total — the determined blocker survives
// beside the field-scoped gap under unverified coverage.
func TestHandlerT06A15MissingFormatOverReject(t *testing.T) {
	policy := t06A8HTTPPolicy(t, map[string]string{
		"ddl.table.row_size.max_bytes.require": "      required: true\n",
	})
	code, payload := postT06A8Audit(t, policy,
		"CREATE TABLE t (c VARCHAR(65534)) ENGINE=InnoDB DEFAULT CHARSET=latin1;")
	if code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %#v", code, payload)
	}
	if payload["verdict"] != "reject" {
		t.Fatalf("verdict = %#v, want reject", payload["verdict"])
	}
	coverage, _ := payload["coverage"].(map[string]any)
	if coverage["status"] != "unverified" {
		t.Fatalf("coverage = %#v, want unverified", coverage)
	}
	statements, _ := payload["statements"].([]any)
	if len(statements) != 1 {
		t.Fatalf("statements = %#v, want 1", statements)
	}
	statement, _ := statements[0].(map[string]any)
	findings, _ := statement["findings"].([]any)
	if len(findings) != 1 {
		t.Fatalf("findings = %#v, want the determined blocker", findings)
	}
	finding, _ := findings[0].(map[string]any)
	metadata, _ := finding["metadata"].(map[string]any)
	if finding["rule_id"] != "ddl.table.row_size.max_bytes.require" ||
		metadata["estimated"] != float64(65536) ||
		metadata["row_format"] != "" {
		t.Fatalf("finding = %#v, want the blocker with row_format=\"\"", finding)
	}
	gaps, _ := statement["evidence_gaps"].([]any)
	if len(gaps) != 1 {
		t.Fatalf("evidence_gaps = %#v, want exactly one", gaps)
	}
	gap, _ := gaps[0].(map[string]any)
	facts, _ := gap["required_facts"].([]any)
	if gap["reason_code"] != "missing_instance_fact" ||
		len(facts) != 1 || facts[0] != "instance.innodb_default_row_format" {
		t.Fatalf("gap = %#v, want the row-format missing_instance_fact", gap)
	}
}
