package queryaccess_test

import (
	"context"
	"testing"

	appqa "github.com/Fanduzi/DeltaScope/internal/application/queryaccess"
	domain "github.com/Fanduzi/DeltaScope/internal/domain/queryaccess"
)

func TestService_Analyze_OfflineModeNoResolver(t *testing.T) {
	t.Parallel()
	svc := &appqa.Service{}

	result, err := svc.Analyze(context.Background(), appqa.QueryAccessRequest{
		SQL:     "SELECT id, name FROM users",
		Dialect: "mysql",
		Mode:    "strict",
	})
	if err != nil {
		t.Fatalf("analyze: %v", err)
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
}

func TestService_Analyze_WithResolver(t *testing.T) {
	t.Parallel()
	svc := &appqa.Service{}

	resolver := newFakeResolver(map[string]appqa.RelationSchema{
		"app.users": {
			Schema: "app", Name: "users", Kind: "table",
			Columns: []appqa.ColumnSchema{
				{Name: "id", Ordinal: 1},
				{Name: "name", Ordinal: 2},
			},
		},
	})

	result, err := svc.Analyze(context.Background(), appqa.QueryAccessRequest{
		SQL:            "SELECT id FROM users",
		Dialect:        "mysql",
		Mode:           "strict",
		DefaultSchema:  "app",
		SchemaResolver: resolver,
	})
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}

	dr := result.DomainResult
	if dr.ReadClassification != domain.ReadOnly {
		t.Errorf("classification: got %q, want %q", dr.ReadClassification, domain.ReadOnly)
	}
	if len(dr.Relations) != 1 {
		t.Fatalf("relations: got %d, want 1", len(dr.Relations))
	}
	if dr.Relations[0].Schema != "app" {
		t.Errorf("resolved schema: got %q, want %q", dr.Relations[0].Schema, "app")
	}
}

func TestService_Analyze_ModeNormalization(t *testing.T) {
	t.Parallel()
	svc := &appqa.Service{}

	result, err := svc.Analyze(context.Background(), appqa.QueryAccessRequest{
		SQL:     "SELECT id FROM users",
		Dialect: "mysql",
		Mode:    "",
	})
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}

	if result.DomainResult.Mode != domain.ModeStrict {
		t.Errorf("mode: got %q, want %q", result.DomainResult.Mode, domain.ModeStrict)
	}
}

func TestService_Analyze_UnsupportedDialect(t *testing.T) {
	t.Parallel()
	svc := &appqa.Service{}

	_, err := svc.Analyze(context.Background(), appqa.QueryAccessRequest{
		SQL:     "SELECT id FROM users",
		Dialect: "oracle",
	})
	if err == nil {
		t.Error("expected error for unsupported dialect")
	}
}

func TestService_Analyze_ClassificationPreservedThroughResolution(t *testing.T) {
	t.Parallel()
	svc := &appqa.Service{}

	resolver := newFakeResolver(map[string]appqa.RelationSchema{
		"app.users": {
			Schema: "app", Name: "users", Kind: "table",
			Columns: []appqa.ColumnSchema{{Name: "id", Ordinal: 1}},
		},
	})

	result, err := svc.Analyze(context.Background(), appqa.QueryAccessRequest{
		SQL:            "SELECT * FROM users FOR UPDATE",
		Dialect:        "mysql",
		DefaultSchema:  "app",
		SchemaResolver: resolver,
	})
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}

	if result.DomainResult.ReadClassification != domain.NotReadOnly {
		t.Errorf("classification: got %q, want %q", result.DomainResult.ReadClassification, domain.NotReadOnly)
	}
	if result.DomainResult.Admission != domain.Rejected {
		t.Errorf("admission: got %q, want %q", result.DomainResult.Admission, domain.Rejected)
	}
}

func TestService_Analyze_Cancellation(t *testing.T) {
	t.Parallel()
	svc := &appqa.Service{}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := svc.Analyze(ctx, appqa.QueryAccessRequest{
		SQL:     "SELECT id FROM users",
		Dialect: "mysql",
	})
	if err == nil {
		t.Error("expected error for canceled context")
	}
}

func TestService_Analyze_WildcardExpansionWithResolver(t *testing.T) {
	t.Parallel()
	svc := &appqa.Service{}

	resolver := newFakeResolver(map[string]appqa.RelationSchema{
		"app.users": {
			Schema: "app", Name: "users", Kind: "table",
			Columns: []appqa.ColumnSchema{
				{Name: "id", Ordinal: 1},
				{Name: "name", Ordinal: 2},
				{Name: "email", Ordinal: 3},
			},
		},
	})

	result, err := svc.Analyze(context.Background(), appqa.QueryAccessRequest{
		SQL:            "SELECT * FROM users",
		Dialect:        "mysql",
		DefaultSchema:  "app",
		SchemaResolver: resolver,
	})
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}

	dr := result.DomainResult
	// TiDB parser puts * into Unresolved, not ReferencedColumns.
	// With resolver, the wildcard unresolved entry should be removed.
	for _, u := range dr.Unresolved {
		if u.Reference == "*" && (u.Reason == domain.ReasonSchemaUnavailable || u.Reason == appqa.ReasonUnresolvedWildcard) {
			t.Error("wildcard unresolved entry should be removed after metadata resolution")
		}
	}
	// Relation schema should be resolved
	if len(dr.Relations) != 1 {
		t.Fatalf("relations: got %d, want 1", len(dr.Relations))
	}
	if dr.Relations[0].Schema != "app" {
		t.Errorf("resolved schema: got %q, want %q", dr.Relations[0].Schema, "app")
	}
}

func TestService_Analyze_UnknownFunctionEffect(t *testing.T) {
	t.Parallel()
	svc := &appqa.Service{}

	result, err := svc.Analyze(context.Background(), appqa.QueryAccessRequest{
		SQL:     "SELECT unknown_func(id) FROM users",
		Dialect: "mysql",
		Mode:    "strict",
	})
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}

	dr := result.DomainResult
	if dr.ReadClassification != domain.Indeterminate {
		t.Errorf("classification: got %q, want %q", dr.ReadClassification, domain.Indeterminate)
	}
	if dr.Admission != domain.IndeterminateAdmission {
		t.Errorf("admission: got %q, want %q", dr.Admission, domain.IndeterminateAdmission)
	}

	hasFunctionEffect := false
	for _, rc := range dr.ReasonCodes {
		if rc == domain.ReasonFunctionEffect {
			hasFunctionEffect = true
			break
		}
	}
	if !hasFunctionEffect {
		t.Errorf("expected reason_codes to include %q, got %v", domain.ReasonFunctionEffect, dr.ReasonCodes)
	}
}
