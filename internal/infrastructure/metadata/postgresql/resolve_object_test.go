package postgresqlmeta

import (
	"context"
	"database/sql/driver"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestResolveObjectEmptyNameReturnsUnavailable(t *testing.T) {
	t.Parallel()
	db, _ := openTestDB(t, nil)
	defer db.Close()
	provider := NewProvider(db)

	snap, err := provider.ResolveObject(context.Background(), spec.DialectPostgreSQL, spec.ObjectLookupRequest{
		Type: "extension", Name: "",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if snap.Status != spec.MetadataStatusUnavailable {
		t.Fatalf("expected unavailable, got %q", snap.Status)
	}
}

func TestResolveObjectEmptyTypeReturnsUnavailable(t *testing.T) {
	t.Parallel()
	db, _ := openTestDB(t, nil)
	defer db.Close()
	provider := NewProvider(db)

	snap, err := provider.ResolveObject(context.Background(), spec.DialectPostgreSQL, spec.ObjectLookupRequest{
		Type: "", Name: "pg_trgm",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if snap.Status != spec.MetadataStatusUnavailable {
		t.Fatalf("expected unavailable, got %q", snap.Status)
	}
}

func TestResolveObjectUnsupportedTypeReturnsUnavailable(t *testing.T) {
	t.Parallel()
	db, _ := openTestDB(t, nil)
	defer db.Close()
	provider := NewProvider(db)

	snap, err := provider.ResolveObject(context.Background(), spec.DialectPostgreSQL, spec.ObjectLookupRequest{
		Type: "function", Name: "my_func",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if snap.Status != spec.MetadataStatusUnavailable {
		t.Fatalf("expected unavailable for unsupported type, got %q", snap.Status)
	}
}

// --- type ---

func TestResolveTypeConfirmedWithSchema(t *testing.T) {
	t.Parallel()
	db, _ := openTestDB(t, map[string]testQueryResult{
		"t.typtype <> 'd'": {
			columns: []string{"typtype"},
			rows:    [][]driver.Value{{"c"}},
		},
	})
	defer db.Close()
	provider := NewProvider(db)

	snap, err := provider.ResolveObject(context.Background(), spec.DialectPostgreSQL, spec.ObjectLookupRequest{
		Schema: "public", Type: "type", Name: "address",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if snap.Status != spec.MetadataStatusConfirmed {
		t.Fatalf("expected confirmed, got %q", snap.Status)
	}
	if !snap.Exists {
		t.Fatal("expected exists true")
	}
	if snap.Attributes["type_kind"] != "composite" {
		t.Fatalf("expected type_kind composite, got %q", snap.Attributes["type_kind"])
	}
}

func TestResolveTypeNotFound(t *testing.T) {
	t.Parallel()
	db, _ := openTestDB(t, map[string]testQueryResult{})
	defer db.Close()
	provider := NewProvider(db)

	snap, err := provider.ResolveObject(context.Background(), spec.DialectPostgreSQL, spec.ObjectLookupRequest{
		Schema: "public", Type: "type", Name: "missing",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if snap.Status != spec.MetadataStatusNotFound {
		t.Fatalf("expected not_found, got %q", snap.Status)
	}
}

func TestResolveTypeAmbiguousUnqualified(t *testing.T) {
	t.Parallel()
	db, _ := openTestDB(t, map[string]testQueryResult{
		"and t.typtype <> 'd'": {
			columns: []string{"nspname"},
			rows:    [][]driver.Value{{"app"}, {"public"}},
		},
	})
	defer db.Close()
	provider := NewProvider(db)

	snap, err := provider.ResolveObject(context.Background(), spec.DialectPostgreSQL, spec.ObjectLookupRequest{
		Type: "type", Name: "address",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if snap.Status != spec.MetadataStatusAmbiguous {
		t.Fatalf("expected ambiguous, got %q", snap.Status)
	}
	if len(snap.AmbiguousCandidates) != 2 {
		t.Fatalf("expected 2 candidates, got %d", len(snap.AmbiguousCandidates))
	}
}

// --- domain ---

func TestResolveDomainConfirmed(t *testing.T) {
	t.Parallel()
	db, _ := openTestDB(t, map[string]testQueryResult{
		"n.nspname = $2 and t.typtype = 'd'": {
			columns: []string{"dummy"},
			rows:    [][]driver.Value{{1}},
		},
	})
	defer db.Close()
	provider := NewProvider(db)

	snap, err := provider.ResolveObject(context.Background(), spec.DialectPostgreSQL, spec.ObjectLookupRequest{
		Schema: "public", Type: "domain", Name: "email",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if snap.Status != spec.MetadataStatusConfirmed {
		t.Fatalf("expected confirmed, got %q", snap.Status)
	}
	if !snap.Exists {
		t.Fatal("expected exists true")
	}
}

// --- extension ---

func TestResolveExtensionConfirmedWithVersion(t *testing.T) {
	t.Parallel()
	db, _ := openTestDB(t, map[string]testQueryResult{
		"pg_catalog.pg_extension": {
			columns: []string{"extversion"},
			rows:    [][]driver.Value{{"1.6"}},
		},
	})
	defer db.Close()
	provider := NewProvider(db)

	snap, err := provider.ResolveObject(context.Background(), spec.DialectPostgreSQL, spec.ObjectLookupRequest{
		Type: "extension", Name: "pg_trgm",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if snap.Status != spec.MetadataStatusConfirmed {
		t.Fatalf("expected confirmed, got %q", snap.Status)
	}
	if snap.Attributes["extension_version"] != "1.6" {
		t.Fatalf("expected extension_version 1.6, got %q", snap.Attributes["extension_version"])
	}
}

func TestResolveExtensionNotFound(t *testing.T) {
	t.Parallel()
	db, _ := openTestDB(t, map[string]testQueryResult{})
	defer db.Close()
	provider := NewProvider(db)

	snap, err := provider.ResolveObject(context.Background(), spec.DialectPostgreSQL, spec.ObjectLookupRequest{
		Type: "extension", Name: "missing_ext",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if snap.Status != spec.MetadataStatusNotFound {
		t.Fatalf("expected not_found, got %q", snap.Status)
	}
}

// --- publication ---

func TestResolvePublicationConfirmed(t *testing.T) {
	t.Parallel()
	db, _ := openTestDB(t, map[string]testQueryResult{
		"pg_catalog.pg_publication": {
			columns: []string{"dummy"},
			rows:    [][]driver.Value{{1}},
		},
	})
	defer db.Close()
	provider := NewProvider(db)

	snap, err := provider.ResolveObject(context.Background(), spec.DialectPostgreSQL, spec.ObjectLookupRequest{
		Type: "publication", Name: "pub_all",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if snap.Status != spec.MetadataStatusConfirmed {
		t.Fatalf("expected confirmed, got %q", snap.Status)
	}
}

// --- subscription (no conninfo leakage) ---

func TestResolveSubscriptionConfirmedEnabled(t *testing.T) {
	t.Parallel()
	db, _ := openTestDB(t, map[string]testQueryResult{
		"pg_catalog.pg_subscription": {
			columns: []string{"subenabled"},
			rows:    [][]driver.Value{{true}},
		},
	})
	defer db.Close()
	provider := NewProvider(db)

	snap, err := provider.ResolveObject(context.Background(), spec.DialectPostgreSQL, spec.ObjectLookupRequest{
		Type: "subscription", Name: "sub",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if snap.Status != spec.MetadataStatusConfirmed {
		t.Fatalf("expected confirmed, got %q", snap.Status)
	}
	if snap.Attributes["enabled"] != "true" {
		t.Fatalf("expected enabled true, got %q", snap.Attributes["enabled"])
	}
	// Verify no conninfo/sensitive fields leaked.
	for _, key := range []string{"conninfo", "connection", "password", "secret"} {
		if _, ok := snap.Attributes[key]; ok {
			t.Fatalf("subscription snapshot must not contain sensitive key %q", key)
		}
	}
}

func TestResolveSubscriptionConfirmedDisabled(t *testing.T) {
	t.Parallel()
	db, _ := openTestDB(t, map[string]testQueryResult{
		"pg_catalog.pg_subscription": {
			columns: []string{"subenabled"},
			rows:    [][]driver.Value{{false}},
		},
	})
	defer db.Close()
	provider := NewProvider(db)

	snap, err := provider.ResolveObject(context.Background(), spec.DialectPostgreSQL, spec.ObjectLookupRequest{
		Type: "subscription", Name: "sub",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if snap.Status != spec.MetadataStatusConfirmed {
		t.Fatalf("expected confirmed, got %q", snap.Status)
	}
	if snap.Attributes["enabled"] != "false" {
		t.Fatalf("expected enabled false, got %q", snap.Attributes["enabled"])
	}
}

// --- foreign_table ---

func TestResolveForeignTableConfirmedWithServer(t *testing.T) {
	t.Parallel()
	db, _ := openTestDB(t, map[string]testQueryResult{
		"pg_catalog.pg_foreign_table": {
			columns: []string{"srvname"},
			rows:    [][]driver.Value{{"srv_prod"}},
		},
	})
	defer db.Close()
	provider := NewProvider(db)

	snap, err := provider.ResolveObject(context.Background(), spec.DialectPostgreSQL, spec.ObjectLookupRequest{
		Schema: "public", Type: "foreign_table", Name: "ft_users",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if snap.Status != spec.MetadataStatusConfirmed {
		t.Fatalf("expected confirmed, got %q", snap.Status)
	}
	if snap.Attributes["server"] != "srv_prod" {
		t.Fatalf("expected server srv_prod, got %q", snap.Attributes["server"])
	}
}

// --- foreign_server (has_options only, no option values) ---

func TestResolveForeignServerConfirmedWithHasOptions(t *testing.T) {
	t.Parallel()
	db, _ := openTestDB(t, map[string]testQueryResult{
		"pg_catalog.pg_foreign_server": {
			columns: []string{"has_options"},
			rows:    [][]driver.Value{{true}},
		},
	})
	defer db.Close()
	provider := NewProvider(db)

	snap, err := provider.ResolveObject(context.Background(), spec.DialectPostgreSQL, spec.ObjectLookupRequest{
		Type: "foreign_server", Name: "srv",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if snap.Status != spec.MetadataStatusConfirmed {
		t.Fatalf("expected confirmed, got %q", snap.Status)
	}
	if snap.Attributes["has_options"] != "true" {
		t.Fatalf("expected has_options true, got %q", snap.Attributes["has_options"])
	}
	// Verify no option values leaked.
	for _, key := range []string{"host", "port", "password", "options"} {
		if _, ok := snap.Attributes[key]; ok {
			t.Fatalf("foreign server snapshot must not contain sensitive key %q", key)
		}
	}
}

// --- user_mapping (has_options only) ---

func TestResolveUserMappingConfirmed(t *testing.T) {
	t.Parallel()
	db, _ := openTestDB(t, map[string]testQueryResult{
		"pg_catalog.pg_user_mapping": {
			columns: []string{"has_options"},
			rows:    [][]driver.Value{{true}},
		},
	})
	defer db.Close()
	provider := NewProvider(db)

	snap, err := provider.ResolveObject(context.Background(), spec.DialectPostgreSQL, spec.ObjectLookupRequest{
		Type: "user_mapping", Name: "app@srv",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if snap.Status != spec.MetadataStatusConfirmed {
		t.Fatalf("expected confirmed, got %q", snap.Status)
	}
	if snap.Attributes["has_options"] != "true" {
		t.Fatalf("expected has_options true, got %q", snap.Attributes["has_options"])
	}
}

func TestResolveUserMappingBadNameReturnsUnavailable(t *testing.T) {
	t.Parallel()
	db, _ := openTestDB(t, nil)
	defer db.Close()
	provider := NewProvider(db)

	snap, err := provider.ResolveObject(context.Background(), spec.DialectPostgreSQL, spec.ObjectLookupRequest{
		Type: "user_mapping", Name: "no_at_sign",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if snap.Status != spec.MetadataStatusUnavailable {
		t.Fatalf("expected unavailable for malformed user mapping name, got %q", snap.Status)
	}
}

// --- foreign_data_wrapper (has_options only) ---

func TestResolveFDWConfirmed(t *testing.T) {
	t.Parallel()
	db, _ := openTestDB(t, map[string]testQueryResult{
		"pg_catalog.pg_foreign_data_wrapper": {
			columns: []string{"has_options"},
			rows:    [][]driver.Value{{false}},
		},
	})
	defer db.Close()
	provider := NewProvider(db)

	snap, err := provider.ResolveObject(context.Background(), spec.DialectPostgreSQL, spec.ObjectLookupRequest{
		Type: "foreign_data_wrapper", Name: "fdw",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if snap.Status != spec.MetadataStatusConfirmed {
		t.Fatalf("expected confirmed, got %q", snap.Status)
	}
	if snap.Attributes["has_options"] != "false" {
		t.Fatalf("expected has_options false, got %q", snap.Attributes["has_options"])
	}
}

// --- event_trigger ---

func TestResolveEventTriggerEnabled(t *testing.T) {
	t.Parallel()
	db, _ := openTestDB(t, map[string]testQueryResult{
		"pg_catalog.pg_event_trigger": {
			columns: []string{"evtenabled"},
			rows:    [][]driver.Value{{"O"}},
		},
	})
	defer db.Close()
	provider := NewProvider(db)

	snap, err := provider.ResolveObject(context.Background(), spec.DialectPostgreSQL, spec.ObjectLookupRequest{
		Type: "event_trigger", Name: "trg_ddl",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if snap.Status != spec.MetadataStatusConfirmed {
		t.Fatalf("expected confirmed, got %q", snap.Status)
	}
	if snap.Attributes["enabled"] != "true" {
		t.Fatalf("expected enabled true, got %q", snap.Attributes["enabled"])
	}
}

func TestResolveEventTriggerDisabled(t *testing.T) {
	t.Parallel()
	db, _ := openTestDB(t, map[string]testQueryResult{
		"pg_catalog.pg_event_trigger": {
			columns: []string{"evtenabled"},
			rows:    [][]driver.Value{{"D"}},
		},
	})
	defer db.Close()
	provider := NewProvider(db)

	snap, err := provider.ResolveObject(context.Background(), spec.DialectPostgreSQL, spec.ObjectLookupRequest{
		Type: "event_trigger", Name: "trg_ddl",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if snap.Attributes["enabled"] != "false" {
		t.Fatalf("expected enabled false for disabled trigger, got %q", snap.Attributes["enabled"])
	}
}

// --- rule ---

func TestResolveRuleConfirmedWithTable(t *testing.T) {
	t.Parallel()
	db, _ := openTestDB(t, map[string]testQueryResult{
		"pg_catalog.pg_rewrite": {
			columns: []string{"dummy"},
			rows:    [][]driver.Value{{1}},
		},
	})
	defer db.Close()
	provider := NewProvider(db)

	snap, err := provider.ResolveObject(context.Background(), spec.DialectPostgreSQL, spec.ObjectLookupRequest{
		Schema:     "public",
		Type:       "rule",
		Name:       "users_insert",
		Qualifiers: map[string]string{"table": "users"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if snap.Status != spec.MetadataStatusConfirmed {
		t.Fatalf("expected confirmed, got %q", snap.Status)
	}
	if snap.Attributes["table"] != "users" {
		t.Fatalf("expected table attribute users, got %q", snap.Attributes["table"])
	}
}

func TestResolveRuleNotFound(t *testing.T) {
	t.Parallel()
	db, _ := openTestDB(t, map[string]testQueryResult{})
	defer db.Close()
	provider := NewProvider(db)

	snap, err := provider.ResolveObject(context.Background(), spec.DialectPostgreSQL, spec.ObjectLookupRequest{
		Schema:     "public",
		Type:       "rule",
		Name:       "missing_rule",
		Qualifiers: map[string]string{"table": "users"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if snap.Status != spec.MetadataStatusNotFound {
		t.Fatalf("expected not_found, got %q", snap.Status)
	}
}

// --- schema ---

func TestResolveSchemaConfirmed(t *testing.T) {
	t.Parallel()
	db, _ := openTestDB(t, map[string]testQueryResult{
		"pg_catalog.pg_namespace": {
			columns: []string{"dummy"},
			rows:    [][]driver.Value{{1}},
		},
	})
	defer db.Close()
	provider := NewProvider(db)

	snap, err := provider.ResolveObject(context.Background(), spec.DialectPostgreSQL, spec.ObjectLookupRequest{
		Type: "schema", Name: "app",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if snap.Status != spec.MetadataStatusConfirmed {
		t.Fatalf("expected confirmed, got %q", snap.Status)
	}
}

func TestResolveSchemaNotFound(t *testing.T) {
	t.Parallel()
	db, _ := openTestDB(t, map[string]testQueryResult{})
	defer db.Close()
	provider := NewProvider(db)

	snap, err := provider.ResolveObject(context.Background(), spec.DialectPostgreSQL, spec.ObjectLookupRequest{
		Type: "schema", Name: "nonexistent",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if snap.Status != spec.MetadataStatusNotFound {
		t.Fatalf("expected not_found, got %q", snap.Status)
	}
}

// --- sequence ---

func TestResolveSequenceConfirmed(t *testing.T) {
	t.Parallel()
	db, _ := openTestDB(t, map[string]testQueryResult{
		"n.nspname = $2 and c.relkind = 'S'": {
			columns: []string{"dummy"},
			rows:    [][]driver.Value{{1}},
		},
	})
	defer db.Close()
	provider := NewProvider(db)

	snap, err := provider.ResolveObject(context.Background(), spec.DialectPostgreSQL, spec.ObjectLookupRequest{
		Schema: "public", Type: "sequence", Name: "seq_order_id",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if snap.Status != spec.MetadataStatusConfirmed {
		t.Fatalf("expected confirmed, got %q", snap.Status)
	}
}

// --- materialized_view ---

func TestResolveMaterializedViewConfirmed(t *testing.T) {
	t.Parallel()
	db, _ := openTestDB(t, map[string]testQueryResult{
		"n.nspname = $2 and c.relkind = 'm'": {
			columns: []string{"dummy"},
			rows:    [][]driver.Value{{1}},
		},
	})
	defer db.Close()
	provider := NewProvider(db)

	snap, err := provider.ResolveObject(context.Background(), spec.DialectPostgreSQL, spec.ObjectLookupRequest{
		Schema: "public", Type: "materialized_view", Name: "mv_stats",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if snap.Status != spec.MetadataStatusConfirmed {
		t.Fatalf("expected confirmed, got %q", snap.Status)
	}
}

// --- annotation targets (comment / security_label) ---

func TestResolveAnnotationTargetTableConfirmed(t *testing.T) {
	t.Parallel()
	db, _ := openTestDB(t, map[string]testQueryResult{
		"pg_catalog.pg_class c": {
			columns: []string{"relkind"},
			rows:    [][]driver.Value{{"r"}},
		},
	})
	defer db.Close()
	provider := NewProvider(db)

	snap, err := provider.ResolveObject(context.Background(), spec.DialectPostgreSQL, spec.ObjectLookupRequest{
		Schema: "public", Type: "comment", Name: "users",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if snap.Status != spec.MetadataStatusConfirmed {
		t.Fatalf("expected confirmed, got %q", snap.Status)
	}
	if snap.Attributes["target_type"] != "table" {
		t.Fatalf("expected target_type table, got %q", snap.Attributes["target_type"])
	}
}

func TestResolveAnnotationTargetSchemaConfirmed(t *testing.T) {
	t.Parallel()
	db, _ := openTestDB(t, map[string]testQueryResult{
		"pg_catalog.pg_namespace": {
			columns: []string{"dummy"},
			rows:    [][]driver.Value{{1}},
		},
	})
	defer db.Close()
	provider := NewProvider(db)

	snap, err := provider.ResolveObject(context.Background(), spec.DialectPostgreSQL, spec.ObjectLookupRequest{
		Schema: "app", Type: "security_label", Name: "app",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if snap.Status != spec.MetadataStatusConfirmed {
		t.Fatalf("expected confirmed, got %q", snap.Status)
	}
}

func TestResolveAnnotationTargetNotFound(t *testing.T) {
	t.Parallel()
	db, _ := openTestDB(t, map[string]testQueryResult{})
	defer db.Close()
	provider := NewProvider(db)

	snap, err := provider.ResolveObject(context.Background(), spec.DialectPostgreSQL, spec.ObjectLookupRequest{
		Schema: "public", Type: "comment", Name: "ghost",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if snap.Status != spec.MetadataStatusNotFound {
		t.Fatalf("expected not_found, got %q", snap.Status)
	}
}

// --- unqualified schema object ambiguous when multiple matches ---

func TestResolveTypeUnqualifiedAmbiguous(t *testing.T) {
	t.Parallel()
	db, _ := openTestDB(t, map[string]testQueryResult{
		"and t.typtype <> 'd'": {
			columns: []string{"nspname"},
			rows:    [][]driver.Value{{"app"}, {"public"}},
		},
	})
	defer db.Close()
	provider := NewProvider(db)

	snap, err := provider.ResolveObject(context.Background(), spec.DialectPostgreSQL, spec.ObjectLookupRequest{
		Type: "type", Name: "address",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if snap.Status != spec.MetadataStatusAmbiguous {
		t.Fatalf("expected ambiguous for multiple schemas, got %q", snap.Status)
	}
	if len(snap.AmbiguousCandidates) != 2 {
		t.Fatalf("expected 2 candidates, got %d", len(snap.AmbiguousCandidates))
	}
}

// --- helpers ---

func TestSplitUserMappingName(t *testing.T) {
	t.Parallel()
	user, server := splitUserMappingName("app@srv")
	if user != "app" || server != "srv" {
		t.Fatalf("expected app/srv, got %s/%s", user, server)
	}
	user, server = splitUserMappingName("no_at")
	if user != "no_at" || server != "" {
		t.Fatalf("expected no_at/<empty>, got %s/%s", user, server)
	}
}

func TestTypeKindName(t *testing.T) {
	t.Parallel()
	cases := []struct{ input, want string }{
		{"c", "composite"},
		{"e", "enum"},
		{"r", "range"},
		{"b", "base"},
		{"x", "x"},
	}
	for _, tc := range cases {
		if got := typeKindName(tc.input); got != tc.want {
			t.Errorf("typeKindName(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestEventTriggerEnabled(t *testing.T) {
	t.Parallel()
	if eventTriggerEnabled("O") != "true" {
		t.Fatal("expected O to map to true")
	}
	if eventTriggerEnabled("D") != "false" {
		t.Fatal("expected D to map to false")
	}
}

func TestClassTargetType(t *testing.T) {
	t.Parallel()
	cases := []struct{ input, want string }{
		{"r", "table"},
		{"p", "table"},
		{"f", "foreign_table"},
		{"S", "sequence"},
		{"m", "materialized_view"},
		{"v", "view"},
		{"x", "x"},
	}
	for _, tc := range cases {
		if got := classTargetType(tc.input); got != tc.want {
			t.Errorf("classTargetType(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}
