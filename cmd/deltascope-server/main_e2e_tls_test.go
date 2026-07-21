//go:build e2e && tls

package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
)

func TestTLSQueryAccessMySQLSucceedsWithTrustedCA(t *testing.T) {
	serverAddr := os.Getenv("TLS_E2E_SERVER_ADDR")
	if serverAddr == "" {
		t.Fatal("TLS_E2E_SERVER_ADDR not set")
	}

	// Use simple SELECT without scalar functions - this is in the admissible set.
	resp, body := doQueryAccessRequest(t, serverAddr, "mysql-tls", "SELECT id, name FROM app.users WHERE id = 1")

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, string(body))
	}

	result := parseJSON(t, body)
	assertOnlineQueryAccessResult(t, result, "mysql", "app.users", "id")
}

func TestTLSQueryAccessPostgreSQLSucceedsWithTrustedCA(t *testing.T) {
	serverAddr := os.Getenv("TLS_E2E_SERVER_ADDR")
	if serverAddr == "" {
		t.Fatal("TLS_E2E_SERVER_ADDR not set")
	}

	// Use simple SELECT without scalar functions - this is in the admissible set.
	resp, body := doQueryAccessRequest(t, serverAddr, "postgresql-tls", "SELECT id, name FROM app.users WHERE id = 1")

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, string(body))
	}

	result := parseJSON(t, body)
	assertOnlineQueryAccessResult(t, result, "postgresql", "app.users", "id")
}

func TestTLSQueryAccessMySQLFailsWithUntrustedCA(t *testing.T) {
	serverAddr := os.Getenv("TLS_E2E_SERVER_ADDR")
	if serverAddr == "" {
		t.Fatal("TLS_E2E_SERVER_ADDR not set")
	}

	resp, body := doQueryAccessRequest(t, serverAddr, "mysql-tls-untrusted", "SELECT id FROM app.users")

	if resp.StatusCode != http.StatusBadGateway {
		t.Fatalf("expected 502, got %d: %s", resp.StatusCode, string(body))
	}

	result := parseJSON(t, body)
	assertBoundedError(t, result, "connection_failed")
	assertNoLeaks(t, body)
}

func TestTLSQueryAccessPostgreSQLFailsWithUntrustedCA(t *testing.T) {
	serverAddr := os.Getenv("TLS_E2E_SERVER_ADDR")
	if serverAddr == "" {
		t.Fatal("TLS_E2E_SERVER_ADDR not set")
	}

	resp, body := doQueryAccessRequest(t, serverAddr, "postgresql-tls-untrusted", "SELECT id FROM app.users")

	if resp.StatusCode != http.StatusBadGateway {
		t.Fatalf("expected 502, got %d: %s", resp.StatusCode, string(body))
	}

	result := parseJSON(t, body)
	assertBoundedError(t, result, "connection_failed")
	assertNoLeaks(t, body)
}

func TestTLSQueryAccessMySQLFailsWithHostnameMismatch(t *testing.T) {
	serverAddr := os.Getenv("TLS_E2E_SERVER_ADDR")
	if serverAddr == "" {
		t.Fatal("TLS_E2E_SERVER_ADDR not set")
	}

	resp, body := doQueryAccessRequest(t, serverAddr, "mysql-tls-hostname-mismatch", "SELECT id FROM app.users")

	if resp.StatusCode != http.StatusBadGateway {
		t.Fatalf("expected 502, got %d: %s", resp.StatusCode, string(body))
	}

	result := parseJSON(t, body)
	assertBoundedError(t, result, "connection_failed")
	assertNoLeaks(t, body)
}

func TestTLSQueryAccessPostgreSQLFailsWithHostnameMismatch(t *testing.T) {
	serverAddr := os.Getenv("TLS_E2E_SERVER_ADDR")
	if serverAddr == "" {
		t.Fatal("TLS_E2E_SERVER_ADDR not set")
	}

	resp, body := doQueryAccessRequest(t, serverAddr, "postgresql-tls-hostname-mismatch", "SELECT id FROM app.users")

	if resp.StatusCode != http.StatusBadGateway {
		t.Fatalf("expected 502, got %d: %s", resp.StatusCode, string(body))
	}

	result := parseJSON(t, body)
	assertBoundedError(t, result, "connection_failed")
	assertNoLeaks(t, body)
}

func TestTLSMySQLAuditSucceedsWithTrustedCA(t *testing.T) {
	serverAddr := os.Getenv("TLS_E2E_SERVER_ADDR")
	if serverAddr == "" {
		t.Fatal("TLS_E2E_SERVER_ADDR not set")
	}

	resp, body := doAuditRequest(t, serverAddr, "mysql-tls", "SELECT id FROM app.users")

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, string(body))
	}

	result := parseJSON(t, body)
	ctx := getContext(t, result)

	if ctx["mode"] != "metadata-aware" {
		t.Fatalf("expected mode metadata-aware, got %v", ctx["mode"])
	}
	if ctx["metadata_source"] != "registry" {
		t.Fatalf("expected metadata_source registry, got %v", ctx["metadata_source"])
	}
	if ctx["dialect"] != "mysql" {
		t.Fatalf("expected dialect mysql, got %v", ctx["dialect"])
	}
}

func TestTLSPostgreSQLAuditSucceedsWithTrustedCA(t *testing.T) {
	serverAddr := os.Getenv("TLS_E2E_SERVER_ADDR")
	if serverAddr == "" {
		t.Fatal("TLS_E2E_SERVER_ADDR not set")
	}

	// Use DELETE instead of SELECT - audit endpoint is for DDL/DML, not SELECT
	resp, body := doAuditRequest(t, serverAddr, "postgresql-tls", "DELETE FROM app.users WHERE id = 1")

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, string(body))
	}

	result := parseJSON(t, body)
	ctx := getContext(t, result)

	if ctx["mode"] != "metadata-aware" {
		t.Fatalf("expected mode metadata-aware, got %v", ctx["mode"])
	}
	if ctx["metadata_source"] != "registry" {
		t.Fatalf("expected metadata_source registry, got %v", ctx["metadata_source"])
	}
	if ctx["dialect"] != "postgresql" {
		t.Fatalf("expected dialect postgresql, got %v", ctx["dialect"])
	}
}

func TestTLSMySQLAuditFailsWithUntrustedCA(t *testing.T) {
	serverAddr := os.Getenv("TLS_E2E_SERVER_ADDR")
	if serverAddr == "" {
		t.Fatal("TLS_E2E_SERVER_ADDR not set")
	}

	resp, body := doAuditRequest(t, serverAddr, "mysql-tls-untrusted", "SELECT id FROM app.users")

	// Must fail with a bounded error (502 Bad Gateway for connection failure).
	if resp.StatusCode != http.StatusBadGateway {
		t.Fatalf("expected 502, got %d: %s", resp.StatusCode, string(body))
	}

	result := parseJSON(t, body)
	assertBoundedError(t, result, "connection_failed")
	assertNoLeaks(t, body)
}

func TestTLSPostgreSQLAuditFailsWithUntrustedCA(t *testing.T) {
	serverAddr := os.Getenv("TLS_E2E_SERVER_ADDR")
	if serverAddr == "" {
		t.Fatal("TLS_E2E_SERVER_ADDR not set")
	}

	resp, body := doAuditRequest(t, serverAddr, "postgresql-tls-untrusted", "SELECT id FROM app.users")

	// Must fail with a bounded error (502 Bad Gateway for connection failure).
	if resp.StatusCode != http.StatusBadGateway {
		t.Fatalf("expected 502, got %d: %s", resp.StatusCode, string(body))
	}

	result := parseJSON(t, body)
	assertBoundedError(t, result, "connection_failed")
	assertNoLeaks(t, body)
}

func TestTLSMySQLAuditFailsWithHostnameMismatch(t *testing.T) {
	serverAddr := os.Getenv("TLS_E2E_SERVER_ADDR")
	if serverAddr == "" {
		t.Fatal("TLS_E2E_SERVER_ADDR not set")
	}

	resp, body := doAuditRequest(t, serverAddr, "mysql-tls-hostname-mismatch", "SELECT id FROM app.users")

	// Must fail with a bounded error (502 Bad Gateway for connection failure).
	// The trusted CA validates the cert, but hostname verification fails because
	// the cert SAN only contains "mysql-tls", not "mysql-tls-wrong".
	if resp.StatusCode != http.StatusBadGateway {
		t.Fatalf("expected 502, got %d: %s", resp.StatusCode, string(body))
	}

	result := parseJSON(t, body)
	assertBoundedError(t, result, "connection_failed")
	assertNoLeaks(t, body)
}

func TestTLSPostgreSQLAuditFailsWithHostnameMismatch(t *testing.T) {
	serverAddr := os.Getenv("TLS_E2E_SERVER_ADDR")
	if serverAddr == "" {
		t.Fatal("TLS_E2E_SERVER_ADDR not set")
	}

	resp, body := doAuditRequest(t, serverAddr, "postgresql-tls-hostname-mismatch", "SELECT id FROM app.users")

	// Must fail with a bounded error (502 Bad Gateway for connection failure).
	// The trusted CA validates the cert, but hostname verification fails because
	// the cert SAN only contains "postgresql-tls", not "postgresql-tls-wrong".
	if resp.StatusCode != http.StatusBadGateway {
		t.Fatalf("expected 502, got %d: %s", resp.StatusCode, string(body))
	}

	result := parseJSON(t, body)
	assertBoundedError(t, result, "connection_failed")
	assertNoLeaks(t, body)
}

// Helper functions

func doAuditRequest(t *testing.T, serverAddr, connectionID, sql string) (*http.Response, []byte) {
	t.Helper()

	body := fmt.Sprintf(`{"sql":"%s","connection_id":"%s"}`, sql, connectionID)
	req, err := http.NewRequest("POST", fmt.Sprintf("http://%s/v1/audit", serverAddr), strings.NewReader(body))
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("send request: %v", err)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read response: %v", err)
	}
	resp.Body.Close()

	return resp, bodyBytes
}

func doQueryAccessRequest(t *testing.T, serverAddr, connectionID, sql string) (*http.Response, []byte) {
	t.Helper()

	body := fmt.Sprintf(`{"sql":"%s","connection_id":"%s"}`, sql, connectionID)
	req, err := http.NewRequest("POST", fmt.Sprintf("http://%s/v1/query-access/analyze", serverAddr), strings.NewReader(body))
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("send request: %v", err)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read response: %v", err)
	}
	resp.Body.Close()

	return resp, bodyBytes
}

func parseJSON(t *testing.T, data []byte) map[string]any {
	t.Helper()
	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("parse JSON: %v\nbody: %s", err, string(data))
	}
	return result
}

func getContext(t *testing.T, result map[string]any) map[string]any {
	t.Helper()
	ctx, ok := result["context"].(map[string]any)
	if !ok {
		t.Fatalf("expected context in response, got %v", result)
	}
	return ctx
}

func assertBoundedError(t *testing.T, result map[string]any, expectedCode string) {
	t.Helper()
	errObj, ok := result["error"].(map[string]any)
	if !ok {
		t.Fatalf("expected error in response, got %v", result)
	}
	code, ok := errObj["code"].(string)
	if !ok {
		t.Fatalf("expected error code, got %v", errObj)
	}
	if code != expectedCode {
		t.Fatalf("expected error code %q, got %q", expectedCode, code)
	}
}

func assertNoLeaks(t *testing.T, body []byte) {
	t.Helper()
	s := string(body)
	leakPatterns := []string{
		"-----BEGIN",
		"-----END",
		"root:",
		"password",
		"secret",
		"tls_ca_file",
		"/etc/deltascope/",
		"mysql-tls-untrusted",
		"postgresql-tls-untrusted",
		"mysql-tls-hostname-mismatch",
		"postgresql-tls-hostname-mismatch",
		"mysql-tls-wrong",
		"postgresql-tls-wrong",
		"MYSQL_PASSWORD",
		"PG_PASSWORD",
		"dsn:",
		"driver:",
		"api_key",
		"SELECT LOWER(name) FROM app.users",
		":3306",
		":5432",
	}
	for _, pattern := range leakPatterns {
		if strings.Contains(strings.ToLower(s), strings.ToLower(pattern)) {
			t.Errorf("response contains potential leak %q: %s", pattern, s)
		}
	}
}

func assertOnlineQueryAccessResult(t *testing.T, result map[string]any, expectedDialect, expectedTable, expectedColumn string) {
	t.Helper()

	// Assert dialect matches expected
	dialect, ok := result["dialect"].(string)
	if !ok {
		t.Fatalf("expected dialect in response, got %v", result)
	}
	if dialect != expectedDialect {
		t.Fatalf("expected dialect %q, got %q", expectedDialect, dialect)
	}

	// Assert this is an online result (not offline fallback)
	classification, ok := result["read_classification"].(string)
	if !ok {
		t.Fatalf("expected read_classification in response, got %v", result)
	}
	if classification != "read_only" {
		t.Fatalf("expected read_classification read_only, got %q", classification)
	}

	admission, ok := result["admission"].(string)
	if !ok {
		t.Fatalf("expected admission in response, got %v", result)
	}
	if admission != "admissible" {
		t.Fatalf("expected admission admissible, got %q", admission)
	}

	// Assert requirements contain expected schema-qualified base table and column
	requirements, ok := result["requirements"].([]any)
	if !ok {
		t.Fatalf("expected requirements in response, got %v", result)
	}
	if len(requirements) == 0 {
		t.Fatal("expected at least one requirement")
	}

	foundTable := false
	foundColumn := false
	for _, req := range requirements {
		reqMap, ok := req.(map[string]any)
		if !ok {
			continue
		}
		object, _ := reqMap["object"].(string)
		privilege, _ := reqMap["privilege"].(string)

		// Check for exact table reference (schema.table format)
		if object == expectedTable {
			foundTable = true
		}
		// Check for exact column reference (schema.table.column format) with read_column privilege
		if object == expectedTable+"."+expectedColumn && privilege == "read_column" {
			foundColumn = true
		}
	}

	if !foundTable {
		t.Errorf("expected exact table %q in requirements, got %v", expectedTable, requirements)
	}
	if !foundColumn {
		t.Errorf("expected exact column %q with read_column privilege in requirements, got %v", expectedColumn, requirements)
	}
}
