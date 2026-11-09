package mysqlmeta

import (
	"context"
	"database/sql/driver"
	"errors"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// T24/#103: the official ResolveObject mysql_resource_group path reads
// information_schema.RESOURCE_GROUPS through the real provider query —
// the bounded NAME/TYPE/ENABLED/VCPU_IDS/THREAD_PRIORITY shape. No fake
// resolver is substituted.
var testMySQLResourceGroupColumns = []string{
	"RESOURCE_GROUP_NAME", "RESOURCE_GROUP_TYPE", "RESOURCE_GROUP_ENABLED",
	"VCPU_IDS", "THREAD_PRIORITY",
}

func t24Resolve(t *testing.T, results map[string]testQueryResult, dialect spec.Dialect, req spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
	t.Helper()
	db := openTestDB(t, results)
	provider := NewProvider(db)
	return provider.ResolveObject(context.Background(), dialect, req)
}

// A well-formed catalog row proves existence and publishes every
// observed definition member — workload effect is never modeled.
func TestT24ResourceGroupLoaderPublishesDefinition(t *testing.T) {
	snapshot, err := t24Resolve(t, map[string]testQueryResult{
		testResourceGroupFragment: {
			columns: testMySQLResourceGroupColumns,
			rows:    [][]driver.Value{{"golden_rg", "USER", "1", "0-3", "10"}},
		},
	}, spec.DialectMySQL, spec.ObjectLookupRequest{Type: "mysql_resource_group", Name: "golden_rg"})
	if err != nil {
		t.Fatalf("ResolveObject: %v", err)
	}
	if !snapshot.IsConfirmed() || !snapshot.Exists {
		t.Fatalf("snapshot = %#v, want confirmed present", snapshot)
	}
	def := snapshot.MySQLResourceGroup
	if def == nil || def.GroupType != spec.MySQLResourceGroupTypeUser ||
		def.Enabled == nil || !*def.Enabled ||
		def.VCPUIDs == nil || *def.VCPUIDs != "0-3" ||
		def.ThreadPriority == nil || *def.ThreadPriority != 10 {
		t.Fatalf("definition = %+v, want every catalog member published", def)
	}
}

// NULL members stay unproven — nil pointers, never guessed defaults or
// conflated zeros. An unrecognized TYPE token stays unproven rather
// than silently mapping into the two-value enum.
func TestT24ResourceGroupLoaderNullableMembers(t *testing.T) {
	for _, tc := range []struct {
		name    string
		typeTok driver.Value
		want    spec.MySQLResourceGroupType
	}{
		{"system", "SYSTEM", spec.MySQLResourceGroupTypeSystem},
		{"user", "USER", spec.MySQLResourceGroupTypeUser},
		{"lower-user", "user", spec.MySQLResourceGroupTypeUser},
		{"unknown-token", "SURPRISING", ""},
		{"null-type", nil, ""},
	} {
		snapshot, err := t24Resolve(t, map[string]testQueryResult{
			testResourceGroupFragment: {
				columns: testMySQLResourceGroupColumns,
				rows:    [][]driver.Value{{"rg", tc.typeTok, nil, nil, nil}},
			},
		}, spec.DialectMySQL, spec.ObjectLookupRequest{Type: "mysql_resource_group", Name: "rg"})
		if err != nil {
			t.Fatalf("ResolveObject(%q): %v", tc.name, err)
		}
		def := snapshot.MySQLResourceGroup
		if def.GroupType != tc.want {
			t.Fatalf("%s: type = %q, want %q", tc.name, def.GroupType, tc.want)
		}
		if def.Enabled != nil || def.VCPUIDs != nil || def.ThreadPriority != nil {
			t.Fatalf("%s: definition = %+v, want NULL members unproven", tc.name, def)
		}
	}
}

// The ENABLED flag maps its observed vocabulary; an unrecognized token
// stays unproven rather than a guessed flag.
func TestT24ResourceGroupLoaderEnabledTokens(t *testing.T) {
	for _, tc := range []struct {
		token driver.Value
		want  *bool
	}{
		{"1", boolPtr(true)},
		{"0", boolPtr(false)},
		{"YES", boolPtr(true)},
		{"NO", boolPtr(false)},
		{"SURPRISING", nil},
		{nil, nil},
	} {
		snapshot, err := t24Resolve(t, map[string]testQueryResult{
			testResourceGroupFragment: {
				columns: testMySQLResourceGroupColumns,
				rows:    [][]driver.Value{{"rg", "USER", tc.token, nil, nil}},
			},
		}, spec.DialectMySQL, spec.ObjectLookupRequest{Type: "mysql_resource_group", Name: "rg"})
		if err != nil {
			t.Fatalf("ResolveObject(%v): %v", tc.token, err)
		}
		def := snapshot.MySQLResourceGroup
		switch {
		case tc.want == nil && def.Enabled != nil:
			t.Fatalf("enabled = %v for token %v, want unproven", *def.Enabled, tc.token)
		case tc.want != nil && (def.Enabled == nil || *def.Enabled != *tc.want):
			t.Fatalf("enabled = %+v for token %v, want %v", def.Enabled, tc.token, *tc.want)
		}
	}
}

// A zero-row answer stays unknown — never a fabricated absence: MySQL's
// catalog contract does not document an unfiltered emit-every-group
// guarantee the way the scoped TiDB v8.5 fact does, so unavailable is
// the honest status, not not_found.
func TestT24ResourceGroupLoaderZeroRowsStaysUnknown(t *testing.T) {
	snapshot, err := t24Resolve(t, map[string]testQueryResult{
		testResourceGroupFragment: {columns: testMySQLResourceGroupColumns},
	}, spec.DialectMySQL, spec.ObjectLookupRequest{Type: "mysql_resource_group", Name: "ghost"})
	if err != nil {
		t.Fatalf("ResolveObject: %v", err)
	}
	if snapshot.IsNotFound() || !snapshot.IsUnavailable() || snapshot.Exists {
		t.Fatalf("snapshot = %#v, want unavailable — zero rows never fabricate absence", snapshot)
	}
}

// Query, scan, and driver errors propagate wrapped with identity
// context — they are never degraded to unavailable and never cached
// as absence.
func TestT24ResourceGroupLoaderErrorPropagates(t *testing.T) {
	want := errors.New("resource_groups select denied")
	_, err := t24Resolve(t, map[string]testQueryResult{
		testResourceGroupFragment: {columns: testMySQLResourceGroupColumns, queryErr: want},
	}, spec.DialectMySQL, spec.ObjectLookupRequest{Type: "mysql_resource_group", Name: "rg"})
	if err == nil {
		t.Fatal("want propagated error")
	}
	if !errors.Is(err, want) {
		t.Fatalf("err = %v, want errors.Is(%v)", err, want)
	}
}

// The MySQL CPU resource-group family is its own surface — under TiDB
// the resolver never emits the MySQL catalog query; under MySQL the
// TiDB RU lookup type never routes here.
func TestT24ResourceGroupLoaderDialectGated(t *testing.T) {
	db := openTestDB(t, map[string]testQueryResult{
		testResourceGroupFragment: {
			columns: testMySQLResourceGroupColumns,
			rows:    [][]driver.Value{{"rg", "USER", "1", nil, nil}},
		},
	})
	provider := NewProvider(db)
	snapshot, err := provider.ResolveObject(context.Background(), spec.DialectTiDB, spec.ObjectLookupRequest{
		Type: "mysql_resource_group", Name: "rg",
	})
	if err != nil {
		t.Fatalf("ResolveObject: %v", err)
	}
	if !snapshot.IsUnavailable() || snapshot.Exists {
		t.Fatalf("snapshot = %#v, want unavailable — TiDB must not read the MySQL catalog", snapshot)
	}
	// The TiDB RU lookup type under MySQL is equally unavailable — the
	// two families never share a catalog read.
	snapshot, err = provider.ResolveObject(context.Background(), spec.DialectMySQL, spec.ObjectLookupRequest{
		Type: "resource_group", Name: "rg",
	})
	if err != nil {
		t.Fatalf("ResolveObject: %v", err)
	}
	if !snapshot.IsUnavailable() || snapshot.Exists {
		t.Fatalf("snapshot = %#v, want unavailable — the RU query is TiDB-only", snapshot)
	}
}
