// Package mcpapi verifies MCP connection resolution and validation behavior.
// input: direct MCP connection parameters, named connection configs, and secret sources
// output: regression coverage for MCP connection normalization and safety rules
// pos: interface-layer tests for MCP metadata-aware connection handling
// note: if this file changes, update this header and module README.md.
package mcpapi

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveAuditConnectionRejectsMutuallyExclusiveConnectionModes(t *testing.T) {
	t.Parallel()

	_, err := ResolveAuditConnection(AuditSQLParams{
		ConnectionRef: "prod_readonly",
		Connection: &ConnectionInput{
			Host:     "127.0.0.1",
			User:     "root",
			Password: "secret",
		},
	}, ResolveConnectionOptions{})
	if err == nil {
		t.Fatal("expected mutually exclusive connection mode error")
	}
	if got := err.Error(); got != "connection_ref and connection are mutually exclusive" {
		t.Fatalf("unexpected error: %q", got)
	}
}

func TestResolveAuditConnectionRejectsMultiplePasswordSources(t *testing.T) {
	t.Parallel()

	_, err := ResolveAuditConnection(AuditSQLParams{
		Connection: &ConnectionInput{
			Host:        "127.0.0.1",
			User:        "root",
			Password:    "secret",
			PasswordEnv: "DB_PASSWORD",
		},
	}, ResolveConnectionOptions{})
	if err == nil {
		t.Fatal("expected multiple password source error")
	}
	if got := err.Error(); got != "connection password sources are mutually exclusive" {
		t.Fatalf("unexpected error: %q", got)
	}
}

func TestResolveAuditConnectionRejectsSocketWithHostOrPort(t *testing.T) {
	t.Parallel()

	_, err := ResolveAuditConnection(AuditSQLParams{
		Connection: &ConnectionInput{
			Host:   "127.0.0.1",
			Port:   3306,
			Socket: "/tmp/mysql.sock",
			User:   "root",
		},
	}, ResolveConnectionOptions{})
	if err == nil {
		t.Fatal("expected socket/tcp conflict error")
	}
	if got := err.Error(); got != "connection socket cannot be combined with host/port TCP options" {
		t.Fatalf("unexpected error: %q", got)
	}
}

func TestResolveAuditConnectionRejectsEmptyDirectConnection(t *testing.T) {
	t.Parallel()

	_, err := ResolveAuditConnection(AuditSQLParams{
		Connection: &ConnectionInput{},
	}, ResolveConnectionOptions{})
	if err == nil {
		t.Fatal("expected empty connection error")
	}
	if got := err.Error(); got != "connection must include at least one non-password field" {
		t.Fatalf("unexpected error: %q", got)
	}
}

func TestResolveAuditConnectionRejectsIncompleteDirectConnection(t *testing.T) {
	t.Parallel()

	_, err := ResolveAuditConnection(AuditSQLParams{
		Connection: &ConnectionInput{Schema: "app"},
	}, ResolveConnectionOptions{})
	if err == nil {
		t.Fatal("expected incomplete connection error")
	}
	if got := err.Error(); got != "connection must include host/user, socket/user, or connection_ref" {
		t.Fatalf("unexpected error: %q", got)
	}
}

func TestResolveAuditConnectionResolvesDirectPasswordEnv(t *testing.T) {
	t.Parallel()

	resolved, err := ResolveAuditConnection(AuditSQLParams{
		Connection: &ConnectionInput{
			Host:        "127.0.0.1",
			Port:        3307,
			User:        "root",
			Schema:      "app",
			Dialect:     "mysql",
			PasswordEnv: "DB_PASSWORD",
		},
	}, ResolveConnectionOptions{
		LookupEnv: func(key string) (string, bool) {
			if key == "DB_PASSWORD" {
				return "from-env", true
			}
			return "", false
		},
	})
	if err != nil {
		t.Fatalf("resolve connection: %v", err)
	}

	if resolved.Source != MetadataSourceDirect {
		t.Fatalf("expected direct source, got %q", resolved.Source)
	}
	if resolved.Password != "from-env" {
		t.Fatalf("expected resolved env password, got %q", resolved.Password)
	}
	if resolved.Host != "127.0.0.1" || resolved.Port != 3307 || resolved.User != "root" || resolved.Schema != "app" || resolved.Dialect != "mysql" {
		t.Fatalf("unexpected resolved connection: %#v", resolved)
	}
}

func TestResolveAuditConnectionLoadsNamedConnectionFromConfig(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "connections.yaml")
	if err := os.WriteFile(path, []byte(`
connections:
  prod_readonly:
    host: 10.0.0.12
    port: 3306
    user: audit_bot
    schema: app
    dialect: mysql
    password_env: PROD_DB_PASSWORD
`), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	resolved, err := ResolveAuditConnection(AuditSQLParams{
		ConnectionRef: "prod_readonly",
	}, ResolveConnectionOptions{
		ConnectionsPath: path,
		LookupEnv: func(key string) (string, bool) {
			if key == "PROD_DB_PASSWORD" {
				return "secret-from-env", true
			}
			return "", false
		},
	})
	if err != nil {
		t.Fatalf("resolve connection: %v", err)
	}

	if resolved.Source != MetadataSourceConnectionRef {
		t.Fatalf("expected connection_ref source, got %q", resolved.Source)
	}
	if resolved.Password != "secret-from-env" {
		t.Fatalf("expected password from env, got %q", resolved.Password)
	}
	if resolved.Host != "10.0.0.12" || resolved.Port != 3306 || resolved.User != "audit_bot" || resolved.Schema != "app" {
		t.Fatalf("unexpected resolved connection: %#v", resolved)
	}
}

func TestResolveAuditConnectionPreservesConnectTimeoutFromDirectInput(t *testing.T) {
	t.Parallel()

	resolved, err := ResolveAuditConnection(AuditSQLParams{
		Connection: &ConnectionInput{
			Host:           "127.0.0.1",
			Port:           3307,
			User:           "root",
			ConnectTimeout: "5s",
		},
	}, ResolveConnectionOptions{})
	if err != nil {
		t.Fatalf("resolve connection: %v", err)
	}
	if resolved.ConnectTimeout != "5s" {
		t.Fatalf("expected ConnectTimeout=5s, got %q", resolved.ConnectTimeout)
	}
}

func TestResolveAuditConnectionPreservesConnectTimeoutFromConfig(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "connections.yaml")
	if err := os.WriteFile(path, []byte(`
connections:
  prod_timeout:
    host: 10.0.0.12
    port: 3306
    user: audit_bot
    connect_timeout: 3s
`), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	resolved, err := ResolveAuditConnection(AuditSQLParams{
		ConnectionRef: "prod_timeout",
	}, ResolveConnectionOptions{
		ConnectionsPath: path,
	})
	if err != nil {
		t.Fatalf("resolve connection: %v", err)
	}
	if resolved.ConnectTimeout != "3s" {
		t.Fatalf("expected ConnectTimeout=3s from config, got %q", resolved.ConnectTimeout)
	}
}

func TestResolveAuditConnectionReturnsOfflineWhenNoConnectionProvided(t *testing.T) {
	t.Parallel()

	resolved, err := ResolveAuditConnection(AuditSQLParams{SQL: "delete from users"}, ResolveConnectionOptions{})
	if err != nil {
		t.Fatalf("resolve connection: %v", err)
	}
	if resolved.Enabled {
		t.Fatalf("expected offline resolution, got %#v", resolved)
	}
	if resolved.Source != MetadataSourceNone {
		t.Fatalf("expected none source, got %q", resolved.Source)
	}
}
