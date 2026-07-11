// Package tidbparser tests the TiDB query access extractor.
// input: SQL text exercising SELECT, JOIN, CTE, subquery, locking, function, and multi-statement forms
// output: QueryAccessFacts assertions for read classification, relations, columns, outputs, and unresolved references
// pos: infrastructure-level tests for the TiDB query access adapter
// note: if this file changes, update this header and module README.md.
package tidbparser

import (
	"context"
	"testing"

	domain "github.com/Fanduzi/DeltaScope/internal/domain/queryaccess"
)

func TestQueryAccessExtractor_SimpleSelect(t *testing.T) {
	t.Parallel()
	extractor := &QueryAccessExtractor{}
	facts, err := extractor.ExtractQueryAccess(context.Background(), "SELECT id, name FROM users", "mysql", "")
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if facts.ReadClassification != string(domain.ReadOnly) {
		t.Errorf("classification: got %q, want %q", facts.ReadClassification, domain.ReadOnly)
	}
	if len(facts.Relations) != 1 {
		t.Fatalf("relations: got %d, want 1", len(facts.Relations))
	}
	if facts.Relations[0].Name != "users" {
		t.Errorf("relation name: got %q, want %q", facts.Relations[0].Name, "users")
	}
	if facts.Relations[0].Kind != string(domain.RelationTable) {
		t.Errorf("relation kind: got %q, want %q", facts.Relations[0].Kind, domain.RelationTable)
	}
	// Should have column references for id and name with projection usage
	foundID := false
	foundName := false
	for _, col := range facts.ColumnReferences {
		if col.Column == "id" && col.Table == "users" {
			foundID = true
			assertUsageContains(t, col.Usages, string(domain.UsageProjection))
		}
		if col.Column == "name" && col.Table == "users" {
			foundName = true
			assertUsageContains(t, col.Usages, string(domain.UsageProjection))
		}
	}
	if !foundID {
		t.Errorf("column reference for users.id not found")
	}
	if !foundName {
		t.Errorf("column reference for users.name not found")
	}
}

func TestQueryAccessExtractor_SelectWithWhere(t *testing.T) {
	t.Parallel()
	extractor := &QueryAccessExtractor{}
	facts, err := extractor.ExtractQueryAccess(context.Background(), "SELECT id FROM users WHERE id = 1", "mysql", "")
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if facts.ReadClassification != string(domain.ReadOnly) {
		t.Errorf("classification: got %q, want %q", facts.ReadClassification, domain.ReadOnly)
	}
	// id should have both projection and filter usage
	for _, col := range facts.ColumnReferences {
		if col.Column == "id" && col.Table == "users" {
			assertUsageContains(t, col.Usages, string(domain.UsageProjection))
			assertUsageContains(t, col.Usages, string(domain.UsageFilter))
			return
		}
	}
	t.Errorf("column reference for users.id not found")
}

func TestQueryAccessExtractor_SelectWithJoin(t *testing.T) {
	t.Parallel()
	extractor := &QueryAccessExtractor{}
	facts, err := extractor.ExtractQueryAccess(context.Background(),
		"SELECT u.id, o.total FROM users u INNER JOIN orders o ON u.id = o.user_id", "mysql", "")
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if facts.ReadClassification != string(domain.ReadOnly) {
		t.Errorf("classification: got %q, want %q", facts.ReadClassification, domain.ReadOnly)
	}
	if len(facts.Relations) != 2 {
		t.Fatalf("relations: got %d, want 2", len(facts.Relations))
	}
	// Check both relations present
	relNames := make(map[string]bool)
	for _, r := range facts.Relations {
		relNames[r.Name] = true
	}
	if !relNames["users"] {
		t.Errorf("users relation not found")
	}
	if !relNames["orders"] {
		t.Errorf("orders relation not found")
	}
	// Check join usage
	for _, col := range facts.ColumnReferences {
		if col.Column == "id" && col.Table == "users" {
			assertUsageContains(t, col.Usages, string(domain.UsageJoin))
		}
		if col.Column == "user_id" && col.Table == "orders" {
			assertUsageContains(t, col.Usages, string(domain.UsageJoin))
		}
	}
}

func TestQueryAccessExtractor_SelectWithAliases(t *testing.T) {
	t.Parallel()
	extractor := &QueryAccessExtractor{}
	facts, err := extractor.ExtractQueryAccess(context.Background(),
		"SELECT u.id, u.name FROM users u", "mysql", "")
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if facts.ReadClassification != string(domain.ReadOnly) {
		t.Errorf("classification: got %q, want %q", facts.ReadClassification, domain.ReadOnly)
	}
	if len(facts.Relations) != 1 {
		t.Fatalf("relations: got %d, want 1", len(facts.Relations))
	}
	if facts.Relations[0].Alias != "u" {
		t.Errorf("relation alias: got %q, want %q", facts.Relations[0].Alias, "u")
	}
	// Columns should resolve through alias to users table
	for _, col := range facts.ColumnReferences {
		if col.Column == "id" {
			if col.Table != "users" {
				t.Errorf("u.id should resolve to users.id, got table %q", col.Table)
			}
		}
	}
}

func TestQueryAccessExtractor_SelectWithCTE(t *testing.T) {
	t.Parallel()
	extractor := &QueryAccessExtractor{}
	facts, err := extractor.ExtractQueryAccess(context.Background(),
		"WITH cte AS (SELECT id FROM users) SELECT * FROM cte", "mysql", "")
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	// SELECT * from CTE → indeterminate (wildcard needs metadata to expand)
	if facts.ReadClassification != string(domain.Indeterminate) {
		t.Errorf("classification: got %q, want %q", facts.ReadClassification, domain.Indeterminate)
	}
	// CTE should appear as relation with kind=cte
	cteFound := false
	for _, r := range facts.Relations {
		if r.Name == "cte" && r.Kind == string(domain.RelationCTE) {
			cteFound = true
		}
	}
	if !cteFound {
		t.Errorf("CTE relation not found in facts")
	}
	// users should also appear as base table
	usersFound := false
	for _, r := range facts.Relations {
		if r.Name == "users" && r.Kind == string(domain.RelationTable) {
			usersFound = true
		}
	}
	if !usersFound {
		t.Errorf("users table not found in CTE body relations")
	}
}

func TestQueryAccessExtractor_SelectWithSubquery(t *testing.T) {
	t.Parallel()
	extractor := &QueryAccessExtractor{}
	facts, err := extractor.ExtractQueryAccess(context.Background(),
		"SELECT id FROM users WHERE EXISTS (SELECT 1 FROM orders WHERE orders.user_id = users.id)", "mysql", "")
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if facts.ReadClassification != string(domain.ReadOnly) {
		t.Errorf("classification: got %q, want %q", facts.ReadClassification, domain.ReadOnly)
	}
	relNames := make(map[string]bool)
	for _, r := range facts.Relations {
		relNames[r.Name] = true
	}
	if !relNames["users"] {
		t.Errorf("users relation not found")
	}
	if !relNames["orders"] {
		t.Errorf("orders relation not found in subquery")
	}
}

func TestQueryAccessExtractor_SelectStar(t *testing.T) {
	t.Parallel()
	extractor := &QueryAccessExtractor{}
	facts, err := extractor.ExtractQueryAccess(context.Background(), "SELECT * FROM users", "mysql", "")
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	// V1: wildcard without metadata → indeterminate (needs metadata to expand)
	if facts.ReadClassification != string(domain.Indeterminate) {
		t.Errorf("classification: got %q, want %q", facts.ReadClassification, domain.Indeterminate)
	}
	// Should have an unresolved reference for the wildcard
	found := false
	for _, u := range facts.Unresolved {
		if u.Reason == "schema_unavailable" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected unresolved wildcard reference")
	}
}

func TestQueryAccessExtractor_SelectWithFunction(t *testing.T) {
	t.Parallel()
	extractor := &QueryAccessExtractor{}
	facts, err := extractor.ExtractQueryAccess(context.Background(), "SELECT NOW()", "mysql", "")
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	// V1 policy: empty function allowlist → indeterminate
	if facts.ReadClassification != string(domain.Indeterminate) {
		t.Errorf("classification: got %q, want %q", facts.ReadClassification, domain.Indeterminate)
	}
}

func TestQueryAccessExtractor_SelectForUpdate(t *testing.T) {
	t.Parallel()
	extractor := &QueryAccessExtractor{}
	facts, err := extractor.ExtractQueryAccess(context.Background(), "SELECT * FROM users FOR UPDATE", "mysql", "")
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if facts.ReadClassification != string(domain.NotReadOnly) {
		t.Errorf("classification: got %q, want %q", facts.ReadClassification, domain.NotReadOnly)
	}
}

func TestQueryAccessExtractor_SelectIntoOutfile(t *testing.T) {
	t.Parallel()
	extractor := &QueryAccessExtractor{}
	facts, err := extractor.ExtractQueryAccess(context.Background(),
		"SELECT * FROM users INTO OUTFILE '/tmp/out.csv'", "mysql", "")
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if facts.ReadClassification != string(domain.NotReadOnly) {
		t.Errorf("classification: got %q, want %q", facts.ReadClassification, domain.NotReadOnly)
	}
}

func TestQueryAccessExtractor_MultiStatementSelectDelete(t *testing.T) {
	t.Parallel()
	extractor := &QueryAccessExtractor{}
	facts, err := extractor.ExtractQueryAccess(context.Background(),
		"SELECT 1; DELETE FROM users;", "mysql", "")
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if facts.ReadClassification != string(domain.NotReadOnly) {
		t.Errorf("classification: got %q, want %q", facts.ReadClassification, domain.NotReadOnly)
	}
}

func TestQueryAccessExtractor_EmptyInput(t *testing.T) {
	t.Parallel()
	extractor := &QueryAccessExtractor{}
	facts, err := extractor.ExtractQueryAccess(context.Background(), "", "mysql", "")
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if facts.ReadClassification != string(domain.Indeterminate) {
		t.Errorf("classification: got %q, want %q", facts.ReadClassification, domain.Indeterminate)
	}
}

func TestQueryAccessExtractor_ParserError(t *testing.T) {
	t.Parallel()
	extractor := &QueryAccessExtractor{}
	facts, err := extractor.ExtractQueryAccess(context.Background(), "INVALID SQL", "mysql", "")
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if facts.ReadClassification != string(domain.Indeterminate) {
		t.Errorf("classification: got %q, want %q", facts.ReadClassification, domain.Indeterminate)
	}
}

func TestQueryAccessExtractor_Union(t *testing.T) {
	t.Parallel()
	extractor := &QueryAccessExtractor{}
	facts, err := extractor.ExtractQueryAccess(context.Background(),
		"SELECT id FROM users UNION SELECT id FROM admins", "mysql", "")
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if facts.ReadClassification != string(domain.ReadOnly) {
		t.Errorf("classification: got %q, want %q", facts.ReadClassification, domain.ReadOnly)
	}
	relNames := make(map[string]bool)
	for _, r := range facts.Relations {
		relNames[r.Name] = true
	}
	if !relNames["users"] {
		t.Errorf("users relation not found")
	}
	if !relNames["admins"] {
		t.Errorf("admins relation not found")
	}
}

func TestQueryAccessExtractor_DerivedTable(t *testing.T) {
	t.Parallel()
	extractor := &QueryAccessExtractor{}
	facts, err := extractor.ExtractQueryAccess(context.Background(),
		"SELECT * FROM (SELECT id, name FROM users) AS sub", "mysql", "")
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	// SELECT * from derived table → indeterminate (wildcard needs metadata)
	if facts.ReadClassification != string(domain.Indeterminate) {
		t.Errorf("classification: got %q, want %q", facts.ReadClassification, domain.Indeterminate)
	}
	// Derived table should appear as relation
	derivedFound := false
	for _, r := range facts.Relations {
		if r.Name == "sub" && r.Kind == string(domain.RelationDerived) {
			derivedFound = true
		}
	}
	if !derivedFound {
		t.Errorf("derived table 'sub' not found in relations")
	}
}

func TestQueryAccessExtractor_GroupByWithFunction(t *testing.T) {
	t.Parallel()
	extractor := &QueryAccessExtractor{}
	facts, err := extractor.ExtractQueryAccess(context.Background(),
		"SELECT dept, COUNT(*) FROM employees GROUP BY dept", "mysql", "")
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	// COUNT(*) → indeterminate (empty function allowlist)
	if facts.ReadClassification != string(domain.Indeterminate) {
		t.Errorf("classification: got %q, want %q", facts.ReadClassification, domain.Indeterminate)
	}
}

func TestQueryAccessExtractor_OrderBy(t *testing.T) {
	t.Parallel()
	extractor := &QueryAccessExtractor{}
	facts, err := extractor.ExtractQueryAccess(context.Background(),
		"SELECT id, name FROM users ORDER BY name ASC", "mysql", "")
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if facts.ReadClassification != string(domain.ReadOnly) {
		t.Errorf("classification: got %q, want %q", facts.ReadClassification, domain.ReadOnly)
	}
	// name should have ordering usage
	for _, col := range facts.ColumnReferences {
		if col.Column == "name" && col.Table == "users" {
			assertUsageContains(t, col.Usages, string(domain.UsageOrdering))
			return
		}
	}
	t.Errorf("ordering usage for users.name not found")
}

func TestQueryAccessExtractor_OutputLineage(t *testing.T) {
	t.Parallel()
	extractor := &QueryAccessExtractor{}
	facts, err := extractor.ExtractQueryAccess(context.Background(),
		"SELECT id, name FROM users", "mysql", "")
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if len(facts.Outputs) != 2 {
		t.Fatalf("outputs: got %d, want 2", len(facts.Outputs))
	}
	if facts.Outputs[0].Name != "id" {
		t.Errorf("output[0].Name: got %q, want %q", facts.Outputs[0].Name, "id")
	}
	if len(facts.Outputs[0].Sources) != 1 || facts.Outputs[0].Sources[0] != "users.id" {
		t.Errorf("output[0].Sources: got %v, want [users.id]", facts.Outputs[0].Sources)
	}
}

func TestQueryAccessExtractor_DefaultSchema(t *testing.T) {
	t.Parallel()
	extractor := &QueryAccessExtractor{}
	facts, err := extractor.ExtractQueryAccess(context.Background(),
		"SELECT id FROM users", "mysql", "app")
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if len(facts.Relations) != 1 {
		t.Fatalf("relations: got %d, want 1", len(facts.Relations))
	}
	if facts.Relations[0].Schema != "app" {
		t.Errorf("relation schema: got %q, want %q", facts.Relations[0].Schema, "app")
	}
}

func TestQueryAccessExtractor_SchemaQualified(t *testing.T) {
	t.Parallel()
	extractor := &QueryAccessExtractor{}
	facts, err := extractor.ExtractQueryAccess(context.Background(),
		"SELECT id FROM app.users", "mysql", "")
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if len(facts.Relations) != 1 {
		t.Fatalf("relations: got %d, want 1", len(facts.Relations))
	}
	if facts.Relations[0].Schema != "app" {
		t.Errorf("relation schema: got %q, want %q", facts.Relations[0].Schema, "app")
	}
}

func TestQueryAccessExtractor_UnqualifiedColumnAmbiguous(t *testing.T) {
	t.Parallel()
	extractor := &QueryAccessExtractor{}
	facts, err := extractor.ExtractQueryAccess(context.Background(),
		"SELECT id FROM users, admins", "mysql", "")
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	// Unqualified "id" with two tables → indeterminate (ambiguous)
	if facts.ReadClassification != string(domain.Indeterminate) {
		t.Errorf("classification: got %q, want %q", facts.ReadClassification, domain.Indeterminate)
	}
}

func TestQueryAccessExtractor_ForShare(t *testing.T) {
	t.Parallel()
	extractor := &QueryAccessExtractor{}
	facts, err := extractor.ExtractQueryAccess(context.Background(),
		"SELECT * FROM users FOR SHARE", "mysql", "")
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if facts.ReadClassification != string(domain.NotReadOnly) {
		t.Errorf("classification: got %q, want %q", facts.ReadClassification, domain.NotReadOnly)
	}
}

func TestQueryAccessExtractor_LockInShareMode(t *testing.T) {
	t.Parallel()
	extractor := &QueryAccessExtractor{}
	facts, err := extractor.ExtractQueryAccess(context.Background(),
		"SELECT * FROM users LOCK IN SHARE MODE", "mysql", "")
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if facts.ReadClassification != string(domain.NotReadOnly) {
		t.Errorf("classification: got %q, want %q", facts.ReadClassification, domain.NotReadOnly)
	}
}

func TestQueryAccessExtractor_DDLNotReadOnly(t *testing.T) {
	t.Parallel()
	extractor := &QueryAccessExtractor{}
	facts, err := extractor.ExtractQueryAccess(context.Background(),
		"CREATE TABLE t1 (id INT)", "mysql", "")
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if facts.ReadClassification != string(domain.NotReadOnly) {
		t.Errorf("classification: got %q, want %q", facts.ReadClassification, domain.NotReadOnly)
	}
}

func TestQueryAccessExtractor_InsertNotReadOnly(t *testing.T) {
	t.Parallel()
	extractor := &QueryAccessExtractor{}
	facts, err := extractor.ExtractQueryAccess(context.Background(),
		"INSERT INTO users (name) VALUES ('alice')", "mysql", "")
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if facts.ReadClassification != string(domain.NotReadOnly) {
		t.Errorf("classification: got %q, want %q", facts.ReadClassification, domain.NotReadOnly)
	}
}

func TestQueryAccessExtractor_RecursiveCTE(t *testing.T) {
	t.Parallel()
	extractor := &QueryAccessExtractor{}
	facts, err := extractor.ExtractQueryAccess(context.Background(),
		"WITH RECURSIVE cte AS (SELECT 1 AS n UNION ALL SELECT n + 1 FROM cte WHERE n < 10) SELECT * FROM cte", "mysql", "")
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	// Recursive CTE with SELECT * → indeterminate (wildcard)
	if facts.ReadClassification != string(domain.Indeterminate) {
		t.Errorf("classification: got %q, want %q", facts.ReadClassification, domain.Indeterminate)
	}
}

// assertUsageContains checks that a usage list contains the expected usage.
func assertUsageContains(t *testing.T, usages []string, expected string) {
	t.Helper()
	for _, u := range usages {
		if u == expected {
			return
		}
	}
	t.Errorf("usages %v does not contain %q", usages, expected)
}
