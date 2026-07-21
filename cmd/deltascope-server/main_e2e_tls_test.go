//go:build e2e && tls

package main

import (
	"bytes"
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
		t.Skip("TLS_E2E_SERVER_ADDR not set")
	}

	body := `{"sql":"SELECT 1","connection_id":"mysql-tls"}`
	req, err := http.NewRequest("POST", fmt.Sprintf("http://%s/v1/audit", serverAddr), strings.NewReader(body))
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("send request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	// Verify metadata-aware mode
	ctx, ok := result["context"].(map[string]any)
	if !ok {
		t.Fatalf("expected context in response, got %v", result)
	}
	if ctx["mode"] != "metadata-aware" {
		t.Fatalf("expected mode metadata-aware, got %v", ctx["mode"])
	}
	if ctx["metadata_source"] != "registry" {
		t.Fatalf("expected metadata_source registry, got %v", ctx["metadata_source"])
	}
}

func TestTLSPostgreSQLAuditSucceedsWithTrustedCA(t *testing.T) {
	serverAddr := os.Getenv("TLS_E2E_SERVER_ADDR")
	if serverAddr == "" {
		t.Skip("TLS_E2E_SERVER_ADDR not set")
	}

	body := `{"sql":"SELECT 1","connection_id":"postgresql-tls"}`
	req, err := http.NewRequest("POST", fmt.Sprintf("http://%s/v1/audit", serverAddr), strings.NewReader(body))
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("send request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	// Verify metadata-aware mode
	ctx, ok := result["context"].(map[string]any)
	if !ok {
		t.Fatalf("expected context in response, got %v", result)
	}
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
	// This test proves that TLS validation is real by verifying that
	// a connection to a server with an untrusted certificate fails.
	// We can't directly test this through the HTTP API because the
	// server's trust store is fixed at build time. Instead, we verify
	// that the server correctly rejects connections when the CA is not trusted.

	// For now, this test documents the requirement. A full implementation
	// would require a second server instance with the untrusted CA installed.
	t.Skip("requires separate server instance with untrusted CA")
}

func TestTLSPostgreSQLAuditFailsWithHostnameMismatch(t *testing.T) {
	// This test proves that hostname validation is real by verifying that
	// a connection to a server with a mismatched hostname fails.
	// We can't directly test this through the HTTP API because the
	// server's connection config uses the correct hostname.

	// For now, this test documents the requirement. A full implementation
	// would require a separate server instance connecting to a hostname
	// not present in the certificate's SANs.
	t.Skip("requires separate server instance with hostname mismatch")
}

// testAuditRequest is a helper that sends an audit request and returns the response.
func testAuditRequest(t *testing.T, serverAddr, connectionID string) (*http.Response, []byte) {
	t.Helper()

	body := fmt.Sprintf(`{"sql":"SELECT 1","connection_id":"%s"}`, connectionID)
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
