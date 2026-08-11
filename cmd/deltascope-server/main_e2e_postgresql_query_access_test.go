//go:build e2e && postgresql

// Package main verifies Docker-backed HTTP PostgreSQL query-access behavior.
// input: the PG17 Docker fixture, named connection runtime config, and HTTP requests
// output: end-to-end proof of the PostgreSQL COUNT(1) connection_id contract
// pos: slower external verification kept outside the default go test loop
// note: if this file changes, update this header and module README.md.
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

const (
	pg17QueryAccessConnectionID = "pg17-query-access"
	pg17QueryAccessAPIKey       = "pg17-query-access-api-key"
	pg17QueryAccessOtherAPIKey  = "pg17-query-access-other-api-key"
	pg17QueryAccessPassword     = "PG17_HTTP_PASSWORD_SECRET"
)

func TestHTTPQueryAccess_PG17_CountIntegerOne(t *testing.T) {
	baseURL, harness := startHTTPServerPG17QueryAccess(t)
	result, body := postPG17QueryAccess(t, baseURL, "SELECT COUNT(1) FROM app.orders", true)
	assertHTTPQueryAccessPG17Positive(t, result)
	assertHTTPQueryAccessPG17NoLeak(t, body, harness, "SELECT COUNT(1) FROM app.orders")
}

func TestHTTPQueryAccess_PG17_ExcludedShapes(t *testing.T) {
	baseURL, harness := startHTTPServerPG17QueryAccess(t)
	queries := []string{
		"SELECT COUNT(NULL) FROM app.orders",
		"SELECT COUNT(2) FROM app.orders",
		"SELECT COUNT(1::integer) FROM app.orders",
		"SELECT COUNT($1) FROM app.orders",
		"SELECT COUNT(1) FILTER (WHERE true) FROM app.orders",
		"SELECT COUNT(1) OVER () FROM app.orders",
		"SELECT COUNT(1) FROM app.orders WHERE true",
		"SELECT COUNT(1) FROM app.orders JOIN app.users ON true",
		"SELECT COUNT(1) FROM app.user_summary",
		"SELECT COUNT(1) FROM app.remote_orders",
		"WITH source AS (SELECT id FROM app.orders) SELECT COUNT(1) FROM source",
		"SELECT COUNT(1) FROM (SELECT id FROM app.orders) AS source",
		"SELECT COUNT(1) FROM orders",
		"SELECT COUNT(1), * FROM app.orders",
		"SELECT COUNT(1) FROM app.missing_relation",
	}
	for index, sqlText := range queries {
		sqlText := sqlText + fmt.Sprintf(" /* HTTP_PG17_EXCLUDED_%02d */", index)
		t.Run(fmt.Sprintf("case_%02d", index), func(t *testing.T) {
			result, body := postPG17QueryAccess(t, baseURL, sqlText, true)
			assertHTTPQueryAccessPG17Indeterminate(t, result)
			assertHTTPQueryAccessPG17NoLeak(t, body, harness, sqlText)
		})
	}
}

func TestHTTPQueryAccess_PG17_NoConnectionID(t *testing.T) {
	baseURL, harness := startHTTPServerPG17QueryAccess(t)
	result, body := postPG17QueryAccess(t, baseURL, "SELECT COUNT(1) FROM app.orders", false)
	assertHTTPQueryAccessPG17Indeterminate(t, result)
	assertHTTPQueryAccessPG17NoLeak(t, body, harness, "root", pg17QueryAccessPassword)
}

func TestHTTPQueryAccess_PG17_Unauthorized(t *testing.T) {
	baseURL, harness := startHTTPServerPG17QueryAccess(t)
	status, body := postPG17QueryAccessRaw(t, baseURL, `{"sql":"SELECT COUNT(1) FROM app.orders","connection_id":"pg17-query-access"}`, pg17QueryAccessOtherAPIKey)
	if status != http.StatusForbidden {
		t.Fatalf("unauthorized status: got %d, want %d: %s", status, http.StatusForbidden, body)
	}
	if !strings.Contains(body, `"not_authorized"`) {
		t.Fatalf("unauthorized response missing not_authorized: %s", body)
	}
	assertHTTPQueryAccessPG17NoLeak(t, body, harness, pg17QueryAccessConnectionID, pg17QueryAccessPassword, pg17QueryAccessOtherAPIKey)
}

func TestHTTPQueryAccess_PG17_NoLeak(t *testing.T) {
	baseURL, harness := startHTTPServerPG17QueryAccess(t)
	marker := "HTTP_PG17_NO_LEAK_MARKER"
	sqlText := "SELECT COUNT(1) /* " + marker + " */ FROM app.orders"
	result, body := postPG17QueryAccess(t, baseURL, sqlText, true)
	assertHTTPQueryAccessPG17Positive(t, result)
	assertHTTPQueryAccessPG17NoLeak(t, body, harness, marker, "root", pg17QueryAccessPassword,
		pg17QueryAccessAPIKey, pg17QueryAccessConnectionID, "secret_should_not_leak",
		"sensitive comment text", "pg_catalog", "database_oid", "role_oid", "backend_pid",
		"session_binding", "catalog_sql", "raw_sql", "dsn", "candidate", "manifest", "credential", "password")
}

func startHTTPServerPG17QueryAccess(t *testing.T) (string, *httpServerHarness) {
	t.Helper()
	t.Setenv("DELTASCOPE_PG17_HTTP_PASSWORD", "root")
	t.Setenv("DELTASCOPE_PG17_HTTP_API_KEY", pg17QueryAccessAPIKey)
	t.Setenv("DELTASCOPE_PG17_HTTP_OTHER_API_KEY", pg17QueryAccessOtherAPIKey)
	config := fmt.Sprintf(`http:
  auth:
    enabled: true
    keys:
      - id: pg17-query-access-key
        secret_env: DELTASCOPE_PG17_HTTP_API_KEY
      - id: pg17-query-access-other-key
        secret_env: DELTASCOPE_PG17_HTTP_OTHER_API_KEY
metadata:
  connections:
    - id: %s
      dialect: postgresql
      host: 127.0.0.1
      port: 5500
      user: root
      password_env: DELTASCOPE_PG17_HTTP_PASSWORD
      database: postgres
      schema: app
      purposes: [query_access]
      allowed_api_key_ids: [pg17-query-access-key]
`, pg17QueryAccessConnectionID)
	configPath := writePG17QueryAccessTempFile(t, config)
	listenAddr := freeTCPAddr(t)
	binaryPath := buildHTTPServerBinaryPG(t)
	cmd := execCommandPG17QueryAccess(binaryPath, listenAddr, configPath)
	harness := &httpServerHarness{cmd: cmd, done: make(chan struct{})}
	cmd.Stdout = &harness.stdout
	cmd.Stderr = &harness.stderr
	if err := cmd.Start(); err != nil {
		t.Fatalf("start pg17 query-access server: %v", err)
	}
	go func() {
		harness.waitErr = cmd.Wait()
		close(harness.done)
	}()
	t.Cleanup(func() { stopHTTPServer(t, harness) })
	baseURL := "http://" + listenAddr
	waitForHealthz(t, baseURL, harness)
	return baseURL, harness
}

func execCommandPG17QueryAccess(binaryPath, listenAddr, configPath string) *exec.Cmd {
	cmd := exec.Command(binaryPath, "-listen", listenAddr, "-runtime-config", configPath)
	cmd.Env = os.Environ()
	return cmd
}

func writePG17QueryAccessTempFile(t *testing.T, content string) string {
	t.Helper()
	file, err := os.CreateTemp(t.TempDir(), "pg17-query-access-*.yaml")
	if err != nil {
		t.Fatalf("create runtime config: %v", err)
	}
	if _, err := file.WriteString(content); err != nil {
		file.Close()
		t.Fatalf("write runtime config: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("close runtime config: %v", err)
	}
	return file.Name()
}

func postPG17QueryAccess(t *testing.T, baseURL, sqlText string, online bool) (map[string]any, string) {
	t.Helper()
	payload := fmt.Sprintf(`{"sql":%q`, sqlText)
	if online {
		payload += fmt.Sprintf(`,"connection_id":%q`, pg17QueryAccessConnectionID)
	}
	payload += `}`
	status, body := postPG17QueryAccessRaw(t, baseURL, payload, pg17QueryAccessAPIKey)
	if status != http.StatusOK {
		t.Fatalf("query-access status: got %d, want 200: %s", status, body)
	}
	var result map[string]any
	if err := json.Unmarshal([]byte(body), &result); err != nil {
		t.Fatalf("decode query-access response: %v: %s", err, body)
	}
	return result, body
}

func postPG17QueryAccessRaw(t *testing.T, baseURL, payload, apiKey string) (int, string) {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, baseURL+"/v1/query-access/analyze", strings.NewReader(payload))
	if err != nil {
		t.Fatalf("build query-access request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Request-ID", pg17QueryAccessRequestID(t))
	if apiKey != "" {
		req.Header.Set("X-API-Key", apiKey)
	}
	resp, err := (&http.Client{Timeout: 60 * time.Second}).Do(req)
	if err != nil {
		t.Fatalf("post query-access request: %v", err)
	}
	defer resp.Body.Close()
	var body bytes.Buffer
	if _, err := body.ReadFrom(resp.Body); err != nil {
		t.Fatalf("read query-access response: %v", err)
	}
	return resp.StatusCode, body.String()
}

func pg17QueryAccessRequestID(t *testing.T) string {
	t.Helper()
	return "pg17-request-" + strings.NewReplacer("/", "-", " ", "-").Replace(t.Name())
}

func assertHTTPQueryAccessPG17Positive(t *testing.T, result map[string]any) {
	t.Helper()
	if result["read_classification"] != "read_only" || result["admission"] != "admissible" {
		t.Fatalf("expected read_only/admissible, got %#v", result)
	}
	requirements, ok := result["requirements"].([]any)
	if !ok || len(requirements) != 1 {
		t.Fatalf("expected one requirement, got %#v", result["requirements"])
	}
	requirement := requirements[0].(map[string]any)
	if requirement["object"] != "app.orders" || requirement["privilege"] != "read_table" {
		t.Fatalf("unexpected requirement: %#v", requirement)
	}
	if columns, ok := result["referenced_columns"]; ok && len(columns.([]any)) != 0 {
		t.Fatalf("COUNT(1) referenced columns: %#v", columns)
	}
}

func assertHTTPQueryAccessPG17Indeterminate(t *testing.T, result map[string]any) {
	t.Helper()
	if result["read_classification"] != "indeterminate" || result["admission"] != "indeterminate" {
		t.Fatalf("expected indeterminate/indeterminate, got %#v", result)
	}
}

func assertHTTPQueryAccessPG17NoLeak(t *testing.T, body string, harness *httpServerHarness, markers ...string) {
	t.Helper()
	harness.stderr.WaitFor(t, `\"msg\":\"http request\"`, 2*time.Second)
	harness.stderr.WaitFor(t, `\"path\":\"/v1/query-access/analyze\"`, 2*time.Second)
	harness.stderr.WaitFor(t, `\"request_id\":\"`+pg17QueryAccessRequestID(t)+`\"`, 2*time.Second)
	combined := body + "\n" + harness.stdout.String() + "\n" + harness.stderr.String()
	if len(combined) > 8192 {
		t.Fatalf("HTTP response/log output exceeded bound: %d", len(combined))
	}
	for _, marker := range markers {
		if strings.Contains(strings.ToLower(combined), strings.ToLower(marker)) {
			t.Errorf("HTTP output/logs leaked %q: %s", marker, combined)
		}
	}
}
