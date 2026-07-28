//go:build integration

// Package httpapi verifies HTTP query-access online behavior for mixed-literal
// operands (COALESCE, NULLIF, IFNULL) and relationless (no FROM) literal-only
// shapes against real Docker-backed MySQL and TiDB.
// input: real MySQL/TiDB fixtures, the HTTP JSON entrypoint, and a test registry
// output: end-to-end proof that mixed-literal scalar operands yield
//
//	read_only + admissible via the HTTP online path across all four MySQL/TiDB
//	versions, with exact requirement assertions and no-leak guards; relationless
//	literal-only shapes additionally yield zero requirements/relations/columns
//
// pos: HTTP online E2E coverage for the mixed-literal scalar operand feature
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
	scanned := sanitizeLogForLeakScan(logOutput)
	for _, marker := range markers {
		if strings.Contains(scanned, marker) {
			t.Errorf("access log leaked %q: %s", marker, logOutput)
		}
	}
}

// requestIDPattern matches the server-generated request_id field. Its value is
// random hex with no relationship to SQL, credentials, host, port, or schema,
// so it must be excluded from no-leak scans to avoid coincidental substring
// matches (e.g. a port number appearing inside the random id).
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
				// Failure connections use the real MySQL 8.4 fixture
				// (host:port) with valid user 'root' but invalid passwords.
				// This proves the failure comes from MySQL auth error 1045
				// (Access denied), not port simulation or input validation.
				{
					ID:          "fail_env_conn",
					Dialect:     "mysql",
					Host:        "127.0.0.1",
					Port:        3840,
					User:        "root",
					PasswordEnv: "E2E_FAIL_ENV_PASSWORD",
					Schema:      "app",
					Purposes:    []string{"query_access"},
				},
				{
					ID:           "fail_file_conn",
					Dialect:      "mysql",
					Host:         "127.0.0.1",
					Port:         3840,
					User:         "root",
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
		name             string
		sql              string
		wantRequirements []map[string]string
		relationless     bool
	}{
		{
			name: "COALESCE",
			sql:  "SELECT COALESCE(name, 'SECRET_LITERAL') FROM app.builtin_semantic_facts",
			wantRequirements: []map[string]string{
				{"object": "app.builtin_semantic_facts", "privilege": "read_table"},
				{"object": "app.builtin_semantic_facts.name", "privilege": "read_column"},
			},
		},
		{
			name: "NULLIF",
			sql:  "SELECT NULLIF(name, 'SECRET_LITERAL') FROM app.builtin_semantic_facts",
			wantRequirements: []map[string]string{
				{"object": "app.builtin_semantic_facts", "privilege": "read_table"},
				{"object": "app.builtin_semantic_facts.name", "privilege": "read_column"},
			},
		},
		{
			name: "IFNULL",
			sql:  "SELECT IFNULL(name, 'SECRET_LITERAL') FROM app.builtin_semantic_facts",
			wantRequirements: []map[string]string{
				{"object": "app.builtin_semantic_facts", "privilege": "read_table"},
				{"object": "app.builtin_semantic_facts.name", "privilege": "read_column"},
			},
		},
		{
			name: "LOWER_literal",
			sql:  "SELECT LOWER('SECRET_LITERAL') FROM app.builtin_semantic_facts",
			wantRequirements: []map[string]string{
				{"object": "app.builtin_semantic_facts", "privilege": "read_table"},
			},
		},
		{
			name: "UPPER_literal",
			sql:  "SELECT UPPER('SECRET_LITERAL') FROM app.builtin_semantic_facts",
			wantRequirements: []map[string]string{
				{"object": "app.builtin_semantic_facts", "privilege": "read_table"},
			},
		},
		{
			name: "LENGTH_literal",
			sql:  "SELECT LENGTH('SECRET_LITERAL') FROM app.builtin_semantic_facts",
			wantRequirements: []map[string]string{
				{"object": "app.builtin_semantic_facts", "privilege": "read_table"},
			},
		},
		{
			name: "CHAR_LENGTH_literal",
			sql:  "SELECT CHAR_LENGTH('SECRET_LITERAL') FROM app.builtin_semantic_facts",
			wantRequirements: []map[string]string{
				{"object": "app.builtin_semantic_facts", "privilege": "read_table"},
			},
		},
		{
			name: "ABS_literal",
			sql:  "SELECT ABS(42) FROM app.builtin_semantic_facts",
			wantRequirements: []map[string]string{
				{"object": "app.builtin_semantic_facts", "privilege": "read_table"},
			},
		},
		{
			name: "CEIL_literal",
			sql:  "SELECT CEIL(42) FROM app.builtin_semantic_facts",
			wantRequirements: []map[string]string{
				{"object": "app.builtin_semantic_facts", "privilege": "read_table"},
			},
		},
		{
			name: "CEILING_literal",
			sql:  "SELECT CEILING(42) FROM app.builtin_semantic_facts",
			wantRequirements: []map[string]string{
				{"object": "app.builtin_semantic_facts", "privilege": "read_table"},
			},
		},
		{
			name: "FLOOR_literal",
			sql:  "SELECT FLOOR(42) FROM app.builtin_semantic_facts",
			wantRequirements: []map[string]string{
				{"object": "app.builtin_semantic_facts", "privilege": "read_table"},
			},
		},
		{
			name: "COUNT_literal",
			sql:  "SELECT COUNT(1) FROM app.builtin_semantic_facts",
			wantRequirements: []map[string]string{
				{"object": "app.builtin_semantic_facts", "privilege": "read_table"},
			},
		},
		{
			name: "COALESCE_reversed",
			sql:  "SELECT COALESCE('SECRET_LITERAL', name) FROM app.builtin_semantic_facts",
			wantRequirements: []map[string]string{
				{"object": "app.builtin_semantic_facts", "privilege": "read_table"},
				{"object": "app.builtin_semantic_facts.name", "privilege": "read_column"},
			},
		},
		{
			name: "NULLIF_reversed",
			sql:  "SELECT NULLIF('SECRET_LITERAL', name) FROM app.builtin_semantic_facts",
			wantRequirements: []map[string]string{
				{"object": "app.builtin_semantic_facts", "privilege": "read_table"},
				{"object": "app.builtin_semantic_facts.name", "privilege": "read_column"},
			},
		},
		{
			name: "IFNULL_reversed",
			sql:  "SELECT IFNULL('SECRET_LITERAL', name) FROM app.builtin_semantic_facts",
			wantRequirements: []map[string]string{
				{"object": "app.builtin_semantic_facts", "privilege": "read_table"},
				{"object": "app.builtin_semantic_facts.name", "privilege": "read_column"},
			},
		},
		{
			name: "COALESCE_all_constant",
			sql:  "SELECT COALESCE('SECRET_LITERAL', 'SECRET_LITERAL2') FROM app.builtin_semantic_facts",
			wantRequirements: []map[string]string{
				{"object": "app.builtin_semantic_facts", "privilege": "read_table"},
			},
		},
		{
			name: "NULLIF_all_constant",
			sql:  "SELECT NULLIF('SECRET_LITERAL', 'SECRET_LITERAL2') FROM app.builtin_semantic_facts",
			wantRequirements: []map[string]string{
				{"object": "app.builtin_semantic_facts", "privilege": "read_table"},
			},
		},
		{
			name: "IFNULL_all_constant",
			sql:  "SELECT IFNULL('SECRET_LITERAL', 'SECRET_LITERAL2') FROM app.builtin_semantic_facts",
			wantRequirements: []map[string]string{
				{"object": "app.builtin_semantic_facts", "privilege": "read_table"},
			},
		},
		// Relationless (no FROM) literal-only shapes: nothing is read.
		{name: "relationless_lower", sql: "SELECT LOWER('SECRET_LITERAL')", wantRequirements: nil, relationless: true},
		{name: "relationless_upper", sql: "SELECT UPPER('SECRET_LITERAL')", wantRequirements: nil, relationless: true},
		{name: "relationless_length", sql: "SELECT LENGTH('SECRET_LITERAL')", wantRequirements: nil, relationless: true},
		{name: "relationless_char_length", sql: "SELECT CHAR_LENGTH('SECRET_LITERAL')", wantRequirements: nil, relationless: true},
		{name: "relationless_abs", sql: "SELECT ABS(42)", wantRequirements: nil, relationless: true},
		{name: "relationless_ceil", sql: "SELECT CEIL(42)", wantRequirements: nil, relationless: true},
		{name: "relationless_ceiling", sql: "SELECT CEILING(42)", wantRequirements: nil, relationless: true},
		{name: "relationless_floor", sql: "SELECT FLOOR(42)", wantRequirements: nil, relationless: true},
		{name: "relationless_count_literal", sql: "SELECT COUNT(1)", wantRequirements: nil, relationless: true},
		{name: "relationless_coalesce", sql: "SELECT COALESCE('SECRET_LITERAL', 'SECRET_LITERAL2')", wantRequirements: nil, relationless: true},
		{name: "relationless_nullif", sql: "SELECT NULLIF('SECRET_LITERAL', 'SECRET_LITERAL2')", wantRequirements: nil, relationless: true},
		{name: "relationless_ifnull", sql: "SELECT IFNULL('SECRET_LITERAL', 'SECRET_LITERAL2')", wantRequirements: nil, relationless: true},
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

					if probe.relationless {
						// Relationless literal-only proves nothing is read:
						// every scope key is absent or empty.
						for _, key := range []string{"requirements", "relations", "referenced_columns", "unresolved"} {
							if v, present := result[key]; present {
								if arr, ok := v.([]any); ok {
									if len(arr) != 0 {
										t.Errorf("relationless %s: got %d items (%v), want 0", key, len(arr), arr)
									}
								} else if v != nil {
									t.Errorf("relationless %s: got non-array %T (%v), want absent/empty", key, v, v)
								}
							}
						}
					} else {
						wantReqs := probe.wantRequirements
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
					}

					bodyStr := body.String()
					for _, marker := range []string{"SECRET_LITERAL", "SECRET_LITERAL2", "root", "E2E_MYSQL_PASSWORD"} {
						if strings.Contains(bodyStr, marker) {
							t.Errorf("response leaked %q: %s", marker, bodyStr)
						}
					}
					raw, _ := json.Marshal(result)
					for _, marker := range []string{"SECRET_LITERAL", "SECRET_LITERAL2"} {
						if strings.Contains(string(raw), marker) {
							t.Errorf("deserialized result leaked %s", marker)
						}
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
				for _, marker := range []string{"SECRET_LITERAL", "SECRET_LITERAL2"} {
					if strings.Contains(bodyStr, marker) {
						t.Errorf("response leaked %s: %s", marker, bodyStr)
					}
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
		user         string
		sqlLiteral   string // unique per sub-case; proves request content doesn't leak on auth failure
		extraMarkers []string
	}{
		{
			name:         "env_credential_failure",
			connectionID: "fail_env_conn",
			password:     "FAIL_SECRET_ENV_pw_9f3b2a1c",
			user:         "root",
			sqlLiteral:   "FAIL_SQL_LITERAL_env_a1b2c3d4",
			extraMarkers: []string{"E2E_FAIL_ENV_PASSWORD"},
		},
		{
			name:         "file_credential_failure",
			connectionID: "fail_file_conn",
			password:     failPWContent,
			user:         "root",
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
				// host, port, and schema (real MySQL 8.4 fixture)
				"127.0.0.1",
				"3840",
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
			// request content doesn't leak to response or access log on auth failure.
			sql := fmt.Sprintf(
				"SELECT COALESCE(name, '%s') FROM app.builtin_semantic_facts",
				fc.sqlLiteral,
			)
			payload := fmt.Sprintf(`{"sql":%q,"connection_id":%q}`, sql, fc.connectionID)
			req, err := http.NewRequest(http.MethodPost, ts.URL+"/v1/query-access/analyze", strings.NewReader(payload))
			if err != nil {
				t.Fatalf("create request: %v; connID=%s", err, fc.connectionID)
			}
			req.Header.Set("Content-Type", "application/json")

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("send request: %v; connID=%s", err, fc.connectionID)
			}
			defer resp.Body.Close()

			var body bytes.Buffer
			if _, err := body.ReadFrom(resp.Body); err != nil {
				t.Fatalf("read response: %v; connID=%s", err, fc.connectionID)
			}

			// 1) HTTP status must be 502 Bad Gateway — proves the request
			//    reached the real MySQL 8.4 fixture and was rejected by auth.
			if resp.StatusCode != http.StatusBadGateway {
				t.Fatalf("status: got %d, want 502; body: %s; connID=%s", resp.StatusCode, body.String(), fc.connectionID)
			}

			// 2) JSON error code must be exactly "connection_failed".
			var result map[string]any
			if err := json.Unmarshal(body.Bytes(), &result); err != nil {
				t.Fatalf("unmarshal: %v; connID=%s", err, fc.connectionID)
			}
			errObj, ok := result["error"].(map[string]any)
			if !ok {
				t.Fatalf("error: not a map; got %T; connID=%s", result["error"], fc.connectionID)
			}
			if errObj["code"] != "connection_failed" {
				t.Errorf("error.code: got %q, want connection_failed; connID=%s", errObj["code"], fc.connectionID)
			}

			// 3) Raw HTTP response body must not leak any marker.
			bodyStr := body.String()
			for _, marker := range failMarkers {
				if strings.Contains(bodyStr, marker) {
					t.Errorf("response body leaked %q: %s; connID=%s", marker, bodyStr, fc.connectionID)
				}
			}

			// 4) Deserialized JSON must not leak any marker.
			raw, _ := json.Marshal(result)
			jsonStr := string(raw)
			for _, marker := range failMarkers {
				if strings.Contains(jsonStr, marker) {
					t.Errorf("deserialized JSON leaked %q: %s; connID=%s", marker, jsonStr, fc.connectionID)
				}
			}

			// 5) Access log must contain the positive entry AND must not leak.
			assertAccessLogEntry(t, &logBuf, "/v1/query-access/analyze")
			logOutput := logBuf.String()
			scannedLog := sanitizeLogForLeakScan(logOutput)
			for _, marker := range failMarkers {
				if strings.Contains(scannedLog, marker) {
					t.Errorf("access log leaked %q: %s; connID=%s", marker, logOutput, fc.connectionID)
				}
			}
		})
	}
}
