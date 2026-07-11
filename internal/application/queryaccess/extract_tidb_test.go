package queryaccess_test

import (
	"context"
	"testing"

	appqa "github.com/Fanduzi/DeltaScope/internal/application/queryaccess"
	domain "github.com/Fanduzi/DeltaScope/internal/domain/queryaccess"
)

func TestExtractTiDBQueryAccess_SimpleSelect(t *testing.T) {
	t.Parallel()
	result, err := appqa.ExtractTiDBQueryAccess(context.Background(), appqa.QueryAccessRequest{
		SQL:     "SELECT id, name FROM users",
		Dialect: "mysql",
	})
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	dr := result.DomainResult
	if dr.ReadClassification != domain.ReadOnly {
		t.Errorf("classification: got %q, want %q", dr.ReadClassification, domain.ReadOnly)
	}
	if dr.Admission != domain.Admissible {
		t.Errorf("admission: got %q, want %q", dr.Admission, domain.Admissible)
	}
	if len(dr.Relations) != 1 {
		t.Fatalf("relations: got %d, want 1", len(dr.Relations))
	}
	if dr.Relations[0].Name != "users" {
		t.Errorf("relation name: got %q, want %q", dr.Relations[0].Name, "users")
	}
	if dr.Relations[0].Kind != domain.RelationTable {
		t.Errorf("relation kind: got %q, want %q", dr.Relations[0].Kind, domain.RelationTable)
	}
	if dr.Relations[0].PermissionRequired != true {
		t.Errorf("table should require permission")
	}
}

func TestExtractTiDBQueryAccess_CTEPermissionNotRequired(t *testing.T) {
	t.Parallel()
	result, err := appqa.ExtractTiDBQueryAccess(context.Background(), appqa.QueryAccessRequest{
		SQL:     "WITH cte AS (SELECT id FROM users) SELECT id FROM cte",
		Dialect: "mysql",
	})
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	for _, rel := range result.DomainResult.Relations {
		if rel.Kind == domain.RelationCTE && rel.PermissionRequired {
			t.Errorf("CTE %q should not require permission", rel.Name)
		}
	}
}

func TestExtractTiDBQueryAccess_ForUpdateRejected(t *testing.T) {
	t.Parallel()
	result, err := appqa.ExtractTiDBQueryAccess(context.Background(), appqa.QueryAccessRequest{
		SQL:     "SELECT * FROM users FOR UPDATE",
		Dialect: "mysql",
	})
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	dr := result.DomainResult
	if dr.ReadClassification != domain.NotReadOnly {
		t.Errorf("classification: got %q, want %q", dr.ReadClassification, domain.NotReadOnly)
	}
	if dr.Admission != domain.Rejected {
		t.Errorf("admission: got %q, want %q", dr.Admission, domain.Rejected)
	}
}

func TestExtractTiDBQueryAccess_IndeterminateAdmission(t *testing.T) {
	t.Parallel()
	result, err := appqa.ExtractTiDBQueryAccess(context.Background(), appqa.QueryAccessRequest{
		SQL:     "SELECT NOW()",
		Dialect: "mysql",
	})
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	dr := result.DomainResult
	if dr.ReadClassification != domain.Indeterminate {
		t.Errorf("classification: got %q, want %q", dr.ReadClassification, domain.Indeterminate)
	}
	if dr.Admission != domain.IndeterminateAdmission {
		t.Errorf("admission: got %q, want %q", dr.Admission, domain.IndeterminateAdmission)
	}
}

func TestExtractTiDBQueryAccess_DefaultSchemaPropagated(t *testing.T) {
	t.Parallel()
	result, err := appqa.ExtractTiDBQueryAccess(context.Background(), appqa.QueryAccessRequest{
		SQL:           "SELECT id FROM users",
		Dialect:       "mysql",
		DefaultSchema: "app",
	})
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if len(result.DomainResult.Relations) != 1 {
		t.Fatalf("relations: got %d, want 1", len(result.DomainResult.Relations))
	}
	if result.DomainResult.Relations[0].Schema != "app" {
		t.Errorf("schema: got %q, want %q", result.DomainResult.Relations[0].Schema, "app")
	}
}

func TestExtractTiDBQueryAccess_ModeNormalization(t *testing.T) {
	t.Parallel()
	result, err := appqa.ExtractTiDBQueryAccess(context.Background(), appqa.QueryAccessRequest{
		SQL:     "SELECT id FROM users",
		Dialect: "mysql",
		Mode:    "",
	})
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if result.DomainResult.Mode != domain.ModeStrict {
		t.Errorf("mode: got %q, want %q", result.DomainResult.Mode, domain.ModeStrict)
	}
}

func TestExtractTiDBQueryAccess_DDLNotReadOnly(t *testing.T) {
	t.Parallel()
	result, err := appqa.ExtractTiDBQueryAccess(context.Background(), appqa.QueryAccessRequest{
		SQL:     "CREATE TABLE t1 (id INT)",
		Dialect: "mysql",
	})
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if result.DomainResult.ReadClassification != domain.NotReadOnly {
		t.Errorf("classification: got %q, want %q", result.DomainResult.ReadClassification, domain.NotReadOnly)
	}
}

func TestExtractTiDBQueryAccess_ColumnUsagesPreserved(t *testing.T) {
	t.Parallel()
	result, err := appqa.ExtractTiDBQueryAccess(context.Background(), appqa.QueryAccessRequest{
		SQL:     "SELECT id FROM users WHERE id = 1",
		Dialect: "mysql",
	})
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	found := false
	for _, col := range result.DomainResult.ReferencedColumns {
		if col.Column == "id" && col.Table == "users" {
			found = true
			hasProjection := false
			hasFilter := false
			for _, u := range col.Usages {
				if u == domain.UsageProjection {
					hasProjection = true
				}
				if u == domain.UsageFilter {
					hasFilter = true
				}
			}
			if !hasProjection {
				t.Errorf("expected projection usage")
			}
			if !hasFilter {
				t.Errorf("expected filter usage")
			}
		}
	}
	if !found {
		t.Errorf("column users.id not found")
	}
}
