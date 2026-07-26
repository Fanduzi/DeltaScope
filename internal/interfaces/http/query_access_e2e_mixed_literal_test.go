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
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/infrastructure/runtimeconfig"
)

// syncBuffer is a thread-safe bytes.Buffer for capturing log output.
// It synchronizes both Write and String operations.
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

// noLeakMarkers returns the full set of markers that must not appear in access logs.
func noLeakMarkers() []string {
	return []string{
		"SECRET_LITERAL",
		"COALESCE(",
		"NULLIF(",
		"IFNULL(",
		"builtin_semantic_facts",
		"root",
		"E2E_MYSQL_PASSWORD",
	}
}

// assertAccessLogEntry verifies the captured log contains at least one structured
// access log entry for the given path, proving the capture sink is wired correctly.
func assertAccessLogEntry(t *testing.T, logOutput, path string) {
	t.Helper()
	if !strings.Contains(logOutput, `"msg":"http request"`) {
		t.Errorf("access log missing structured entry: no 'http request' msg found in: %s", logOutput)
	}
	if !strings.Contains(logOutput, path) {
		t.Errorf("access log missing path %q in: %s", path, logOutput)
	}
}

// assertNoLogLeaks checks the log output does not contain any forbidden markers.
func assertNoLogLeaks(t *testing.T, logOutput string, markers []string) {
	t.Helper()
	for _, marker := range markers {
		if strings.Contains(logOutput, marker) {
			t.Errorf("access log leaked %q: %s", marker, logOutput)
		}
	}
}

func TestQueryAccessOnline_MixedLiteralScalars(t *testing.T) {
	t.Setenv("E2E_MYSQL_PASSWORD", "root")

	emptyPWFile := writeEmptyTempFile(t)

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

					logOutput := logBuf.String()
					assertAccessLogEntry(t, logOutput, "/v1/query-access/analyze")
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

				logOutput := logBuf.String()
				assertAccessLogEntry(t, logOutput, "/v1/query-access/analyze")
				assertNoLogLeaks(t, logOutput, noLeakMarkers())
			})
		}
	})
}

// writeEmptyTempFile creates a temporary empty file for TiDB password-less auth.
func writeEmptyTempFile(t *testing.T) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "empty-pw-*")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	f.Close()
	return f.Name()
}
