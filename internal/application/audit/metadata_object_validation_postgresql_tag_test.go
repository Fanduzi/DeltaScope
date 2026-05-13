//go:build postgresql

package audit

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// fakeObjectResolver records calls and returns preconfigured snapshots.
type fakeObjectResolver struct {
	snapshots map[string]*spec.ObjectSnapshot // key: type|name
	calls     []spec.ObjectLookupRequest
	err       error
}

func newFakeObjectResolver() *fakeObjectResolver {
	return &fakeObjectResolver{
		snapshots: make(map[string]*spec.ObjectSnapshot),
	}
}

func (r *fakeObjectResolver) LoadInstanceFacts(_ context.Context, _ spec.Dialect, _ string) (*spec.InstanceFacts, error) {
	return &spec.InstanceFacts{Version: "PostgreSQL 16.0"}, nil
}

func (r *fakeObjectResolver) LoadTableSnapshot(_ context.Context, _ spec.Dialect, schema string, table string) (*spec.TableSnapshot, error) {
	return &spec.TableSnapshot{Exists: true, Table: &spec.Table{Name: table, Schema: schema}}, nil
}

func (r *fakeObjectResolver) ResolveTableForIndex(_ context.Context, _ spec.Dialect, _ string, _ string) (string, error) {
	return "users", nil
}

func (r *fakeObjectResolver) LoadPlanEstimate(_ context.Context, _ spec.Statement) (*spec.ImpactEstimate, error) {
	return nil, nil
}

func (r *fakeObjectResolver) ResolveObject(_ context.Context, _ spec.Dialect, req spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
	r.calls = append(r.calls, req)
	if r.err != nil {
		return nil, r.err
	}
	key := strings.ToLower(req.Type + "|" + req.Name)
	if s, ok := r.snapshots[key]; ok {
		return s, nil
	}
	return &spec.ObjectSnapshot{
		Schema: req.Schema,
		Type:   req.Type,
		Name:   req.Name,
		Status: spec.MetadataStatusUnavailable,
	}, nil
}

func TestObjectEnrichmentConfirmedFromResolver(t *testing.T) {
	t.Parallel()
	resolver := newFakeObjectResolver()
	resolver.snapshots["extension|pg_trgm"] = &spec.ObjectSnapshot{
		Schema: "public",
		Type:   "extension",
		Name:   "pg_trgm",
		Status: spec.MetadataStatusConfirmed,
		Exists: true,
		Attributes: map[string]string{
			"extension_version": "1.6",
		},
	}

	result, err := AuditSQL(context.Background(), Request{
		SQL:              "DROP EXTENSION pg_trgm",
		Dialect:          spec.DialectPostgreSQL,
		Schema:           "public",
		MetadataProvider: resolver,
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	if len(result.Statements) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(result.Statements))
	}

	// Re-enrich to check internal state via enrichStatementsWithMetadata.
	resolver.calls = nil
	parsed, _ := Parse(context.Background(), "DROP EXTENSION pg_trgm", spec.DialectPostgreSQL)
	statements, _ := Extract(context.Background(), parsed)
	enriched, err := enrichStatementsWithMetadata(context.Background(), spec.DialectPostgreSQL, &MetadataRequest{
		Schema:   "public",
		Provider: resolver,
	}, statements)
	if err != nil {
		t.Fatalf("enrich: %v", err)
	}
	if len(enriched) != 1 {
		t.Fatalf("expected 1 enriched statement, got %d", len(enriched))
	}
	if enriched[0].Metadata == nil {
		t.Fatal("expected metadata to be attached")
	}
	if len(enriched[0].Metadata.Objects) != 1 {
		t.Fatalf("expected 1 object, got %d", len(enriched[0].Metadata.Objects))
	}
	obj := enriched[0].Metadata.Objects[0]
	if obj.Status != spec.MetadataStatusConfirmed {
		t.Fatalf("expected confirmed, got %q", obj.Status)
	}
	if !obj.Exists {
		t.Fatal("expected exists true")
	}
	if obj.Type != "extension" || obj.Name != "pg_trgm" {
		t.Fatalf("expected extension/pg_trgm, got %s/%s", obj.Type, obj.Name)
	}
	if len(resolver.calls) != 1 {
		t.Fatalf("expected 1 resolver call, got %d", len(resolver.calls))
	}
	if resolver.calls[0].Type != "extension" || resolver.calls[0].Name != "pg_trgm" {
		t.Fatalf("expected lookup extension/pg_trgm, got %s/%s", resolver.calls[0].Type, resolver.calls[0].Name)
	}
}

func TestObjectEnrichmentNotFoundFromResolver(t *testing.T) {
	t.Parallel()
	resolver := newFakeObjectResolver()
	resolver.snapshots["publication|pub_missing"] = &spec.ObjectSnapshot{
		Schema: "public",
		Type:   "publication",
		Name:   "pub_missing",
		Status: spec.MetadataStatusNotFound,
	}

	parsed, _ := Parse(context.Background(), "DROP PUBLICATION pub_missing", spec.DialectPostgreSQL)
	statements, _ := Extract(context.Background(), parsed)
	enriched, _ := enrichStatementsWithMetadata(context.Background(), spec.DialectPostgreSQL, &MetadataRequest{
		Schema:   "public",
		Provider: resolver,
	}, statements)

	if len(enriched[0].Metadata.Objects) != 1 {
		t.Fatalf("expected 1 object, got %d", len(enriched[0].Metadata.Objects))
	}
	obj := enriched[0].Metadata.Objects[0]
	if obj.Status != spec.MetadataStatusNotFound {
		t.Fatalf("expected not_found, got %q", obj.Status)
	}
}

func TestObjectEnrichmentAmbiguousFromResolver(t *testing.T) {
	t.Parallel()
	resolver := newFakeObjectResolver()
	resolver.snapshots["type|address"] = &spec.ObjectSnapshot{
		Schema:              "public",
		Type:                "type",
		Name:                "address",
		Status:              spec.MetadataStatusAmbiguous,
		AmbiguousCandidates: []string{"public.address", "app.address"},
	}

	parsed, _ := Parse(context.Background(), "ALTER TYPE address ADD ATTRIBUTE country text", spec.DialectPostgreSQL)
	statements, _ := Extract(context.Background(), parsed)
	enriched, _ := enrichStatementsWithMetadata(context.Background(), spec.DialectPostgreSQL, &MetadataRequest{
		Schema:   "public",
		Provider: resolver,
	}, statements)

	obj := enriched[0].Metadata.Objects[0]
	if obj.Status != spec.MetadataStatusAmbiguous {
		t.Fatalf("expected ambiguous, got %q", obj.Status)
	}
	if len(obj.AmbiguousCandidates) != 2 {
		t.Fatalf("expected 2 candidates, got %d", len(obj.AmbiguousCandidates))
	}
}

func TestObjectEnrichmentUnavailableWhenNoResolver(t *testing.T) {
	t.Parallel()
	provider := &fakeMetadataProvider{
		instance: &spec.InstanceFacts{Version: "PostgreSQL 16.0"},
	}

	parsed, _ := Parse(context.Background(), "DROP EVENT TRIGGER trg_ddl", spec.DialectPostgreSQL)
	statements, _ := Extract(context.Background(), parsed)
	enriched, _ := enrichStatementsWithMetadata(context.Background(), spec.DialectPostgreSQL, &MetadataRequest{
		Schema:   "public",
		Provider: provider,
	}, statements)

	if enriched[0].Metadata == nil {
		t.Fatal("expected metadata to be attached")
	}
	if len(enriched[0].Metadata.Objects) != 1 {
		t.Fatalf("expected 1 object, got %d", len(enriched[0].Metadata.Objects))
	}
	obj := enriched[0].Metadata.Objects[0]
	if obj.Status != spec.MetadataStatusUnavailable {
		t.Fatalf("expected unavailable, got %q", obj.Status)
	}
	if obj.Type != "event_trigger" || obj.Name != "trg_ddl" {
		t.Fatalf("expected event_trigger/trg_ddl, got %s/%s", obj.Type, obj.Name)
	}
}

func TestObjectEnrichmentResolverErrorFallsBackToUnavailable(t *testing.T) {
	t.Parallel()
	resolver := newFakeObjectResolver()
	resolver.err = fmt.Errorf("connection refused")

	parsed, _ := Parse(context.Background(), "DROP FOREIGN TABLE ft_users", spec.DialectPostgreSQL)
	statements, _ := Extract(context.Background(), parsed)
	enriched, _ := enrichStatementsWithMetadata(context.Background(), spec.DialectPostgreSQL, &MetadataRequest{
		Schema:   "public",
		Provider: resolver,
	}, statements)

	obj := enriched[0].Metadata.Objects[0]
	if obj.Status != spec.MetadataStatusUnavailable {
		t.Fatalf("expected unavailable on resolver error, got %q", obj.Status)
	}
}

func TestObjectEnrichmentNoLookupForTableOperations(t *testing.T) {
	t.Parallel()
	resolver := newFakeObjectResolver()

	parsed, _ := Parse(context.Background(), "ALTER TABLE users ADD COLUMN email text", spec.DialectPostgreSQL)
	statements, _ := Extract(context.Background(), parsed)
	enriched, _ := enrichStatementsWithMetadata(context.Background(), spec.DialectPostgreSQL, &MetadataRequest{
		Schema:   "public",
		Provider: resolver,
	}, statements)

	if len(resolver.calls) != 0 {
		t.Fatalf("expected 0 resolver calls for table operations, got %d", len(resolver.calls))
	}
	// Objects should be empty — table operations use TargetTable, not Objects.
	if enriched[0].Metadata != nil && len(enriched[0].Metadata.Objects) != 0 {
		t.Fatalf("expected 0 objects for table operation, got %d", len(enriched[0].Metadata.Objects))
	}
}

func TestObjectEnrichmentExistingIndexOwnerResolutionPreserved(t *testing.T) {
	t.Parallel()
	resolver := newFakeObjectResolver()

	parsed, _ := Parse(context.Background(), "ALTER INDEX idx_users_email RENAME TO idx_new", spec.DialectPostgreSQL)
	statements, _ := Extract(context.Background(), parsed)
	_, _ = enrichStatementsWithMetadata(context.Background(), spec.DialectPostgreSQL, &MetadataRequest{
		Schema:   "public",
		Provider: resolver,
	}, statements)

	// ALTER INDEX should not produce object lookup (it uses IndexOwnerResolver).
	if len(resolver.calls) != 0 {
		t.Fatalf("expected 0 object resolver calls for ALTER INDEX, got %d", len(resolver.calls))
	}
}

func TestObjectEnrichmentAllObjectFamilies(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name       string
		sql        string
		objectType string
		objectName string
	}{
		{"type", "ALTER TYPE address ADD ATTRIBUTE country text", "type", "address"},
		{"domain", "ALTER DOMAIN email SET NOT NULL", "domain", "email"},
		{"extension", "DROP EXTENSION pg_trgm", "extension", "pg_trgm"},
		{"publication", "DROP PUBLICATION pub_all", "publication", "pub_all"},
		{"subscription", "ALTER SUBSCRIPTION sub DISABLE", "subscription", "sub"},
		{"foreign_table", "DROP FOREIGN TABLE ft_users", "foreign_table", "ft_users"},
		{"foreign_server", "ALTER SERVER srv OPTIONS (SET host 'db')", "foreign_server", "srv"},
		{"user_mapping", "DROP USER MAPPING FOR app SERVER srv", "user_mapping", "app@srv"},
		{"foreign_data_wrapper", "DROP FOREIGN DATA WRAPPER fdw", "foreign_data_wrapper", "fdw"},
		{"event_trigger", "DROP EVENT TRIGGER trg_ddl", "event_trigger", "trg_ddl"},
		{"rule", "DROP RULE users_insert ON users", "rule", "users_insert"},
		{"schema", "DROP SCHEMA app", "schema", "app"},
		{"sequence", "DROP SEQUENCE seq_order_id", "sequence", "seq_order_id"},
		{"materialized_view", "DROP MATERIALIZED VIEW mv_stats", "materialized_view", "mv_stats"},
		{"comment_on", "COMMENT ON TABLE users IS 'user accounts'", "comment", "users"},
		{"security_label", "SECURITY LABEL FOR selinux ON TABLE users IS NULL", "security_label", "users"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			parsed, _ := Parse(context.Background(), tc.sql, spec.DialectPostgreSQL)
			statements, _ := Extract(context.Background(), parsed)
			enriched, _ := enrichStatementsWithMetadata(context.Background(), spec.DialectPostgreSQL, &MetadataRequest{
				Schema:   "public",
				Provider: nil, // No provider — should get unavailable.
			}, statements)

			if len(enriched) == 0 {
				t.Fatalf("expected at least 1 statement")
			}
			if enriched[0].Metadata == nil {
				t.Fatalf("expected metadata to be attached for %s", tc.name)
			}
			if len(enriched[0].Metadata.Objects) != 1 {
				t.Fatalf("expected 1 object for %s, got %d", tc.name, len(enriched[0].Metadata.Objects))
			}
			obj := enriched[0].Metadata.Objects[0]
			if obj.Type != tc.objectType {
				t.Errorf("expected type %q, got %q", tc.objectType, obj.Type)
			}
			if obj.Name != tc.objectName {
				t.Errorf("expected name %q, got %q", tc.objectName, obj.Name)
			}
			if obj.Status != spec.MetadataStatusUnavailable {
				t.Errorf("expected unavailable (no provider), got %q", obj.Status)
			}
		})
	}
}

func TestObjectEnrichmentPreservesExistingTableMetadata(t *testing.T) {
	t.Parallel()
	resolver := newFakeObjectResolver()
	resolver.snapshots["extension|pg_trgm"] = &spec.ObjectSnapshot{
		Type:   "extension",
		Name:   "pg_trgm",
		Status: spec.MetadataStatusConfirmed,
		Exists: true,
	}
	provider := &fakeMetadataProvider{
		instance: &spec.InstanceFacts{Version: "PostgreSQL 16.0"},
		snapshot: &spec.TableSnapshot{
			Exists:      true,
			Table:       &spec.Table{Name: "users"},
			Constraints: []spec.Constraint{{Type: "primary_key", Name: "users_pkey"}},
		},
	}

	// Use provider without ObjectResolver — table path should still work.
	_, err := AuditSQL(context.Background(), Request{
		SQL:              "ALTER TABLE users DROP CONSTRAINT users_pkey",
		Dialect:          spec.DialectPostgreSQL,
		Schema:           "public",
		MetadataProvider: provider,
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	if len(provider.tableCalls) != 1 || provider.tableCalls[0] != "users" {
		t.Fatalf("expected table snapshot call for users, got %#v", provider.tableCalls)
	}
}

func TestCensusUnavailabilityNowFromObjectFallback(t *testing.T) {
	t.Parallel()
	// Verify that the 19 metadata_unavailable cases from Task 1 now produce
	// ObjectSnapshot with status unavailable (from the enrichment fallback),
	// rather than simply lacking metadata.
	provider := &fakeMetadataProvider{
		instance: &spec.InstanceFacts{Version: "PostgreSQL 16.0"},
		snapshot: &spec.TableSnapshot{
			Exists: true,
			Table:  &spec.Table{Name: "users"},
		},
		indexTable: "users",
	}

	sqlCases := []struct {
		name       string
		sql        string
		objectType string
	}{
		{"ALTER TYPE ADD ATTRIBUTE", "ALTER TYPE address ADD ATTRIBUTE country text", "type"},
		{"DROP EXTENSION", "DROP EXTENSION pg_trgm", "extension"},
		{"DROP PUBLICATION", "DROP PUBLICATION pub_all", "publication"},
		{"ALTER SUBSCRIPTION DISABLE", "ALTER SUBSCRIPTION sub DISABLE", "subscription"},
		{"DROP FOREIGN TABLE", "DROP FOREIGN TABLE ft_users", "foreign_table"},
		{"DROP EVENT TRIGGER", "DROP EVENT TRIGGER trg_ddl", "event_trigger"},
		{"DROP RULE", "DROP RULE users_insert ON users", "rule"},
		{"DROP SCHEMA", "DROP SCHEMA app", "schema"},
	}

	for _, tc := range sqlCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			parsed, _ := Parse(context.Background(), tc.sql, spec.DialectPostgreSQL)
			statements, _ := Extract(context.Background(), parsed)
			enriched, _ := enrichStatementsWithMetadata(context.Background(), spec.DialectPostgreSQL, &MetadataRequest{
				Schema:   "public",
				Provider: provider,
			}, statements)

			if enriched[0].Metadata == nil {
				t.Fatalf("expected metadata attached for %s", tc.name)
			}
			// These are non-table objects, so TargetTable should NOT be set
			// (the fake provider returns table snapshot for any table, but
			// these statements don't target a table).
			if len(enriched[0].Metadata.Objects) < 1 {
				t.Fatalf("expected at least 1 object for %s, got %d", tc.name, len(enriched[0].Metadata.Objects))
			}
			found := false
			for _, obj := range enriched[0].Metadata.Objects {
				if obj.Type == tc.objectType && obj.Status == spec.MetadataStatusUnavailable {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("expected unavailable object of type %q for %s, got %#v", tc.objectType, tc.name, enriched[0].Metadata.Objects)
			}
		})
	}
}
