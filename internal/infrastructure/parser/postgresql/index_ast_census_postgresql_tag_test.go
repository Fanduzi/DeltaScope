//go:build postgresql

package postgresql

import (
	"slices"
	"testing"

	pg_query "github.com/pganalyze/pg_query_go/v6"
)

// TestPostgreSQLIndexASTCensus characterizes pg_query_go AST facts for advanced
// CREATE INDEX forms. This is a read-only characterization test: it asserts
// exactly what the parser exposes for partial, expression, INCLUDE, non-btree,
// and CONCURRENTLY variants. No production code is modified.
func TestPostgreSQLIndexASTCensus(t *testing.T) {
	cases := []struct {
		Name         string
		SQL          string
		AccessMethod string
		Concurrent   bool
		Unique       bool
		HasWhere     bool
		KeyNames     []string
		IncludeNames []string
		ExprCount    int
	}{
		{
			Name:         "btree column",
			SQL:          "CREATE INDEX idx_users_email ON users (email)",
			AccessMethod: "btree",
			Concurrent:   false,
			Unique:       false,
			HasWhere:     false,
			KeyNames:     []string{"email"},
			IncludeNames: nil,
			ExprCount:    0,
		},
		{
			Name:         "partial",
			SQL:          "CREATE INDEX idx_users_active_email ON users (email) WHERE active = true",
			AccessMethod: "btree",
			Concurrent:   false,
			Unique:       false,
			HasWhere:     true,
			KeyNames:     []string{"email"},
			IncludeNames: nil,
			ExprCount:    0,
		},
		{
			Name:         "expression",
			SQL:          "CREATE INDEX idx_users_lower_email ON users (LOWER(email))",
			AccessMethod: "btree",
			Concurrent:   false,
			Unique:       false,
			HasWhere:     false,
			KeyNames:     nil,
			IncludeNames: nil,
			ExprCount:    1,
		},
		{
			Name:         "include",
			SQL:          "CREATE INDEX idx_users_email_cover ON users (email) INCLUDE (name, active)",
			AccessMethod: "btree",
			Concurrent:   false,
			Unique:       false,
			HasWhere:     false,
			KeyNames:     []string{"email"},
			IncludeNames: []string{"name", "active"},
			ExprCount:    0,
		},
		{
			Name:         "gin",
			SQL:          "CREATE INDEX idx_docs_body ON docs USING gin (body)",
			AccessMethod: "gin",
			Concurrent:   false,
			Unique:       false,
			HasWhere:     false,
			KeyNames:     []string{"body"},
			IncludeNames: nil,
			ExprCount:    0,
		},
		{
			Name:         "concurrent partial",
			SQL:          "CREATE INDEX CONCURRENTLY idx_users_active_email ON users (email) WHERE active = true",
			AccessMethod: "btree",
			Concurrent:   true,
			Unique:       false,
			HasWhere:     true,
			KeyNames:     []string{"email"},
			IncludeNames: nil,
			ExprCount:    0,
		},
	}

	t.Log("")
	t.Log("=== PostgreSQL Index AST Census ===")
	t.Logf("%-22s | %-6s | %-10s | %-6s | %-6s | %-9s | %-30s | %-25s | %-4s",
		"Case", "Method", "Concurrent", "Unique", "Where", "KeyNames", "IncludeNames", "ExprCount", "OK?")
	t.Log("---------------------------------------------------------------------------------------------------------------------------")

	for _, tc := range cases {
		tree, err := pg_query.Parse(tc.SQL)
		if err != nil {
			t.Fatalf("%s: parse failed: %v", tc.Name, err)
		}
		if len(tree.Stmts) != 1 {
			t.Fatalf("%s: expected 1 statement, got %d", tc.Name, len(tree.Stmts))
		}

		node := tree.Stmts[0].Stmt
		idx, ok := node.GetNode().(*pg_query.Node_IndexStmt)
		if !ok {
			t.Fatalf("%s: expected IndexStmt, got %T", tc.Name, node.GetNode())
		}
		stmt := idx.IndexStmt

		accessMethod := stmt.GetAccessMethod()
		concurrent := stmt.GetConcurrent()
		unique := stmt.GetUnique()
		hasWhere := stmt.GetWhereClause() != nil
		keyNames := testIndexElemNames(stmt.GetIndexParams())
		includeNames := testIndexElemNames(stmt.GetIndexIncludingParams())
		exprCount := testExpressionIndexElemCount(stmt.GetIndexParams())

		allOK := true

		if accessMethod != tc.AccessMethod {
			t.Errorf("%s: access method = %q, want %q", tc.Name, accessMethod, tc.AccessMethod)
			allOK = false
		}
		if concurrent != tc.Concurrent {
			t.Errorf("%s: concurrent = %v, want %v", tc.Name, concurrent, tc.Concurrent)
			allOK = false
		}
		if unique != tc.Unique {
			t.Errorf("%s: unique = %v, want %v", tc.Name, unique, tc.Unique)
			allOK = false
		}
		if hasWhere != tc.HasWhere {
			t.Errorf("%s: hasWhere = %v, want %v", tc.Name, hasWhere, tc.HasWhere)
			allOK = false
		}
		if !sameStrings(keyNames, tc.KeyNames) {
			t.Errorf("%s: keyNames = %v, want %v", tc.Name, keyNames, tc.KeyNames)
			allOK = false
		}
		if !sameStrings(includeNames, tc.IncludeNames) {
			t.Errorf("%s: includeNames = %v, want %v", tc.Name, includeNames, tc.IncludeNames)
			allOK = false
		}
		if exprCount != tc.ExprCount {
			t.Errorf("%s: exprCount = %d, want %d", tc.Name, exprCount, tc.ExprCount)
			allOK = false
		}

		okStr := "PASS"
		if !allOK {
			okStr = "FAIL"
		}

		t.Logf("%-22s | %-6s | %-10v | %-6v | %-6v | %-9v | %-30v | %-25v | %-4s",
			tc.Name, accessMethod, concurrent, unique, hasWhere, keyNames, includeNames, exprCount, okStr)
	}
}

// testIndexElemNames returns the Name field of each IndexElem node.
func testIndexElemNames(nodes []*pg_query.Node) []string {
	var names []string
	for _, n := range nodes {
		elem := n.GetIndexElem()
		if elem != nil && elem.GetName() != "" {
			names = append(names, elem.GetName())
		}
	}
	return names
}

func testExpressionIndexElemCount(nodes []*pg_query.Node) int {
	count := 0
	for _, n := range nodes {
		elem := n.GetIndexElem()
		if elem == nil || elem.GetExpr() != nil {
			count++
		}
	}
	return count
}

// sameStrings returns true if a and b contain the same strings in the same
// order. nil and empty slices are considered equal.
func sameStrings(a, b []string) bool {
	if len(a) == 0 && len(b) == 0 {
		return true
	}
	return slices.Equal(a, b)
}
