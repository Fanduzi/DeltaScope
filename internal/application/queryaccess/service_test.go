package queryaccess_test

import (
	"context"
	"errors"
	"fmt"
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

func TestService_Analyze_OutputPreservesDeclarationOrder(t *testing.T) {
	t.Parallel()
	svc := &appqa.Service{}

	result, err := svc.Analyze(context.Background(), appqa.QueryAccessRequest{
		SQL:     "SELECT b, a FROM users",
		Dialect: "mysql",
		Mode:    "strict",
	})
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}

	dr := result.DomainResult
	if len(dr.Outputs) < 2 {
		t.Fatalf("outputs: got %d, want >= 2", len(dr.Outputs))
	}
	if dr.Outputs[0].Name != "b" {
		t.Errorf("outputs[0].Name: got %q, want %q", dr.Outputs[0].Name, "b")
	}
	if dr.Outputs[1].Name != "a" {
		t.Errorf("outputs[1].Name: got %q, want %q", dr.Outputs[1].Name, "a")
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

func TestService_Analyze_RelationLookupFailsStrict(t *testing.T) {
	t.Parallel()
	svc := &appqa.Service{}

	resolver := newFakeResolver(map[string]appqa.RelationSchema{})

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
	if dr.Admission != domain.IndeterminateAdmission {
		t.Errorf("admission: got %q, want %q", dr.Admission, domain.IndeterminateAdmission)
	}

	hasRelationNotFound := false
	for _, u := range dr.Unresolved {
		if u.Reason == appqa.ReasonRelationNotFound {
			hasRelationNotFound = true
			break
		}
	}
	if !hasRelationNotFound {
		t.Error("expected relation_not_found unresolved entry")
	}
}

func TestService_Analyze_RelationLookupFailsProjectionOnly(t *testing.T) {
	t.Parallel()
	svc := &appqa.Service{}

	resolver := newFakeResolver(map[string]appqa.RelationSchema{})

	result, err := svc.Analyze(context.Background(), appqa.QueryAccessRequest{
		SQL:            "SELECT id FROM users",
		Dialect:        "mysql",
		Mode:           "projection_only",
		DefaultSchema:  "app",
		SchemaResolver: resolver,
	})
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}

	dr := result.DomainResult
	if dr.Admission != domain.IndeterminateAdmission {
		t.Errorf("admission: got %q, want %q", dr.Admission, domain.IndeterminateAdmission)
	}
}

func TestService_Analyze_ColumnLookupFails(t *testing.T) {
	t.Parallel()
	svc := &appqa.Service{}

	resolver := newFakeResolver(map[string]appqa.RelationSchema{
		"app.users": {
			Schema: "app", Name: "users", Kind: "table",
			Columns: []appqa.ColumnSchema{{Name: "id", Ordinal: 1}},
		},
	})

	result, err := svc.Analyze(context.Background(), appqa.QueryAccessRequest{
		SQL:            "SELECT users.missing FROM users",
		Dialect:        "mysql",
		Mode:           "strict",
		DefaultSchema:  "app",
		SchemaResolver: resolver,
	})
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}

	dr := result.DomainResult
	if dr.Admission != domain.IndeterminateAdmission {
		t.Errorf("admission: got %q, want %q", dr.Admission, domain.IndeterminateAdmission)
	}

	hasColumnNotFound := false
	for _, u := range dr.Unresolved {
		if u.Reason == appqa.ReasonColumnNotFound {
			hasColumnNotFound = true
			break
		}
	}
	if !hasColumnNotFound {
		t.Error("expected column_not_found unresolved entry")
	}
}

func TestService_Analyze_ProviderError(t *testing.T) {
	t.Parallel()
	svc := &appqa.Service{}

	resolver := &errorResolver{err: errors.New("connection refused")}

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
	if dr.Admission != domain.IndeterminateAdmission {
		t.Errorf("admission: got %q, want %q", dr.Admission, domain.IndeterminateAdmission)
	}
}

func TestService_Analyze_WildcardExpansionProducesPhysicalColumns(t *testing.T) {
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
		Mode:           "strict",
		DefaultSchema:  "app",
		SchemaResolver: resolver,
	})
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}

	dr := result.DomainResult

	if len(dr.ReferencedColumns) != 3 {
		t.Fatalf("ReferencedColumns: got %d, want 3", len(dr.ReferencedColumns))
	}
	colSet := make(map[string]bool)
	for _, col := range dr.ReferencedColumns {
		colSet[col.Column] = true
	}
	for _, want := range []string{"id", "name", "email"} {
		if !colSet[want] {
			t.Errorf("ReferencedColumns missing column %q", want)
		}
	}

	if dr.ReadClassification != domain.ReadOnly {
		t.Errorf("ReadClassification: got %q, want %q", dr.ReadClassification, domain.ReadOnly)
	}

	if dr.Admission != domain.Admissible {
		t.Errorf("Admission: got %q, want %q", dr.Admission, domain.Admissible)
	}

	readColumnCount := 0
	for _, req := range dr.Requirements {
		if req.Privilege == "read_column" {
			readColumnCount++
		}
	}
	if readColumnCount != 3 {
		t.Errorf("read_column requirements: got %d, want 3", readColumnCount)
	}

	for _, u := range dr.Unresolved {
		if u.Reason == domain.ReasonSchemaUnavailable || u.Reason == appqa.ReasonUnresolvedWildcard {
			t.Errorf("wildcard unresolved should be removed: %s %s", u.Reference, u.Reason)
		}
	}

	if len(dr.Outputs) != 1 {
		t.Fatalf("Outputs: got %d, want 1", len(dr.Outputs))
	}
	if len(dr.Outputs[0].Sources) == 0 {
		t.Error("Output sources should not be empty after wildcard expansion")
	}
}

func TestService_Analyze_PartialWildcardAB(t *testing.T) {
	t.Parallel()
	svc := &appqa.Service{}

	resolver := newFakeResolver(map[string]appqa.RelationSchema{
		"schema_a.a": {
			Schema: "schema_a", Name: "a", Kind: "table",
			Columns: []appqa.ColumnSchema{{Name: "id", Ordinal: 1}},
		},
	})

	result, err := svc.Analyze(context.Background(), appqa.QueryAccessRequest{
		SQL:            "SELECT a.*, b.* FROM schema_a.a a JOIN schema_b.b b ON a.id = b.id",
		Dialect:        "mysql",
		Mode:           "strict",
		DefaultSchema:  "",
		SchemaResolver: resolver,
	})
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}

	dr := result.DomainResult

	aExpanded := false
	for _, col := range dr.ReferencedColumns {
		if col.Schema == "schema_a" && col.Table == "a" && col.Column == "id" {
			aExpanded = true
		}
	}
	if !aExpanded {
		t.Error("wildcard a.* should have been expanded to physical columns")
	}

	hasBUnresolved := false
	for _, u := range dr.Unresolved {
		if u.Reason == appqa.ReasonRelationNotFound || u.Reason == appqa.ReasonUnresolvedWildcard {
			hasBUnresolved = true
		}
	}
	if !hasBUnresolved {
		t.Error("unresolved for b.* should persist when b relation not found")
	}

	if dr.Admission != domain.IndeterminateAdmission {
		t.Errorf("Admission: got %q, want %q", dr.Admission, domain.IndeterminateAdmission)
	}
}

func TestService_Analyze_BothWildcardsSucceed(t *testing.T) {
	t.Parallel()
	svc := &appqa.Service{}

	resolver := newFakeResolver(map[string]appqa.RelationSchema{
		"schema_a.a": {
			Schema: "schema_a", Name: "a", Kind: "table",
			Columns: []appqa.ColumnSchema{{Name: "id", Ordinal: 1}},
		},
		"schema_b.b": {
			Schema: "schema_b", Name: "b", Kind: "table",
			Columns: []appqa.ColumnSchema{{Name: "id", Ordinal: 1}, {Name: "val", Ordinal: 2}},
		},
	})

	result, err := svc.Analyze(context.Background(), appqa.QueryAccessRequest{
		SQL:            "SELECT a.*, b.* FROM schema_a.a a JOIN schema_b.b b ON a.id = b.id",
		Dialect:        "mysql",
		Mode:           "strict",
		DefaultSchema:  "",
		SchemaResolver: resolver,
	})
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}

	dr := result.DomainResult

	if len(dr.ReferencedColumns) < 3 {
		t.Errorf("ReferencedColumns: got %d, want >= 3", len(dr.ReferencedColumns))
	}

	aHasID := false
	bHasID := false
	bHasVal := false
	for _, col := range dr.ReferencedColumns {
		if col.Schema == "schema_a" && col.Table == "a" && col.Column == "id" {
			aHasID = true
		}
		if col.Schema == "schema_b" && col.Table == "b" && col.Column == "id" {
			bHasID = true
		}
		if col.Schema == "schema_b" && col.Table == "b" && col.Column == "val" {
			bHasVal = true
		}
	}
	if !aHasID {
		t.Error("a.id should be in ReferencedColumns")
	}
	if !bHasID {
		t.Error("b.id should be in ReferencedColumns")
	}
	if !bHasVal {
		t.Error("b.val should be in ReferencedColumns")
	}

	for _, u := range dr.Unresolved {
		if u.Reason == domain.ReasonSchemaUnavailable || u.Reason == appqa.ReasonUnresolvedWildcard {
			t.Errorf("wildcard unresolved should be removed: %s %s", u.Reference, u.Reason)
		}
	}

	if dr.Admission != domain.Admissible {
		t.Errorf("Admission: got %q, want %q", dr.Admission, domain.Admissible)
	}
}

func TestService_Analyze_BothWildcardsFail(t *testing.T) {
	t.Parallel()
	svc := &appqa.Service{}

	resolver := newFakeResolver(map[string]appqa.RelationSchema{})

	result, err := svc.Analyze(context.Background(), appqa.QueryAccessRequest{
		SQL:            "SELECT a.*, b.* FROM schema_a.a a JOIN schema_b.b b ON a.id = b.id",
		Dialect:        "mysql",
		Mode:           "strict",
		DefaultSchema:  "",
		SchemaResolver: resolver,
	})
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}

	dr := result.DomainResult

	if len(dr.Unresolved) < 2 {
		t.Errorf("Unresolved: got %d, want at least 2", len(dr.Unresolved))
	}

	if dr.Admission != domain.IndeterminateAdmission {
		t.Errorf("Admission: got %q, want %q", dr.Admission, domain.IndeterminateAdmission)
	}
}

func TestService_Analyze_GlobalStarWithResolver(t *testing.T) {
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
		SQL:            "SELECT * FROM users",
		Dialect:        "mysql",
		Mode:           "strict",
		DefaultSchema:  "app",
		SchemaResolver: resolver,
	})
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}

	dr := result.DomainResult

	if len(dr.ReferencedColumns) != 2 {
		t.Errorf("ReferencedColumns: got %d, want 2", len(dr.ReferencedColumns))
	}

	if dr.ReadClassification != domain.ReadOnly {
		t.Errorf("ReadClassification: got %q, want %q", dr.ReadClassification, domain.ReadOnly)
	}

	if dr.Admission != domain.Admissible {
		t.Errorf("Admission: got %q, want %q", dr.Admission, domain.Admissible)
	}
}

func TestService_Analyze_WildcardProjectionOnly(t *testing.T) {
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
		SQL:            "SELECT * FROM users WHERE id = 1",
		Dialect:        "mysql",
		Mode:           "projection_only",
		DefaultSchema:  "app",
		SchemaResolver: resolver,
	})
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}

	dr := result.DomainResult

	if len(dr.ReferencedColumns) < 2 {
		t.Errorf("ReferencedColumns: got %d, want >= 2", len(dr.ReferencedColumns))
	}

	if dr.Admission != domain.Admissible {
		t.Errorf("Admission: got %q, want %q", dr.Admission, domain.Admissible)
	}
}

func TestService_Analyze_WildcardNoLeak(t *testing.T) {
	t.Parallel()
	svc := &appqa.Service{}

	resolver := newFakeResolver(map[string]appqa.RelationSchema{
		"app.users": {
			Schema: "app", Name: "users", Kind: "table",
			Columns: []appqa.ColumnSchema{{Name: "id", Ordinal: 1}},
		},
	})

	result, err := svc.Analyze(context.Background(), appqa.QueryAccessRequest{
		SQL:            "SELECT * FROM users",
		Dialect:        "mysql",
		Mode:           "strict",
		DefaultSchema:  "app",
		SchemaResolver: resolver,
	})
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}

	dr := result.DomainResult

	j := fmt.Sprintf("%+v", dr)
	forbidden := []string{"SELECT", "password", "credential", "secret", "token"}
	for _, f := range forbidden {
		if contains(j, f) {
			t.Errorf("result should not contain %q", f)
		}
	}
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestService_Analyze_AmbiguousRelationNoSchema(t *testing.T) {
	t.Parallel()
	svc := &appqa.Service{}

	resolver := newFakeResolver(map[string]appqa.RelationSchema{
		"schema_a.users": {
			Schema: "schema_a", Name: "users", Kind: "table",
			Columns: []appqa.ColumnSchema{{Name: "id", Ordinal: 1}},
		},
		"schema_b.users": {
			Schema: "schema_b", Name: "users", Kind: "table",
			Columns: []appqa.ColumnSchema{{Name: "id", Ordinal: 1}},
		},
	})

	result, err := svc.Analyze(context.Background(), appqa.QueryAccessRequest{
		SQL:            "SELECT id FROM users",
		Dialect:        "mysql",
		Mode:           "strict",
		DefaultSchema:  "",
		SchemaResolver: resolver,
	})
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}

	dr := result.DomainResult
	if dr.Admission != domain.IndeterminateAdmission {
		t.Errorf("admission: got %q, want %q", dr.Admission, domain.IndeterminateAdmission)
	}
}

func TestService_Analyze_GlobalWildcardDeterministicOrder(t *testing.T) {
	t.Parallel()
	svc := &appqa.Service{}

	resolver := newFakeResolver(map[string]appqa.RelationSchema{
		"app.b": {
			Schema: "app", Name: "b", Kind: "table",
			Columns: []appqa.ColumnSchema{{Name: "id", Ordinal: 1}},
		},
		"app.a": {
			Schema: "app", Name: "a", Kind: "table",
			Columns: []appqa.ColumnSchema{{Name: "id", Ordinal: 1}, {Name: "name", Ordinal: 2}},
		},
	})

	expected := []string{"app.b.id", "app.a.id", "app.a.name"}
	for i := 0; i < 10; i++ {
		result, err := svc.Analyze(context.Background(), appqa.QueryAccessRequest{
			SQL:            "SELECT * FROM b JOIN a ON b.id = a.id",
			Dialect:        "mysql",
			Mode:           "strict",
			DefaultSchema:  "app",
			SchemaResolver: resolver,
		})
		if err != nil {
			t.Fatalf("run %d: analyze: %v", i, err)
		}

		dr := result.DomainResult
		if len(dr.Outputs) == 0 {
			t.Fatalf("run %d: no outputs", i)
		}
		if len(dr.Outputs[0].Sources) != len(expected) {
			t.Fatalf("run %d: sources count: got %d, want %d", i, len(dr.Outputs[0].Sources), len(expected))
		}
		for j, want := range expected {
			if dr.Outputs[0].Sources[j] != want {
				t.Errorf("run %d: sources[%d]: got %q, want %q", i, j, dr.Outputs[0].Sources[j], want)
			}
		}
	}
}

func TestService_Analyze_GlobalWildcardReversedJoinOrder(t *testing.T) {
	t.Parallel()
	svc := &appqa.Service{}

	resolver := newFakeResolver(map[string]appqa.RelationSchema{
		"app.b": {
			Schema: "app", Name: "b", Kind: "table",
			Columns: []appqa.ColumnSchema{{Name: "id", Ordinal: 1}},
		},
		"app.a": {
			Schema: "app", Name: "a", Kind: "table",
			Columns: []appqa.ColumnSchema{{Name: "id", Ordinal: 1}, {Name: "name", Ordinal: 2}},
		},
	})

	expected := []string{"app.a.id", "app.a.name", "app.b.id"}
	for i := 0; i < 10; i++ {
		result, err := svc.Analyze(context.Background(), appqa.QueryAccessRequest{
			SQL:            "SELECT * FROM a JOIN b ON a.id = b.id",
			Dialect:        "mysql",
			Mode:           "strict",
			DefaultSchema:  "app",
			SchemaResolver: resolver,
		})
		if err != nil {
			t.Fatalf("run %d: analyze: %v", i, err)
		}

		dr := result.DomainResult
		if len(dr.Outputs) == 0 {
			t.Fatalf("run %d: no outputs", i)
		}
		if len(dr.Outputs[0].Sources) != len(expected) {
			t.Fatalf("run %d: sources count: got %d, want %d", i, len(dr.Outputs[0].Sources), len(expected))
		}
		for j, want := range expected {
			if dr.Outputs[0].Sources[j] != want {
				t.Errorf("run %d: sources[%d]: got %q, want %q", i, j, dr.Outputs[0].Sources[j], want)
			}
		}
	}
}

func TestService_Analyze_TableQualifiedWildcardOrder(t *testing.T) {
	t.Parallel()
	svc := &appqa.Service{}

	resolver := newFakeResolver(map[string]appqa.RelationSchema{
		"app.b": {
			Schema: "app", Name: "b", Kind: "table",
			Columns: []appqa.ColumnSchema{{Name: "id", Ordinal: 1}, {Name: "val", Ordinal: 2}},
		},
		"app.a": {
			Schema: "app", Name: "a", Kind: "table",
			Columns: []appqa.ColumnSchema{{Name: "id", Ordinal: 1}, {Name: "name", Ordinal: 2}},
		},
	})

	result, err := svc.Analyze(context.Background(), appqa.QueryAccessRequest{
		SQL:            "SELECT b.* FROM b JOIN a ON b.id = a.id",
		Dialect:        "mysql",
		Mode:           "strict",
		DefaultSchema:  "app",
		SchemaResolver: resolver,
	})
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}

	dr := result.DomainResult
	if len(dr.Outputs) == 0 {
		t.Fatal("no outputs")
	}

	expected := []string{"app.b.id", "app.b.val"}
	if len(dr.Outputs[0].Sources) != len(expected) {
		t.Fatalf("sources count: got %d, want %d", len(dr.Outputs[0].Sources), len(expected))
	}
	for j, want := range expected {
		if dr.Outputs[0].Sources[j] != want {
			t.Errorf("sources[%d]: got %q, want %q", j, dr.Outputs[0].Sources[j], want)
		}
	}

	hasAColProjection := false
	for _, col := range dr.ReferencedColumns {
		if col.Table == "a" {
			for _, u := range col.Usages {
				if u == domain.UsageProjection {
					hasAColProjection = true
				}
			}
		}
	}
	if hasAColProjection {
		t.Error("table-qualified b.* should not include projection columns from a")
	}
}
