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
	// Must not leak sensitive information
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
	}
	for _, pattern := range leakPatterns {
		if strings.Contains(strings.ToLower(s), strings.ToLower(pattern)) {
			t.Errorf("response contains potential leak %q: %s", pattern, s)
		}
	}
}
