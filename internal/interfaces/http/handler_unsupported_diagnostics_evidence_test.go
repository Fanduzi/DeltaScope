package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestUnsupportedDiagnosticsEvidenceHTTPParserError(t *testing.T) {
	t.Parallel()

	handler, err := NewHandler("", "test-build")
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/audit", bytes.NewBufferString(`{"sql":"CREATE FUNCTION hello() RETURNS VARCHAR(20) RETURN 'secret_body_value'","dialect":"mysql"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code == http.StatusOK {
		t.Fatalf("expected non-200 status for parser-error SQL, got %d: %s", rec.Code, rec.Body.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if diags, ok := payload["diagnostics"].([]any); ok && len(diags) > 0 {
		first, ok := diags[0].(map[string]any)
		if !ok {
			t.Fatalf("expected diagnostic object, got %T", diags[0])
		}
		classification, _ := first["classification"].(string)
		if classification != "parser_error" {
			t.Fatalf("expected classification parser_error, got %q", classification)
		}
		reason, _ := first["reason"].(string)
		if !strings.Contains(strings.ToLower(reason), "not audited") {
			t.Fatalf("expected reason containing 'not audited', got %q", reason)
		}
		actionHint, _ := first["action_hint"].(string)
		if !strings.Contains(strings.ToLower(actionHint), "verify the selected dialect") {
			t.Fatalf("expected action_hint, got %q", actionHint)
		}
		audited, _ := first["audited"].(bool)
		if audited {
			t.Fatal("expected audited=false")
		}
		dialect, _ := first["dialect"].(string)
		if dialect != "mysql" {
			t.Fatalf("expected dialect mysql, got %q", dialect)
		}
		guidanceCode, _ := first["guidance_code"].(string)
		if guidanceCode != "parser_upgrade_candidate" {
			t.Fatalf("expected guidance_code parser_upgrade_candidate, got %q", guidanceCode)
		}
		evidenceRef, _ := first["evidence_ref"].(string)
		if !strings.HasPrefix(evidenceRef, "https://github.com/Fanduzi/DeltaScope/") {
			t.Fatalf("expected GitHub evidence_ref URL, got %q", evidenceRef)
		}
	} else {
		errPayload, ok := payload["error"].(map[string]any)
		if !ok {
			t.Fatalf("expected error envelope or diagnostics, got %#v", payload)
		}
		message, _ := errPayload["message"].(string)
		lower := strings.ToLower(message)
		if !strings.Contains(lower, "not audited") && !strings.Contains(lower, "parser_error") {
			t.Fatalf("expected not-audited or parser_error in error message, got %q", message)
		}
		if !strings.Contains(lower, "action_hint") && !strings.Contains(lower, "verify the selected dialect") {
			t.Fatalf("expected action_hint or verify-the-dialect guidance in error message, got %q", message)
		}
		if !strings.Contains(lower, "classification") && !strings.Contains(lower, "parser_error") {
			t.Fatalf("expected classification field or parser_error mention, got %q", message)
		}
	}

	body := strings.ToLower(rec.Body.String())
	if strings.Contains(body, "secret_body_value") {
		t.Fatalf("HTTP response leaked forbidden payload in %q", rec.Body.String())
	}
	if strings.Contains(body, "near ") {
		t.Fatalf("HTTP response leaked raw parser fragment in %q", rec.Body.String())
	}
}
