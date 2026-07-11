// Package tidbparser characterizes TiDB parser AST fields for SELECT and
// query-related statements. This is an architecture gate: it proves what the
// parser exposes before any production query-access extraction code is written.
//
// Classification legend:
//
//	approved      — read-only candidate; parser AST fields are sufficient for
//	                conservative relation/column extraction
//	not_read_only — recognized write, lock, session mutation, file output, or
//	                DDL/DML that makes the statement non-read-only
//	indeterminate — parser error, unknown function effect, or AST form that
//	                cannot be modeled conservatively
package tidbparser

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/pingcap/tidb/pkg/parser/ast"
)

// queryAccessTestCase defines one characterization test row.
type queryAccessTestCase struct {
	name                string
	sql                 string
	wantErr             bool   // true if parse should fail
	classify            string // approved | not_read_only | indeterminate
	wantNode            string // expected AST node type name (empty = don't check)
	wantZeroStmts       bool   // true if input should produce zero parsed statements
	expectFuncIndicator bool   // true if indeterminate case expects a function call indicator
	notes               string // human-readable evidence note
}

// TiDBQueryAccessCensus is the characterization matrix for TiDB SELECT-related
// AST forms. Each row documents what the parser exposes and how the form should
// be classified for query-access analysis.
var TiDBQueryAccessCensus = []queryAccessTestCase{
	// --- Simple SELECT ---
	{
		name:     "simple_select",
		sql:      "SELECT id, name FROM users",
		classify: "approved",
		wantNode: "*ast.SelectStmt",
		notes:    "Fields: FROM (*ast.TableRefsClause), Fields (*ast.FieldList), Where=nil, GroupBy=nil, Having=nil, OrderBy=nil, LockInfo=nil, SelectIntoOpt=nil, With=nil",
	},
	{
		name:     "select_with_aliases",
		sql:      "SELECT u.id, u.name FROM users u",
		classify: "approved",
		wantNode: "*ast.SelectStmt",
		notes:    "FROM contains TableSource with alias; column refs carry qualifier",
	},
	{
		name:     "schema_qualified",
		sql:      "SELECT id FROM app.users",
		classify: "approved",
		wantNode: "*ast.SelectStmt",
		notes:    "TableName.Schema='app', TableName.Name='users'",
	},
	// --- JOINs ---
	{
		name:     "inner_join",
		sql:      "SELECT u.id, o.total FROM users u INNER JOIN orders o ON u.id = o.user_id",
		classify: "approved",
		wantNode: "*ast.SelectStmt",
		notes:    "FROM contains *ast.Join with Tp=CrossJoin, On != nil",
	},
	{
		name:     "left_join",
		sql:      "SELECT u.id, o.total FROM users u LEFT JOIN orders o ON u.id = o.user_id",
		classify: "approved",
		wantNode: "*ast.SelectStmt",
		notes:    "Join.Tp indicates LEFT; On != nil",
	},
	{
		name:     "right_join",
		sql:      "SELECT u.id, o.total FROM users u RIGHT JOIN orders o ON u.id = o.user_id",
		classify: "approved",
		wantNode: "*ast.SelectStmt",
		notes:    "Join.Tp indicates RIGHT; On != nil",
	},
	{
		name:     "cross_join",
		sql:      "SELECT * FROM users CROSS JOIN orders",
		classify: "approved",
		wantNode: "*ast.SelectStmt",
		notes:    "Join.Tp=CrossJoin, On=nil",
	},
	{
		name:     "full_outer_join",
		sql:      "SELECT * FROM users FULL OUTER JOIN orders ON users.id = orders.user_id",
		wantErr:  true,
		classify: "indeterminate",
		notes:    "TiDB parser does not support FULL OUTER JOIN; parse error → indeterminate",
	},
	{
		name:     "using_join",
		sql:      "SELECT * FROM users JOIN orders USING (user_id)",
		classify: "approved",
		wantNode: "*ast.SelectStmt",
		notes:    "Join.Using contains column names; On=nil",
	},
	{
		name:     "natural_join",
		sql:      "SELECT * FROM users NATURAL JOIN orders",
		classify: "approved",
		wantNode: "*ast.SelectStmt",
		notes:    "Join.NaturalJoin=true; implicit column matching",
	},
	// --- WHERE predicates ---
	{
		name:     "where_literal_equality",
		sql:      "SELECT id FROM users WHERE id = 1",
		classify: "approved",
		wantNode: "*ast.SelectStmt",
		notes:    "Where != nil; BinaryOperationExpr with opcode.EQ",
	},
	{
		name:     "where_comparison",
		sql:      "SELECT id FROM users WHERE salary > 100000",
		classify: "approved",
		wantNode: "*ast.SelectStmt",
		notes:    "Where != nil; BinaryOperationExpr with opcode.GT",
	},
	{
		name:     "where_exists_subquery",
		sql:      "SELECT id FROM users WHERE EXISTS (SELECT 1 FROM orders WHERE orders.user_id = users.id)",
		classify: "approved",
		wantNode: "*ast.SelectStmt",
		notes:    "Where contains ExistsSubqueryExpr; inner SelectStmt is correlated",
	},
	// --- GROUP BY, HAVING, ORDER BY ---
	{
		name:                "group_by",
		sql:                 "SELECT dept, COUNT(*) FROM employees GROUP BY dept",
		classify:            "indeterminate",
		wantNode:            "*ast.SelectStmt",
		expectFuncIndicator: true,
		notes:               "V1 policy: COUNT(*) function → indeterminate under empty allowlist",
	},
	{
		name:                "having",
		sql:                 "SELECT dept, COUNT(*) c FROM employees GROUP BY dept HAVING c > 5",
		classify:            "indeterminate",
		wantNode:            "*ast.SelectStmt",
		expectFuncIndicator: true,
		notes:               "V1 policy: COUNT(*) function → indeterminate under empty allowlist",
	},
	{
		name:     "order_by",
		sql:      "SELECT id, name FROM users ORDER BY name ASC",
		classify: "approved",
		wantNode: "*ast.SelectStmt",
		notes:    "OrderBy != nil; OrderBy.Items[0].Expr is ColumnNameExpr",
	},
	// --- Window functions ---
	{
		name:                "window_function",
		sql:                 "SELECT ROW_NUMBER() OVER (PARTITION BY dept ORDER BY salary DESC) AS rn FROM employees",
		classify:            "indeterminate",
		wantNode:            "*ast.SelectStmt",
		expectFuncIndicator: true,
		notes:               "V1 policy: ROW_NUMBER() window function → indeterminate under empty allowlist",
	},
	// --- Subqueries ---
	{
		name:                "scalar_subquery",
		sql:                 "SELECT (SELECT COUNT(*) FROM orders WHERE orders.user_id = users.id) FROM users",
		classify:            "indeterminate",
		wantNode:            "*ast.SelectStmt",
		expectFuncIndicator: true,
		notes:               "V1 policy: COUNT(*) in subquery → indeterminate under empty allowlist",
	},
	{
		name:                "correlated_subquery_in_where",
		sql:                 "SELECT id FROM users WHERE salary > (SELECT AVG(salary) FROM employees WHERE employees.dept = users.dept)",
		classify:            "indeterminate",
		wantNode:            "*ast.SelectStmt",
		expectFuncIndicator: true,
		notes:               "V1 policy: AVG() function in subquery → indeterminate under empty allowlist",
	},
	{
		name:     "derived_table",
		sql:      "SELECT * FROM (SELECT id, name FROM users) AS sub",
		classify: "approved",
		wantNode: "*ast.SelectStmt",
		notes:    "FROM contains SubqueryExpr as TableSource",
	},
	// --- CTEs ---
	{
		name:     "simple_cte",
		sql:      "WITH cte AS (SELECT id FROM users) SELECT * FROM cte",
		classify: "approved",
		wantNode: "*ast.SelectStmt",
		notes:    "With != nil; With.CTEs contains CommonTableExpr with Select ast",
	},
	{
		name:     "recursive_cte",
		sql:      "WITH RECURSIVE cte AS (SELECT 1 AS n UNION ALL SELECT n + 1 FROM cte WHERE n < 10) SELECT * FROM cte",
		classify: "approved",
		wantNode: "*ast.SelectStmt",
		notes:    "With.IsRecursive=true; CTE body contains UNION ALL set operation",
	},
	// --- Set operations ---
	{
		name:     "union",
		sql:      "SELECT id FROM users UNION SELECT id FROM admins",
		classify: "approved",
		wantNode: "*ast.SetOprStmt",
		notes:    "TiDB parser returns SetOprStmt for UNION; contains SelectList with child SelectStmt nodes",
	},
	{
		name:     "union_all",
		sql:      "SELECT id FROM users UNION ALL SELECT id FROM admins",
		classify: "approved",
		wantNode: "*ast.SetOprStmt",
		notes:    "UNION ALL → SetOprStmt; preserves duplicates",
	},
	{
		name:     "intersect",
		sql:      "SELECT id FROM users INTERSECT SELECT id FROM admins",
		classify: "approved",
		wantNode: "*ast.SetOprStmt",
		notes:    "INTERSECT → SetOprStmt; TiDB parser accepts",
	},
	{
		name:     "except",
		sql:      "SELECT id FROM users EXCEPT SELECT id FROM admins",
		classify: "approved",
		wantNode: "*ast.SetOprStmt",
		notes:    "EXCEPT → SetOprStmt; TiDB parser accepts",
	},
	// --- Wildcards ---
	{
		name:     "select_star",
		sql:      "SELECT * FROM users",
		classify: "approved",
		wantNode: "*ast.SelectStmt",
		notes:    "Fields contains wildcard column; requires metadata expansion",
	},
	{
		name:     "qualified_wildcard",
		sql:      "SELECT users.* FROM users",
		classify: "approved",
		wantNode: "*ast.SelectStmt",
		notes:    "Wildcard carries table qualifier; requires metadata expansion",
	},
	// --- Locking reads ---
	{
		name:     "for_update",
		sql:      "SELECT * FROM users FOR UPDATE",
		classify: "not_read_only",
		wantNode: "*ast.SelectStmt",
		notes:    "LockInfo != nil; LockTp=ForUpdate",
	},
	{
		name:     "for_share",
		sql:      "SELECT * FROM users FOR SHARE",
		classify: "not_read_only",
		wantNode: "*ast.SelectStmt",
		notes:    "LockInfo != nil; LockTp=ForShare",
	},
	{
		name:     "lock_in_share_mode",
		sql:      "SELECT * FROM users LOCK IN SHARE MODE",
		classify: "not_read_only",
		wantNode: "*ast.SelectStmt",
		notes:    "LockInfo != nil; LockTp=LockInShareMode",
	},
	// --- SELECT INTO ---
	{
		name:     "select_into_outfile",
		sql:      "SELECT * FROM users INTO OUTFILE '/tmp/out.csv'",
		classify: "not_read_only",
		wantNode: "*ast.SelectStmt",
		notes:    "SelectIntoOpt != nil; writes server-side file",
	},
	{
		name:     "select_into_dumpfile",
		sql:      "SELECT * FROM users INTO DUMPFILE '/tmp/out.bin'",
		wantErr:  true,
		classify: "indeterminate",
		notes:    "TiDB parser does not support INTO DUMPFILE; parse error → indeterminate",
	},
	{
		name:     "select_into_variable",
		sql:      "SELECT id INTO @var FROM users",
		wantErr:  true,
		classify: "indeterminate",
		notes:    "TiDB parser does not support SELECT INTO @var syntax; parse error → indeterminate",
	},
	// --- Functions ---
	{
		name:                "builtin_now",
		sql:                 "SELECT NOW()",
		classify:            "indeterminate",
		wantNode:            "*ast.SelectStmt",
		expectFuncIndicator: true,
		notes:               "V1 policy: empty known-pure allowlist; all function-bearing expressions → indeterminate",
	},
	{
		name:                "builtin_concat",
		sql:                 "SELECT CONCAT(a, b) FROM t",
		classify:            "indeterminate",
		wantNode:            "*ast.SelectStmt",
		expectFuncIndicator: true,
		notes:               "V1 policy: function call → indeterminate",
	},
	{
		name:                "unknown_function",
		sql:                 "SELECT unknown_func(id) FROM users",
		classify:            "indeterminate",
		wantNode:            "*ast.SelectStmt",
		expectFuncIndicator: true,
		notes:               "V1 policy: unknown function → indeterminate",
	},
	// --- Multi-statements ---
	{
		name:     "multi_select",
		sql:      "SELECT 1; SELECT 2;",
		classify: "approved",
		wantNode: "*ast.SelectStmt",
		notes:    "Parser returns 2 statements; both are SELECT → approved per-statement",
	},
	{
		name:     "mixed_dml_select",
		sql:      "SELECT 1; DELETE FROM users;",
		classify: "not_read_only",
		wantNode: "*ast.SelectStmt",
		notes:    "Multi-statement: second statement is DELETE → whole input not_read_only",
	},
	// --- Parser errors ---
	{
		name:     "parser_error_select_from",
		sql:      "SELECT FROM",
		wantErr:  true,
		classify: "indeterminate",
		notes:    "Parser error → indeterminate",
	},
	{
		name:     "parser_error_invalid",
		sql:      "INVALID SQL",
		wantErr:  true,
		classify: "indeterminate",
		notes:    "Parser error → indeterminate",
	},
	// --- EXPLAIN ---
	{
		name:     "explain_select",
		sql:      "EXPLAIN SELECT * FROM users",
		classify: "approved",
		wantNode: "*ast.ExplainStmt",
		notes:    "EXPLAIN is read-only; analyzes query plan without executing",
	},
	// --- VALUES ---
	{
		name:     "values_row",
		sql:      "VALUES ROW(1, 2), ROW(3, 4)",
		classify: "approved",
		wantNode: "*ast.SelectStmt",
		notes:    "VALUES is a table value constructor; read-only",
	},
	// --- DDL/DML reach audit as non-query ---
	{
		name:     "create_table_is_ddl",
		sql:      "CREATE TABLE t1 (id INT)",
		classify: "not_read_only",
		wantNode: "*ast.CreateTableStmt",
		notes:    "DDL → not_read_only; classified as KindDDL by audit classify()",
	},
	{
		name:     "insert_is_dml",
		sql:      "INSERT INTO users (name) VALUES ('alice')",
		classify: "not_read_only",
		wantNode: "*ast.InsertStmt",
		notes:    "DML → not_read_only; classified as KindDML by audit classify()",
	},
	{
		name:     "update_is_dml",
		sql:      "UPDATE users SET name = 'bob' WHERE id = 1",
		classify: "not_read_only",
		wantNode: "*ast.UpdateStmt",
		notes:    "DML → not_read_only; classified as KindDML by audit classify()",
	},
	{
		name:     "delete_is_dml",
		sql:      "DELETE FROM users WHERE id = 1",
		classify: "not_read_only",
		wantNode: "*ast.DeleteStmt",
		notes:    "DML → not_read_only; classified as KindDML by audit classify()",
	},
	// --- Zero-statement and nil-node invariants (P1-4) ---
	{
		name:          "empty_input",
		sql:           "",
		classify:      "indeterminate",
		wantZeroStmts: true,
		notes:         "Empty input → zero statements → indeterminate; fail-closed invariant",
	},
	{
		name:          "comment_only",
		sql:           "-- this is a comment",
		classify:      "indeterminate",
		wantZeroStmts: true,
		notes:         "Comment-only input → zero statements → indeterminate; fail-closed invariant",
	},
	{
		name:          "semicolon_only",
		sql:           ";;;",
		classify:      "indeterminate",
		wantZeroStmts: true,
		notes:         "Semicolons only → zero statements → indeterminate; fail-closed invariant",
	},
}

// TestQueryAccessASTCensus runs the full characterization matrix against the
// TiDB parser. Each test asserts classification-critical AST structural facts,
// not just that fields are accessible.
func TestQueryAccessASTCensus(t *testing.T) {
	t.Parallel()
	p := New()

	for _, tc := range TiDBQueryAccessCensus {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			result, err := p.Parse(context.Background(), tc.sql)

			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected parse error for %q, got nil", tc.sql)
				}
				if tc.classify != "indeterminate" {
					t.Fatalf("parser error test %q must be classified indeterminate, got %q", tc.name, tc.classify)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected parse error for %q: %v", tc.sql, err)
			}

			// P1-4: zero-statement invariant (explicit expectation or detected)
			if tc.wantZeroStmts || len(result.Statements) == 0 {
				if len(result.Statements) != 0 {
					t.Fatalf("wantZeroStmts=true for %q but got %d statements", tc.name, len(result.Statements))
				}
				if tc.classify != "indeterminate" {
					t.Fatalf("zero-statement input %q must be classified indeterminate, got %q", tc.name, tc.classify)
				}
				t.Logf("census[%s]: zero statements → indeterminate (fail-closed invariant)", tc.name)
				return
			}

			// Assert first statement matches expected node type (P1-1: multi-statement)
			for i, stmt := range result.Statements {
				nodeType := nodeTypeName(stmt)
				if tc.wantNode != "" && i == 0 && nodeType != tc.wantNode {
					t.Fatalf("statement %d: expected node type %q, got %q for %q", i, tc.wantNode, nodeType, tc.sql)
				}
			}

			// Assert classification-critical structural facts on EACH statement (P1-1)
			for i, stmt := range result.Statements {
				switch tc.classify {
				case "approved":
					assertApprovedFacts(t, tc.name, stmt)
				case "not_read_only":
					assertNotReadOnlyFacts(t, tc.name, stmt, tc.sql, len(result.Statements), i)
				case "indeterminate":
					assertIndeterminateFacts(t, tc.name, stmt, tc.sql, tc.expectFuncIndicator)
				}
			}

			// Assert multi-statement: all statements classified consistently (P1-1)
			if len(result.Statements) > 1 {
				assertMultiStatementFacts(t, tc.name, tc.classify, result.Statements, tc.sql)
			}
		})
	}
}

// assertApprovedFacts verifies that approved queries have nil locking and INTO fields.
func assertApprovedFacts(t *testing.T, name string, stmt ast.StmtNode) {
	t.Helper()
	if sel, ok := stmt.(*ast.SelectStmt); ok {
		if sel.LockInfo != nil {
			t.Errorf("approved %q: expected LockInfo == nil, got non-nil", name)
		}
		if sel.SelectIntoOpt != nil {
			t.Errorf("approved %q: expected SelectIntoOpt == nil, got non-nil", name)
		}
	}
}

// assertNotReadOnlyFacts verifies that not_read_only queries have decisive write indicators.
func assertNotReadOnlyFacts(t *testing.T, name string, stmt ast.StmtNode, sql string, stmtCount int, stmtIndex int) {
	t.Helper()
	switch s := stmt.(type) {
	case *ast.SelectStmt:
		hasLock := s.LockInfo != nil
		hasInto := s.SelectIntoOpt != nil
		if hasLock {
			t.Logf("census[%s]: stmt[%d] LockInfo.LockType=%d confirms not_read_only", name, stmtIndex, s.LockInfo.LockType)
		}
		if hasInto {
			t.Logf("census[%s]: stmt[%d] SelectIntoOpt != nil confirms not_read_only", name, stmtIndex)
		}
		caseName := name
		if stmtIndex > 0 {
			caseName = fmt.Sprintf("%s_stmt%d", name, stmtIndex)
		}
		if strings.Contains(caseName, "lock") || strings.Contains(caseName, "for_update") || strings.Contains(caseName, "for_share") {
			if !hasLock {
				t.Errorf("not_read_only %q[%d]: locking case requires LockInfo != nil", name, stmtIndex)
			}
		}
		if strings.Contains(caseName, "into") {
			if !hasInto {
				t.Errorf("not_read_only %q[%d]: INTO case requires SelectIntoOpt != nil", name, stmtIndex)
			}
		}
		if stmtCount == 1 && !hasLock && !hasInto {
			t.Errorf("not_read_only %q[%d]: single-statement SELECT has no LockInfo and no SelectIntoOpt", name, stmtIndex)
		}
	case *ast.ExplainStmt:
		if s.Analyze {
			t.Logf("census[%s]: stmt[%d] ExplainStmt.Analyze=true confirms not_read_only", name, stmtIndex)
		} else {
			t.Errorf("not_read_only %q[%d]: ExplainStmt expected Analyze=true, got false", name, stmtIndex)
		}
	}
}

// assertIndeterminateFacts verifies function-bearing nodes or parse-error classification.
func assertIndeterminateFacts(t *testing.T, name string, stmt ast.StmtNode, sql string, expectFunc bool) {
	t.Helper()
	if sel, ok := stmt.(*ast.SelectStmt); ok {
		if containsFunctionCall(sel) {
			t.Logf("census[%s]: FuncCallExpr discovered → indeterminate under empty allowlist", name)
		} else if expectFunc {
			t.Errorf("indeterminate %q: expected function call indicator, none found", name)
		}
	}
}

// assertMultiStatementFacts verifies all statements in multi-statement input.
func assertMultiStatementFacts(t *testing.T, name string, classify string, stmts []ast.StmtNode, sql string) {
	t.Helper()
	for i, stmt := range stmts {
		t.Logf("census[%s]: multi-statement[%d] node=%s", name, i, nodeTypeName(stmt))
	}

	switch classify {
	case "approved":
		for i, stmt := range stmts {
			if isWriteNode(stmt) {
				t.Errorf("multi-statement %q[%d]: expected all read-only, got write node %s", name, i, nodeTypeName(stmt))
			}
			if sel, ok := stmt.(*ast.SelectStmt); ok {
				if sel.LockInfo != nil {
					t.Errorf("multi-statement %q[%d]: approved but has LockInfo", name, i)
				}
				if sel.SelectIntoOpt != nil {
					t.Errorf("multi-statement %q[%d]: approved but has SelectIntoOpt", name, i)
				}
				if containsFunctionCall(sel) {
					t.Errorf("multi-statement %q[%d]: approved but has function call indicator", name, i)
				}
			}
		}
	case "not_read_only":
		hasWrite := false
		for _, s := range stmts {
			if isWriteNode(s) {
				hasWrite = true
				break
			}
		}
		if !hasWrite {
			hasLockOrInto := false
			for _, s := range stmts {
				if sel, ok := s.(*ast.SelectStmt); ok {
					if sel.LockInfo != nil || sel.SelectIntoOpt != nil {
						hasLockOrInto = true
						break
					}
				}
			}
			if !hasLockOrInto {
				t.Errorf("multi-statement %q: classified not_read_only but no write node, lock, or INTO indicator found", name)
			}
		}
	case "indeterminate":
		for i, stmt := range stmts {
			if sel, ok := stmt.(*ast.SelectStmt); ok {
				if containsFunctionCall(sel) {
					t.Logf("census[%s]: multi-statement[%d] has function call → indeterminate", name, i)
				}
			}
		}
	}
}

// containsFunctionCall walks the AST to find FuncCallExpr or WindowFuncExpr nodes.
func containsFunctionCall(node ast.Node) bool {
	found := false
	if node == nil {
		return false
	}
	node.Accept(&funcCallVisitor{found: &found})
	return found
}

type funcCallVisitor struct {
	found *bool
}

func (v *funcCallVisitor) Enter(in ast.Node) (ast.Node, bool) {
	if *v.found {
		return in, true
	}
	switch in.(type) {
	case *ast.FuncCallExpr, *ast.WindowFuncExpr, *ast.AggregateFuncExpr:
		*v.found = true
		return in, true
	}
	return in, false
}

func (v *funcCallVisitor) Leave(in ast.Node) (ast.Node, bool) {
	return in, true
}

// isWriteNode returns true if the statement is a DDL, DML, or write-indicator node.
func isWriteNode(stmt ast.StmtNode) bool {
	switch stmt.(type) {
	case *ast.InsertStmt, *ast.UpdateStmt, *ast.DeleteStmt,
		*ast.CreateTableStmt, *ast.AlterTableStmt, *ast.DropTableStmt,
		*ast.CreateIndexStmt, *ast.DropIndexStmt, *ast.TruncateTableStmt,
		*ast.CreateDatabaseStmt, *ast.DropDatabaseStmt:
		return true
	}
	return false
}

// TestQueryAccessClassificationConsistency proves that audit classify() returns
// KindUnknown for SELECT statements, confirming SELECT reaches audit with no
// applicable rules.
func TestQueryAccessClassificationConsistency(t *testing.T) {
	t.Parallel()
	p := New()

	selectSQL := "SELECT id, name FROM users WHERE id = 1"
	result, err := p.Parse(context.Background(), selectSQL)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	wrapped := WrapStatements(result.Statements, result.Warnings)
	if len(wrapped) != 1 {
		t.Fatalf("expected 1 wrapped statement, got %d", len(wrapped))
	}

	// SELECT must be KindUnknown in the current audit classify()
	if wrapped[0].Kind != "unknown" {
		t.Fatalf("expected audit classify() to return KindUnknown for SELECT, got %q", wrapped[0].Kind)
	}
}

// TestQueryAccessAuditRegression proves that existing DDL/DML classification
// is unchanged by the characterization tests.
func TestQueryAccessAuditRegression(t *testing.T) {
	t.Parallel()
	p := New()

	cases := []struct {
		name     string
		sql      string
		wantKind string
	}{
		{"create_table", "CREATE TABLE t1 (id INT)", "ddl"},
		{"insert", "INSERT INTO t1 (id) VALUES (1)", "dml"},
		{"update", "UPDATE t1 SET id = 2 WHERE id = 1", "dml"},
		{"delete", "DELETE FROM t1 WHERE id = 1", "dml"},
		{"alter_table", "ALTER TABLE t1 ADD COLUMN name VARCHAR(255)", "ddl"},
		{"drop_table", "DROP TABLE t1", "ddl"},
		{"truncate", "TRUNCATE TABLE t1", "ddl"},
		{"select", "SELECT * FROM t1", "unknown"},
		{"explain", "EXPLAIN SELECT * FROM t1", "unknown"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			result, err := p.Parse(context.Background(), tc.sql)
			if err != nil {
				t.Fatalf("parse %q: %v", tc.sql, err)
			}
			wrapped := WrapStatements(result.Statements, result.Warnings)
			if len(wrapped) != 1 {
				t.Fatalf("expected 1 statement, got %d", len(wrapped))
			}
			if string(wrapped[0].Kind) != tc.wantKind {
				t.Fatalf("expected kind %q for %q, got %q", tc.wantKind, tc.name, wrapped[0].Kind)
			}
		})
	}
}

// nodeTypeName returns the type name of an ast.StmtNode for assertion.
func nodeTypeName(stmt ast.StmtNode) string {
	if stmt == nil {
		return "<nil>"
	}
	switch stmt.(type) {
	case *ast.SelectStmt:
		return "*ast.SelectStmt"
	case *ast.CreateTableStmt:
		return "*ast.CreateTableStmt"
	case *ast.InsertStmt:
		return "*ast.InsertStmt"
	case *ast.UpdateStmt:
		return "*ast.UpdateStmt"
	case *ast.DeleteStmt:
		return "*ast.DeleteStmt"
	case *ast.ExplainStmt:
		return "*ast.ExplainStmt"
	case *ast.SetOprStmt:
		return "*ast.SetOprStmt"
	default:
		return fmt.Sprintf("%T", stmt)
	}
}
