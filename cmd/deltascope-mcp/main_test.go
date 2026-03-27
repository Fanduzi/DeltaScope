// Package main verifies the DeltaScope MCP bootstrap behavior.
// input: CLI args, stub MCP server builders, and captured stdout/stderr buffers
// output: regression coverage for version fast-path and connections-path startup wiring
// pos: command-layer tests for the MCP stdio entrypoint
// note: if this file changes, update this header and module README.md.
package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"testing"

	mcpapi "github.com/Fanduzi/DeltaScope/internal/interfaces/mcp"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

const runAsMCPServer = "_DELTASCOPE_MCP_RUN_AS_SERVER"

func TestMain(m *testing.M) {
	if os.Getenv(runAsMCPServer) != "" {
		os.Unsetenv(runAsMCPServer)
		os.Exit(run([]string{}, os.Stdout, os.Stderr))
	}
	os.Exit(m.Run())
}

func TestRunVersionPrintsVersionWithoutBuildingServer(t *testing.T) {
	previousVersion := Version
	previousNewServer := newMCPServer
	previousRunServer := runMCPServer
	Version = "v9.9.9"
	t.Cleanup(func() {
		Version = previousVersion
		newMCPServer = previousNewServer
		runMCPServer = previousRunServer
	})

	newMCPServer = func(config mcpapi.Config) *sdkmcp.Server {
		t.Fatal("expected -version to avoid server construction")
		return nil
	}
	runMCPServer = func(server *sdkmcp.Server) error {
		t.Fatal("expected -version to avoid server startup")
		return nil
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := run([]string{"-version"}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("expected zero exit code, got %d", exitCode)
	}
	if got := stdout.String(); got != "v9.9.9\n" {
		t.Fatalf("unexpected stdout: %q", got)
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected empty stderr, got %q", stderr.String())
	}
}

func TestRunPassesConnectionsPathToServerConfig(t *testing.T) {
	previousNewServer := newMCPServer
	previousRunServer := runMCPServer
	t.Cleanup(func() {
		newMCPServer = previousNewServer
		runMCPServer = previousRunServer
	})

	var gotConfig mcpapi.Config
	server := sdkmcp.NewServer(&sdkmcp.Implementation{Name: "test", Version: "v0.0.1"}, nil)
	newMCPServer = func(config mcpapi.Config) *sdkmcp.Server {
		gotConfig = config
		return server
	}
	runMCPServer = func(gotServer *sdkmcp.Server) error {
		if gotServer != server {
			t.Fatalf("unexpected server pointer: %p", gotServer)
		}
		return nil
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := run([]string{"-connections-path", "/tmp/custom-connections.yaml"}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("expected zero exit code, got %d", exitCode)
	}
	if gotConfig.ConnectionsPath != "/tmp/custom-connections.yaml" {
		t.Fatalf("unexpected connections path: %q", gotConfig.ConnectionsPath)
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected empty stderr, got %q", stderr.String())
	}
}

func TestRunServesMCPOverRealStdio(t *testing.T) {
	ctx := context.Background()
	cmd := createMCPServerCommand(t)

	client := sdkmcp.NewClient(&sdkmcp.Implementation{Name: "test-client", Version: "v0.0.1"}, nil)
	session, err := client.Connect(ctx, &sdkmcp.CommandTransport{Command: cmd}, nil)
	if err != nil {
		t.Fatalf("connect stdio mcp server: %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })
	if info := session.InitializeResult().ServerInfo; info == nil || info.Name != "deltascope-mcp" {
		t.Fatalf("unexpected server info: %#v", info)
	}

	result, err := session.CallTool(ctx, &sdkmcp.CallToolParams{
		Name:      "audit_sql",
		Arguments: map[string]any{"sql": "delete from users"},
	})
	if err != nil {
		t.Fatalf("call audit_sql over stdio: %v", err)
	}
	if result == nil || result.IsError {
		t.Fatalf("expected successful tool result, got %#v", result)
	}
	if len(result.Content) == 0 {
		t.Fatalf("expected non-empty tool content, got %#v", result)
	}

	body, ok := result.StructuredContent.(map[string]any)
	if !ok {
		t.Fatalf("expected structured result body, got %T", result.StructuredContent)
	}
	if body["verdict"] != "reject" {
		t.Fatalf("unexpected verdict: %#v", body["verdict"])
	}
	contextValue, ok := body["context"].(map[string]any)
	if !ok {
		t.Fatalf("expected audit context object, got %#v", body["context"])
	}
	if contextValue["mode"] != "offline" {
		t.Fatalf("unexpected audit mode: %#v", contextValue["mode"])
	}
}

func createMCPServerCommand(t *testing.T) *exec.Cmd {
	t.Helper()

	exe, err := os.Executable()
	if err != nil {
		t.Fatalf("resolve test executable: %v", err)
	}

	cmd := exec.Command(exe)
	cmd.Env = append(os.Environ(), runAsMCPServer+"=1")
	return cmd
}
