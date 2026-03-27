//go:build e2e

// Package main verifies Docker-backed MCP metadata-aware end-to-end behavior.
// input: real MySQL fixtures, the MCP stdio entrypoint, and SDK command-transport clients
// output: end-to-end proof that deltascope-mcp can audit through the live metadata path
// pos: slower external e2e verification kept outside the default go test loop
// note: if this file changes, update this header and module README.md.
package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestRunServesMetadataAwareAuditOverRealMySQL(t *testing.T) {
	ctx := context.Background()
	cmd := createMCPServerCommand(t)

	client := sdkmcp.NewClient(&sdkmcp.Implementation{Name: "test-client", Version: "v0.0.1"}, nil)
	session, err := client.Connect(ctx, &sdkmcp.CommandTransport{Command: cmd}, nil)
	if err != nil {
		t.Fatalf("connect stdio mcp server: %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })

	result, err := session.CallTool(ctx, &sdkmcp.CallToolParams{
		Name: "audit_sql",
		Arguments: map[string]any{
			"sql": "delete from orders where id = 1",
			"connection": map[string]any{
				"host":     "127.0.0.1",
				"port":     3406,
				"user":     "root",
				"password": "root",
			},
		},
	})
	if err != nil {
		t.Fatalf("call audit_sql over stdio: %v", err)
	}
	if result == nil || result.IsError {
		t.Fatalf("expected successful tool result, got %#v", result)
	}

	body, ok := result.StructuredContent.(map[string]any)
	if !ok {
		t.Fatalf("expected structured result body, got %T", result.StructuredContent)
	}
	contextValue, ok := body["context"].(map[string]any)
	if !ok {
		t.Fatalf("expected audit context object, got %#v", body["context"])
	}
	if contextValue["mode"] != "metadata-aware" {
		t.Fatalf("unexpected audit mode: %#v", contextValue["mode"])
	}
	if contextValue["dialect"] != "mysql" {
		t.Fatalf("unexpected dialect: %#v", contextValue["dialect"])
	}
	if contextValue["schema"] != "app" {
		t.Fatalf("unexpected schema: %#v", contextValue["schema"])
	}
	if contextValue["metadata_source"] != "direct" {
		t.Fatalf("unexpected metadata source: %#v", contextValue["metadata_source"])
	}
}

func TestRunServesMetadataAwareAuditOverConnectionRef(t *testing.T) {
	ctx := context.Background()
	home := t.TempDir()
	configDir := filepath.Join(home, ".config", "deltascope")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}
	configPath := filepath.Join(configDir, "connections.yaml")
	if err := os.WriteFile(configPath, []byte(`
connections:
  prod_readonly:
    host: 127.0.0.1
    port: 3406
    user: root
    schema: app
    dialect: mysql
    password: root
`), 0o600); err != nil {
		t.Fatalf("write connections config: %v", err)
	}

	cmd := createMCPServerCommand(t)
	cmd.Env = append(cmd.Env, "HOME="+home)

	client := sdkmcp.NewClient(&sdkmcp.Implementation{Name: "test-client", Version: "v0.0.1"}, nil)
	session, err := client.Connect(ctx, &sdkmcp.CommandTransport{Command: cmd}, nil)
	if err != nil {
		t.Fatalf("connect stdio mcp server: %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })

	result, err := session.CallTool(ctx, &sdkmcp.CallToolParams{
		Name:      "audit_sql",
		Arguments: map[string]any{"sql": "delete from orders where id = 1", "connection_ref": "prod_readonly"},
	})
	if err != nil {
		t.Fatalf("call audit_sql over stdio: %v", err)
	}
	if result == nil || result.IsError {
		t.Fatalf("expected successful tool result, got %#v", result)
	}

	body, ok := result.StructuredContent.(map[string]any)
	if !ok {
		t.Fatalf("expected structured result body, got %T", result.StructuredContent)
	}
	contextValue, ok := body["context"].(map[string]any)
	if !ok {
		t.Fatalf("expected audit context object, got %#v", body["context"])
	}
	if contextValue["mode"] != "metadata-aware" {
		t.Fatalf("unexpected audit mode: %#v", contextValue["mode"])
	}
	if contextValue["schema"] != "app" {
		t.Fatalf("unexpected schema: %#v", contextValue["schema"])
	}
	if contextValue["metadata_source"] != "connection_ref" {
		t.Fatalf("unexpected metadata source: %#v", contextValue["metadata_source"])
	}
	if contextValue["schema_source"] != "config" {
		t.Fatalf("unexpected schema source: %#v", contextValue["schema_source"])
	}
}

func TestRunServesMetadataAwareAuditOverRealTiDB(t *testing.T) {
	ctx := context.Background()
	cmd := createMCPServerCommand(t)

	client := sdkmcp.NewClient(&sdkmcp.Implementation{Name: "test-client", Version: "v0.0.1"}, nil)
	session, err := client.Connect(ctx, &sdkmcp.CommandTransport{Command: cmd}, nil)
	if err != nil {
		t.Fatalf("connect stdio mcp server: %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })

	result, err := session.CallTool(ctx, &sdkmcp.CallToolParams{
		Name: "audit_sql",
		Arguments: map[string]any{
			"sql": "delete from orders where id = 1",
			"connection": map[string]any{
				"host": "127.0.0.1",
				"port": 4400,
				"user": "root",
			},
		},
	})
	if err != nil {
		t.Fatalf("call audit_sql over stdio: %v", err)
	}
	if result == nil || result.IsError {
		t.Fatalf("expected successful tool result, got %#v", result)
	}

	body, ok := result.StructuredContent.(map[string]any)
	if !ok {
		t.Fatalf("expected structured result body, got %T", result.StructuredContent)
	}
	contextValue, ok := body["context"].(map[string]any)
	if !ok {
		t.Fatalf("expected audit context object, got %#v", body["context"])
	}
	if contextValue["mode"] != "metadata-aware" {
		t.Fatalf("unexpected audit mode: %#v", contextValue["mode"])
	}
	if contextValue["dialect"] != "tidb" {
		t.Fatalf("unexpected dialect: %#v", contextValue["dialect"])
	}
	if contextValue["schema"] != "app" {
		t.Fatalf("unexpected schema: %#v", contextValue["schema"])
	}
	if contextValue["metadata_source"] != "direct" {
		t.Fatalf("unexpected metadata source: %#v", contextValue["metadata_source"])
	}
}

func TestRunServesMetadataAwareAuditOverTiDBConnectionRef(t *testing.T) {
	ctx := context.Background()
	home := t.TempDir()
	configDir := filepath.Join(home, ".config", "deltascope")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}
	configPath := filepath.Join(configDir, "connections.yaml")
	if err := os.WriteFile(configPath, []byte(`
connections:
  tidb_readonly:
    host: 127.0.0.1
    port: 4400
    user: root
    schema: app
    dialect: tidb
`), 0o600); err != nil {
		t.Fatalf("write connections config: %v", err)
	}

	cmd := createMCPServerCommand(t)
	cmd.Env = append(cmd.Env, "HOME="+home)

	client := sdkmcp.NewClient(&sdkmcp.Implementation{Name: "test-client", Version: "v0.0.1"}, nil)
	session, err := client.Connect(ctx, &sdkmcp.CommandTransport{Command: cmd}, nil)
	if err != nil {
		t.Fatalf("connect stdio mcp server: %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })

	result, err := session.CallTool(ctx, &sdkmcp.CallToolParams{
		Name:      "audit_sql",
		Arguments: map[string]any{"sql": "delete from orders where id = 1", "connection_ref": "tidb_readonly"},
	})
	if err != nil {
		t.Fatalf("call audit_sql over stdio: %v", err)
	}
	if result == nil || result.IsError {
		t.Fatalf("expected successful tool result, got %#v", result)
	}

	body, ok := result.StructuredContent.(map[string]any)
	if !ok {
		t.Fatalf("expected structured result body, got %T", result.StructuredContent)
	}
	contextValue, ok := body["context"].(map[string]any)
	if !ok {
		t.Fatalf("expected audit context object, got %#v", body["context"])
	}
	if contextValue["mode"] != "metadata-aware" {
		t.Fatalf("unexpected audit mode: %#v", contextValue["mode"])
	}
	if contextValue["dialect"] != "tidb" {
		t.Fatalf("unexpected dialect: %#v", contextValue["dialect"])
	}
	if contextValue["schema"] != "app" {
		t.Fatalf("unexpected schema: %#v", contextValue["schema"])
	}
	if contextValue["metadata_source"] != "connection_ref" {
		t.Fatalf("unexpected metadata source: %#v", contextValue["metadata_source"])
	}
	if contextValue["schema_source"] != "config" {
		t.Fatalf("unexpected schema source: %#v", contextValue["schema_source"])
	}
}
