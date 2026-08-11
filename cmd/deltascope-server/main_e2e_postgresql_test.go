//go:build e2e && postgresql

// Package main verifies Docker-backed HTTP metadata-aware end-to-end behavior for PostgreSQL.
// input: real PostgreSQL fixtures, named connection runtime config, and a PG-capable server binary
// output: end-to-end proof that deltascope-server serves metadata-aware PG audit results over HTTP
// pos: slower external e2e verification kept outside the default go test loop
// note: if this file changes, update this header and module README.md.
package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

const (
	pgHTTPAuditAppConnectionID     = "pg-audit-app"
	pgHTTPAuditArchiveConnectionID = "pg-audit-archive"
)

func TestRunServesMetadataAwareAuditOverRealPostgreSQL(t *testing.T) {
	ctx := context.Background()
	baseURL := startHTTPServerPG(t)

	// Case 1: qualified schema detection from SQL
	t.Run("qualified_schema", func(t *testing.T) {
		payload := map[string]any{
			"sql":           "delete from app.users where id = 1",
			"connection_id": pgHTTPAuditAppConnectionID,
		}
		body := postAuditRequest(t, ctx, baseURL, payload)

		contextValue := mustContext(t, body)
		if got := contextValue["mode"]; got != "metadata-aware" {
			t.Fatalf("unexpected audit mode: %#v", got)
		}
		if got := contextValue["dialect"]; got != "postgresql" {
			t.Fatalf("unexpected dialect: %#v", got)
		}
		if got := contextValue["schema"]; got != "app" {
			t.Fatalf("unexpected schema: %#v", got)
		}
		if got := contextValue["metadata_source"]; got != "registry" {
			t.Fatalf("unexpected metadata source: %#v", got)
		}
	})

	// Case 2: explicit schema from the named connection
	t.Run("explicit_schema", func(t *testing.T) {
		payload := map[string]any{
			"sql":           "delete from users where id = 1",
			"connection_id": pgHTTPAuditArchiveConnectionID,
		}
		body := postAuditRequest(t, ctx, baseURL, payload)

		contextValue := mustContext(t, body)
		if got := contextValue["schema"]; got != "archive" {
			t.Fatalf("unexpected schema: %#v", got)
		}
	})

	// Case 3: table existence check — create table that already exists
	t.Run("table_exists", func(t *testing.T) {
		payload := map[string]any{
			"sql":           "create table app.users (id bigserial primary key)",
			"connection_id": pgHTTPAuditAppConnectionID,
		}
		body := postAuditRequest(t, ctx, baseURL, payload)

		assertFindingPresent(t, body, "ddl.table.exists.create.forbid")
	})

	// Case 4: DELETE with plan estimation
	t.Run("plan_delete", func(t *testing.T) {
		payload := map[string]any{
			"sql":           "delete from app.orders where user_id = 1",
			"connection_id": pgHTTPAuditAppConnectionID,
		}
		body := postAuditRequest(t, ctx, baseURL, payload)

		assertStatementImpactSource(t, body, "plan")
	})

	// Case 5: UPDATE with plan estimation
	t.Run("plan_update", func(t *testing.T) {
		payload := map[string]any{
			"sql":           "update app.users set name = 'x' where id = 1",
			"connection_id": pgHTTPAuditAppConnectionID,
		}
		body := postAuditRequest(t, ctx, baseURL, payload)

		assertStatementImpactSource(t, body, "plan")
	})

	// Case 6: DROP CONSTRAINT → primary key mapping
	t.Run("drop_pk", func(t *testing.T) {
		payload := map[string]any{
			"sql":           "alter table app.accounts drop constraint accounts_pkey",
			"connection_id": pgHTTPAuditAppConnectionID,
		}
		body := postAuditRequest(t, ctx, baseURL, payload)

		assertFindingPresent(t, body, "ddl.alter.drop_primary_key.forbid")
	})

	// Case 7: rename column — column does not exist
	t.Run("rename_col_missing", func(t *testing.T) {
		payload := map[string]any{
			"sql":           "alter table app.users rename column missing_col to email",
			"connection_id": pgHTTPAuditAppConnectionID,
		}
		body := postAuditRequest(t, ctx, baseURL, payload)

		assertFindingPresent(t, body, "ddl.alter.rename_column.exists.require")
	})

	// Case 8: PostgreSQL ALTER INDEX ... RENAME maps to the PG-specific notice rule
	t.Run("rename_idx_notice", func(t *testing.T) {
		payload := map[string]any{
			"sql":           "alter index idx_accounts_email rename to idx_new",
			"connection_id": pgHTTPAuditAppConnectionID,
		}
		body := postAuditRequest(t, ctx, baseURL, payload)

		assertFindingPresent(t, body, "ddl.pg.alter_index.rename.notice")
	})

	// Case 9: drop column — column does not exist
	t.Run("drop_col_missing", func(t *testing.T) {
		payload := map[string]any{
			"sql":           "alter table app.users drop column missing_col",
			"connection_id": pgHTTPAuditAppConnectionID,
		}
		body := postAuditRequest(t, ctx, baseURL, payload)

		assertFindingPresent(t, body, "ddl.alter.drop_column.exists.require")
	})
}

func startHTTPServerPG(t *testing.T) string {
	t.Helper()

	t.Setenv("DELTASCOPE_PG_HTTP_PASSWORD", "root")
	config := fmt.Sprintf(`metadata:
  connections:
    - id: %s
      dialect: postgresql
      host: 127.0.0.1
      port: 5500
      user: root
      password_env: DELTASCOPE_PG_HTTP_PASSWORD
      database: postgres
      schema: app
      purposes: [audit]
    - id: %s
      dialect: postgresql
      host: 127.0.0.1
      port: 5500
      user: root
      password_env: DELTASCOPE_PG_HTTP_PASSWORD
      database: postgres
      schema: archive
      purposes: [audit]
`, pgHTTPAuditAppConnectionID, pgHTTPAuditArchiveConnectionID)

	configPath := writePGHTTPAuditTempFile(t, config)
	listenAddr := freeTCPAddr(t)
	cmd := createHTTPServerCommandPG(t, listenAddr, configPath)
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

func createHTTPServerCommandPG(t *testing.T, listenAddr, configPath string) *exec.Cmd {
	t.Helper()

	binaryPath := buildHTTPServerBinaryPG(t)
	cmd := exec.Command(binaryPath, "-listen", listenAddr, "-runtime-config", configPath)
	cmd.Env = os.Environ()
	return cmd
}

func buildHTTPServerBinaryPG(t *testing.T) string {
	t.Helper()

	moduleRoot := findModuleRoot(t)
	outDir := t.TempDir()
	binaryPath := filepath.Join(outDir, "deltascope-server")

	cmd := exec.Command("go", "build", "-tags", "postgresql", "-o", binaryPath, "./cmd/deltascope-server")
	cmd.Dir = moduleRoot
	cmd.Env = append(os.Environ(), "CGO_ENABLED=1")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("build pg-capable http server binary: %v\n%s", err, string(output))
	}
	return binaryPath
}

func writePGHTTPAuditTempFile(t *testing.T, content string) string {
	t.Helper()
	file, err := os.CreateTemp(t.TempDir(), "pg-http-audit-*.yaml")
	if err != nil {
		t.Fatalf("create runtime config: %v", err)
	}
	if _, err := file.WriteString(content); err != nil {
		file.Close()
		t.Fatalf("write runtime config: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("close runtime config: %v", err)
	}
	return file.Name()
}

func assertStatementImpactSource(t *testing.T, body map[string]any, expectedSource string) {
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
