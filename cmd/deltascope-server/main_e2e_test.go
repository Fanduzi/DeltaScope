//go:build e2e

// Package main verifies Docker-backed HTTP metadata-aware end-to-end behavior via registry-backed connection_id selection.
// input: real MySQL/TiDB fixtures, registry-backed authorized connection_id runtime config, and the test binary as a server process
// output: end-to-end proof that deltascope-server serves metadata-aware audit results over HTTP without inline connection details
// pos: slower external e2e verification kept outside the default go test loop
// note: if this file changes, update this header and module README.md.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

const (
	mysqlHTTPAuditConnectionID = "mysql-http-audit"
	tidbHTTPAuditConnectionID  = "tidb-http-audit"
	httpAuditPasswordEnv       = "DELTASCOPE_MYSQL_TIDB_HTTP_PASSWORD"
	httpAuditPassword          = "root"
)

func TestRunServesMetadataAwareAuditOverRealMySQL(t *testing.T) {
	ctx := context.Background()
	baseURL, harness, passwordFile := startHTTPServerMySQLTiDB(t)

	body := postAuditConnectionRequest(t, ctx, baseURL,
		"create table app.users (id bigint unsigned not null auto_increment comment 'id', created_at timestamp not null default current_timestamp comment 'created', updated_at timestamp not null default current_timestamp on update current_timestamp comment 'updated', primary key (id)) comment='dup users'",
		mysqlHTTPAuditConnectionID,
	)
	assertHTTPAuditNoCredentialLeak(t, harness, body, "127.0.0.1:3406", passwordFile)

	contextValue := mustContext(t, body)
	if got := contextValue["mode"]; got != "metadata-aware" {
		t.Fatalf("unexpected audit mode: %#v", got)
	}
	if got := contextValue["dialect"]; got != "mysql" {
		t.Fatalf("unexpected dialect: %#v", got)
	}
	if got := contextValue["schema"]; got != "app" {
		t.Fatalf("unexpected schema: %#v", got)
	}
	if got := contextValue["metadata_source"]; got != "registry" {
		t.Fatalf("unexpected metadata source: %#v", got)
	}
	assertFindingPresent(t, body, "ddl.table.exists.create.forbid")
}

func TestRunServesMetadataAwareAuditOverRealTiDB(t *testing.T) {
	ctx := context.Background()
	baseURL, harness, passwordFile := startHTTPServerMySQLTiDB(t)

	body := postAuditConnectionRequest(t, ctx, baseURL, "delete from orders where id = 1", tidbHTTPAuditConnectionID)
	assertHTTPAuditNoCredentialLeak(t, harness, body, "127.0.0.1:4400", passwordFile)

	contextValue := mustContext(t, body)
	if got := contextValue["mode"]; got != "metadata-aware" {
		t.Fatalf("unexpected audit mode: %#v", got)
	}
	if got := contextValue["dialect"]; got != "tidb" {
		t.Fatalf("unexpected dialect: %#v", got)
	}
	if got := contextValue["schema"]; got != "app" {
		t.Fatalf("unexpected schema: %#v", got)
	}
	if got := contextValue["metadata_source"]; got != "registry" {
		t.Fatalf("unexpected metadata source: %#v", got)
	}
}

func startHTTPServer(t *testing.T) string {
	t.Helper()
	baseURL, _ := startHTTPServerWithRuntimeConfig(t, "")
	return baseURL
}

func startHTTPServerMySQLTiDB(t *testing.T) (string, *httpServerHarness, string) {
	t.Helper()
	t.Setenv(httpAuditPasswordEnv, httpAuditPassword)
	tidbPasswordFile := filepath.Join(t.TempDir(), "tidb-password")
	if err := os.WriteFile(tidbPasswordFile, nil, 0o600); err != nil {
		t.Fatalf("create TiDB password file: %v", err)
	}
	config := fmt.Sprintf(`metadata:
  connections:
    - id: %s
      dialect: mysql
      host: 127.0.0.1
      port: 3406
      user: root
      password_env: %s
      schema: app
      purposes: [audit]
    - id: %s
      dialect: tidb
      host: 127.0.0.1
      port: 4400
      user: root
      password_file: %s
      schema: app
      purposes: [audit]
`, mysqlHTTPAuditConnectionID, httpAuditPasswordEnv, tidbHTTPAuditConnectionID, tidbPasswordFile)
	baseURL, harness := startHTTPServerWithRuntimeConfig(t, config)
	return baseURL, harness, tidbPasswordFile
}

func startHTTPServerWithRuntimeConfig(t *testing.T, config string, buildTags ...string) (string, *httpServerHarness) {
	t.Helper()

	configPath := ""
	if strings.TrimSpace(config) != "" {
		configPath = writeHTTPRuntimeConfigTempFile(t, config)
	}
	listenAddr := freeTCPAddr(t)
	cmd := createHTTPServerCommandWithConfig(t, listenAddr, configPath, buildTags...)
	harness := &httpServerHarness{
		cmd:  cmd,
		done: make(chan struct{}),
	}
	cmd.Stdout = &harness.stdout
	cmd.Stderr = &harness.stderr

	if err := cmd.Start(); err != nil {
		t.Fatalf("start http server: %v\nstdout:\n%s\nstderr:\n%s", err, harness.stdout.String(), harness.stderr.String())
	}
	go func() {
		harness.waitErr = cmd.Wait()
		close(harness.done)
	}()
	t.Cleanup(func() {
		stopHTTPServer(t, harness)
	})

	baseURL := "http://" + listenAddr
	waitForHealthz(t, baseURL, harness)
	return baseURL, harness
}

func createHTTPServerCommand(t *testing.T, listenAddr string) *exec.Cmd {
	t.Helper()
	return createHTTPServerCommandWithConfig(t, listenAddr, "")
}

func createHTTPServerCommandWithConfig(t *testing.T, listenAddr, configPath string, buildTags ...string) *exec.Cmd {
	t.Helper()

	binaryPath := buildHTTPServerBinary(t, buildTags...)
	args := []string{"-listen", listenAddr}
	if configPath != "" {
		args = append(args, "-runtime-config", configPath)
	}
	cmd := exec.Command(binaryPath, args...)
	cmd.Env = os.Environ()
	return cmd
}

func buildHTTPServerBinary(t *testing.T, buildTags ...string) string {
	t.Helper()

	moduleRoot := findModuleRoot(t)
	outDir := t.TempDir()
	binaryPath := filepath.Join(outDir, "deltascope-server")

	args := []string{"build"}
	if len(buildTags) > 0 {
		args = append(args, "-tags", strings.Join(buildTags, ","))
	}
	args = append(args, "-o", binaryPath, "./cmd/deltascope-server")
	cmd := exec.Command("go", args...)
	cmd.Dir = moduleRoot
	cmd.Env = os.Environ()
	if len(buildTags) > 0 {
		cmd.Env = append(cmd.Env, "CGO_ENABLED=1")
	}
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("build http server binary: %v\n%s", err, string(output))
	}
	return binaryPath
}

func writeHTTPRuntimeConfigTempFile(t *testing.T, content string) string {
	t.Helper()
	file, err := os.CreateTemp(t.TempDir(), "http-audit-runtime-*.yaml")
	if err != nil {
		t.Fatalf("create runtime config: %v", err)
	}
	if _, err := file.WriteString(content); err != nil {
		_ = file.Close()
		t.Fatalf("write runtime config: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("close runtime config: %v", err)
	}
	return file.Name()
}

type httpServerHarness struct {
	cmd     *exec.Cmd
	stdout  processOutputSink
	stderr  processOutputSink
	done    chan struct{}
	waitErr error
}

type processOutputSink struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (s *processOutputSink) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.Write(p)
}

func (s *processOutputSink) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.String()
}

func (s *processOutputSink) WaitFor(t *testing.T, substr string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if strings.Contains(s.String(), substr) {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("process output missing %q within %s: %s", substr, timeout, s.String())
}

func stopHTTPServer(t *testing.T, h *httpServerHarness) {
	t.Helper()

	if h == nil || h.cmd == nil || h.cmd.Process == nil {
		return
	}

	select {
	case <-h.done:
		if h.waitErr != nil {
			t.Errorf("http server exited before cleanup: %v\nstdout:\n%s\nstderr:\n%s", h.waitErr, h.stdout.String(), h.stderr.String())
		} else {
			t.Errorf("http server exited before cleanup unexpectedly\nstdout:\n%s\nstderr:\n%s", h.stdout.String(), h.stderr.String())
		}
		return
	default:
	}

	if err := h.cmd.Process.Signal(os.Interrupt); err != nil && !strings.Contains(err.Error(), "process already finished") {
		t.Errorf("signal http server: %v\nstdout:\n%s\nstderr:\n%s", err, h.stdout.String(), h.stderr.String())
	}

	select {
	case <-h.done:
		if h.waitErr != nil {
			t.Errorf("wait http server: %v\nstdout:\n%s\nstderr:\n%s", h.waitErr, h.stdout.String(), h.stderr.String())
		}
	case <-time.After(15 * time.Second):
		if h.cmd.Process != nil {
			_ = h.cmd.Process.Kill()
		}
		<-h.done
		t.Errorf("force-killed http server after timeout: %v\nstdout:\n%s\nstderr:\n%s", h.waitErr, h.stdout.String(), h.stderr.String())
	}
}

func findModuleRoot(t *testing.T) string {
	t.Helper()

	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("resolve working directory: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not locate module root from working directory")
		}
		dir = parent
	}
}

func freeTCPAddr(t *testing.T) string {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("allocate listen port: %v", err)
	}
	defer ln.Close()
	return ln.Addr().String()
}

func waitForHealthz(t *testing.T, baseURL string, h *httpServerHarness) {
	t.Helper()

	client := &http.Client{Timeout: 500 * time.Millisecond}
	deadline := time.Now().Add(45 * time.Second)
	for time.Now().Before(deadline) {
		select {
		case <-h.done:
			if h.waitErr != nil {
				t.Fatalf("server exited before /healthz became ready: %v\nstdout:\n%s\nstderr:\n%s", h.waitErr, h.stdout.String(), h.stderr.String())
			}
			t.Fatalf("server exited before /healthz became ready\nstdout:\n%s\nstderr:\n%s", h.stdout.String(), h.stderr.String())
		default:
		}
		req, err := http.NewRequest(http.MethodGet, baseURL+"/healthz", nil)
		if err != nil {
			t.Fatalf("build health request: %v", err)
		}
		resp, err := client.Do(req)
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatalf("server at %s did not become healthy\nstdout:\n%s\nstderr:\n%s", baseURL, h.stdout.String(), h.stderr.String())
}

func postAuditConnectionRequest(t *testing.T, ctx context.Context, baseURL, sql, connectionID string) map[string]any {
	t.Helper()
	return postAuditRequest(t, ctx, baseURL, map[string]any{
		"sql":           sql,
		"connection_id": connectionID,
	}, connectionID)
}

func postAuditRequest(t *testing.T, ctx context.Context, baseURL string, payload map[string]any, expectedConnectionID ...string) map[string]any {
	t.Helper()

	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal audit payload: %v", err)
	}
	if len(expectedConnectionID) > 0 {
		assertConnectionOnlyPayload(t, body, expectedConnectionID[0])
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/v1/audit", strings.NewReader(string(body)))
	if err != nil {
		t.Fatalf("build audit request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("post audit request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errorBody map[string]any
		_ = json.NewDecoder(resp.Body).Decode(&errorBody)
		t.Fatalf("unexpected audit status %d: %#v", resp.StatusCode, errorBody)
	}

	var decoded map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		t.Fatalf("decode audit response: %v", err)
	}
	return decoded
}

func assertConnectionOnlyPayload(t *testing.T, body []byte, expectedConnectionID string) {
	t.Helper()
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("decode audit payload: %v", err)
	}
	if len(payload) != 2 {
		t.Fatalf("expected payload to contain only sql and connection_id, got %s", body)
	}
	if got := payload["connection_id"]; got != expectedConnectionID {
		t.Fatalf("expected connection_id %q, got %#v", expectedConnectionID, got)
	}
	if _, ok := payload["sql"].(string); !ok {
		t.Fatalf("expected SQL string in payload, got %#v", payload["sql"])
	}
	for _, forbidden := range []string{"connection", "host", "port", "user", "password", "secret", "tls"} {
		if _, ok := payload[forbidden]; ok {
			t.Fatalf("forbidden connection detail %q in payload: %s", forbidden, body)
		}
	}
}

func assertHTTPAuditNoCredentialLeak(t *testing.T, harness *httpServerHarness, body map[string]any, endpoint, secretSource string) {
	t.Helper()
	harness.stderr.WaitFor(t, "/v1/audit", 2*time.Second)
	response, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal audit response for leak check: %v", err)
	}
	combined := string(response) + harness.stdout.String() + harness.stderr.String()
	forbidden := []string{httpAuditPassword, httpAuditPasswordEnv, endpoint, secretSource, "Error 1045", "driver:"}
	if secretSource != "" {
		forbidden = append(forbidden, secretSource)
	}
	for _, forbidden := range forbidden {
		if strings.Contains(combined, forbidden) {
			t.Fatalf("audit response or server output leaked %q\nresponse: %s\nstdout: %s\nstderr: %s", forbidden, response, harness.stdout.String(), harness.stderr.String())
		}
	}
}

func mustContext(t *testing.T, body map[string]any) map[string]any {
	t.Helper()

	contextValue, ok := body["context"].(map[string]any)
	if !ok {
		t.Fatalf("expected context object, got %#v", body["context"])
	}
	return contextValue
}

func assertFindingPresent(t *testing.T, body map[string]any, ruleID string) {
	t.Helper()

	statements, ok := body["statements"].([]any)
	if !ok {
		t.Fatalf("expected statements array, got %#v", body["statements"])
	}
	for _, rawStatement := range statements {
		statement, ok := rawStatement.(map[string]any)
		if !ok {
			continue
		}
		findings, ok := statement["findings"].([]any)
		if !ok {
			continue
		}
		for _, rawFinding := range findings {
			finding, ok := rawFinding.(map[string]any)
			if !ok {
				continue
			}
			if finding["rule_id"] == ruleID {
				return
			}
		}
	}
	t.Fatalf("expected finding for rule %q, got %#v", ruleID, body)
}
