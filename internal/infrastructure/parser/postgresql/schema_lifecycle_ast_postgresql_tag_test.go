//go:build postgresql

// Package postgresql verifies pg_query parser adapter behavior.
// input: SQL text covering schema lifecycle DDL AST facts
// output: characterization of pg_query AST for CREATE SCHEMA variants
// pos: infrastructure parser adapter AST characterization
// note: if this file changes, update this header and module README.md.
package postgresql

import (
	"fmt"
	"testing"

	pg_query "github.com/pganalyze/pg_query_go/v6"
)

// TestSchemaLifecycleASTCharacterization locks down pg_query AST facts for
// CREATE SCHEMA statements. This is a read-only characterization test —
// no production code is modified.
func TestSchemaLifecycleASTCharacterization(t *testing.T) {
	t.Parallel()
	cases := []struct {
		Name        string
		SQL         string
		ParserError bool
	}{
		{Name: "create schema", SQL: "CREATE SCHEMA app"},
		{Name: "create schema if not exists", SQL: "CREATE SCHEMA IF NOT EXISTS app"},
		{Name: "create schema authorization", SQL: "CREATE SCHEMA AUTHORIZATION app_owner"},
		{Name: "create schema name authorization", SQL: "CREATE SCHEMA app AUTHORIZATION app_owner"},
		{Name: "create schema nested table", SQL: "CREATE SCHEMA app CREATE TABLE users (id bigint)"},
		// PostgreSQL rejects IF NOT EXISTS combined with schema elements:
		// "CREATE SCHEMA IF NOT EXISTS cannot include schema elements"
		{Name: "create schema if not exists nested table", SQL: "CREATE SCHEMA IF NOT EXISTS app CREATE TABLE users (id bigint)", ParserError: true},
	}

	t.Log("")
	t.Log("=== PostgreSQL Schema Lifecycle AST Characterization ===")
	t.Logf("%-44s | %-25s | %-6s | %-5s | %-12s | %s",
		"Case", "Node Type", "Name", "INE", "AuthRole", "SchemaElts")
	t.Log(string(make([]byte, 0, 140)))

	for _, tc := range cases {
		tree, err := pg_query.Parse(tc.SQL)
		if err != nil {
			if tc.ParserError {
				t.Logf("%-44s | %-25s | parser error: %v", tc.Name, "(parse error)", err)
				continue
			}
			t.Fatalf("%s: unexpected parse error: %v", tc.Name, err)
		}
		if tc.ParserError {
			t.Fatalf("%s: expected parser error but SQL parsed successfully", tc.Name)
		}
		if len(tree.Stmts) != 1 {
			t.Fatalf("%s: expected 1 statement, got %d", tc.Name, len(tree.Stmts))
		}

		node := tree.Stmts[0].Stmt
		characterizeSchemaASTNode(t, tc.Name, node)
	}
}

func characterizeSchemaASTNode(t *testing.T, name string, node *pg_query.Node) {
	t.Helper()

	switch n := node.GetNode().(type) {
	case *pg_query.Node_CreateSchemaStmt:
		stmt := n.CreateSchemaStmt
		schemaName := stmt.GetSchemaname()
		ifNotExists := stmt.GetIfNotExists()
		authRole := characterizeAuthRole(stmt.GetAuthrole())
		schemaElts := len(stmt.GetSchemaElts())

		t.Logf("%-44s | %-25s | %-6s | %-5v | %-12s | %d elements",
			name, "CreateSchemaStmt", schemaName, ifNotExists, authRole, schemaElts)

		switch name {
		case "create schema":
			assertSchemaName(t, name, schemaName, "app")
			assertIfNotExists(t, name, ifNotExists, false)
			assertNoAuthRole(t, name, stmt.GetAuthrole())
			assertNoSchemaElts(t, name, schemaElts)

		case "create schema if not exists":
			assertSchemaName(t, name, schemaName, "app")
			assertIfNotExists(t, name, ifNotExists, true)
			assertNoAuthRole(t, name, stmt.GetAuthrole())
			assertNoSchemaElts(t, name, schemaElts)

		case "create schema authorization":
			// AUTHORIZATION without name: schemaname is empty, role is the effective name
			assertSchemaName(t, name, schemaName, "")
			assertIfNotExists(t, name, ifNotExists, false)
			assertAuthRole(t, name, stmt.GetAuthrole(), "app_owner")
			assertNoSchemaElts(t, name, schemaElts)

		case "create schema name authorization":
			assertSchemaName(t, name, schemaName, "app")
			assertIfNotExists(t, name, ifNotExists, false)
			assertAuthRole(t, name, stmt.GetAuthrole(), "app_owner")
			assertNoSchemaElts(t, name, schemaElts)

		case "create schema nested table":
			assertSchemaName(t, name, schemaName, "app")
			assertIfNotExists(t, name, ifNotExists, false)
			assertNoAuthRole(t, name, stmt.GetAuthrole())
			assertSchemaElts(t, name, schemaElts, 1)
		}

	default:
		t.Fatalf("%s: unexpected node type %T", name, node.GetNode())
	}
}

func assertSchemaName(t *testing.T, name, got, want string) {
	t.Helper()
	if got != want {
		t.Errorf("%s: schemaname=%q, want %q", name, got, want)
	}
}

func assertIfNotExists(t *testing.T, name string, got, want bool) {
	t.Helper()
	if got != want {
		t.Errorf("%s: if_not_exists=%v, want %v", name, got, want)
	}
}

func assertNoAuthRole(t *testing.T, name string, role *pg_query.RoleSpec) {
	t.Helper()
	if role != nil {
		t.Errorf("%s: expected nil authrole, got rolename=%q", name, role.GetRolename())
	}
}

func assertAuthRole(t *testing.T, name string, role *pg_query.RoleSpec, wantName string) {
	t.Helper()
	if role == nil {
		t.Fatalf("%s: expected non-nil authrole", name)
	}
	if role.GetRolename() != wantName {
		t.Errorf("%s: authrole.rolename=%q, want %q", name, role.GetRolename(), wantName)
	}
	roletype := role.GetRoletype()
	t.Logf("%s: authrole.roletype=%v (%s)", name, roletype, roleSpecTypeName(roletype))
}

func assertNoSchemaElts(t *testing.T, name string, count int) {
	t.Helper()
	if count != 0 {
		t.Errorf("%s: expected 0 schema elements, got %d", name, count)
	}
}

func assertSchemaElts(t *testing.T, name string, got, want int) {
	t.Helper()
	if got != want {
		t.Errorf("%s: expected %d schema elements, got %d", name, want, got)
	}
}

func characterizeAuthRole(role *pg_query.RoleSpec) string {
	if role == nil {
		return "(none)"
	}
	return fmt.Sprintf("%s (%s)", role.GetRolename(), roleSpecTypeName(role.GetRoletype()))
}

func roleSpecTypeName(rt pg_query.RoleSpecType) string {
	switch rt {
	case pg_query.RoleSpecType_ROLESPEC_CSTRING:
		return "CSTRING"
	case pg_query.RoleSpecType_ROLESPEC_CURRENT_ROLE:
		return "CURRENT_ROLE"
	case pg_query.RoleSpecType_ROLESPEC_CURRENT_USER:
		return "CURRENT_USER"
	case pg_query.RoleSpecType_ROLESPEC_SESSION_USER:
		return "SESSION_USER"
	case pg_query.RoleSpecType_ROLESPEC_PUBLIC:
		return "PUBLIC"
	default:
		return fmt.Sprintf("UNKNOWN(%d)", rt)
	}
}

// TestSchemaLifecycleNormalizationBoundaries documents which CREATE SCHEMA
// forms are recommended for Task 3 normalization vs. deferred.
func TestSchemaLifecycleNormalizationBoundaries(t *testing.T) {
	t.Parallel()

	t.Log("")
	t.Log("=== PostgreSQL Schema Lifecycle Normalization Boundaries ===")

	// Recommended for Task 3: simple CREATE SCHEMA [IF NOT EXISTS] name
	supported := []struct {
		Name string
		SQL  string
	}{
		{Name: "create schema", SQL: "CREATE SCHEMA app"},
		{Name: "create schema if not exists", SQL: "CREATE SCHEMA IF NOT EXISTS app"},
	}

	t.Log("Supported for Task 3 normalization:")
	for _, tc := range supported {
		tree, err := pg_query.Parse(tc.SQL)
		if err != nil {
			t.Fatalf("%s: parse failed: %v", tc.Name, err)
		}
		stmt := tree.Stmts[0].Stmt.GetCreateSchemaStmt()
		if stmt == nil {
			t.Fatalf("%s: expected CreateSchemaStmt", tc.Name)
		}
		t.Logf("  %s: schemaname=%q if_not_exists=%v authrole=nil schema_elts=0",
			tc.Name, stmt.GetSchemaname(), stmt.GetIfNotExists())
		if stmt.GetAuthrole() != nil {
			t.Errorf("%s: expected nil authrole for supported form", tc.Name)
		}
		if len(stmt.GetSchemaElts()) != 0 {
			t.Errorf("%s: expected 0 schema elements for supported form", tc.Name)
		}
	}

	// Deferred: AUTHORIZATION and nested schema elements
	deferred := []struct {
		Name string
		SQL  string
	}{
		{Name: "create schema authorization", SQL: "CREATE SCHEMA AUTHORIZATION app_owner"},
		{Name: "create schema nested", SQL: "CREATE SCHEMA app CREATE TABLE users (id bigint)"},
	}

	t.Log("Deferred (AUTHORIZATION / nested objects):")
	for _, tc := range deferred {
		tree, err := pg_query.Parse(tc.SQL)
		if err != nil {
			t.Fatalf("%s: parse failed: %v", tc.Name, err)
		}
		stmt := tree.Stmts[0].Stmt.GetCreateSchemaStmt()
		if stmt == nil {
			t.Fatalf("%s: expected CreateSchemaStmt", tc.Name)
		}
		hasAuth := stmt.GetAuthrole() != nil
		hasElts := len(stmt.GetSchemaElts()) > 0
		t.Logf("  %s: schemaname=%q authrole=%v schema_elts=%d",
			tc.Name, stmt.GetSchemaname(), hasAuth, len(stmt.GetSchemaElts()))
		if !hasAuth && !hasElts {
			t.Errorf("%s: expected authrole or schema elements for deferred form", tc.Name)
		}
	}
}
