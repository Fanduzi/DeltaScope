//go:build integration

// Package httpapi verifies HTTP query-access online behavior for mixed-literal
// operands (COALESCE, NULLIF, IFNULL) against real Docker-backed MySQL and TiDB.
// input: real MySQL/TiDB fixtures, the HTTP JSON entrypoint, and a test registry
// output: end-to-end proof that mixed-literal scalar operands yield
//
//	read_only + admissible via the HTTP online path across all four MySQL/TiDB
//	versions, with exact requirement assertions and no-leak guards
//
// pos: HTTP online E2E coverage for the mixed-literal scalar operand feature
// note: if this file changes, update this header and module README.md.
package httpapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
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
		"COALESCE(",
		"NULLIF(",
		"IFNULL(",
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
	// Gin's access-log middleware writes after c.Next() returns but before
	// the response is flushed to the client. A bounded poll avoids a flaky
	// race while keeping the test fast.
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
	for _, marker := range markers {
		if strings.Contains(logOutput, marker) {
			t.Errorf("access log leaked %q: %s", marker, logOutput)
		}
	}
}

// listenAndCloseOnAccept keeps a listener open and returns its port. A
// background goroutine accepts exactly one connection then immediately closes
// it, guaranteeing the port cannot be reused by another process before the
// test exercises the connection-failure path.
func listenAndCloseOnAccept(t *testing.T) (int, net.Listener) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen for failure port: %v", err)
	}
	go func() {
		conn, err := ln.Accept()
		if err == nil {
			conn.Close()
		}
	}()
	return ln.Addr().(*net.TCPAddr).Port, ln
}

func writeTempFile(t *testing.T, prefix, content string) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), prefix)
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	if _, err := f.WriteString(content); err != nil {
		f.Close()
		t.Fatalf("write temp file: %v", err)
	}
	f.Close()
	return f.Name()
}

func TestQueryAccessOnline_MixedLiteralScalars(t *testing.T) {
	t.Setenv("E2E_MYSQL_PASSWORD", "root")
	t.Setenv("E2E_FAIL_ENV_PASSWORD", "FAIL_SECRET_ENV_pw_9f3b2a1c")

	emptyPWFile := writeTempFile(t, "empty-pw-*", "")
	failPWContent := "FAIL_SECRET_FILE_pw_4d5e6f7a"
	failPWFile := writeTempFile(t, "fail-pw-*", failPWContent)

	failEnvPort, failEnvLn := listenAndCloseOnAccept(t)
	defer failEnvLn.Close()
	failFilePort, failFileLn := listenAndCloseOnAccept(t)
	defer failFileLn.Close()

	cfg := runtimeconfig.Config{
		Metadata: runtimeconfig.MetadataConfig{
			Connections: []runtimeconfig.ConnectionConfig{
				{
					ID:          "mysql57",
					Dialect:     "mysql",
					Host:        "127.0.0.1",
					Port:        3507,
					User:        "root",
					PasswordEnv: "E2E_MYSQL_PASSWORD",
					Schema:      "app",
					Purposes:    []string{"query_access"},
				},
				{
					ID:          "mysql80",
					Dialect:     "mysql",
					Host:        "127.0.0.1",
					Port:        3800,
					User:        "root",
					PasswordEnv: "E2E_MYSQL_PASSWORD",
					Schema:      "app",
					Purposes:    []string{"query_access"},
				},
				{
					ID:          "mysql84",
					Dialect:     "mysql",
					Host:        "127.0.0.1",
					Port:        3840,
					User:        "root",
					PasswordEnv: "E2E_MYSQL_PASSWORD",
					Schema:      "app",
					Purposes:    []string{"query_access"},
				},
				{
					ID:           "tidb85",
					Dialect:      "tidb",
					Host:         "127.0.0.1",
					Port:         4850,
					User:         "root",
					PasswordFile: emptyPWFile,
					Schema:       "app",
					Purposes:     []string{"query_access"},
				},
				{
					ID:          "fail_env_conn",
					Dialect:     "mysql",
					Host:        "127.0.0.1",
					Port:        failEnvPort,
					User:        "fail_user_e8a1b2c3",
					PasswordEnv: "E2E_FAIL_ENV_PASSWORD",
					Schema:      "app",
					Purposes:    []string{"query_access"},
				},
				{
					ID:           "fail_file_conn",
					Dialect:      "mysql",
					Host:         "127.0.0.1",
					Port:         failFilePort,
					User:         "fail_user_f7c6d5e4",
					PasswordFile: failPWFile,
					Schema:       "app",
					Purposes:     []string{"query_access"},
				},
			},
		},
	}
	reg, err := runtimeconfig.ValidateAndBuildRegistry(cfg)
	if err != nil {
		t.Fatalf("build registry: %v", err)
	}

	var logBuf syncBuffer
	captureLogger := log.New(&logBuf, "", 0)

	handler, err := NewHandler("", "test-build",
		WithRegistry(reg),
		WithMiddlewareConfig(MiddlewareConfig{
			Logger: captureLogger,
		}),
	)
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}
	ts := httptest.NewServer(handler)
	defer ts.Close()

	cases := []struct {
		name         string
		connectionID string
	}{
		{"mysql57", "mysql57"},
		{"mysql80", "mysql80"},
		{"mysql84", "mysql84"},
		{"tidb85", "tidb85"},
	}
	probes := []struct {
		name string
		sql  string
	}{
		{"COALESCE", "SELECT COALESCE(name, 'SECRET_LITERAL') FROM app.builtin_semantic_facts"},
		{"NULLIF", "SELECT NULLIF(name, 'SECRET_LITERAL') FROM app.builtin_semantic_facts"},
		{"IFNULL", "SELECT IFNULL(name, 'SECRET_LITERAL') FROM app.builtin_semantic_facts"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for _, probe := range probes {
				t.Run(probe.name, func(t *testing.T) {
					logBuf.Reset()

					payload := fmt.Sprintf(`{"sql":%q,"connection_id":%q}`, probe.sql, tc.connectionID)
					req, err := http.NewRequest(http.MethodPost, ts.URL+"/v1/query-access/analyze", strings.NewReader(payload))
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

					var result map[string]any
					if err := json.Unmarshal(body.Bytes(), &result); err != nil {
						t.Fatalf("unmarshal: %v", err)
					}
					if result["read_classification"] != "read_only" {
						t.Errorf("classification: got %q, want read_only", result["read_classification"])
					}
					if result["admission"] != "admissible" {
						t.Errorf("admission: got %q, want admissible", result["admission"])
					}

					wantReqs := []map[string]string{
						{"object": "app.builtin_semantic_facts", "privilege": "read_table"},
						{"object": "app.builtin_semantic_facts.name", "privilege": "read_column"},
					}
					rawReqs, ok := result["requirements"].([]any)
					if !ok {
						t.Fatalf("requirements: not a slice; got %T", result["requirements"])
					}
					if len(rawReqs) != len(wantReqs) {
						t.Fatalf("requirements: got %d items, want %d", len(rawReqs), len(wantReqs))
					}
					for i, raw := range rawReqs {
						reqMap, ok := raw.(map[string]any)
						if !ok {
							t.Fatalf("requirements[%d]: not a map; got %T", i, raw)
						}
						if reqMap["object"] != wantReqs[i]["object"] || reqMap["privilege"] != wantReqs[i]["privilege"] {
							t.Errorf("requirements[%d]: got %+v, want %+v", i, reqMap, wantReqs[i])
						}
					}

					bodyStr := body.String()
					for _, marker := range []string{"SECRET_LITERAL", "root", "E2E_MYSQL_PASSWORD"} {
						if strings.Contains(bodyStr, marker) {
							t.Errorf("response leaked %q: %s", marker, bodyStr)
						}
					}
					raw, _ := json.Marshal(result)
					if strings.Contains(string(raw), "SECRET_LITERAL") {
						t.Errorf("deserialized result leaked SECRET_LITERAL")
					}

					assertAccessLogEntry(t, &logBuf, "/v1/query-access/analyze")
					logOutput := logBuf.String()
					assertNoLogLeaks(t, logOutput, noLeakMarkers())
				})
			}
		})
	}

	t.Run("default_path_indeterminate", func(t *testing.T) {
		for _, probe := range probes {
			t.Run(probe.name, func(t *testing.T) {
				logBuf.Reset()

				payload := fmt.Sprintf(`{"sql":%q}`, probe.sql)
				req, err := http.NewRequest(http.MethodPost, ts.URL+"/v1/query-access/analyze", strings.NewReader(payload))
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

				var result map[string]any
				if err := json.Unmarshal(body.Bytes(), &result); err != nil {
					t.Fatalf("unmarshal: %v", err)
				}
				if result["read_classification"] != "indeterminate" {
					t.Errorf("classification: got %q, want indeterminate", result["read_classification"])
				}
				if result["admission"] != "indeterminate" {
					t.Errorf("admission: got %q, want indeterminate", result["admission"])
				}

				bodyStr := body.String()
				if strings.Contains(bodyStr, "SECRET_LITERAL") {
					t.Errorf("response leaked SECRET_LITERAL: %s", bodyStr)
				}

				assertAccessLogEntry(t, &logBuf, "/v1/query-access/analyze")
				logOutput := logBuf.String()
				assertNoLogLeaks(t, logOutput, noLeakMarkers())
			})
		}
	})

	// failPWBase is the basename of the temp password file; it must not leak
	// in any HTTP response, JSON body, or access log entry.
	failPWBase := filepath.Base(failPWFile)

	failureCases := []struct {
		name         string
		connectionID string
		password     string
		port         int
		user         string
		sqlLiteral   string // unique per sub-case; proves request content doesn't leak on session-open failure
		extraMarkers []string
	}{
		{
			name:         "env_credential_failure",
			connectionID: "fail_env_conn",
			password:     "FAIL_SECRET_ENV_pw_9f3b2a1c",
			port:         failEnvPort,
			user:         "fail_user_e8a1b2c3",
			sqlLiteral:   "FAIL_SQL_LITERAL_env_a1b2c3d4",
			extraMarkers: []string{"E2E_FAIL_ENV_PASSWORD"},
		},
		{
			name:         "file_credential_failure",
			connectionID: "fail_file_conn",
			password:     failPWContent,
			port:         failFilePort,
			user:         "fail_user_f7c6d5e4",
			sqlLiteral:   "FAIL_SQL_LITERAL_file_e5f6a7b8",
			extraMarkers: []string{failPWFile, failPWBase, "fail-pw-"},
		},
	}

	for _, fc := range failureCases {
		t.Run(fc.name, func(t *testing.T) {
			logBuf.Reset()

			// Build the full set of markers from the current sub-case's real
			// configuration values. Every marker must originate from this
			// sub-case's actual config or request — never from a shared constant.
			failMarkers := []string{
				// host, dynamic port, and schema
				"127.0.0.1",
				strconv.Itoa(fc.port),
				"app",
				// credential identity
				fc.user,
				fc.connectionID,
				fc.password,
				// SQL literal marker unique to this sub-case
				fc.sqlLiteral,
				// SQL structural fragments
				"COALESCE(",
				"builtin_semantic_facts",
				// driver/connection error substrings
				"dial tcp",
				"connection refused",
				"Access denied",
				"driver:",
				"Error 1",
			}
			failMarkers = append(failMarkers, fc.extraMarkers...)

			// Build SQL with the unique literal marker so the test can prove
			// request content doesn't leak to response or access log on session-open failure.
			sql := fmt.Sprintf(
				"SELECT COALESCE(name, '%s') FROM app.builtin_semantic_facts",
				fc.sqlLiteral,
			)
			payload := fmt.Sprintf(`{"sql":%q,"connection_id":%q}`, sql, fc.connectionID)
			req, err := http.NewRequest(http.MethodPost, ts.URL+"/v1/query-access/analyze", strings.NewReader(payload))
			if err != nil {
				t.Fatalf("create request: %v; port=%d; connID=%s", err, fc.port, fc.connectionID)
			}
			req.Header.Set("Content-Type", "application/json")

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("send request: %v; port=%d; connID=%s", err, fc.port, fc.connectionID)
			}
			defer resp.Body.Close()

			var body bytes.Buffer
			if _, err := body.ReadFrom(resp.Body); err != nil {
				t.Fatalf("read response: %v; port=%d; connID=%s", err, fc.port, fc.connectionID)
			}

			// 1) HTTP status must be 502 Bad Gateway — proves the request
			//    reached the actual dial path, not input validation.
			if resp.StatusCode != http.StatusBadGateway {
				t.Fatalf("status: got %d, want 502; body: %s; port=%d; connID=%s", resp.StatusCode, body.String(), fc.port, fc.connectionID)
			}

			// 2) JSON error code must be exactly "connection_failed".
			var result map[string]any
			if err := json.Unmarshal(body.Bytes(), &result); err != nil {
				t.Fatalf("unmarshal: %v; port=%d; connID=%s", err, fc.port, fc.connectionID)
			}
			errObj, ok := result["error"].(map[string]any)
			if !ok {
				t.Fatalf("error: not a map; got %T; port=%d; connID=%s", result["error"], fc.port, fc.connectionID)
			}
			if errObj["code"] != "connection_failed" {
				t.Errorf("error.code: got %q, want connection_failed; port=%d; connID=%s", errObj["code"], fc.port, fc.connectionID)
			}

			// 3) Raw HTTP response body must not leak any marker.
			bodyStr := body.String()
			for _, marker := range failMarkers {
				if strings.Contains(bodyStr, marker) {
					t.Errorf("response body leaked %q: %s; port=%d; connID=%s", marker, bodyStr, fc.port, fc.connectionID)
				}
			}

			// 4) Deserialized JSON must not leak any marker.
			raw, _ := json.Marshal(result)
			jsonStr := string(raw)
			for _, marker := range failMarkers {
				if strings.Contains(jsonStr, marker) {
					t.Errorf("deserialized JSON leaked %q: %s; port=%d; connID=%s", marker, jsonStr, fc.port, fc.connectionID)
				}
			}

			// 5) Access log must contain the positive entry AND must not leak.
			assertAccessLogEntry(t, &logBuf, "/v1/query-access/analyze")
			logOutput := logBuf.String()
			for _, marker := range failMarkers {
				if strings.Contains(logOutput, marker) {
					t.Errorf("access log leaked %q: %s; port=%d; connID=%s", marker, logOutput, fc.port, fc.connectionID)
				}
			}
		})
	}
}
