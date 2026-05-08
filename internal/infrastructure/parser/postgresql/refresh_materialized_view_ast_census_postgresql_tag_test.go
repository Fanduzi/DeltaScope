//go:build postgresql

package postgresql

import (
	"context"
	"fmt"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
	pg_query "github.com/pganalyze/pg_query_go/v6"
)

// TestPostgreSQLRefreshMaterializedViewASTCensus characterizes the pg_query_go
// AST facts for REFRESH MATERIALIZED VIEW variants. This is a read-only
// characterization test — no production code is modified.
func TestPostgreSQLRefreshMaterializedViewASTCensus(t *testing.T) {
	cases := []struct {
		name string
		sql  string
	}{
		{name: "refresh_basic", sql: "REFRESH MATERIALIZED VIEW mv_stats"},
		{name: "refresh_concurrently", sql: "REFRESH MATERIALIZED VIEW CONCURRENTLY mv_stats"},
		{name: "refresh_with_data", sql: "REFRESH MATERIALIZED VIEW mv_stats WITH DATA"},
		{name: "refresh_with_no_data", sql: "REFRESH MATERIALIZED VIEW mv_stats WITH NO DATA"},
	}

	t.Log("")
	t.Log("=== PostgreSQL REFRESH MATERIALIZED VIEW AST Census ===")
	t.Logf("%-25s | %-20s | %-12s | %-12s | %-12s | %s",
		"Case", "Node Kind", "Relation", "Concurrent", "SkipData", "Notes")
	t.Log(string(make([]byte, 0, 140)))

	for _, tc := range cases {
		tree, err := pg_query.Parse(tc.sql)
		if err != nil {
			t.Fatalf("%s: parse failed: %v", tc.name, err)
		}
		if len(tree.Stmts) != 1 {
			t.Fatalf("%s: expected 1 statement, got %d", tc.name, len(tree.Stmts))
		}

		node := tree.Stmts[0].Stmt
		refreshMatViewAssertASTFacts(t, tc.name, node)
	}
}

func refreshMatViewAssertASTFacts(t *testing.T, name string, node *pg_query.Node) {
	t.Helper()

	n, ok := node.GetNode().(*pg_query.Node_RefreshMatViewStmt)
	if !ok {
		t.Fatalf("%s: expected RefreshMatViewStmt, got %T", name, node.GetNode())
	}

	stmt := n.RefreshMatViewStmt
	if stmt == nil {
		t.Fatalf("%s: RefreshMatViewStmt is nil", name)
	}

	relName := rangeVarName(stmt.GetRelation())
	concurrent := stmt.GetConcurrent()
	skipData := stmt.GetSkipData()

	notes := ""
	if name == "refresh_with_data" && !skipData {
		notes = "WITH DATA == implicit default (skip_data=false)"
	}
	if name == "refresh_with_no_data" && skipData {
		notes = "WITH NO DATA sets skip_data=true"
	}

	t.Logf("%-25s | %-20s | %-12s | %-12v | %-12v | %s",
		name, "RefreshMatViewStmt", relName, concurrent, skipData, notes)

	// Stable assertions.
	if relName != "mv_stats" {
		t.Errorf("%s: expected relation name %q, got %q", name, "mv_stats", relName)
	}
	if relName == "" {
		t.Errorf("%s: expected non-empty relation name", name)
	}

	switch name {
	case "refresh_basic":
		if concurrent {
			t.Errorf("%s: expected concurrent=false, got true", name)
		}
		if skipData {
			t.Errorf("%s: expected skip_data=false, got true", name)
		}
	case "refresh_concurrently":
		if !concurrent {
			t.Errorf("%s: expected concurrent=true, got false", name)
		}
		if skipData {
			t.Errorf("%s: expected skip_data=false, got true", name)
		}
	case "refresh_with_data":
		if concurrent {
			t.Errorf("%s: expected concurrent=false, got true", name)
		}
		if skipData {
			t.Errorf("%s: expected skip_data=false, got true (WITH DATA should not set skip_data)", name)
		}
	case "refresh_with_no_data":
		if concurrent {
			t.Errorf("%s: expected concurrent=false, got true", name)
		}
		if !skipData {
			t.Errorf("%s: expected skip_data=true, got false", name)
		}
	}
}

// TestPostgreSQLRefreshMaterializedViewCurrentExtractionBaseline proves the
// DeltaScope parser/extractor normalizes all REFRESH MATERIALIZED VIEW
// variants into DDL with the refresh_materialized_view operation.
func TestPostgreSQLRefreshMaterializedViewCurrentExtractionBaseline(t *testing.T) {
	cases := []struct {
		name         string
		sql          string
		concurrently string
		withNoData   string
	}{
		{name: "refresh_basic", sql: "REFRESH MATERIALIZED VIEW mv_stats", concurrently: "false", withNoData: "false"},
		{name: "refresh_concurrently", sql: "REFRESH MATERIALIZED VIEW CONCURRENTLY mv_stats", concurrently: "true", withNoData: "false"},
		{name: "refresh_with_data", sql: "REFRESH MATERIALIZED VIEW mv_stats WITH DATA", concurrently: "false", withNoData: "false"},
		{name: "refresh_with_no_data", sql: "REFRESH MATERIALIZED VIEW mv_stats WITH NO DATA", concurrently: "false", withNoData: "true"},
	}

	t.Log("")
	t.Log("=== PostgreSQL REFRESH MATERIALIZED VIEW Current Extraction Baseline ===")
	t.Logf("%-25s | %-8s | %-12s | %-30s | %s",
		"Case", "Kind", "Unsupported?", "Operation", "Options")
	t.Log(string(make([]byte, 0, 140)))

	for _, tc := range cases {
		p := New()
		result, parseErr := p.Parse(context.Background(), tc.sql)
		if parseErr != nil {
			t.Fatalf("%s: unexpected parse error: %v", tc.name, parseErr)
		}
		if len(result.Statements) != 1 {
			t.Fatalf("%s: expected 1 statement, got %d", tc.name, len(result.Statements))
		}

		es := result.Statements[0]
		stmt, extractErr := es.Extractor.Extract(spec.DialectPostgreSQL, es.RawSQL)
		if extractErr != nil {
			t.Fatalf("%s: unexpected extract error: %v", tc.name, extractErr)
		}

		unsupported := "no"
		detail := ""
		if stmt.Unsupported != nil {
			unsupported = "yes"
			detail = fmt.Sprintf("%s: %s", stmt.Unsupported.Feature, stmt.Unsupported.Reason)
		} else if stmt.DDL != nil {
			detail = string(stmt.DDL.Operation)
		}

		optsStr := ""
		if stmt.DDL != nil {
			optsStr = fmt.Sprintf("concurrently=%s with_no_data=%s", stmt.DDL.Options["concurrently"], stmt.DDL.Options["with_no_data"])
		}

		t.Logf("%-25s | %-8s | %-12s | %-30s | %s",
			tc.name, stmt.Kind, unsupported, detail, optsStr)

		// All refresh variants must be normalized as DDL.
		if stmt.Unsupported != nil {
			t.Fatalf("%s: expected normalized DDL, got unsupported: %s: %s", tc.name, stmt.Unsupported.Feature, stmt.Unsupported.Reason)
		}
		if stmt.Kind != spec.KindDDL {
			t.Errorf("%s: expected kind=%s, got %s", tc.name, spec.KindDDL, stmt.Kind)
		}
		if stmt.DDL == nil {
			t.Fatalf("%s: expected DDL metadata", tc.name)
		}
		if stmt.DDL.Operation != spec.DDLOperationRefreshMaterializedView {
			t.Errorf("%s: expected operation %q, got %q", tc.name, spec.DDLOperationRefreshMaterializedView, stmt.DDL.Operation)
		}
		if stmt.DDL.ObjectName != "mv_stats" {
			t.Errorf("%s: expected object_name %q, got %q", tc.name, "mv_stats", stmt.DDL.ObjectName)
		}
		if stmt.DDL.ObjectType != "materialized_view" {
			t.Errorf("%s: expected object_type %q, got %q", tc.name, "materialized_view", stmt.DDL.ObjectType)
		}
		if stmt.DDL.Options["concurrently"] != tc.concurrently {
			t.Errorf("%s: expected concurrently=%q, got %q", tc.name, tc.concurrently, stmt.DDL.Options["concurrently"])
		}
		if stmt.DDL.Options["with_no_data"] != tc.withNoData {
			t.Errorf("%s: expected with_no_data=%q, got %q", tc.name, tc.withNoData, stmt.DDL.Options["with_no_data"])
		}
	}
}

// refreshMatViewNodeKind returns the top-level AST node kind name for a SQL
// statement. Used by the report helper below.
func refreshMatViewNodeKind(sql string) string {
	tree, err := pg_query.Parse(sql)
	if err != nil {
		return fmt.Sprintf("parse-error: %v", err)
	}
	if len(tree.Stmts) != 1 || tree.Stmts[0].Stmt == nil {
		return "no-statements"
	}
	node := tree.Stmts[0].Stmt
	switch node.GetNode().(type) {
	case *pg_query.Node_RefreshMatViewStmt:
		return "RefreshMatViewStmt"
	default:
		return fmt.Sprintf("%T", node.GetNode())
	}
}

// refreshMatViewRelationName returns the relation name from a REFRESH
// MATERIALIZED VIEW statement's AST node.
func refreshMatViewRelationName(sql string) string {
	tree, err := pg_query.Parse(sql)
	if err != nil {
		return ""
	}
	if len(tree.Stmts) != 1 {
		return ""
	}
	n, ok := tree.Stmts[0].Stmt.GetNode().(*pg_query.Node_RefreshMatViewStmt)
	if !ok || n.RefreshMatViewStmt == nil {
		return ""
	}
	return rangeVarName(n.RefreshMatViewStmt.GetRelation())
}

// refreshMatViewFacts returns the concurrent and skip_data flags from a
// REFRESH MATERIALIZED VIEW statement's AST node.
func refreshMatViewFacts(sql string) (concurrent bool, skipData bool) {
	tree, err := pg_query.Parse(sql)
	if err != nil {
		return
	}
	if len(tree.Stmts) != 1 {
		return
	}
	n, ok := tree.Stmts[0].Stmt.GetNode().(*pg_query.Node_RefreshMatViewStmt)
	if !ok || n.RefreshMatViewStmt == nil {
		return
	}
	return n.RefreshMatViewStmt.GetConcurrent(), n.RefreshMatViewStmt.GetSkipData()
}

// refreshMatViewCurrentStatus returns the current DeltaScope extraction status
// for a SQL statement: kind, unsupported-feature, unsupported-reason.
func refreshMatViewCurrentStatus(sql string) (kind spec.Kind, feature string, reason string) {
	p := New()
	result, err := p.Parse(context.Background(), sql)
	if err != nil {
		return spec.KindUnknown, "parse-error", err.Error()
	}
	if len(result.Statements) != 1 {
		return spec.KindUnknown, "no-statements", ""
	}
	stmt, extractErr := result.Statements[0].Extractor.Extract(spec.DialectPostgreSQL, sql)
	if extractErr != nil {
		return spec.KindUnknown, "extract-error", extractErr.Error()
	}
	if stmt.Unsupported != nil {
		return stmt.Kind, stmt.Unsupported.Feature, stmt.Unsupported.Reason
	}
	return stmt.Kind, "", ""
}
