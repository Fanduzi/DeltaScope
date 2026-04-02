//go:build e2e

// Package main verifies Docker-backed HTTP metadata-aware end-to-end behavior.
// input: real MySQL/TiDB fixtures, the HTTP JSON entrypoint, and the test binary as a server process
// output: end-to-end proof that deltascope-server serves metadata-aware audit results over HTTP
// pos: slower external e2e verification kept outside the default go test loop
// note: if this file changes, update this header and module README.md.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRunServesMetadataAwareAuditOverRealMySQL(t *testing.T) {
	ctx := context.Background()
	baseURL := startHTTPServer(t)

	payload := map[string]any{
		"sql": "create table app.users (id bigint unsigned not null auto_increment comment 'id', created_at timestamp not null default current_timestamp comment 'created', updated_at timestamp not null default current_timestamp on update current_timestamp comment 'updated', primary key (id)) comment='dup users'",
		"connection": map[string]any{
			"host":     "127.0.0.1",
			"port":     3406,
			"user":     "root",
			"password": "root",
		},
	}
	body := postAuditRequest(t, ctx, baseURL, payload)

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
	if got := contextValue["metadata_source"]; got != "direct" {
		t.Fatalf("unexpected metadata source: %#v", got)
	}
	assertFindingPresent(t, body, "ddl.table.exists.create.forbid")
}

func TestRunServesMetadataAwareAuditOverRealTiDB(t *testing.T) {
	ctx := context.Background()
	baseURL := startHTTPServer(t)

	payload := map[string]any{
		"sql": "delete from orders where id = 1",
		"connection": map[string]any{
			"host": "127.0.0.1",
			"port": 4400,
			"user": "root",
		},
	}
	body := postAuditRequest(t, ctx, baseURL, payload)

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
	if got := contextValue["metadata_source"]; got != "direct" {
		t.Fatalf("unexpected metadata source: %#v", got)
	}
}

func startHTTPServer(t *testing.T) string {
	t.Helper()

	listenAddr := freeTCPAddr(t)
	cmd := createHTTPServerCommand(t, listenAddr)
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
	return baseURL
}

func createHTTPServerCommand(t *testing.T, listenAddr string) *exec.Cmd {
	t.Helper()

	binaryPath := buildHTTPServerBinary(t)
	cmd := exec.Command(binaryPath, "-listen", listenAddr)
	cmd.Env = os.Environ()
	return cmd
}

func buildHTTPServerBinary(t *testing.T) string {
	t.Helper()

	moduleRoot := findModuleRoot(t)
	outDir := t.TempDir()
	binaryPath := filepath.Join(outDir, "deltascope-server")

	cmd := exec.Command("go", "build", "-o", binaryPath, "./cmd/deltascope-server")
	cmd.Dir = moduleRoot
	cmd.Env = os.Environ()
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("build http server binary: %v\n%s", err, string(output))
	}
	return binaryPath
}

type httpServerHarness struct {
	cmd     *exec.Cmd
	stdout  bytes.Buffer
	stderr  bytes.Buffer
	done    chan struct{}
	waitErr error
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

func postAuditRequest(t *testing.T, ctx context.Context, baseURL string, payload map[string]any) map[string]any {
	t.Helper()

	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal audit payload: %v", err)
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
