package queryaccess_test

import (
	"context"
	"errors"
	"testing"

	appqa "github.com/Fanduzi/DeltaScope/internal/application/queryaccess"
	domain "github.com/Fanduzi/DeltaScope/internal/domain/queryaccess"
)

// fakeResolver is an in-memory SchemaResolver for testing.
type fakeResolver struct {
	schemas map[string]appqa.RelationSchema
	err     error
}

func (f *fakeResolver) ResolveRelation(_ context.Context, _ string, schema, name string) (appqa.RelationSchema, error) {
	if f.err != nil {
		return appqa.RelationSchema{}, f.err
	}
	key := formatFakeKey(schema, name)
	rs, ok := f.schemas[key]
	if !ok {
		return appqa.RelationSchema{}, errors.New("relation not found: " + key)
	}
	return rs, nil
}

func formatFakeKey(schema, name string) string {
	if schema == "" {
		return name
	}
	return schema + "." + name
}

func newFakeResolver(schemas map[string]appqa.RelationSchema) *fakeResolver {
	return &fakeResolver{schemas: schemas}
}

func TestResolveMetadata_SchemaDefaulting(t *testing.T) {
	t.Parallel()
	resolver := newFakeResolver(map[string]appqa.RelationSchema{
		"app.users": {
			Schema: "app", Name: "users", Kind: "table",
			Columns: []appqa.ColumnSchema{{Name: "id", Ordinal: 1}},
		},
	})

	result := domain.Result{
		Dialect: "mysql",
		Mode:    domain.ModeStrict,
		Relations: []domain.RelationReference{
			{Name: "users", Kind: domain.RelationTable, PermissionRequired: true},
		},
		ReferencedColumns: []domain.ColumnReference{
			{Table: "users", Column: "id", Usages: []domain.UsageContext{domain.UsageProjection}},
		},
	}

	resolved := appqa.ResolveMetadata(context.Background(), resolver, "mysql", "app", result)

	if len(resolved.Relations) != 1 {
		t.Fatalf("relations: got %d, want 1", len(resolved.Relations))
	}
	if resolved.Relations[0].Schema != "app" {
		t.Errorf("relation schema: got %q, want %q", resolved.Relations[0].Schema, "app")
	}
}

func TestResolveMetadata_CanonicalCacheKeys(t *testing.T) {
	t.Parallel()
	var callCount int
	resolver := &countingResolver{
		schemas: map[string]appqa.RelationSchema{
			"app.users": {
				Schema: "app", Name: "users", Kind: "table",
				Columns: []appqa.ColumnSchema{{Name: "id", Ordinal: 1}},
			},
		},
		count: &callCount,
	}

	result := domain.Result{
		Dialect: "mysql",
		Mode:    domain.ModeStrict,
		Relations: []domain.RelationReference{
			{Schema: "app", Name: "users", Kind: domain.RelationTable, PermissionRequired: true},
			{Schema: "app", Name: "users", Alias: "u", Kind: domain.RelationTable, PermissionRequired: true},
		},
		ReferencedColumns: []domain.ColumnReference{
			{Schema: "app", Table: "users", Column: "id", Usages: []domain.UsageContext{domain.UsageProjection}},
			{Table: "u", Column: "id", Usages: []domain.UsageContext{domain.UsageFilter}},
		},
	}

	appqa.ResolveMetadata(context.Background(), resolver, "mysql", "app", result)

	// Should resolve only once for the same schema.name key
	if callCount != 1 {
		t.Errorf("expected 1 resolver call (cached), got %d", callCount)
	}
}

type countingResolver struct {
	schemas map[string]appqa.RelationSchema
	count   *int
}

func (c *countingResolver) ResolveRelation(ctx context.Context, dialect string, schema, name string) (appqa.RelationSchema, error) {
	*c.count++
	resolver := newFakeResolver(c.schemas)
	return resolver.ResolveRelation(ctx, dialect, schema, name)
}

func TestResolveMetadata_QualifiedColumn(t *testing.T) {
	t.Parallel()
	resolver := newFakeResolver(map[string]appqa.RelationSchema{
		"app.users": {
			Schema: "app", Name: "users", Kind: "table",
			Columns: []appqa.ColumnSchema{
				{Name: "id", Ordinal: 1},
				{Name: "name", Ordinal: 2},
			},
		},
	})

	result := domain.Result{
		Dialect: "mysql",
		Mode:    domain.ModeStrict,
		Relations: []domain.RelationReference{
			{Schema: "app", Name: "users", Kind: domain.RelationTable, PermissionRequired: true},
		},
		ReferencedColumns: []domain.ColumnReference{
			{Table: "users", Column: "name", Usages: []domain.UsageContext{domain.UsageProjection}},
		},
	}

	resolved := appqa.ResolveMetadata(context.Background(), resolver, "mysql", "app", result)

	if len(resolved.ReferencedColumns) != 1 {
		t.Fatalf("columns: got %d, want 1", len(resolved.ReferencedColumns))
	}
	col := resolved.ReferencedColumns[0]
	if col.Schema != "app" {
		t.Errorf("column schema: got %q, want %q", col.Schema, "app")
	}
	if col.Table != "users" {
		t.Errorf("column table: got %q, want %q", col.Table, "users")
	}
}

func TestResolveMetadata_UnqualifiedSingleMatch(t *testing.T) {
	t.Parallel()
	resolver := newFakeResolver(map[string]appqa.RelationSchema{
		"app.users": {
			Schema: "app", Name: "users", Kind: "table",
			Columns: []appqa.ColumnSchema{
				{Name: "id", Ordinal: 1},
				{Name: "email", Ordinal: 2},
			},
		},
	})

	result := domain.Result{
		Dialect: "mysql",
		Mode:    domain.ModeStrict,
		Relations: []domain.RelationReference{
			{Schema: "app", Name: "users", Kind: domain.RelationTable, PermissionRequired: true},
		},
		ReferencedColumns: []domain.ColumnReference{
			{Column: "email", Usages: []domain.UsageContext{domain.UsageFilter}},
		},
	}

	resolved := appqa.ResolveMetadata(context.Background(), resolver, "mysql", "app", result)

	if len(resolved.ReferencedColumns) != 1 {
		t.Fatalf("columns: got %d, want 1", len(resolved.ReferencedColumns))
	}
	col := resolved.ReferencedColumns[0]
	if col.Schema != "app" || col.Table != "users" {
		t.Errorf("unqualified column should resolve to app.users, got %s.%s", col.Schema, col.Table)
	}
}

func TestResolveMetadata_AmbiguousMultiMatch(t *testing.T) {
	t.Parallel()
	resolver := newFakeResolver(map[string]appqa.RelationSchema{
		"app.users": {
			Schema: "app", Name: "users", Kind: "table",
			Columns: []appqa.ColumnSchema{{Name: "id", Ordinal: 1}},
		},
		"app.orders": {
			Schema: "app", Name: "orders", Kind: "table",
			Columns: []appqa.ColumnSchema{{Name: "id", Ordinal: 1}},
		},
	})

	result := domain.Result{
		Dialect: "mysql",
		Mode:    domain.ModeStrict,
		Relations: []domain.RelationReference{
			{Schema: "app", Name: "users", Kind: domain.RelationTable, PermissionRequired: true},
			{Schema: "app", Name: "orders", Kind: domain.RelationTable, PermissionRequired: true},
		},
		ReferencedColumns: []domain.ColumnReference{
			{Column: "id", Usages: []domain.UsageContext{domain.UsageProjection}},
		},
	}

	resolved := appqa.ResolveMetadata(context.Background(), resolver, "mysql", "app", result)

	// Ambiguous: should remain unqualified
	if len(resolved.ReferencedColumns) != 1 {
		t.Fatalf("columns: got %d, want 1", len(resolved.ReferencedColumns))
	}
	col := resolved.ReferencedColumns[0]
	if col.Schema != "" || col.Table != "" {
		t.Errorf("ambiguous column should remain unqualified, got %s.%s", col.Schema, col.Table)
	}
}

func TestResolveMetadata_MissingObject(t *testing.T) {
	t.Parallel()
	resolver := newFakeResolver(map[string]appqa.RelationSchema{})

	result := domain.Result{
		Dialect: "mysql",
		Mode:    domain.ModeStrict,
		Relations: []domain.RelationReference{
			{Schema: "app", Name: "nonexistent", Kind: domain.RelationTable, PermissionRequired: true},
		},
		ReferencedColumns: []domain.ColumnReference{
			{Table: "nonexistent", Column: "id", Usages: []domain.UsageContext{domain.UsageProjection}},
		},
	}

	resolved := appqa.ResolveMetadata(context.Background(), resolver, "mysql", "app", result)

	// Should preserve original references when object not found
	if len(resolved.Relations) != 1 {
		t.Fatalf("relations: got %d, want 1", len(resolved.Relations))
	}
	if resolved.Relations[0].Schema != "app" {
		t.Errorf("relation schema preserved: got %q", resolved.Relations[0].Schema)
	}
}

func TestResolveMetadata_MissingColumn(t *testing.T) {
	t.Parallel()
	resolver := newFakeResolver(map[string]appqa.RelationSchema{
		"app.users": {
			Schema: "app", Name: "users", Kind: "table",
			Columns: []appqa.ColumnSchema{{Name: "id", Ordinal: 1}},
		},
	})

	result := domain.Result{
		Dialect: "mysql",
		Mode:    domain.ModeStrict,
		Relations: []domain.RelationReference{
			{Schema: "app", Name: "users", Kind: domain.RelationTable, PermissionRequired: true},
		},
		ReferencedColumns: []domain.ColumnReference{
			{Table: "users", Column: "nonexistent_col", Usages: []domain.UsageContext{domain.UsageProjection}},
		},
	}

	resolved := appqa.ResolveMetadata(context.Background(), resolver, "mysql", "app", result)

	// Column not found in relation: should preserve original reference
	if len(resolved.ReferencedColumns) != 1 {
		t.Fatalf("columns: got %d, want 1", len(resolved.ReferencedColumns))
	}
	col := resolved.ReferencedColumns[0]
	if col.Column != "nonexistent_col" {
		t.Errorf("column name preserved: got %q", col.Column)
	}
}

func TestResolveMetadata_ProviderError(t *testing.T) {
	t.Parallel()
	resolver := &errorResolver{err: errors.New("connection refused")}

	result := domain.Result{
		Dialect: "mysql",
		Mode:    domain.ModeStrict,
		Relations: []domain.RelationReference{
			{Schema: "app", Name: "users", Kind: domain.RelationTable, PermissionRequired: true},
		},
		ReferencedColumns: []domain.ColumnReference{
			{Table: "users", Column: "id", Usages: []domain.UsageContext{domain.UsageProjection}},
		},
	}

	resolved := appqa.ResolveMetadata(context.Background(), resolver, "mysql", "app", result)

	// Should preserve original references on error
	if len(resolved.Relations) != 1 {
		t.Fatalf("relations: got %d, want 1", len(resolved.Relations))
	}
	if len(resolved.ReferencedColumns) != 1 {
		t.Fatalf("columns: got %d, want 1", len(resolved.ReferencedColumns))
	}
}

type errorResolver struct {
	err error
}

func (e *errorResolver) ResolveRelation(_ context.Context, _ string, _, _ string) (appqa.RelationSchema, error) {
	return appqa.RelationSchema{}, e.err
}

func TestResolveMetadata_Cancellation(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	resolver := newFakeResolver(map[string]appqa.RelationSchema{
		"app.users": {
			Schema: "app", Name: "users", Kind: "table",
			Columns: []appqa.ColumnSchema{{Name: "id", Ordinal: 1}},
		},
	})

	result := domain.Result{
		Dialect: "mysql",
		Mode:    domain.ModeStrict,
		Relations: []domain.RelationReference{
			{Schema: "app", Name: "users", Kind: domain.RelationTable, PermissionRequired: true},
		},
		ReferencedColumns: []domain.ColumnReference{
			{Table: "users", Column: "id", Usages: []domain.UsageContext{domain.UsageProjection}},
		},
	}

	resolved := appqa.ResolveMetadata(ctx, resolver, "mysql", "app", result)

	// Should preserve original on cancellation
	if len(resolved.Relations) != 1 {
		t.Fatalf("relations: got %d, want 1", len(resolved.Relations))
	}
}

func TestResolveMetadata_DeterministicOrdinalStarExpansion(t *testing.T) {
	t.Parallel()
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

	result := domain.Result{
		Dialect: "mysql",
		Mode:    domain.ModeStrict,
		Relations: []domain.RelationReference{
			{Schema: "app", Name: "users", Kind: domain.RelationTable, PermissionRequired: true},
		},
		ReferencedColumns: []domain.ColumnReference{
			{Table: "users", Column: "*", Usages: []domain.UsageContext{domain.UsageProjection}},
		},
	}

	resolved := appqa.ResolveMetadata(context.Background(), resolver, "mysql", "app", result)

	if len(resolved.ReferencedColumns) != 3 {
		t.Fatalf("expanded columns: got %d, want 3", len(resolved.ReferencedColumns))
	}
	// Verify ordinal order
	expected := []string{"id", "name", "email"}
	for i, want := range expected {
		if resolved.ReferencedColumns[i].Column != want {
			t.Errorf("column[%d]: got %q, want %q", i, resolved.ReferencedColumns[i].Column, want)
		}
	}
}

func TestResolveMetadata_GlobalStarExpansion(t *testing.T) {
	t.Parallel()
	resolver := newFakeResolver(map[string]appqa.RelationSchema{
		"app.users": {
			Schema: "app", Name: "users", Kind: "table",
			Columns: []appqa.ColumnSchema{
				{Name: "id", Ordinal: 1},
				{Name: "name", Ordinal: 2},
			},
		},
	})

	result := domain.Result{
		Dialect: "mysql",
		Mode:    domain.ModeStrict,
		Relations: []domain.RelationReference{
			{Schema: "app", Name: "users", Kind: domain.RelationTable, PermissionRequired: true},
		},
		ReferencedColumns: []domain.ColumnReference{
			{Column: "*", Usages: []domain.UsageContext{domain.UsageProjection}},
		},
	}

	resolved := appqa.ResolveMetadata(context.Background(), resolver, "mysql", "app", result)

	if len(resolved.ReferencedColumns) != 2 {
		t.Fatalf("expanded columns: got %d, want 2", len(resolved.ReferencedColumns))
	}
}

func TestResolveMetadata_ViewDetection(t *testing.T) {
	t.Parallel()
	resolver := newFakeResolver(map[string]appqa.RelationSchema{
		"app.user_view": {
			Schema: "app", Name: "user_view", Kind: "view", IsView: true,
			Columns: []appqa.ColumnSchema{{Name: "id", Ordinal: 1}},
		},
	})

	result := domain.Result{
		Dialect: "mysql",
		Mode:    domain.ModeStrict,
		Relations: []domain.RelationReference{
			{Schema: "app", Name: "user_view", Kind: domain.RelationTable, PermissionRequired: true},
		},
		ReferencedColumns: []domain.ColumnReference{
			{Table: "user_view", Column: "id", Usages: []domain.UsageContext{domain.UsageProjection}},
		},
	}

	resolved := appqa.ResolveMetadata(context.Background(), resolver, "mysql", "app", result)

	if len(resolved.Relations) != 1 {
		t.Fatalf("relations: got %d, want 1", len(resolved.Relations))
	}
	if resolved.Relations[0].Kind != domain.RelationView {
		t.Errorf("view kind: got %q, want %q", resolved.Relations[0].Kind, domain.RelationView)
	}
}

func TestResolveMetadata_CTERelationsNotResolved(t *testing.T) {
	t.Parallel()
	resolver := newFakeResolver(map[string]appqa.RelationSchema{})

	result := domain.Result{
		Dialect: "mysql",
		Mode:    domain.ModeStrict,
		Relations: []domain.RelationReference{
			{Name: "my_cte", Kind: domain.RelationCTE, PermissionRequired: false},
		},
	}

	resolved := appqa.ResolveMetadata(context.Background(), resolver, "mysql", "app", result)

	// CTE should pass through without resolver call
	if len(resolved.Relations) != 1 {
		t.Fatalf("relations: got %d, want 1", len(resolved.Relations))
	}
	if resolved.Relations[0].Kind != domain.RelationCTE {
		t.Errorf("CTE kind preserved: got %q", resolved.Relations[0].Kind)
	}
}

func TestResolveMetadata_DerivedTableNotResolved(t *testing.T) {
	t.Parallel()
	resolver := newFakeResolver(map[string]appqa.RelationSchema{})

	result := domain.Result{
		Dialect: "mysql",
		Mode:    domain.ModeStrict,
		Relations: []domain.RelationReference{
			{Name: "subquery_0", Kind: domain.RelationDerived, PermissionRequired: true},
		},
	}

	resolved := appqa.ResolveMetadata(context.Background(), resolver, "mysql", "app", result)

	if len(resolved.Relations) != 1 {
		t.Fatalf("relations: got %d, want 1", len(resolved.Relations))
	}
	if resolved.Relations[0].Kind != domain.RelationDerived {
		t.Errorf("derived kind preserved: got %q", resolved.Relations[0].Kind)
	}
}

func TestResolveMetadata_AliasResolution(t *testing.T) {
	t.Parallel()
	resolver := newFakeResolver(map[string]appqa.RelationSchema{
		"app.users": {
			Schema: "app", Name: "users", Kind: "table",
			Columns: []appqa.ColumnSchema{{Name: "id", Ordinal: 1}, {Name: "name", Ordinal: 2}},
		},
	})

	result := domain.Result{
		Dialect: "mysql",
		Mode:    domain.ModeStrict,
		Relations: []domain.RelationReference{
			{Schema: "app", Name: "users", Alias: "u", Kind: domain.RelationTable, PermissionRequired: true},
		},
		ReferencedColumns: []domain.ColumnReference{
			{Table: "u", Column: "id", Usages: []domain.UsageContext{domain.UsageProjection}},
		},
	}

	resolved := appqa.ResolveMetadata(context.Background(), resolver, "mysql", "app", result)

	if len(resolved.ReferencedColumns) != 1 {
		t.Fatalf("columns: got %d, want 1", len(resolved.ReferencedColumns))
	}
	col := resolved.ReferencedColumns[0]
	if col.Schema != "app" || col.Table != "users" {
		t.Errorf("alias should resolve to app.users, got %s.%s", col.Schema, col.Table)
	}
}

func TestResolveMetadata_AliasStarExpansion(t *testing.T) {
	t.Parallel()
	resolver := newFakeResolver(map[string]appqa.RelationSchema{
		"app.users": {
			Schema: "app", Name: "users", Kind: "table",
			Columns: []appqa.ColumnSchema{{Name: "id", Ordinal: 1}, {Name: "name", Ordinal: 2}},
		},
	})

	result := domain.Result{
		Dialect: "mysql",
		Mode:    domain.ModeStrict,
		Relations: []domain.RelationReference{
			{Schema: "app", Name: "users", Alias: "u", Kind: domain.RelationTable, PermissionRequired: true},
		},
		ReferencedColumns: []domain.ColumnReference{
			{Table: "u", Column: "*", Usages: []domain.UsageContext{domain.UsageProjection}},
		},
	}

	resolved := appqa.ResolveMetadata(context.Background(), resolver, "mysql", "app", result)

	if len(resolved.ReferencedColumns) != 2 {
		t.Fatalf("expanded columns: got %d, want 2", len(resolved.ReferencedColumns))
	}
	for _, col := range resolved.ReferencedColumns {
		if col.Schema != "app" || col.Table != "users" {
			t.Errorf("expanded column should reference app.users, got %s.%s", col.Schema, col.Table)
		}
	}
}

func TestResolveMetadata_NilResolver(t *testing.T) {
	t.Parallel()
	result := domain.Result{
		Dialect: "mysql",
		Mode:    domain.ModeStrict,
		Relations: []domain.RelationReference{
			{Schema: "app", Name: "users", Kind: domain.RelationTable, PermissionRequired: true},
		},
		ReferencedColumns: []domain.ColumnReference{
			{Table: "users", Column: "id", Usages: []domain.UsageContext{domain.UsageProjection}},
		},
	}

	resolved := appqa.ResolveMetadata(context.Background(), nil, "mysql", "app", result)

	// Nil resolver should pass through unchanged
	if len(resolved.Relations) != 1 {
		t.Fatalf("relations: got %d, want 1", len(resolved.Relations))
	}
	if resolved.Relations[0].Schema != "app" {
		t.Errorf("schema preserved: got %q", resolved.Relations[0].Schema)
	}
}

func TestResolveMetadata_WildcardRemovedWhenMetadataAvailable(t *testing.T) {
	t.Parallel()
	resolver := newFakeResolver(map[string]appqa.RelationSchema{
		"app.users": {
			Schema: "app", Name: "users", Kind: "table",
			Columns: []appqa.ColumnSchema{{Name: "id", Ordinal: 1}},
		},
	})

	result := domain.Result{
		Dialect: "mysql",
		Mode:    domain.ModeStrict,
		Relations: []domain.RelationReference{
			{Schema: "app", Name: "users", Kind: domain.RelationTable, PermissionRequired: true},
		},
		Unresolved: []domain.Unresolved{
			{Reference: "*", Reason: domain.ReasonSchemaUnavailable},
		},
	}

	resolved := appqa.ResolveMetadata(context.Background(), resolver, "mysql", "app", result)

	if len(resolved.Unresolved) != 0 {
		t.Errorf("wildcard unresolved should be removed, got %d", len(resolved.Unresolved))
	}
	if len(resolved.ReferencedColumns) != 1 {
		t.Fatalf("expanded columns: got %d, want 1", len(resolved.ReferencedColumns))
	}
	if resolved.ReferencedColumns[0].Column != "id" {
		t.Errorf("expanded column: got %q, want %q", resolved.ReferencedColumns[0].Column, "id")
	}
}

func TestResolveMetadata_OutputLineage(t *testing.T) {
	t.Parallel()
	resolver := newFakeResolver(map[string]appqa.RelationSchema{
		"app.users": {
			Schema: "app", Name: "users", Kind: "table",
			Columns: []appqa.ColumnSchema{{Name: "id", Ordinal: 1}},
		},
	})

	result := domain.Result{
		Dialect: "mysql",
		Mode:    domain.ModeStrict,
		Relations: []domain.RelationReference{
			{Schema: "app", Name: "users", Kind: domain.RelationTable, PermissionRequired: true},
		},
		Outputs: []domain.OutputColumn{
			{Name: "id", Sources: []string{"users.id"}},
		},
	}

	resolved := appqa.ResolveMetadata(context.Background(), resolver, "mysql", "app", result)

	if len(resolved.Outputs) != 1 {
		t.Fatalf("outputs: got %d, want 1", len(resolved.Outputs))
	}
	if len(resolved.Outputs[0].Sources) != 1 {
		t.Fatalf("sources: got %d, want 1", len(resolved.Outputs[0].Sources))
	}
	if resolved.Outputs[0].Sources[0] != "app.users.id" {
		t.Errorf("source lineage: got %q, want %q", resolved.Outputs[0].Sources[0], "app.users.id")
	}
}

func TestResolveMetadata_AmbiguousRelationReturnsEmpty(t *testing.T) {
	t.Parallel()
	resolver := newFakeResolver(map[string]appqa.RelationSchema{
		"schema1.users": {
			Schema: "schema1", Name: "users", Kind: "table",
			Columns: []appqa.ColumnSchema{{Name: "id", Ordinal: 1}},
		},
		"schema2.users": {
			Schema: "schema2", Name: "users", Kind: "table",
			Columns: []appqa.ColumnSchema{{Name: "id", Ordinal: 1}},
		},
	})

	result := domain.Result{
		Dialect: "mysql",
		Mode:    domain.ModeStrict,
		Relations: []domain.RelationReference{
			{Schema: "schema1", Name: "users", Kind: domain.RelationTable, PermissionRequired: true},
			{Schema: "schema2", Name: "users", Kind: domain.RelationTable, PermissionRequired: true},
		},
		ReferencedColumns: []domain.ColumnReference{
			{Table: "users", Column: "id", Usages: []domain.UsageContext{domain.UsageProjection}},
		},
	}

	resolved := appqa.ResolveMetadata(context.Background(), resolver, "mysql", "", result)

	// Ambiguous: unqualified "users" should NOT resolve to a specific schema.
	if len(resolved.ReferencedColumns) != 1 {
		t.Fatalf("columns: got %d, want 1", len(resolved.ReferencedColumns))
	}
	col := resolved.ReferencedColumns[0]
	if col.Schema != "" {
		t.Errorf("ambiguous column should not have schema, got %q", col.Schema)
	}
	// Table preserved as-is (unresolved).
	if col.Table != "users" {
		t.Errorf("ambiguous column table preserved: got %q, want %q", col.Table, "users")
	}
}

func TestResolveMetadata_ResolverErrorKeepsUnresolved(t *testing.T) {
	t.Parallel()
	resolver := &errorResolver{err: errors.New("connection refused")}

	result := domain.Result{
		Dialect: "mysql",
		Mode:    domain.ModeStrict,
		Relations: []domain.RelationReference{
			{Schema: "app", Name: "users", Kind: domain.RelationTable, PermissionRequired: true},
		},
		ReferencedColumns: []domain.ColumnReference{
			{Table: "users", Column: "*", Usages: []domain.UsageContext{domain.UsageProjection}},
		},
		Unresolved: []domain.Unresolved{
			{Reference: "*", Reason: domain.ReasonSchemaUnavailable},
		},
	}

	resolved := appqa.ResolveMetadata(context.Background(), resolver, "mysql", "app", result)

	// Resolver error: wildcard unresolved should persist, plus new unresolved for relation and wildcard failures.
	if len(resolved.Unresolved) < 1 {
		t.Errorf("unresolved: got %d, want at least 1", len(resolved.Unresolved))
	}
	// Verify the original unresolved entry is still present.
	found := false
	for _, u := range resolved.Unresolved {
		if u.Reference == "*" && u.Reason == domain.ReasonSchemaUnavailable {
			found = true
			break
		}
	}
	if !found {
		t.Error("original wildcard unresolved entry should persist")
	}
}

func TestResolveMetadata_ColumnNotFoundKeepsOriginal(t *testing.T) {
	t.Parallel()
	resolver := newFakeResolver(map[string]appqa.RelationSchema{
		"app.users": {
			Schema: "app", Name: "users", Kind: "table",
			Columns: []appqa.ColumnSchema{{Name: "id", Ordinal: 1}},
		},
	})

	result := domain.Result{
		Dialect: "mysql",
		Mode:    domain.ModeStrict,
		Relations: []domain.RelationReference{
			{Schema: "app", Name: "users", Kind: domain.RelationTable, PermissionRequired: true},
		},
		ReferencedColumns: []domain.ColumnReference{
			{Table: "users", Column: "nonexistent", Usages: []domain.UsageContext{domain.UsageProjection}},
		},
	}

	resolved := appqa.ResolveMetadata(context.Background(), resolver, "mysql", "app", result)

	if len(resolved.ReferencedColumns) != 1 {
		t.Fatalf("columns: got %d, want 1", len(resolved.ReferencedColumns))
	}
	if resolved.ReferencedColumns[0].Column != "nonexistent" {
		t.Errorf("column name preserved: got %q", resolved.ReferencedColumns[0].Column)
	}
}

func TestResolveMetadata_WildcardExpansionFailsKeepsUnresolved(t *testing.T) {
	t.Parallel()
	resolver := newFakeResolver(map[string]appqa.RelationSchema{})

	result := domain.Result{
		Dialect: "mysql",
		Mode:    domain.ModeStrict,
		Relations: []domain.RelationReference{
			{Schema: "app", Name: "users", Kind: domain.RelationTable, PermissionRequired: true},
		},
		ReferencedColumns: []domain.ColumnReference{
			{Table: "users", Column: "*", Usages: []domain.UsageContext{domain.UsageProjection}},
		},
		Unresolved: []domain.Unresolved{
			{Reference: "*", Reason: domain.ReasonSchemaUnavailable},
		},
	}

	resolved := appqa.ResolveMetadata(context.Background(), resolver, "mysql", "app", result)

	if len(resolved.Unresolved) < 1 {
		t.Errorf("unresolved: got %d, want at least 1", len(resolved.Unresolved))
	}
	found := false
	for _, u := range resolved.Unresolved {
		if u.Reference == "*" && u.Reason == domain.ReasonSchemaUnavailable {
			found = true
			break
		}
	}
	if !found {
		t.Error("original wildcard unresolved entry should persist")
	}
}

func TestResolveMetadata_PartialWildcardSuccessPreservesFailures(t *testing.T) {
	t.Parallel()
	resolver := newFakeResolver(map[string]appqa.RelationSchema{
		"schema_a.a": {
			Schema: "schema_a", Name: "a", Kind: "table",
			Columns: []appqa.ColumnSchema{{Name: "id", Ordinal: 1}},
		},
	})

	result := domain.Result{
		Dialect: "mysql",
		Mode:    domain.ModeStrict,
		Relations: []domain.RelationReference{
			{Schema: "schema_a", Name: "a", Alias: "a", Kind: domain.RelationTable, PermissionRequired: true},
			{Schema: "schema_b", Name: "b", Alias: "b", Kind: domain.RelationTable, PermissionRequired: true},
		},
		ReferencedColumns: []domain.ColumnReference{
			{Table: "a", Column: "*", Usages: []domain.UsageContext{domain.UsageProjection}},
			{Table: "b", Column: "*", Usages: []domain.UsageContext{domain.UsageProjection}},
		},
	}

	resolved := appqa.ResolveMetadata(context.Background(), resolver, "mysql", "", result)

	hasRelationNotFound := false
	for _, u := range resolved.Unresolved {
		if u.Reason == appqa.ReasonRelationNotFound || u.Reason == appqa.ReasonUnresolvedWildcard {
			hasRelationNotFound = true
		}
	}
	if !hasRelationNotFound {
		t.Error("unresolved for failed relation b should persist")
	}

	aExpanded := false
	for _, col := range resolved.ReferencedColumns {
		if col.Schema == "schema_a" && col.Table == "a" && col.Column == "id" {
			aExpanded = true
		}
	}
	if !aExpanded {
		t.Error("wildcard a.* should have been expanded")
	}
}

func TestResolveMetadata_ResolverErrorKeepsAllWildcards(t *testing.T) {
	t.Parallel()
	resolver := &errorResolver{err: errors.New("connection refused")}

	result := domain.Result{
		Dialect: "mysql",
		Mode:    domain.ModeStrict,
		Relations: []domain.RelationReference{
			{Schema: "schema_a", Name: "a", Kind: domain.RelationTable, PermissionRequired: true},
			{Schema: "schema_b", Name: "b", Kind: domain.RelationTable, PermissionRequired: true},
		},
		ReferencedColumns: []domain.ColumnReference{
			{Table: "a", Column: "*", Usages: []domain.UsageContext{domain.UsageProjection}},
			{Table: "b", Column: "*", Usages: []domain.UsageContext{domain.UsageProjection}},
		},
		Unresolved: []domain.Unresolved{
			{Reference: "*", Reason: domain.ReasonSchemaUnavailable},
		},
	}

	resolved := appqa.ResolveMetadata(context.Background(), resolver, "mysql", "", result)

	if len(resolved.Unresolved) < 1 {
		t.Errorf("unresolved: got %d, want at least 1", len(resolved.Unresolved))
	}
}
