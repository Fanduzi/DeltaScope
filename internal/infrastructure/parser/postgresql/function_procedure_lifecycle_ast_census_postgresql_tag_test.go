//go:build postgresql

package postgresql

import (
	"fmt"
	"testing"

	pg_query "github.com/pganalyze/pg_query_go/v6"
)

// TestFunctionProcedureASTCensus characterizes pg_query_go AST facts for
// CREATE FUNCTION, CREATE OR REPLACE FUNCTION, CREATE FUNCTION ... SECURITY DEFINER,
// CREATE PROCEDURE, DROP FUNCTION, DROP FUNCTION IF EXISTS, and DROP PROCEDURE.
func TestFunctionProcedureASTCensus(t *testing.T) {
	t.Parallel()
	cases := []struct {
		Name string
		SQL  string
	}{
		{Name: "create function basic", SQL: "CREATE FUNCTION add(a int, b int) RETURNS int LANGUAGE sql AS $$ SELECT a + b $$"},
		{Name: "create or replace function", SQL: "CREATE OR REPLACE FUNCTION add(a int, b int) RETURNS int LANGUAGE sql AS $$ SELECT a + b $$"},
		{Name: "create function security definer", SQL: "CREATE FUNCTION admin_task() RETURNS void LANGUAGE plpgsql SECURITY DEFINER AS $$ BEGIN NULL; END $$"},
		{Name: "create function security invoker", SQL: "CREATE FUNCTION safe_task() RETURNS void LANGUAGE plpgsql SECURITY INVOKER AS $$ BEGIN NULL; END $$"},
		{Name: "create procedure basic", SQL: "CREATE PROCEDURE reset_counter() LANGUAGE plpgsql AS $$ BEGIN NULL; END $$"},
		{Name: "drop function", SQL: "DROP FUNCTION add(int, int)"},
		{Name: "drop function if exists", SQL: "DROP FUNCTION IF EXISTS add(int, int)"},
		{Name: "drop function cascade", SQL: "DROP FUNCTION add(int, int) CASCADE"},
		{Name: "drop procedure", SQL: "DROP PROCEDURE reset_counter()"},
		{Name: "drop procedure if exists", SQL: "DROP PROCEDURE IF EXISTS reset_counter()"},
		{Name: "drop procedure cascade", SQL: "DROP PROCEDURE reset_counter() CASCADE"},
		{Name: "create function schema qualified", SQL: "CREATE FUNCTION api.get_user(uid int) RETURNS text LANGUAGE sql AS $$ SELECT 'user' $$"},
	}

	t.Log("")
	t.Log("=== Function/Procedure AST Census ===")
	t.Logf("%-45s | %-25s | %s", "Case", "Node Kind", "AST Facts")
	t.Log(string(make([]byte, 0, 120)))

	for _, tc := range cases {
		tree, err := pg_query.Parse(tc.SQL)
		if err != nil {
			t.Fatalf("%s: parse failed: %v", tc.Name, err)
		}
		if len(tree.Stmts) != 1 {
			t.Fatalf("%s: expected 1 statement, got %d", tc.Name, len(tree.Stmts))
		}

		node := tree.Stmts[0].Stmt
		assertFunctionASTFacts(t, tc.Name, node)
	}
}

func assertFunctionASTFacts(t *testing.T, name string, node *pg_query.Node) {
	t.Helper()

	switch n := node.GetNode().(type) {
	case *pg_query.Node_CreateFunctionStmt:
		stmt := n.CreateFunctionStmt
		isProcedure := stmt.GetIsProcedure()
		replace := stmt.GetReplace()
		funcName := firstStringFromNodes(stmt.GetFuncname())
		optionNames := defElemNames(stmt.GetOptions())
		security := defElemValue(stmt.GetOptions(), "security")

		t.Logf("%-45s | %-25s | name=%q is_procedure=%v replace=%v options=%v security=%q",
			name, "CreateFunctionStmt", funcName, isProcedure, replace, optionNames, security)

		if funcName == "" {
			t.Errorf("%s: expected non-empty function name", name)
		}

	case *pg_query.Node_DropStmt:
		stmt := n.DropStmt
		removeType := stmt.GetRemoveType().String()
		missingOk := stmt.GetMissingOk()
		cascade := stmt.GetBehavior() == pg_query.DropBehavior_DROP_CASCADE
		objName := dropFunctionName(stmt)

		t.Logf("%-45s | %-25s | remove_type=%s object=%q missing_ok=%v cascade=%v",
			name, "DropStmt", removeType, objName, missingOk, cascade)

		if objName == "" {
			t.Errorf("%s: expected non-empty object name", name)
		}

	default:
		t.Fatalf("%s: unexpected node type %T", name, node.GetNode())
	}
}

func dropFunctionName(stmt *pg_query.DropStmt) string {
	for _, obj := range stmt.GetObjects() {
		owa := obj.GetObjectWithArgs()
		if owa == nil {
			continue
		}
		for _, name := range owa.GetObjname() {
			if s := name.GetString_(); s != nil && s.GetSval() != "" {
				return s.GetSval()
			}
		}
	}
	return ""
}

func defElemValue(nodes []*pg_query.Node, targetName string) string {
	for _, n := range nodes {
		elem := n.GetDefElem()
		if elem == nil {
			continue
		}
		if elem.GetDefname() == targetName {
			if arg := elem.GetArg(); arg != nil {
				if s := arg.GetString_(); s != nil {
					return s.GetSval()
				}
				if b := arg.GetBoolean(); b != nil {
					return fmt.Sprintf("%v", b.GetBoolval())
				}
			}
			return "(present)"
		}
	}
	return ""
}
