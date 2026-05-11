//go:build postgresql

package postgresql

import (
	"testing"

	pg_query "github.com/pganalyze/pg_query_go/v6"
)

// TestPostgreSQLAdvancedViewLifecycleASTCensus characterizes pg_query_go AST
// facts for advanced view lifecycle DDL candidates: OR REPLACE, TEMP/TEMPORARY,
// CHECK OPTION, ALTER VIEW RENAME, ALTER VIEW SET SCHEMA, and DROP VIEW CASCADE.
func TestPostgreSQLAdvancedViewLifecycleASTCensus(t *testing.T) {
	t.Parallel()
	cases := []struct {
		Name string
		SQL  string
	}{
		{Name: "create view basic", SQL: "CREATE VIEW v AS SELECT 1"},
		{Name: "create or replace view", SQL: "CREATE OR REPLACE VIEW v AS SELECT 1"},
		{Name: "create temp view", SQL: "CREATE TEMP VIEW v AS SELECT 1"},
		{Name: "create temporary view", SQL: "CREATE TEMPORARY VIEW v AS SELECT 1"},
		{Name: "create view with check option", SQL: "CREATE VIEW v AS SELECT * FROM t WITH CHECK OPTION"},
		{Name: "create view with local check option", SQL: "CREATE VIEW v AS SELECT * FROM t WITH LOCAL CHECK OPTION"},
		{Name: "create view with cascaded check option", SQL: "CREATE VIEW v AS SELECT * FROM t WITH CASCADED CHECK OPTION"},
		{Name: "alter view rename", SQL: "ALTER VIEW v RENAME TO v2"},
		{Name: "alter view set schema", SQL: "ALTER VIEW v SET SCHEMA newschema"},
		{Name: "drop view cascade", SQL: "DROP VIEW v CASCADE"},
		{Name: "drop view if exists cascade", SQL: "DROP VIEW IF EXISTS v CASCADE"},
	}

	t.Log("")
	t.Log("=== PostgreSQL Advanced View Lifecycle AST Census ===")
	t.Logf("%-42s | %-25s | %s", "Case", "Node Kind", "AST Facts")
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
		switch n := node.GetNode().(type) {
		case *pg_query.Node_ViewStmt:
			stmt := n.ViewStmt
			replace := stmt.GetReplace()
			check := stmt.GetWithCheckOption()
			persistence := ""
			if stmt.GetView() != nil {
				persistence = stmt.GetView().GetRelpersistence()
			}
			t.Logf("%-42s | %-25s | replace=%v check_option=%s persistence=%q",
				tc.Name, "ViewStmt", replace, check, persistence)

		case *pg_query.Node_RenameStmt:
			stmt := n.RenameStmt
			renameType := stmt.GetRenameType().String()
			relName := ""
			if stmt.GetRelation() != nil {
				relName = stmt.GetRelation().GetRelname()
			}
			t.Logf("%-42s | %-25s | rename_type=%s relation=%q new_name=%q",
				tc.Name, "RenameStmt", renameType, relName, stmt.GetNewname())

		case *pg_query.Node_AlterObjectSchemaStmt:
			stmt := n.AlterObjectSchemaStmt
			objType := stmt.GetObjectType().String()
			newSchema := stmt.GetNewschema()
			t.Logf("%-42s | %-25s | object_type=%s new_schema=%q",
				tc.Name, "AlterObjectSchemaStmt", objType, newSchema)

		case *pg_query.Node_DropStmt:
			stmt := n.DropStmt
			removeType := stmt.GetRemoveType().String()
			cascade := stmt.GetBehavior() == pg_query.DropBehavior_DROP_CASCADE
			missingOk := stmt.GetMissingOk()
			t.Logf("%-42s | %-25s | remove_type=%s cascade=%v missing_ok=%v",
				tc.Name, "DropStmt", removeType, cascade, missingOk)

		default:
			t.Fatalf("%s: unexpected node type %T", tc.Name, node.GetNode())
		}
	}
}
