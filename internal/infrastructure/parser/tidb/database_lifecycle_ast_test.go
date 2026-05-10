// Package tidbparser verifies TiDB-backed parser adapter behavior.
// input: SQL text covering database/schema lifecycle DDL AST facts
// output: characterization of TiDB parser AST for CREATE/DROP DATABASE|SCHEMA
// pos: infrastructure parser adapter AST characterization
// note: if this file changes, update this header and module README.md.
package tidbparser

import (
	"strings"
	"testing"

	"github.com/pingcap/tidb/pkg/parser/ast"
)

// TestDatabaseLifecycleASTCharacterization locks down TiDB parser AST facts for
// CREATE DATABASE, CREATE SCHEMA, DROP DATABASE, and DROP SCHEMA statements.
// This is a read-only characterization test — no production code is modified.
func TestDatabaseLifecycleASTCharacterization(t *testing.T) {
	t.Parallel()
	cases := []struct {
		Name string
		SQL  string
	}{
		{Name: "create database", SQL: "CREATE DATABASE app"},
		{Name: "create database if not exists", SQL: "CREATE DATABASE IF NOT EXISTS app"},
		{Name: "create schema", SQL: "CREATE SCHEMA app"},
		{Name: "create schema if not exists", SQL: "CREATE SCHEMA IF NOT EXISTS app"},
		{Name: "drop database", SQL: "DROP DATABASE app"},
		{Name: "drop database if exists", SQL: "DROP DATABASE IF EXISTS app"},
		{Name: "drop schema", SQL: "DROP SCHEMA app"},
		{Name: "drop schema if exists", SQL: "DROP SCHEMA IF EXISTS app"},
		{Name: "create database charset", SQL: "CREATE DATABASE app CHARACTER SET utf8mb4"},
		{Name: "create database collate", SQL: "CREATE DATABASE app COLLATE utf8mb4_bin"},
	}

	t.Log("")
	t.Log("=== TiDB/MySQL Database Lifecycle AST Characterization ===")
	t.Logf("%-36s | %-25s | %-6s | %-12s | %s",
		"Case", "AST Node Type", "Name", "IF Flag", "Options")
	t.Log(string(make([]byte, 0, 120)))

	for _, tc := range cases {
		node := parsedNode(t, tc.SQL)
		characterizeDatabaseASTNode(t, tc.Name, node)
	}
}

func characterizeDatabaseASTNode(t *testing.T, name string, node ast.StmtNode) {
	t.Helper()

	switch stmt := node.(type) {
	case *ast.CreateDatabaseStmt:
		dbName := stmt.Name.O
		ifNotExists := stmt.IfNotExists
		optionSummary := summarizeDatabaseOptions(stmt.Options)

		t.Logf("%-36s | %-25s | %-6s | if_not_exists=%v | %s",
			name, "CreateDatabaseStmt", dbName, ifNotExists, optionSummary)

		if dbName != "app" {
			t.Errorf("%s: expected database name %q, got %q", name, "app", dbName)
		}
		assertIfNotExists(t, name, stmt.IfNotExists)

		if len(stmt.Options) > 0 {
			for _, opt := range stmt.Options {
				t.Logf("  option: type=%d value=%q", opt.Tp, opt.Value)
			}
		}

	case *ast.DropDatabaseStmt:
		dbName := stmt.Name.O
		ifExists := stmt.IfExists

		t.Logf("%-36s | %-25s | %-6s | if_exists=%v | (none)",
			name, "DropDatabaseStmt", dbName, ifExists)

		if dbName != "app" {
			t.Errorf("%s: expected database name %q, got %q", name, "app", dbName)
		}
		assertIfExists(t, name, ifExists)

	default:
		t.Fatalf("%s: unexpected AST node type %T", name, node)
	}
}

// assertIfNotExists validates IF NOT EXISTS for CREATE statements.
func assertIfNotExists(t *testing.T, name string, val bool) {
	t.Helper()
	want := strings.Contains(name, "if not exists")
	if val != want {
		t.Errorf("%s: if_not_exists=%v, want %v", name, val, want)
	}
}

// assertIfExists validates IF EXISTS for DROP statements.
func assertIfExists(t *testing.T, name string, val bool) {
	t.Helper()
	want := strings.Contains(name, "if exists")
	if val != want {
		t.Errorf("%s: if_exists=%v, want %v", name, val, want)
	}
}

// summarizeDatabaseOptions returns a human-readable summary of DatabaseOption values.
func summarizeDatabaseOptions(opts []*ast.DatabaseOption) string {
	if len(opts) == 0 {
		return "(none)"
	}
	names := make([]string, 0, len(opts))
	for _, opt := range opts {
		switch opt.Tp {
		case ast.DatabaseOptionCharset:
			names = append(names, "charset="+opt.Value)
		case ast.DatabaseOptionCollate:
			names = append(names, "collate="+opt.Value)
		default:
			names = append(names, "unknown")
		}
	}
	return joinStrings(names, ", ")
}

func joinStrings(ss []string, sep string) string {
	result := ""
	for i, s := range ss {
		if i > 0 {
			result += sep
		}
		result += s
	}
	return result
}

// TestDatabaseLifecycleCreateSchemaIsAlias confirms that CREATE SCHEMA parses
// into the same AST node type (CreateDatabaseStmt) as CREATE DATABASE,
// which is the MySQL/TiDB synonym behavior.
func TestDatabaseLifecycleCreateSchemaIsAlias(t *testing.T) {
	t.Parallel()

	dbNode := parsedNode(t, "CREATE DATABASE app")
	schemaNode := parsedNode(t, "CREATE SCHEMA app")

	_, dbOk := dbNode.(*ast.CreateDatabaseStmt)
	_, schemaOk := schemaNode.(*ast.CreateDatabaseStmt)

	if !dbOk {
		t.Fatal("CREATE DATABASE must parse to CreateDatabaseStmt")
	}
	if !schemaOk {
		t.Fatal("CREATE SCHEMA must parse to CreateDatabaseStmt (MySQL synonym)")
	}

	t.Logf("CREATE DATABASE -> %T", dbNode)
	t.Logf("CREATE SCHEMA   -> %T", schemaNode)
}

// TestDatabaseLifecycleDropSchemaIsAlias confirms that DROP SCHEMA parses
// into the same AST node type (DropDatabaseStmt) as DROP DATABASE.
func TestDatabaseLifecycleDropSchemaIsAlias(t *testing.T) {
	t.Parallel()

	dbNode := parsedNode(t, "DROP DATABASE app")
	schemaNode := parsedNode(t, "DROP SCHEMA app")

	_, dbOk := dbNode.(*ast.DropDatabaseStmt)
	_, schemaOk := schemaNode.(*ast.DropDatabaseStmt)

	if !dbOk {
		t.Fatal("DROP DATABASE must parse to DropDatabaseStmt")
	}
	if !schemaOk {
		t.Fatal("DROP SCHEMA must parse to DropDatabaseStmt (MySQL synonym)")
	}

	t.Logf("DROP DATABASE -> %T", dbNode)
	t.Logf("DROP SCHEMA   -> %T", schemaNode)
}

// TestDatabaseLifecycleCharsetCollation confirms that TiDB parser preserves
// CHARACTER SET and COLLATE options on CREATE DATABASE as DatabaseOption nodes.
func TestDatabaseLifecycleCharsetCollation(t *testing.T) {
	t.Parallel()

	node := parsedNode(t, "CREATE DATABASE app CHARACTER SET utf8mb4 COLLATE utf8mb4_bin")
	stmt, ok := node.(*ast.CreateDatabaseStmt)
	if !ok {
		t.Fatalf("expected CreateDatabaseStmt, got %T", node)
	}

	if len(stmt.Options) < 2 {
		t.Fatalf("expected at least 2 options (charset+collate), got %d", len(stmt.Options))
	}

	foundCharset, foundCollate := false, false
	for _, opt := range stmt.Options {
		switch opt.Tp {
		case ast.DatabaseOptionCharset:
			if opt.Value != "utf8mb4" {
				t.Errorf("charset value: got %q, want %q", opt.Value, "utf8mb4")
			}
			foundCharset = true
			t.Logf("charset option: value=%q", opt.Value)
		case ast.DatabaseOptionCollate:
			if opt.Value != "utf8mb4_bin" {
				t.Errorf("collate value: got %q, want %q", opt.Value, "utf8mb4_bin")
			}
			foundCollate = true
			t.Logf("collate option: value=%q", opt.Value)
		default:
			t.Logf("unexpected option type: %d value=%q", opt.Tp, opt.Value)
		}
	}

	if !foundCharset {
		t.Error("expected charset option not found")
	}
	if !foundCollate {
		t.Error("expected collate option not found")
	}
}
