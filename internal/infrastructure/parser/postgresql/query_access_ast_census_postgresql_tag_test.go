//go:build postgresql

// Package postgresql characterizes pg_query_go parser AST fields for SELECT and
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
package postgresql

import (
	"fmt"
	"testing"

	pg_query "github.com/pganalyze/pg_query_go/v6"
)

// pgQueryAccessTestCase defines one characterization test row.
type pgQueryAccessTestCase struct {
	name       string
	sql        string
	wantErr    bool   // true if parse should fail
	classify   string // approved | not_read_only | indeterminate
	wantNode      string // expected AST node type name (empty = don't check)
	wantZeroStmts     bool   // true if input should produce zero parsed statements
	expectFuncIndicator bool // true if indeterminate case expects a function call indicator
	notes             string // human-readable evidence note
}

// PGQueryAccessCensus is the characterization matrix for PostgreSQL SELECT-related
// AST forms. Each row documents what the parser exposes and how the form should
// be classified for query-access analysis.
var PGQueryAccessCensus = []pgQueryAccessTestCase{
	// --- Simple SELECT ---
	{
		name:     "simple_select",
		sql:      "SELECT id, name FROM users",
		classify: "approved",
		wantNode: "*pg_query.SelectStmt",
		notes:    "TargetList, FromClause, WhereClause=nil, GroupClause=nil, HavingClause=nil, SortClause=nil, LockingClause=nil, WithClause=nil",
	},
	{
		name:     "select_with_aliases",
		sql:      "SELECT u.id, u.name FROM users u",
		classify: "approved",
		wantNode: "*pg_query.SelectStmt",
		notes:    "FromClause contains RangeVar with alias; column refs carry qualifier",
	},
	{
		name:     "schema_qualified",
		sql:      "SELECT id FROM app.users",
		classify: "approved",
		wantNode: "*pg_query.SelectStmt",
		notes:    "RangeVar.Schemaname='app', RangeVar.Relname='users'",
	},
	// --- JOINs ---
	{
		name:     "inner_join",
		sql:      "SELECT u.id, o.total FROM users u INNER JOIN orders o ON u.id = o.user_id",
		classify: "indeterminate",
		wantNode: "*pg_query.SelectStmt",
		notes:    "V1 policy: ON clause = operator → indeterminate under operator-effect policy",
	},
	{
		name:     "left_join",
		sql:      "SELECT u.id, o.total FROM users u LEFT JOIN orders o ON u.id = o.user_id",
		classify: "indeterminate",
		wantNode: "*pg_query.SelectStmt",
		notes:    "V1 policy: ON clause = operator → indeterminate under operator-effect policy",
	},
	{
		name:     "right_join",
		sql:      "SELECT u.id, o.total FROM users u RIGHT JOIN orders o ON u.id = o.user_id",
		classify: "indeterminate",
		wantNode: "*pg_query.SelectStmt",
		notes:    "V1 policy: ON clause = operator → indeterminate under operator-effect policy",
	},
	{
		name:     "cross_join",
		sql:      "SELECT * FROM users CROSS JOIN orders",
		classify: "approved",
		wantNode: "*pg_query.SelectStmt",
		notes:    "JoinExpr.jointype=JOIN_INNER (CROSS maps to INNER with no quals); quals=nil",
	},
	{
		name:     "full_outer_join",
		sql:      "SELECT * FROM users FULL OUTER JOIN orders ON users.id = orders.user_id",
		classify: "indeterminate",
		wantNode: "*pg_query.SelectStmt",
		notes:    "V1 policy: ON clause = operator → indeterminate under operator-effect policy",
	},
	{
		name:     "using_join",
		sql:      "SELECT * FROM users JOIN orders USING (user_id)",
		classify: "approved",
		wantNode: "*pg_query.SelectStmt",
		notes:    "JoinExpr.jointype=JOIN_INNER; JoinExpr.usingClause contains column list",
	},
	{
		name:     "natural_join",
		sql:      "SELECT * FROM users NATURAL JOIN orders",
		classify: "approved",
		wantNode: "*pg_query.SelectStmt",
		notes:    "JoinExpr.isNatural=true; implicit column matching",
	},
	// --- LATERAL join ---
	{
		name:     "lateral_join",
		sql:      "SELECT * FROM users u, LATERAL (SELECT * FROM orders WHERE user_id = u.id) o",
		classify: "indeterminate",
		wantNode: "*pg_query.SelectStmt",
		notes:    "V1 policy: correlated = operator in LATERAL subquery → indeterminate under operator-effect policy",
	},
	// --- WHERE predicates ---
	{
		name:     "where_literal_equality",
		sql:      "SELECT id FROM users WHERE id = 1",
		classify: "indeterminate",
		wantNode: "*pg_query.SelectStmt",
		notes:    "V1 policy: WHERE = operator → indeterminate under operator-effect policy",
	},
	{
		name:     "where_comparison",
		sql:      "SELECT id FROM users WHERE salary > 100000",
		classify: "indeterminate",
		wantNode: "*pg_query.SelectStmt",
		notes:    "V1 policy: WHERE > operator → indeterminate under operator-effect policy",
	},
	{
		name:     "where_exists_subquery",
		sql:      "SELECT id FROM users WHERE EXISTS (SELECT 1 FROM orders WHERE orders.user_id = users.id)",
		classify: "indeterminate",
		wantNode: "*pg_query.SelectStmt",
		notes:    "V1 policy: inner = operator → indeterminate under operator-effect policy",
	},
	// --- GROUP BY, HAVING, ORDER BY ---
	{
		name:                "group_by",
		sql:                 "SELECT dept, COUNT(*) FROM employees GROUP BY dept",
		classify:            "indeterminate",
		wantNode:            "*pg_query.SelectStmt",
		expectFuncIndicator: true,
		notes:               "V1 policy: COUNT(*) function → indeterminate under empty allowlist",
	},
	{
		name:                "having",
		sql:                 "SELECT dept, COUNT(*) c FROM employees GROUP BY dept HAVING COUNT(*) > 5",
		classify:            "indeterminate",
		wantNode:            "*pg_query.SelectStmt",
		expectFuncIndicator: true,
		notes:               "V1 policy: COUNT(*) function and > operator → indeterminate under empty allowlist and operator-effect policy",
	},
	{
		name:     "order_by",
		sql:      "SELECT id, name FROM users ORDER BY name ASC",
		classify: "approved",
		wantNode: "*pg_query.SelectStmt",
		notes:    "SortClause != nil; SortBy node with column ref",
	},
	// --- Window functions ---
	{
		name:                "window_function",
		sql:                 "SELECT ROW_NUMBER() OVER (PARTITION BY dept ORDER BY salary DESC) AS rn FROM employees",
		classify:            "indeterminate",
		wantNode:            "*pg_query.SelectStmt",
		expectFuncIndicator: true,
		notes:               "V1 policy: ROW_NUMBER() window function → indeterminate under empty allowlist",
	},
	// --- Subqueries ---
	{
		name:                "scalar_subquery",
		sql:                 "SELECT (SELECT COUNT(*) FROM orders WHERE orders.user_id = users.id) FROM users",
		classify:            "indeterminate",
		wantNode:            "*pg_query.SelectStmt",
		expectFuncIndicator: true,
		notes:               "V1 policy: COUNT(*) in subquery and = operator → indeterminate under empty allowlist and operator-effect policy",
	},
	{
		name:                "correlated_subquery_in_where",
		sql:                 "SELECT id FROM users WHERE salary > (SELECT AVG(salary) FROM employees WHERE employees.dept = users.dept)",
		classify:            "indeterminate",
		wantNode:            "*pg_query.SelectStmt",
		expectFuncIndicator: true,
		notes:               "V1 policy: AVG() function in subquery and >, = operators → indeterminate under empty allowlist and operator-effect policy",
	},
	{
		name:     "derived_table",
		sql:      "SELECT * FROM (SELECT id, name FROM users) AS sub",
		classify: "approved",
		wantNode: "*pg_query.SelectStmt",
		notes:    "FromClause contains RangeSubselect with subquery",
	},
	// --- CTEs ---
	{
		name:     "simple_cte",
		sql:      "WITH cte AS (SELECT id FROM users) SELECT * FROM cte",
		classify: "approved",
		wantNode: "*pg_query.SelectStmt",
		notes:    "WithClause != nil; CommonTableExpr with CTEquery (SelectStmt)",
	},
	{
		name:     "recursive_cte",
		sql:      "WITH RECURSIVE cte AS (SELECT 1 AS n UNION ALL SELECT n + 1 FROM cte WHERE n < 10) SELECT * FROM cte",
		classify: "indeterminate",
		wantNode: "*pg_query.SelectStmt",
		notes:    "V1 policy: + and < operators in CTE body → indeterminate under operator-effect policy",
	},
	// --- Data-modifying CTE (PostgreSQL-specific) ---
	{
		name:     "data_modifying_cte",
		sql:      "WITH deleted AS (DELETE FROM users RETURNING id) SELECT id FROM deleted",
		classify: "not_read_only",
		wantNode: "*pg_query.SelectStmt",
		notes:    "WithClause contains CTE with DELETE statement; data-modifying CTE → not_read_only",
	},
	// --- Set operations ---
	{
		name:     "union",
		sql:      "SELECT id FROM users UNION SELECT id FROM admins",
		classify: "approved",
		wantNode: "*pg_query.SelectStmt",
		notes:    "SelectStmt with op=SETOP_UNION; larg/rarg contain child selects",
	},
	{
		name:     "union_all",
		sql:      "SELECT id FROM users UNION ALL SELECT id FROM admins",
		classify: "approved",
		wantNode: "*pg_query.SelectStmt",
		notes:    "SelectStmt with op=SETOP_UNION_ALL",
	},
	{
		name:     "intersect",
		sql:      "SELECT id FROM users INTERSECT SELECT id FROM admins",
		classify: "approved",
		wantNode: "*pg_query.SelectStmt",
		notes:    "SelectStmt with op=SETOP_INTERSECT",
	},
	{
		name:     "except",
		sql:      "SELECT id FROM users EXCEPT SELECT id FROM admins",
		classify: "approved",
		wantNode: "*pg_query.SelectStmt",
		notes:    "SelectStmt with op=SETOP_EXCEPT",
	},
	// --- Wildcards ---
	{
		name:     "select_star",
		sql:      "SELECT * FROM users",
		classify: "approved",
		wantNode: "*pg_query.SelectStmt",
		notes:    "TargetList contains ResTarget with A_Star; requires metadata expansion",
	},
	{
		name:     "qualified_wildcard",
		sql:      "SELECT users.* FROM users",
		classify: "approved",
		wantNode: "*pg_query.SelectStmt",
		notes:    "TargetList contains ResTarget with ColumnRef carrying table qualifier and A_Star",
	},
	// --- Locking reads ---
	{
		name:     "for_update",
		sql:      "SELECT * FROM users FOR UPDATE",
		classify: "not_read_only",
		wantNode: "*pg_query.SelectStmt",
		notes:    "LockingClause != nil; LockClauseItem with strength=LockForUpdate",
	},
	{
		name:     "for_share",
		sql:      "SELECT * FROM users FOR SHARE",
		classify: "not_read_only",
		wantNode: "*pg_query.SelectStmt",
		notes:    "LockingClause != nil; LockClauseItem with strength=LockForShare",
	},
	{
		name:     "for_no_key_update",
		sql:      "SELECT * FROM users FOR NO KEY UPDATE",
		classify: "not_read_only",
		wantNode: "*pg_query.SelectStmt",
		notes:    "LockingClause != nil; LockClauseItem with strength=LockForNoKeyUpdate",
	},
	{
		name:     "for_key_share",
		sql:      "SELECT * FROM users FOR KEY SHARE",
		classify: "not_read_only",
		wantNode: "*pg_query.SelectStmt",
		notes:    "LockingClause != nil; LockClauseItem with strength=LockForKeyShare",
	},
	// --- SELECT INTO (PostgreSQL-specific: creates table) ---
	{
		name:     "select_into",
		sql:      "SELECT id INTO archive_users FROM users",
		classify: "not_read_only",
		wantNode: "*pg_query.SelectStmt",
		notes:    "SelectStmt.intoClause != nil; creates a new table → not_read_only",
	},
	// --- Set-returning functions in FROM ---
	{
		name:                "generate_series",
		sql:                 "SELECT * FROM generate_series(1, 10)",
		classify:            "indeterminate",
		wantNode:            "*pg_query.SelectStmt",
		expectFuncIndicator: true,
		notes:               "V1 policy: empty known-pure allowlist; set-returning function → indeterminate",
	},
	// --- Functions ---
	{
		name:                "builtin_now",
		sql:                 "SELECT NOW()",
		classify:            "indeterminate",
		wantNode:            "*pg_query.SelectStmt",
		expectFuncIndicator: true,
		notes:               "V1 policy: empty known-pure allowlist; all function-bearing expressions → indeterminate",
	},
	{
		name:                "builtin_concat",
		sql:                 "SELECT CONCAT(a, b) FROM t",
		classify:            "indeterminate",
		wantNode:            "*pg_query.SelectStmt",
		expectFuncIndicator: true,
		notes:               "V1 policy: function call → indeterminate",
	},
	{
		name:                "unknown_function",
		sql:                 "SELECT unknown_func(id) FROM users",
		classify:            "indeterminate",
		wantNode:            "*pg_query.SelectStmt",
		expectFuncIndicator: true,
		notes:               "V1 policy: unknown function → indeterminate",
	},
	// --- Multi-statements ---
	{
		name:     "multi_select",
		sql:      "SELECT 1; SELECT 2;",
		classify: "approved",
		wantNode: "*pg_query.SelectStmt",
		notes:    "Parser returns 2 statements; both are SELECT → approved per-statement",
	},
	{
		name:     "mixed_dml_select",
		sql:      "SELECT 1; DELETE FROM users;",
		classify: "not_read_only",
		wantNode: "*pg_query.SelectStmt",
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
		wantNode: "*pg_query.ExplainStmt",
		notes:    "EXPLAIN is read-only; analyzes query plan without executing",
	},
	{
		name:     "explain_analyze",
		sql:      "EXPLAIN ANALYZE SELECT * FROM users",
		classify: "not_read_only",
		wantNode: "*pg_query.ExplainStmt",
		notes:    "EXPLAIN ANALYZE actually executes the query; not_read_only",
	},
	// --- VALUES ---
	{
		name:     "values_row",
		sql:      "VALUES (1, 2), (3, 4)",
		classify: "approved",
		wantNode: "*pg_query.SelectStmt",
		notes:    "VALUES is a table value constructor; read-only",
	},
	// --- DDL/DML reach audit as non-query ---
	{
		name:     "create_table_is_ddl",
		sql:      "CREATE TABLE t1 (id INT)",
		classify: "not_read_only",
		wantNode: "*pg_query.CreateStmt",
		notes:    "DDL → not_read_only; classified as KindDDL by audit classify()",
	},
	{
		name:     "insert_is_dml",
		sql:      "INSERT INTO users (name) VALUES ('alice')",
		classify: "not_read_only",
		wantNode: "*pg_query.InsertStmt",
		notes:    "DML → not_read_only; classified as KindDML by audit classify()",
	},
	{
		name:     "update_is_dml",
		sql:      "UPDATE users SET name = 'bob' WHERE id = 1",
		classify: "not_read_only",
		wantNode: "*pg_query.UpdateStmt",
		notes:    "DML → not_read_only; classified as KindDML by audit classify()",
	},
	{
		name:     "delete_is_dml",
		sql:      "DELETE FROM users WHERE id = 1",
		classify: "not_read_only",
		wantNode: "*pg_query.DeleteStmt",
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

// TestPGQueryAccessASTCensus runs the full characterization matrix against the
// PostgreSQL parser. Each test asserts classification-critical AST structural
// facts, not just that fields are accessible.
func TestPGQueryAccessASTCensus(t *testing.T) {
	t.Parallel()

	for _, tc := range PGQueryAccessCensus {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			result, err := pg_query.Parse(tc.sql)

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

			stmts := result.GetStmts()
			// P1-4: zero-statement invariant (explicit expectation or detected)
			if tc.wantZeroStmts || len(stmts) == 0 {
				if len(stmts) != 0 {
					t.Fatalf("wantZeroStmts=true for %q but got %d statements", tc.name, len(stmts))
				}
				if tc.classify != "indeterminate" {
					t.Fatalf("zero-statement input %q must be classified indeterminate, got %q", tc.name, tc.classify)
				}
				t.Logf("census[%s]: zero statements → indeterminate (fail-closed invariant)", tc.name)
				return
			}

			// Assert ALL statements match expected node type (P1-1: multi-statement)
			for i, rawStmt := range stmts {
				node := rawStmt.GetStmt()
				if node == nil {
					t.Fatalf("statement %d: stmt node is nil", i)
				}
				nodeType := pgNodeTypeName(node)
				if tc.wantNode != "" && i == 0 && nodeType != tc.wantNode {
					t.Fatalf("statement %d: expected node type %q, got %q for %q", i, tc.wantNode, nodeType, tc.sql)
				}
			}

			firstNode := stmts[0].GetStmt()

			// Assert classification-critical structural facts per classification
			switch tc.classify {
			case "approved":
				assertPGApprovedFacts(t, tc.name, firstNode)
			case "not_read_only":
				assertPGNotReadOnlyFacts(t, tc.name, firstNode, tc.sql, len(stmts))
			case "indeterminate":
				assertPGIndeterminateFacts(t, tc.name, firstNode, tc.sql, tc.expectFuncIndicator)
			}

			// Assert multi-statement: all statements classified consistently (P1-1)
			if len(stmts) > 1 {
				assertPGMultiStatementFacts(t, tc.name, tc.classify, stmts, tc.sql)
			}
		})
	}
}

// TestPGQueryAccessClassificationConsistency proves that the PostgreSQL audit
// classify() returns KindUnknown for SELECT statements, confirming SELECT
// reaches audit as an unsupported boundary.
func TestPGQueryAccessClassificationConsistency(t *testing.T) {
	t.Parallel()

	selectSQL := "SELECT id, name FROM users WHERE id = 1"
	parser := New()
	result, err := parser.Parse(t.Context(), selectSQL)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(result.Statements) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(result.Statements))
	}

	// SELECT must be KindUnknown in the current audit classify()
	if result.Statements[0].Kind != "unknown" {
		t.Fatalf("expected audit classify() to return KindUnknown for SELECT, got %q", result.Statements[0].Kind)
	}
}

// TestPGQueryAccessAuditRegression proves that existing DDL/DML classification
// is unchanged by the characterization tests.
func TestPGQueryAccessAuditRegression(t *testing.T) {
	t.Parallel()

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

	parser := New()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			result, err := parser.Parse(t.Context(), tc.sql)
			if err != nil {
				t.Fatalf("parse %q: %v", tc.sql, err)
			}
			if len(result.Statements) != 1 {
				t.Fatalf("expected 1 statement, got %d", len(result.Statements))
			}
			if string(result.Statements[0].Kind) != tc.wantKind {
				t.Fatalf("expected kind %q for %q, got %q", tc.wantKind, tc.name, result.Statements[0].Kind)
			}
		})
	}
}

// pgNodeTypeName returns the type name of a pg_query.Node for assertion.
func pgNodeTypeName(node *pg_query.Node) string {
	if node == nil {
		return "<nil>"
	}
	switch node.GetNode().(type) {
	case *pg_query.Node_SelectStmt:
		return "*pg_query.SelectStmt"
	case *pg_query.Node_CreateStmt:
		return "*pg_query.CreateStmt"
	case *pg_query.Node_InsertStmt:
		return "*pg_query.InsertStmt"
	case *pg_query.Node_UpdateStmt:
		return "*pg_query.UpdateStmt"
	case *pg_query.Node_DeleteStmt:
		return "*pg_query.DeleteStmt"
	case *pg_query.Node_ExplainStmt:
		return "*pg_query.ExplainStmt"
	default:
		return fmt.Sprintf("%T", node.GetNode())
	}
}

func assertPGApprovedFacts(t *testing.T, name string, node *pg_query.Node) {
	t.Helper()
	if sel := node.GetSelectStmt(); sel != nil {
		if len(sel.GetLockingClause()) > 0 {
			t.Errorf("approved %q: expected LockingClause empty, got %d items", name, len(sel.GetLockingClause()))
		}
		if sel.GetIntoClause() != nil {
			t.Errorf("approved %q: expected IntoClause == nil, got non-nil", name)
		}
	}
}

func assertPGNotReadOnlyFacts(t *testing.T, name string, node *pg_query.Node, sql string, stmtCount int) {
	t.Helper()
	switch n := node.GetNode().(type) {
	case *pg_query.Node_SelectStmt:
		sel := n.SelectStmt
		hasLock := len(sel.GetLockingClause()) > 0
		hasInto := sel.GetIntoClause() != nil
		hasDataModCTE := pgSelectHasDataModifyingCTE(sel)
		if hasLock {
			t.Logf("census[%s]: LockingClause has %d items confirms not_read_only", name, len(sel.GetLockingClause()))
		}
		if hasInto {
			t.Logf("census[%s]: IntoClause != nil confirms not_read_only", name)
		}
		if hasDataModCTE {
			t.Logf("census[%s]: data-modifying CTE confirms not_read_only", name)
		}
		if stmtCount == 1 && !hasLock && !hasInto && !hasDataModCTE {
			t.Errorf("not_read_only %q: single-statement SELECT has no LockingClause, IntoClause, or data-modifying CTE", name)
		}
	case *pg_query.Node_ExplainStmt:
		hasAnalyze := false
		for _, opt := range n.ExplainStmt.GetOptions() {
			if defElem := opt.GetDefElem(); defElem != nil && defElem.GetDefname() == "analyze" {
				hasAnalyze = true
				t.Logf("census[%s]: ExplainStmt has analyze option confirms not_read_only", name)
			}
		}
		if !hasAnalyze {
			t.Errorf("not_read_only %q: ExplainStmt expected analyze option, none found", name)
		}
	}
}

func assertPGIndeterminateFacts(t *testing.T, name string, node *pg_query.Node, sql string, expectFunc bool) {
	t.Helper()
	if sel := node.GetSelectStmt(); sel != nil {
		if pgContainsFunctionCall(node) {
			t.Logf("census[%s]: function call discovered → indeterminate under empty allowlist", name)
		} else if expectFunc {
			t.Errorf("indeterminate %q: expected function call indicator, none found", name)
		}
	}
}

func assertPGMultiStatementFacts(t *testing.T, name string, classify string, stmts []*pg_query.RawStmt, sql string) {
	t.Helper()
	for i, rawStmt := range stmts {
		node := rawStmt.GetStmt()
		if node == nil {
			t.Errorf("multi-statement %q[%d]: stmt node is nil", name, i)
			continue
		}
		t.Logf("census[%s]: multi-statement[%d] node=%s", name, i, pgNodeTypeName(node))
	}

	switch classify {
	case "approved":
		for i, rawStmt := range stmts {
			node := rawStmt.GetStmt()
			if node != nil && pgIsWriteNode(node) {
				t.Errorf("multi-statement %q[%d]: expected all read-only, got write node %s", name, i, pgNodeTypeName(node))
			}
		}
	case "not_read_only":
		hasWrite := false
		for _, rawStmt := range stmts {
			node := rawStmt.GetStmt()
			if node != nil && pgIsWriteNode(node) {
				hasWrite = true
				break
			}
		}
		if !hasWrite {
			t.Errorf("multi-statement %q: classified not_read_only but no write node found", name)
		}
	}
}

func pgSelectHasDataModifyingCTE(sel *pg_query.SelectStmt) bool {
	with := sel.GetWithClause()
	if with == nil {
		return false
	}
	for _, cteNode := range with.GetCtes() {
		if cteNode == nil {
			continue
		}
		cte := cteNode.GetCommonTableExpr()
		if cte == nil {
			continue
		}
		cteQuery := cte.GetCtequery()
		if cteQuery == nil {
			continue
		}
		switch cteQuery.GetNode().(type) {
		case *pg_query.Node_InsertStmt, *pg_query.Node_UpdateStmt, *pg_query.Node_DeleteStmt:
			return true
		}
	}
	return false
}

func pgContainsFunctionCall(node *pg_query.Node) bool {
	if node == nil {
		return false
	}
	switch n := node.GetNode().(type) {
	case *pg_query.Node_FuncCall:
		return true
	case *pg_query.Node_SelectStmt:
		sel := n.SelectStmt
		for _, target := range sel.GetTargetList() {
			if target != nil && pgContainsFunctionCall(target) {
				return true
			}
		}
		if sel.GetWhereClause() != nil && pgContainsFunctionCall(sel.GetWhereClause()) {
			return true
		}
		for _, from := range sel.GetFromClause() {
			if pgContainsFunctionCall(from) {
				return true
			}
		}
	case *pg_query.Node_ResTarget:
		if n.ResTarget.GetVal() != nil && pgContainsFunctionCall(n.ResTarget.GetVal()) {
			return true
		}
	case *pg_query.Node_AExpr:
		if n.AExpr.GetLexpr() != nil && pgContainsFunctionCall(n.AExpr.GetLexpr()) {
			return true
		}
		if n.AExpr.GetRexpr() != nil && pgContainsFunctionCall(n.AExpr.GetRexpr()) {
			return true
		}
	case *pg_query.Node_RangeFunction:
		return true
	case *pg_query.Node_SubLink:
		if n.SubLink.GetSubselect() != nil {
			return pgContainsFunctionCall(n.SubLink.GetSubselect())
		}
	}
	return false
}

func pgIsWriteNode(node *pg_query.Node) bool {
	if node == nil {
		return false
	}
	switch node.GetNode().(type) {
	case *pg_query.Node_InsertStmt, *pg_query.Node_UpdateStmt, *pg_query.Node_DeleteStmt,
		*pg_query.Node_CreateStmt, *pg_query.Node_AlterTableStmt, *pg_query.Node_DropStmt,
		*pg_query.Node_IndexStmt, *pg_query.Node_TruncateStmt, *pg_query.Node_ViewStmt:
		return true
	}
	return false
}
