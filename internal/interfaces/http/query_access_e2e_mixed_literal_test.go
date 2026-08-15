//go:build integration

// Package httpapi verifies HTTP Query Access transport smoke and error boundaries
// against real Docker-backed MySQL 8.4 and TiDB 8.5.
// input: real MySQL/TiDB fixtures, HTTP JSON requests, and configured registries
// output: admitted and fail-closed transport results, bounded failures, and no-leak logs
// pos: HTTP online Query Access real-route smoke and credential-error coverage
// note: if this file changes, update this header and module README.md.
package httpapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Fanduzi/DeltaScope/internal/infrastructure/runtimeconfig"
)

type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (s *syncBuffer) Write(p []byte) (n int, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.Write(p)
}

func (s *syncBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.String()
}

func (s *syncBuffer) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.buf.Reset()
}

func noLeakMarkers() []string {
	return []string{
		"SECRET_LITERAL",
		"SECRET_LITERAL2",
		"COALESCE(",
		"NULLIF(",
		"IFNULL(",
		"LOWER(",
		"COUNT(",
		"builtin_semantic_facts",
		"root",
		"E2E_MYSQL_PASSWORD",
		"3507",
		"3800",
		"3840",
		"4850",
		"empty-pw-",
		"Error 1",
		"Access denied",
		"driver:",
	}
}

func assertAccessLogEntry(t *testing.T, logBuf *syncBuffer, path string) {
	t.Helper()
	deadline := time.After(2 * time.Second)
	for {
		output := logBuf.String()
		if strings.Contains(output, `"msg":"http request"`) && strings.Contains(output, path) {
			return
		}
		select {
		case <-deadline:
			t.Fatalf("access log missing structured entry for %q within 2s; log=%s", path, logBuf.String())
		case <-time.After(10 * time.Millisecond):
		}
	}
}

func assertNoLogLeaks(t *testing.T, logOutput string, markers []string) {
	t.Helper()
	scanned := sanitizeLogForLeakScan(logOutput)
	for _, marker := range markers {
		if strings.Contains(scanned, marker) {
			t.Errorf("access log leaked %q: %s", marker, logOutput)
		}
	}
}

func assertRequestIDLogged(t *testing.T, logBuf *syncBuffer, requestID string) {
	t.Helper()
	if requestID == "" {
		t.Fatal("response omitted X-Request-ID")
	}
	assertAccessLogEntry(t, logBuf, "/v1/query-access/analyze")
	if !strings.Contains(logBuf.String(), `"request_id":"`+requestID+`"`) {
		t.Fatalf("access log omitted response request ID %q: %s", requestID, logBuf.String())
	}
}

var requestIDPattern = regexp.MustCompile(`"request_id":"[^"]*"`)

func sanitizeLogForLeakScan(logOutput string) string {
	return requestIDPattern.ReplaceAllString(logOutput, `"request_id":"<redacted>"`)
}

func writeTempFile(t *testing.T, prefix, content string) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), prefix)
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	if _, err := f.WriteString(content); err != nil {
		_ = f.Close()
		t.Fatalf("write temp file: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("close temp file: %v", err)
	}
	return f.Name()
}

func TestQueryAccessOnline_TransportSmoke(t *testing.T) {
	t.Setenv("E2E_MYSQL_PASSWORD", "root")
	emptyPWFile := writeTempFile(t, "empty-pw-*", "")
	reg, err := runtimeconfig.ValidateAndBuildRegistry(runtimeconfig.Config{
		Metadata: runtimeconfig.MetadataConfig{Connections: []runtimeconfig.ConnectionConfig{
			{
				ID: "mysql84", Dialect: "mysql", Host: "127.0.0.1", Port: 3840, User: "root",
				PasswordEnv: "E2E_MYSQL_PASSWORD", Schema: "app", Purposes: []string{"query_access"},
			},
			{
				ID: "tidb85", Dialect: "tidb", Host: "127.0.0.1", Port: 4850, User: "root",
				PasswordFile: emptyPWFile, Schema: "app", Purposes: []string{"query_access"},
			},
		}},
	})
	if err != nil {
		t.Fatalf("build registry: %v", err)
	}

	var logBuf syncBuffer
	handler, err := NewHandler("", "test-build", WithRegistry(reg), WithMiddlewareConfig(MiddlewareConfig{Logger: log.New(&logBuf, "", 0)}))
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}
	ts := httptest.NewServer(handler)
	defer ts.Close()

	cases := []struct {
		name, connectionID, sql, requirement, reason, classification, admission string
	}{
		{"mysql84_admissible", "mysql84", "SELECT COUNT(1) FROM app.builtin_semantic_facts", "app.builtin_semantic_facts=read_table", "", "read_only", "admissible"},
		{"mysql84_unknown_function", "mysql84", "SELECT app_specific_rollup(amount) FROM app.builtin_semantic_facts", "", "unknown_function_effect", "indeterminate", "indeterminate"},
		{"tidb85_admissible", "tidb85", "SELECT COUNT(1) FROM app.builtin_semantic_facts", "app.builtin_semantic_facts=read_table", "", "read_only", "admissible"},
		{"tidb85_unknown_function", "tidb85", "SELECT app_specific_rollup(amount) FROM app.builtin_semantic_facts", "", "unknown_function_effect", "indeterminate", "indeterminate"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			logBuf.Reset()
			marker := "HTTP_TRANSPORT_" + strings.ToUpper(tc.name) + "_MARKER"
			req, err := http.NewRequest(http.MethodPost, ts.URL+"/v1/query-access/analyze", strings.NewReader(fmt.Sprintf(`{"sql":%q,"connection_id":%q}`, tc.sql+" /* "+marker+" */", tc.connectionID)))
			if err != nil {
				t.Fatalf("create request: %v", err)
			}
			req.Header.Set("Content-Type", "application/json")
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("send request: %v", err)
			}
			defer resp.Body.Close()

			var body bytes.Buffer
			if _, err := body.ReadFrom(resp.Body); err != nil {
				t.Fatalf("read response: %v", err)
			}
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("status: got %d, want 200; body: %s", resp.StatusCode, body.String())
			}
			requestID := resp.Header.Get("X-Request-ID")
			var result map[string]any
			if err := json.Unmarshal(body.Bytes(), &result); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if result["read_classification"] != tc.classification || result["admission"] != tc.admission {
				t.Fatalf("result: got %q/%q, want %q/%q", result["read_classification"], result["admission"], tc.classification, tc.admission)
			}
			if tc.requirement != "" {
				requirements, ok := result["requirements"].([]any)
				if !ok || len(requirements) != 1 {
					t.Fatalf("requirements: got %#v, want %q", result["requirements"], tc.requirement)
				}
				requirement, ok := requirements[0].(map[string]any)
				if !ok || requirement["object"].(string)+"="+requirement["privilege"].(string) != tc.requirement {
					t.Fatalf("requirements: got %#v, want %q", requirements, tc.requirement)
				}
			}
			if tc.reason != "" {
				found := false
				for _, reason := range result["reason_codes"].([]any) {
					if reason == tc.reason {
						found = true
						break
					}
				}
				if !found {
					t.Fatalf("reason_codes: got %#v, want %q", result["reason_codes"], tc.reason)
				}
			}
			if strings.Contains(body.String(), marker) {
				t.Fatalf("response leaked SQL marker: %s", body.String())
			}
			assertRequestIDLogged(t, &logBuf, requestID)
			assertNoLogLeaks(t, logBuf.String(), append(noLeakMarkers(), marker))
		})
	}
}

func TestQueryAccessOnline_DefaultOffline(t *testing.T) {
	var logBuf syncBuffer
	handler, err := NewHandler("", "test-build", WithMiddlewareConfig(MiddlewareConfig{Logger: log.New(&logBuf, "", 0)}))
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}
	marker := "HTTP_DEFAULT_OFFLINE_MARKER"
	status, body := postHTTPQueryAccess(t, handler, fmt.Sprintf(`{"sql":%q}`, "SELECT COUNT(1) /* "+marker+" */"))
	if status != http.StatusOK {
		t.Fatalf("status: got %d, want 200; body: %s", status, body)
	}
	var result map[string]any
	if err := json.Unmarshal([]byte(body), &result); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if result["read_classification"] != "indeterminate" || result["admission"] != "indeterminate" {
		t.Fatalf("offline result: %#v", result)
	}
	if strings.Contains(body, marker) {
		t.Fatalf("response leaked SQL marker: %s", body)
	}
	assertAccessLogEntry(t, &logBuf, "/v1/query-access/analyze")
	assertNoLogLeaks(t, logBuf.String(), append(noLeakMarkers(), marker))
}

func TestQueryAccessOnline_ConnectionFailureNoLeak(t *testing.T) {
	t.Setenv("E2E_FAIL_ENV_PASSWORD", "FAIL_SECRET_ENV_pw_9f3b2a1c")
	failPWContent := "FAIL_SECRET_FILE_pw_4d5e6f7a"
	failPWFile := writeTempFile(t, "fail-pw-*", failPWContent)
	reg, err := runtimeconfig.ValidateAndBuildRegistry(runtimeconfig.Config{
		Metadata: runtimeconfig.MetadataConfig{Connections: []runtimeconfig.ConnectionConfig{
			{
				ID: "fail_env_conn", Dialect: "mysql", Host: "127.0.0.1", Port: 3840, User: "root",
				PasswordEnv: "E2E_FAIL_ENV_PASSWORD", Schema: "app", Purposes: []string{"query_access"},
			},
			{
				ID: "fail_file_conn", Dialect: "mysql", Host: "127.0.0.1", Port: 3840, User: "root",
				PasswordFile: failPWFile, Schema: "app", Purposes: []string{"query_access"},
			},
		}},
	})
	if err != nil {
		t.Fatalf("build registry: %v", err)
	}

	var logBuf syncBuffer
	handler, err := NewHandler("", "test-build", WithRegistry(reg), WithMiddlewareConfig(MiddlewareConfig{Logger: log.New(&logBuf, "", 0)}))
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}
	ts := httptest.NewServer(handler)
	defer ts.Close()

	cases := []struct {
		name, connectionID, password, sqlMarker string
		extraMarkers                            []string
	}{
		{"env_credential_failure", "fail_env_conn", "FAIL_SECRET_ENV_pw_9f3b2a1c", "FAIL_SQL_LITERAL_env_a1b2c3d4", []string{"E2E_FAIL_ENV_PASSWORD"}},
		{"file_credential_failure", "fail_file_conn", failPWContent, "FAIL_SQL_LITERAL_file_e5f6a7b8", []string{failPWFile, filepath.Base(failPWFile), "fail-pw-"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			logBuf.Reset()
			sql := fmt.Sprintf("SELECT COALESCE(name, '%s') FROM app.builtin_semantic_facts", tc.sqlMarker)
			req, err := http.NewRequest(http.MethodPost, ts.URL+"/v1/query-access/analyze", strings.NewReader(fmt.Sprintf(`{"sql":%q,"connection_id":%q}`, sql, tc.connectionID)))
			if err != nil {
				t.Fatalf("create request: %v", err)
			}
			req.Header.Set("Content-Type", "application/json")
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("send request: %v", err)
			}
			defer resp.Body.Close()

			var body bytes.Buffer
			if _, err := body.ReadFrom(resp.Body); err != nil {
				t.Fatalf("read response: %v", err)
			}
			if resp.StatusCode != http.StatusBadGateway {
				t.Fatalf("status: got %d, want 502; body: %s", resp.StatusCode, body.String())
			}
			requestID := resp.Header.Get("X-Request-ID")
			var result map[string]any
			if err := json.Unmarshal(body.Bytes(), &result); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			errorBody, ok := result["error"].(map[string]any)
			if !ok || errorBody["code"] != "connection_failed" {
				t.Fatalf("error: got %#v, want connection_failed", result["error"])
			}
			markers := append([]string{
				"127.0.0.1", "3840", "app", "root", tc.connectionID, tc.password, tc.sqlMarker,
				"COALESCE(", "builtin_semantic_facts", "dial tcp", "connection refused", "Access denied", "driver:", "Error 1",
			}, tc.extraMarkers...)
			for _, marker := range markers {
				if strings.Contains(body.String(), marker) {
					t.Errorf("response leaked %q: %s", marker, body.String())
				}
			}
			assertRequestIDLogged(t, &logBuf, requestID)
			assertNoLogLeaks(t, logBuf.String(), markers)
		})
	}
}

func postHTTPQueryAccess(t *testing.T, handler http.Handler, payload string) (int, string) {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/v1/query-access/analyze", bytes.NewBufferString(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec.Code, rec.Body.String()
}
