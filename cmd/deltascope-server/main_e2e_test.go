//go:build e2e

// Package main verifies Docker-backed HTTP metadata-aware end-to-end behavior.
// input: real MySQL/TiDB fixtures, the HTTP JSON entrypoint, and the test binary as a server process
// output: end-to-end proof that deltascope-server serves metadata-aware audit results over HTTP
// pos: slower external e2e verification kept outside the default go test loop
// note: if this file changes, update this header and module README.md.
package main

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

const runAsHTTPServer = "_DELTASCOPE_SERVER_RUN_AS_SERVER"

func TestMain(m *testing.M) {
	if os.Getenv(runAsHTTPServer) != "" {
		os.Unsetenv(runAsHTTPServer)
		main()
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func TestRunServesMetadataAwareAuditOverRealMySQL(t *testing.T) {
	ctx := context.Background()
	baseURL, stop := startHTTPServer(t)
	t.Cleanup(stop)

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
	assertFindingPresent(t, body, "ddl.table.exists.create.forbid")
}

func TestRunServesMetadataAwareAuditOverRealTiDB(t *testing.T) {
	ctx := context.Background()
	baseURL, stop := startHTTPServer(t)
	t.Cleanup(stop)

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
}

func startHTTPServer(t *testing.T) (string, func()) {
	t.Helper()

	listenAddr := freeTCPAddr(t)
	cmd := createHTTPServerCommand(t, listenAddr)

	if err := cmd.Start(); err != nil {
		t.Fatalf("start http server: %v", err)
	}

	baseURL := "http://" + listenAddr
	waitForHealthz(t, baseURL)

	stopped := false
	return baseURL, func() {
		if stopped {
			return
		}
		stopped = true
		if cmd.Process == nil {
			return
		}
		_ = cmd.Process.Signal(os.Interrupt)
		done := make(chan error, 1)
		go func() {
			done <- cmd.Wait()
		}()
		select {
		case <-time.After(15 * time.Second):
			_ = cmd.Process.Kill()
			<-done
		case <-done:
		}
	}
}

func createHTTPServerCommand(t *testing.T, listenAddr string) *exec.Cmd {
	t.Helper()

	exe, err := os.Executable()
	if err != nil {
		t.Fatalf("resolve test executable: %v", err)
	}

	cmd := exec.Command(exe, "-listen", listenAddr)
	cmd.Env = append(os.Environ(), runAsHTTPServer+"=1")
	return cmd
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

func waitForHealthz(t *testing.T, baseURL string) {
	t.Helper()

	client := &http.Client{Timeout: 500 * time.Millisecond}
	deadline := time.Now().Add(45 * time.Second)
	for time.Now().Before(deadline) {
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
	t.Fatalf("server at %s did not become healthy", baseURL)
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
