//go:build postgresql

package postgresql

import (
	"context"
	"fmt"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
	pg_query "github.com/pganalyze/pg_query_go/v6"
)

// TestPostgreSQLObjectLifecycleASTCensus characterizes pg_query_go AST facts for
// object lifecycle DDL candidates: SCHEMA, SEQUENCE, MATERIALIZED VIEW, and
// REFRESH MATERIALIZED VIEW. This is a read-only characterization test — no
// production code is modified.
func TestPostgreSQLObjectLifecycleASTCensus(t *testing.T) {
	t.Parallel()
	cases := []struct {
		Name string
		SQL  string
	}{
		{Name: "create schema", SQL: "CREATE SCHEMA staging"},
		{Name: "drop schema cascade", SQL: "DROP SCHEMA IF EXISTS staging CASCADE"},
		{Name: "create sequence basic", SQL: "CREATE SEQUENCE seq_order_id START WITH 1 INCREMENT BY 1"},
		{Name: "create sequence cycle", SQL: "CREATE SEQUENCE seq_order_id CYCLE"},
		{Name: "alter sequence restart", SQL: "ALTER SEQUENCE seq_order_id RESTART WITH 100"},
		{Name: "alter sequence cycle", SQL: "ALTER SEQUENCE seq_order_id CYCLE"},
		{Name: "drop sequence cascade", SQL: "DROP SEQUENCE IF EXISTS seq_order_id CASCADE"},
		{Name: "create materialized view", SQL: "CREATE MATERIALIZED VIEW mv_stats AS SELECT COUNT(*) FROM users"},
		{Name: "create materialized view no data", SQL: "CREATE MATERIALIZED VIEW mv_stats AS SELECT COUNT(*) FROM users WITH NO DATA"},
		{Name: "drop materialized view cascade", SQL: "DROP MATERIALIZED VIEW IF EXISTS mv_stats CASCADE"},
		{Name: "refresh materialized view", SQL: "REFRESH MATERIALIZED VIEW mv_stats"},
		{Name: "refresh materialized view concurrently", SQL: "REFRESH MATERIALIZED VIEW CONCURRENTLY mv_stats"},
	}

	t.Log("")
	t.Log("=== PostgreSQL Object Lifecycle AST Census ===")
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
		assertASTFacts(t, tc.Name, node)
	}
}

func assertASTFacts(t *testing.T, name string, node *pg_query.Node) {
	t.Helper()

	switch n := node.GetNode().(type) {
	case *pg_query.Node_CreateSchemaStmt:
		stmt := n.CreateSchemaStmt
		schemaName := stmt.GetSchemaname()
		ifNotExists := stmt.GetIfNotExists()
		t.Logf("%-42s | %-25s | schemaname=%q if_not_exists=%v", name, "CreateSchemaStmt", schemaName, ifNotExists)
		if schemaName == "" {
			t.Errorf("%s: expected non-empty schemaname", name)
		}
		if ifNotExists {
			t.Errorf("%s: expected if_not_exists=false for CREATE SCHEMA without IF NOT EXISTS", name)
		}

	case *pg_query.Node_DropStmt:
		stmt := n.DropStmt
		removeType := stmt.GetRemoveType().String()
		missingOk := stmt.GetMissingOk()
		cascade := stmt.GetBehavior() == pg_query.DropBehavior_DROP_CASCADE
		objName := dropObjectName(stmt)
		t.Logf("%-42s | %-25s | remove_type=%s object=%q missing_ok=%v cascade=%v",
			name, "DropStmt", removeType, objName, missingOk, cascade)
		if objName == "" {
			t.Errorf("%s: expected non-empty object name", name)
		}

	case *pg_query.Node_CreateSeqStmt:
		stmt := n.CreateSeqStmt
		seqName := rangeVarName(stmt.GetSequence())
		optNames := defElemNames(stmt.GetOptions())
		ifNotExists := stmt.GetIfNotExists()
		t.Logf("%-42s | %-25s | sequence=%q options=%v if_not_exists=%v",
			name, "CreateSeqStmt", seqName, optNames, ifNotExists)
		if seqName == "" {
			t.Errorf("%s: expected non-empty sequence name", name)
		}

	case *pg_query.Node_AlterSeqStmt:
		stmt := n.AlterSeqStmt
		seqName := rangeVarName(stmt.GetSequence())
		optNames := defElemNames(stmt.GetOptions())
		missingOk := stmt.GetMissingOk()
		t.Logf("%-42s | %-25s | sequence=%q options=%v missing_ok=%v",
			name, "AlterSeqStmt", seqName, optNames, missingOk)
		if seqName == "" {
			t.Errorf("%s: expected non-empty sequence name", name)
		}

	case *pg_query.Node_CreateTableAsStmt:
		stmt := n.CreateTableAsStmt
		objType := stmt.GetObjtype().String()
		into := stmt.GetInto()
		relName := ""
		skipData := false
		if into != nil {
			relName = rangeVarName(into.GetRel())
			skipData = into.GetSkipData()
		}
		t.Logf("%-42s | %-25s | objtype=%s relation=%q skip_data=%v",
			name, "CreateTableAsStmt", objType, relName, skipData)
		if relName == "" {
			t.Errorf("%s: expected non-empty relation name", name)
		}

	case *pg_query.Node_RefreshMatViewStmt:
		stmt := n.RefreshMatViewStmt
		relName := rangeVarName(stmt.GetRelation())
		concurrent := stmt.GetConcurrent()
		t.Logf("%-42s | %-25s | relation=%q concurrent=%v",
			name, "RefreshMatViewStmt", relName, concurrent)
		if relName == "" {
			t.Errorf("%s: expected non-empty relation name", name)
		}

	default:
		t.Fatalf("%s: unexpected node type %T", name, node.GetNode())
	}
}

// defElemNames extracts DefElem names from a list of option nodes.
func defElemNames(nodes []*pg_query.Node) []string {
	var names []string
	for _, n := range nodes {
		elem := n.GetDefElem()
		if elem != nil && elem.GetDefname() != "" {
			names = append(names, elem.GetDefname())
		}
	}
	return names
}

// dropObjectName extracts the target name from a DropStmt's objects list.
// It handles both nested-list format (tables, views, sequences, mat views)
// and direct-string format (schemas).
func dropObjectName(stmt *pg_query.DropStmt) string {
	if name := objectNameFromObjectName(stmt.GetObjects()); name != "" {
		return name
	}
	// Fallback: some drop types store the name directly as a String node
	// in a List (e.g., DROP SCHEMA), not in the nested list-of-lists format.
	for _, obj := range stmt.GetObjects() {
		if s := obj.GetString_(); s != nil && s.GetSval() != "" {
			return s.GetSval()
		}
		if list := obj.GetList(); list != nil {
			for _, item := range list.GetItems() {
				if s := item.GetString_(); s != nil && s.GetSval() != "" {
					return s.GetSval()
				}
			}
		}
	}
	return ""
}

// TestPostgreSQLObjectLifecycleClassification characterizes how the current
// DeltaScope pipeline classifies and extracts the object lifecycle DDL
// candidates. No production code is modified.
func TestPostgreSQLObjectLifecycleClassification(t *testing.T) {
	t.Parallel()
	cases := []struct {
		Name string
		SQL  string
	}{
		{Name: "create schema", SQL: "CREATE SCHEMA staging"},
		{Name: "drop schema cascade", SQL: "DROP SCHEMA IF EXISTS staging CASCADE"},
		{Name: "create sequence basic", SQL: "CREATE SEQUENCE seq_order_id START WITH 1 INCREMENT BY 1"},
		{Name: "create sequence cycle", SQL: "CREATE SEQUENCE seq_order_id CYCLE"},
		{Name: "alter sequence restart", SQL: "ALTER SEQUENCE seq_order_id RESTART WITH 100"},
		{Name: "alter sequence cycle", SQL: "ALTER SEQUENCE seq_order_id CYCLE"},
		{Name: "drop sequence cascade", SQL: "DROP SEQUENCE IF EXISTS seq_order_id CASCADE"},
		{Name: "create materialized view", SQL: "CREATE MATERIALIZED VIEW mv_stats AS SELECT COUNT(*) FROM users"},
		{Name: "create materialized view no data", SQL: "CREATE MATERIALIZED VIEW mv_stats AS SELECT COUNT(*) FROM users WITH NO DATA"},
		{Name: "drop materialized view cascade", SQL: "DROP MATERIALIZED VIEW IF EXISTS mv_stats CASCADE"},
		{Name: "refresh materialized view", SQL: "REFRESH MATERIALIZED VIEW mv_stats"},
		{Name: "refresh materialized view concurrently", SQL: "REFRESH MATERIALIZED VIEW CONCURRENTLY mv_stats"},
	}

	t.Log("")
	t.Log("=== PostgreSQL Object Lifecycle DeltaScope Classification ===")
	t.Logf("%-42s | %-6s | %-8s | %-12s | %s",
		"Case", "Parse?", "Kind", "Unsupported?", "Detail")
	t.Log(string(make([]byte, 0, 120)))

	for _, tc := range cases {
		p := New()
		result, parseErr := p.Parse(context.Background(), tc.SQL)
		if parseErr != nil {
			t.Logf("%-42s | %-6s | %-8s | %-12s | parse error: %v",
				tc.Name, "FAIL", "-", "-", parseErr)
			t.Errorf("%s: unexpected parse error: %v", tc.Name, parseErr)
			continue
		}
		if len(result.Statements) != 1 {
			t.Fatalf("%s: expected 1 statement, got %d", tc.Name, len(result.Statements))
		}

		es := result.Statements[0]
		stmt, extractErr := es.Extractor.Extract(spec.DialectPostgreSQL, es.RawSQL)
		if extractErr != nil {
			t.Logf("%-42s | %-6s | %-8s | %-12s | extract error: %v",
				tc.Name, "OK", es.Kind, "-", extractErr)
			t.Errorf("%s: unexpected extract error: %v", tc.Name, extractErr)
			continue
		}

		unsupported := "no"
		detail := ""
		if stmt.Unsupported != nil {
			unsupported = "yes"
			detail = fmt.Sprintf("%s: %s", stmt.Unsupported.Feature, stmt.Unsupported.Reason)
		} else if stmt.DDL != nil {
			detail = fmt.Sprintf("op=%s", stmt.DDL.Operation)
		}

		t.Logf("%-42s | %-6s | %-8s | %-12s | %s",
			tc.Name, "OK", stmt.Kind, unsupported, detail)

		// Assert broad current behavior: all candidates must parse successfully.
		if parseErr != nil {
			t.Errorf("%s: must parse successfully", tc.Name)
		}
	}
}
