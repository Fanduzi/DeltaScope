//go:build postgresql

package postgresql

import (
	"context"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
	pg_query "github.com/pganalyze/pg_query_go/v6"
)

func extractPostgreSQLStatement(t *testing.T, sql string) spec.Statement {
	t.Helper()

	parser := New()
	result, err := parser.Parse(context.Background(), sql)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(result.Statements) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(result.Statements))
	}

	statement, err := result.Statements[0].Extractor.Extract(spec.DialectPostgreSQL, result.Statements[0].RawSQL)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	return statement
}

// parseCreateStmtAST parses SQL and returns the raw *pg_query.CreateStmt node
// for direct AST inspection. Used by characterization tests.
func parseCreateStmtAST(t *testing.T, sql string) *pg_query.CreateStmt {
	t.Helper()
	result, err := pg_query.Parse(sql)
	if err != nil {
		t.Fatalf("pg_query.Parse: %v", err)
	}
	stmts := result.GetStmts()
	if len(stmts) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(stmts))
	}
	node := stmts[0].GetStmt()
	if node == nil {
		t.Fatal("stmt node is nil")
	}
	createStmt, ok := node.GetNode().(*pg_query.Node_CreateStmt)
	if !ok {
		t.Fatalf("expected *Node_CreateStmt, got %T", node.GetNode())
	}
	return createStmt.CreateStmt
}

func parseAlterTableStmtAST(t *testing.T, sql string) *pg_query.AlterTableStmt {
	t.Helper()
	result, err := pg_query.Parse(sql)
	if err != nil {
		t.Fatalf("pg_query.Parse: %v", err)
	}
	stmts := result.GetStmts()
	if len(stmts) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(stmts))
	}
	node := stmts[0].GetStmt()
	if node == nil {
		t.Fatal("stmt node is nil")
	}
	alterStmt, ok := node.GetNode().(*pg_query.Node_AlterTableStmt)
	if !ok {
		t.Fatalf("expected *Node_AlterTableStmt, got %T", node.GetNode())
	}
	return alterStmt.AlterTableStmt
}
