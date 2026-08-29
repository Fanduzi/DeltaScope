//go:build e2e && postgresql

// Package main verifies Docker-backed MCP metadata-aware end-to-end behavior for PostgreSQL.
// input: real PostgreSQL fixtures, the MCP stdio entrypoint, and a PG-capable MCP binary
// output: end-to-end proof that deltascope-mcp can audit through the live PG metadata path with distinct database/schema inputs
// pos: slower external e2e verification kept outside the default go test loop
// note: if this file changes, update this header and module README.md.
package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestRunServesMetadataAwareAuditOverRealPostgreSQL(t *testing.T) {
	ctx := context.Background()

	// Case 1: qualified schema detection from SQL
	t.Run("qualified_schema", func(t *testing.T) {
		cmd := createMCPServerCommandPG(t)

		client := sdkmcp.NewClient(&sdkmcp.Implementation{Name: "test-client", Version: "v0.0.1"}, nil)
		session, err := client.Connect(ctx, &sdkmcp.CommandTransport{Command: cmd}, nil)
		if err != nil {
			t.Fatalf("connect stdio mcp server: %v", err)
		}
		t.Cleanup(func() { _ = session.Close() })

		result, err := session.CallTool(ctx, &sdkmcp.CallToolParams{
			Name: "audit_sql",
			Arguments: map[string]any{
				"sql": "delete from app.users where id = 1",
				"connection": map[string]any{
					"host":     "127.0.0.1",
					"port":     5500,
					"user":     "root",
					"password": "root",
					"dialect":  "postgresql",
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
		if contextValue["dialect"] != "postgresql" {
			t.Fatalf("unexpected dialect: %#v", contextValue["dialect"])
		}
		if contextValue["schema"] != "app" {
			t.Fatalf("unexpected schema: %#v", contextValue["schema"])
		}
		if contextValue["metadata_source"] != "direct" {
			t.Fatalf("unexpected metadata source: %#v", contextValue["metadata_source"])
		}
	})

	// Case 2: explicit schema via connection block
	t.Run("explicit_schema", func(t *testing.T) {
		cmd := createMCPServerCommandPG(t)

		client := sdkmcp.NewClient(&sdkmcp.Implementation{Name: "test-client", Version: "v0.0.1"}, nil)
		session, err := client.Connect(ctx, &sdkmcp.CommandTransport{Command: cmd}, nil)
		if err != nil {
			t.Fatalf("connect stdio mcp server: %v", err)
		}
		t.Cleanup(func() { _ = session.Close() })

		result, err := session.CallTool(ctx, &sdkmcp.CallToolParams{
			Name: "audit_sql",
			Arguments: map[string]any{
				"sql":     "delete from users where id = 1",
				"dialect": "postgresql",
				"connection": map[string]any{
					"host":     "127.0.0.1",
					"port":     5500,
					"user":     "root",
					"password": "root",
					"database": "postgres",
					"schema":   "archive",
					"dialect":  "postgresql",
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
		if contextValue["schema"] != "archive" {
			t.Fatalf("unexpected schema: %#v", contextValue["schema"])
		}
	})

	// Case 3: supplied database selects the catalog used for table metadata
	t.Run("database_selection", func(t *testing.T) {
		cmd := createMCPServerCommandPG(t)

		client := sdkmcp.NewClient(&sdkmcp.Implementation{Name: "test-client", Version: "v0.0.1"}, nil)
		session, err := client.Connect(ctx, &sdkmcp.CommandTransport{Command: cmd}, nil)
		if err != nil {
			t.Fatalf("connect stdio mcp server: %v", err)
		}
		t.Cleanup(func() { _ = session.Close() })

		result, err := session.CallTool(ctx, &sdkmcp.CallToolParams{
			Name: "audit_sql",
			Arguments: map[string]any{
				"sql": "create table app.query_access_only (id bigint)",
				"connection": map[string]any{
					"host":     "127.0.0.1",
					"port":     5500,
					"user":     "root",
					"password": "root",
					"database": "query_access_e2e",
					"schema":   "app",
					"dialect":  "postgresql",
				},
			},
		})
		if err != nil {
			t.Fatalf("call audit_sql over stdio: %v", err)
		}
		if result == nil || result.IsError {
			t.Fatalf("expected successful tool result, got %#v", result)
		}

		body := result.StructuredContent.(map[string]any)
		assertMCPFindingPresent(t, body, "ddl.table.exists.create.forbid")
	})

	// Case 4: table existence check
	t.Run("table_exists", func(t *testing.T) {
		cmd := createMCPServerCommandPG(t)

		client := sdkmcp.NewClient(&sdkmcp.Implementation{Name: "test-client", Version: "v0.0.1"}, nil)
		session, err := client.Connect(ctx, &sdkmcp.CommandTransport{Command: cmd}, nil)
		if err != nil {
			t.Fatalf("connect stdio mcp server: %v", err)
		}
		t.Cleanup(func() { _ = session.Close() })

		result, err := session.CallTool(ctx, &sdkmcp.CallToolParams{
			Name: "audit_sql",
			Arguments: map[string]any{
				"sql": "create table app.users (id bigserial primary key)",
				"connection": map[string]any{
					"host":     "127.0.0.1",
					"port":     5500,
					"user":     "root",
					"password": "root",
					"dialect":  "postgresql",
				},
			},
		})
		if err != nil {
			t.Fatalf("call audit_sql over stdio: %v", err)
		}
		if result == nil || result.IsError {
			t.Fatalf("expected successful tool result, got %#v", result)
		}

		body := result.StructuredContent.(map[string]any)
		assertMCPFindingPresent(t, body, "ddl.table.exists.create.forbid")
	})

	// Case 5: DELETE with plan estimation
	t.Run("plan_delete", func(t *testing.T) {
		cmd := createMCPServerCommandPG(t)

		client := sdkmcp.NewClient(&sdkmcp.Implementation{Name: "test-client", Version: "v0.0.1"}, nil)
		session, err := client.Connect(ctx, &sdkmcp.CommandTransport{Command: cmd}, nil)
		if err != nil {
			t.Fatalf("connect stdio mcp server: %v", err)
		}
		t.Cleanup(func() { _ = session.Close() })

		result, err := session.CallTool(ctx, &sdkmcp.CallToolParams{
			Name: "audit_sql",
			Arguments: map[string]any{
				"sql": "delete from app.orders where user_id = 1",
				"connection": map[string]any{
					"host":     "127.0.0.1",
					"port":     5500,
					"user":     "root",
					"password": "root",
					"dialect":  "postgresql",
				},
			},
		})
		if err != nil {
			t.Fatalf("call audit_sql over stdio: %v", err)
		}
		if result == nil || result.IsError {
			t.Fatalf("expected successful tool result, got %#v", result)
		}

		body := result.StructuredContent.(map[string]any)
		assertMCPImpactSource(t, body, "plan")
	})

	// Case 6: UPDATE with plan estimation
	t.Run("plan_update", func(t *testing.T) {
		cmd := createMCPServerCommandPG(t)

		client := sdkmcp.NewClient(&sdkmcp.Implementation{Name: "test-client", Version: "v0.0.1"}, nil)
		session, err := client.Connect(ctx, &sdkmcp.CommandTransport{Command: cmd}, nil)
		if err != nil {
			t.Fatalf("connect stdio mcp server: %v", err)
		}
		t.Cleanup(func() { _ = session.Close() })

		result, err := session.CallTool(ctx, &sdkmcp.CallToolParams{
			Name: "audit_sql",
			Arguments: map[string]any{
				"sql": "update app.users set name = 'x' where id = 1",
				"connection": map[string]any{
					"host":     "127.0.0.1",
					"port":     5500,
					"user":     "root",
					"password": "root",
					"dialect":  "postgresql",
				},
			},
		})
		if err != nil {
			t.Fatalf("call audit_sql over stdio: %v", err)
		}
		if result == nil || result.IsError {
			t.Fatalf("expected successful tool result, got %#v", result)
		}

		body := result.StructuredContent.(map[string]any)
		assertMCPImpactSource(t, body, "plan")
	})

	// Case 7: DROP CONSTRAINT → primary key mapping
	t.Run("drop_pk", func(t *testing.T) {
		cmd := createMCPServerCommandPG(t)

		client := sdkmcp.NewClient(&sdkmcp.Implementation{Name: "test-client", Version: "v0.0.1"}, nil)
		session, err := client.Connect(ctx, &sdkmcp.CommandTransport{Command: cmd}, nil)
		if err != nil {
			t.Fatalf("connect stdio mcp server: %v", err)
		}
		t.Cleanup(func() { _ = session.Close() })

		result, err := session.CallTool(ctx, &sdkmcp.CallToolParams{
			Name: "audit_sql",
			Arguments: map[string]any{
				"sql": "alter table app.accounts drop constraint accounts_pkey",
				"connection": map[string]any{
					"host":     "127.0.0.1",
					"port":     5500,
					"user":     "root",
					"password": "root",
					"dialect":  "postgresql",
				},
			},
		})
		if err != nil {
			t.Fatalf("call audit_sql over stdio: %v", err)
		}
		if result == nil || result.IsError {
			t.Fatalf("expected successful tool result, got %#v", result)
		}

		body := result.StructuredContent.(map[string]any)
		assertMCPFindingPresent(t, body, "ddl.alter.drop_primary_key.forbid")
	})

	// Case 8: rename column — column does not exist
	t.Run("rename_col_missing", func(t *testing.T) {
		cmd := createMCPServerCommandPG(t)

		client := sdkmcp.NewClient(&sdkmcp.Implementation{Name: "test-client", Version: "v0.0.1"}, nil)
		session, err := client.Connect(ctx, &sdkmcp.CommandTransport{Command: cmd}, nil)
		if err != nil {
			t.Fatalf("connect stdio mcp server: %v", err)
		}
		t.Cleanup(func() { _ = session.Close() })

		result, err := session.CallTool(ctx, &sdkmcp.CallToolParams{
			Name: "audit_sql",
			Arguments: map[string]any{
				"sql": "alter table app.users rename column missing_col to email",
				"connection": map[string]any{
					"host":     "127.0.0.1",
					"port":     5500,
					"user":     "root",
					"password": "root",
					"dialect":  "postgresql",
				},
			},
		})
		if err != nil {
			t.Fatalf("call audit_sql over stdio: %v", err)
		}
		if result == nil || result.IsError {
			t.Fatalf("expected successful tool result, got %#v", result)
		}

		body := result.StructuredContent.(map[string]any)
		assertMCPFindingPresent(t, body, "ddl.alter.rename_column.exists.require")
	})

	// Case 9: rename index forbid for existing index
	t.Run("rename_idx_forbid", func(t *testing.T) {
		cmd := createMCPServerCommandPG(t)

		client := sdkmcp.NewClient(&sdkmcp.Implementation{Name: "test-client", Version: "v0.0.1"}, nil)
		session, err := client.Connect(ctx, &sdkmcp.CommandTransport{Command: cmd}, nil)
		if err != nil {
			t.Fatalf("connect stdio mcp server: %v", err)
		}
		t.Cleanup(func() { _ = session.Close() })

		result, err := session.CallTool(ctx, &sdkmcp.CallToolParams{
			Name: "audit_sql",
			Arguments: map[string]any{
				"sql":     "alter index idx_accounts_email rename to idx_new",
				"dialect": "postgresql",
				"connection": map[string]any{
					"host":     "127.0.0.1",
					"port":     5500,
					"user":     "root",
					"password": "root",
					"database": "postgres",
					"schema":   "app",
					"dialect":  "postgresql",
				},
			},
		})
		if err != nil {
			t.Fatalf("call audit_sql over stdio: %v", err)
		}
		if result == nil || result.IsError {
			t.Fatalf("expected successful tool result, got %#v", result)
		}

		body := result.StructuredContent.(map[string]any)
		assertMCPFindingPresent(t, body, "ddl.pg.alter_index.rename.notice")
	})

	// Case 10: drop column — column does not exist
	t.Run("drop_col_missing", func(t *testing.T) {
		cmd := createMCPServerCommandPG(t)

		client := sdkmcp.NewClient(&sdkmcp.Implementation{Name: "test-client", Version: "v0.0.1"}, nil)
		session, err := client.Connect(ctx, &sdkmcp.CommandTransport{Command: cmd}, nil)
		if err != nil {
			t.Fatalf("connect stdio mcp server: %v", err)
		}
		t.Cleanup(func() { _ = session.Close() })

		result, err := session.CallTool(ctx, &sdkmcp.CallToolParams{
			Name: "audit_sql",
			Arguments: map[string]any{
				"sql": "alter table app.users drop column missing_col",
				"connection": map[string]any{
					"host":     "127.0.0.1",
					"port":     5500,
					"user":     "root",
					"password": "root",
					"dialect":  "postgresql",
				},
			},
		})
		if err != nil {
			t.Fatalf("call audit_sql over stdio: %v", err)
		}
		if result == nil || result.IsError {
			t.Fatalf("expected successful tool result, got %#v", result)
		}

		body := result.StructuredContent.(map[string]any)
		assertMCPFindingPresent(t, body, "ddl.alter.drop_column.exists.require")
	})
}

func createMCPServerCommandPG(t *testing.T) *exec.Cmd {
	t.Helper()

	moduleRoot := findModuleRoot(t)
	outDir := t.TempDir()
	binaryPath := filepath.Join(outDir, "deltascope-mcp")

	buildCmd := exec.Command("go", "build", "-tags", "postgresql", "-o", binaryPath, "./cmd/deltascope-mcp")
	buildCmd.Dir = moduleRoot
	buildCmd.Env = append(os.Environ(), "CGO_ENABLED=1")
	output, err := buildCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("build pg-capable mcp binary: %v\n%s", err, string(output))
	}

	cmd := exec.Command(binaryPath)
	cmd.Env = os.Environ()
	return cmd
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

func assertMCPFindingPresent(t *testing.T, body map[string]any, ruleID string) {
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

func assertMCPImpactSource(t *testing.T, body map[string]any, expectedSource string) {
	t.Helper()

	statements, ok := body["statements"].([]any)
	if !ok {
		t.Fatalf("expected statements array, got %#v", body["statements"])
	}
	if len(statements) == 0 {
		t.Fatal("expected at least one statement")
	}
	statement, ok := statements[0].(map[string]any)
	if !ok {
		t.Fatalf("expected statement object, got %T", statements[0])
	}
	impact, ok := statement["impact"].(map[string]any)
	if !ok {
		t.Fatalf("expected impact object, got %#v", statement["impact"])
	}
	if got := impact["source"]; got != expectedSource {
		t.Fatalf("expected impact source %q, got %q", expectedSource, got)
	}
}
