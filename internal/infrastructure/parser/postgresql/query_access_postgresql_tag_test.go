//go:build postgresql

// Package postgresql tests the PostgreSQL query access extractor.
// input: SQL text exercising SELECT, JOIN, CTE, subquery, locking, function, and multi-statement forms
// output: QueryAccessFacts assertions for read classification, relations, columns, outputs, and unresolved references
// pos: infrastructure-level tests for the PostgreSQL query access adapter
// note: if this file changes, update this header and module README.md.
package postgresql

import (
	"context"
	"testing"

	domain "github.com/Fanduzi/DeltaScope/internal/domain/queryaccess"
)

func TestQueryAccessExtractor_SimpleSelect(t *testing.T) {
	t.Parallel()
	extractor := &QueryAccessExtractor{}
	facts, err := extractor.ExtractQueryAccess(context.Background(), "SELECT id, name FROM users", "postgresql", "")
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
	facts, err := extractor.ExtractQueryAccess(context.Background(), "SELECT id FROM users WHERE id = 1", "postgresql", "")
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	// PostgreSQL V1: WHERE contains operator → indeterminate
	if facts.ReadClassification != string(domain.Indeterminate) {
		t.Errorf("classification: got %q, want %q", facts.ReadClassification, domain.Indeterminate)
	}
}

func TestQueryAccessExtractor_SelectWithJoin(t *testing.T) {
	t.Parallel()
	extractor := &QueryAccessExtractor{}
	facts, err := extractor.ExtractQueryAccess(context.Background(),
		"SELECT u.id, o.total FROM users u INNER JOIN orders o ON u.id = o.user_id", "postgresql", "")
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	// PostgreSQL V1: ON clause contains operator → indeterminate
	if facts.ReadClassification != string(domain.Indeterminate) {
		t.Errorf("classification: got %q, want %q", facts.ReadClassification, domain.Indeterminate)
	}
}

func TestQueryAccessExtractor_SelectWithAliases(t *testing.T) {
	t.Parallel()
	extractor := &QueryAccessExtractor{}
	facts, err := extractor.ExtractQueryAccess(context.Background(),
		"SELECT u.id, u.name FROM users u", "postgresql", "")
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
		"WITH cte AS (SELECT id FROM users) SELECT * FROM cte", "postgresql", "")
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if facts.ReadClassification != string(domain.Indeterminate) {
		t.Errorf("classification: got %q, want %q", facts.ReadClassification, domain.Indeterminate)
	}
	cteFound := false
	for _, r := range facts.Relations {
		if r.Name == "cte" && r.Kind == string(domain.RelationCTE) {
			cteFound = true
		}
	}
	if !cteFound {
		t.Errorf("CTE relation not found in facts")
	}
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

func TestQueryAccessExtractor_DataModifyingCTE(t *testing.T) {
	t.Parallel()
	extractor := &QueryAccessExtractor{}
	facts, err := extractor.ExtractQueryAccess(context.Background(),
		"WITH deleted AS (DELETE FROM users RETURNING id) SELECT id FROM deleted", "postgresql", "")
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if facts.ReadClassification != string(domain.NotReadOnly) {
		t.Errorf("classification: got %q, want %q", facts.ReadClassification, domain.NotReadOnly)
	}
}

func TestQueryAccessExtractor_SelectWithFunction(t *testing.T) {
	t.Parallel()
	extractor := &QueryAccessExtractor{}
	facts, err := extractor.ExtractQueryAccess(context.Background(), "SELECT NOW()", "postgresql", "")
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if facts.ReadClassification != string(domain.Indeterminate) {
		t.Errorf("classification: got %q, want %q", facts.ReadClassification, domain.Indeterminate)
	}
}

func TestQueryAccessExtractor_SelectForUpdate(t *testing.T) {
	t.Parallel()
	extractor := &QueryAccessExtractor{}
	facts, err := extractor.ExtractQueryAccess(context.Background(), "SELECT * FROM users FOR UPDATE", "postgresql", "")
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if facts.ReadClassification != string(domain.NotReadOnly) {
		t.Errorf("classification: got %q, want %q", facts.ReadClassification, domain.NotReadOnly)
	}
}

func TestQueryAccessExtractor_SelectInto(t *testing.T) {
	t.Parallel()
	extractor := &QueryAccessExtractor{}
	facts, err := extractor.ExtractQueryAccess(context.Background(),
		"SELECT id INTO archive_users FROM users", "postgresql", "")
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if facts.ReadClassification != string(domain.NotReadOnly) {
		t.Errorf("classification: got %q, want %q", facts.ReadClassification, domain.NotReadOnly)
	}
}

func TestQueryAccessExtractor_ExplainSelect(t *testing.T) {
	t.Parallel()
	extractor := &QueryAccessExtractor{}
	facts, err := extractor.ExtractQueryAccess(context.Background(), "EXPLAIN SELECT * FROM users", "postgresql", "")
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if facts.ReadClassification != string(domain.Indeterminate) {
		t.Errorf("classification: got %q, want %q", facts.ReadClassification, domain.Indeterminate)
	}
}

func TestQueryAccessExtractor_ExplainAnalyze(t *testing.T) {
	t.Parallel()
	extractor := &QueryAccessExtractor{}
	facts, err := extractor.ExtractQueryAccess(context.Background(), "EXPLAIN ANALYZE SELECT * FROM users", "postgresql", "")
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if facts.ReadClassification != string(domain.NotReadOnly) {
		t.Errorf("classification: got %q, want %q", facts.ReadClassification, domain.NotReadOnly)
	}
}

func TestQueryAccessExtractor_LateralJoin(t *testing.T) {
	t.Parallel()
	extractor := &QueryAccessExtractor{}
	facts, err := extractor.ExtractQueryAccess(context.Background(),
		"SELECT * FROM users u, LATERAL (SELECT * FROM orders WHERE user_id = u.id) o", "postgresql", "")
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if facts.ReadClassification != string(domain.Indeterminate) {
		t.Errorf("classification: got %q, want %q", facts.ReadClassification, domain.Indeterminate)
	}
}

func TestQueryAccessExtractor_EmptyInput(t *testing.T) {
	t.Parallel()
	extractor := &QueryAccessExtractor{}
	facts, err := extractor.ExtractQueryAccess(context.Background(), "", "postgresql", "")
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
	facts, err := extractor.ExtractQueryAccess(context.Background(), "INVALID SQL", "postgresql", "")
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if facts.ReadClassification != string(domain.Indeterminate) {
		t.Errorf("classification: got %q, want %q", facts.ReadClassification, domain.Indeterminate)
	}
}

func TestQueryAccessExtractor_MultiStatementSelectDelete(t *testing.T) {
	t.Parallel()
	extractor := &QueryAccessExtractor{}
	facts, err := extractor.ExtractQueryAccess(context.Background(),
		"SELECT 1; DELETE FROM users;", "postgresql", "")
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if facts.ReadClassification != string(domain.NotReadOnly) {
		t.Errorf("classification: got %q, want %q", facts.ReadClassification, domain.NotReadOnly)
	}
}

func TestQueryAccessExtractor_OrderBy(t *testing.T) {
	t.Parallel()
	extractor := &QueryAccessExtractor{}
	facts, err := extractor.ExtractQueryAccess(context.Background(),
		"SELECT id, name FROM users ORDER BY name ASC", "postgresql", "")
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if facts.ReadClassification != string(domain.ReadOnly) {
		t.Errorf("classification: got %q, want %q", facts.ReadClassification, domain.ReadOnly)
	}
	for _, col := range facts.ColumnReferences {
		if col.Column == "name" && col.Table == "users" {
			assertUsageContains(t, col.Usages, string(domain.UsageOrdering))
			return
		}
	}
	t.Errorf("ordering usage for users.name not found")
}

func TestQueryAccessExtractor_CrossJoin(t *testing.T) {
	t.Parallel()
	extractor := &QueryAccessExtractor{}
	facts, err := extractor.ExtractQueryAccess(context.Background(),
		"SELECT * FROM users CROSS JOIN orders", "postgresql", "")
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	// CROSS JOIN has no ON clause → no operator → indeterminate (wildcard)
	if facts.ReadClassification != string(domain.Indeterminate) {
		t.Errorf("classification: got %q, want %q", facts.ReadClassification, domain.Indeterminate)
	}
}

func TestQueryAccessExtractor_Union(t *testing.T) {
	t.Parallel()
	extractor := &QueryAccessExtractor{}
	facts, err := extractor.ExtractQueryAccess(context.Background(),
		"SELECT id FROM users UNION SELECT id FROM admins", "postgresql", "")
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
		"SELECT * FROM (SELECT id, name FROM users) AS sub", "postgresql", "")
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	// Derived table with SELECT * → indeterminate (wildcard needs metadata)
	if facts.ReadClassification != string(domain.Indeterminate) {
		t.Errorf("classification: got %q, want %q", facts.ReadClassification, domain.Indeterminate)
	}
}

func TestQueryAccessExtractor_OutputLineage(t *testing.T) {
	t.Parallel()
	extractor := &QueryAccessExtractor{}
	facts, err := extractor.ExtractQueryAccess(context.Background(),
		"SELECT id, name FROM users", "postgresql", "")
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
		"SELECT id FROM users", "postgresql", "app")
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if len(facts.Relations) != 1 {
		t.Fatalf("relations: got %d, want 1", len(facts.Relations))
	}
	if facts.Relations[0].Schema != "" {
		t.Errorf("relation schema: got %q, want empty (unqualified uses search_path)", facts.Relations[0].Schema)
	}
}

func TestQueryAccessExtractor_SchemaQualified(t *testing.T) {
	t.Parallel()
	extractor := &QueryAccessExtractor{}
	facts, err := extractor.ExtractQueryAccess(context.Background(),
		"SELECT id FROM app.users", "postgresql", "")
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

func TestQueryAccessExtractor_AuditRegression(t *testing.T) {
	t.Parallel()
	extractor := &QueryAccessExtractor{}
	// SELECT reaches audit as KindUnknown (unsupported boundary)
	facts, err := extractor.ExtractQueryAccess(context.Background(), "SELECT id FROM users", "postgresql", "")
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if facts.ReadClassification == "" {
		t.Errorf("classification should not be empty")
	}
	// DDL/DML extraction is unchanged (tested in audit regression tests)
}

func TestQueryAccessExtractor_ContextCancelled(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	extractor := &QueryAccessExtractor{}
	_, err := extractor.ExtractQueryAccess(ctx, "SELECT id FROM users", "postgresql", "")
	if err == nil {
		t.Errorf("expected error for cancelled context")
	}
}

func TestQueryAccessExtractor_ScalarSubqueryRelations(t *testing.T) {
	t.Parallel()
	extractor := &QueryAccessExtractor{}
	facts, err := extractor.ExtractQueryAccess(context.Background(),
		"SELECT id, (SELECT max(amount) FROM orders WHERE user_id = users.id) FROM users", "postgresql", "")
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	relNames := make(map[string]bool)
	for _, r := range facts.Relations {
		relNames[r.Name] = true
	}
	if !relNames["users"] {
		t.Errorf("users relation not found")
	}
	if !relNames["orders"] {
		t.Errorf("orders relation not found in scalar subquery")
	}
}

func TestQueryAccessExtractor_UnionColumnReferences(t *testing.T) {
	t.Parallel()
	extractor := &QueryAccessExtractor{}
	facts, err := extractor.ExtractQueryAccess(context.Background(),
		"SELECT id FROM users UNION ALL SELECT id FROM admins", "postgresql", "")
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	foundUsersID := false
	foundAdminsID := false
	for _, col := range facts.ColumnReferences {
		if col.Column == "id" && col.Table == "users" {
			foundUsersID = true
		}
		if col.Column == "id" && col.Table == "admins" {
			foundAdminsID = true
		}
	}
	if !foundUsersID {
		t.Errorf("users.id column reference not found in UNION")
	}
	if !foundAdminsID {
		t.Errorf("admins.id column reference not found in UNION")
	}
}

func TestQueryAccessExtractor_JoinUsingReasonCode(t *testing.T) {
	t.Parallel()
	extractor := &QueryAccessExtractor{}
	facts, err := extractor.ExtractQueryAccess(context.Background(),
		"SELECT * FROM users JOIN orders USING (user_id)", "postgresql", "")
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	found := false
	for _, code := range facts.ReasonCodes {
		if code == "unsupported_traversal" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected unsupported_traversal reason code for JOIN USING, got reason codes: %v", facts.ReasonCodes)
	}
}

func TestQueryAccessExtractor_ThreePartColumnRefSchema(t *testing.T) {
	t.Parallel()
	extractor := &QueryAccessExtractor{}
	facts, err := extractor.ExtractQueryAccess(context.Background(),
		"SELECT public.users.id FROM public.users", "postgresql", "")
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if len(facts.Relations) != 1 {
		t.Fatalf("relations: got %d, want 1", len(facts.Relations))
	}
	if facts.Relations[0].Schema != "public" {
		t.Errorf("relation schema: got %q, want %q", facts.Relations[0].Schema, "public")
	}
	found := false
	for _, col := range facts.ColumnReferences {
		if col.Column == "id" && col.Table == "users" {
			found = true
			if col.Schema != "public" {
				t.Errorf("column schema: got %q, want %q", col.Schema, "public")
			}
		}
	}
	if !found {
		t.Errorf("users.id column reference not found")
	}
}

func TestQueryAccessExtractor_ThreePartOutputLineage(t *testing.T) {
	t.Parallel()
	extractor := &QueryAccessExtractor{}
	facts, err := extractor.ExtractQueryAccess(context.Background(),
		"SELECT public.users.id FROM public.users", "postgresql", "")
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if len(facts.Outputs) != 1 {
		t.Fatalf("outputs: got %d, want 1", len(facts.Outputs))
	}
	if len(facts.Outputs[0].Sources) != 1 {
		t.Fatalf("output[0].Sources: got %d, want 1", len(facts.Outputs[0].Sources))
	}
	want := "public.users.id"
	if facts.Outputs[0].Sources[0] != want {
		t.Errorf("output[0].Sources[0]: got %q, want %q", facts.Outputs[0].Sources[0], want)
	}
}

func TestQueryAccessExtractor_ThreePartOutputLineageWildcard(t *testing.T) {
	t.Parallel()
	extractor := &QueryAccessExtractor{}
	facts, err := extractor.ExtractQueryAccess(context.Background(),
		"SELECT public.users.* FROM public.users", "postgresql", "")
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if len(facts.Outputs) != 1 {
		t.Fatalf("outputs: got %d, want 1", len(facts.Outputs))
	}
	if len(facts.Outputs[0].Sources) != 1 {
		t.Fatalf("output[0].Sources: got %d, want 1", len(facts.Outputs[0].Sources))
	}
	want := "public.users.*"
	if facts.Outputs[0].Sources[0] != want {
		t.Errorf("output[0].Sources[0]: got %q, want %q", facts.Outputs[0].Sources[0], want)
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
