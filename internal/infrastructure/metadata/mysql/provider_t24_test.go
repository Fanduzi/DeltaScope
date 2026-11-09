package mysqlmeta

import (
	"context"
	"database/sql/driver"
	"errors"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// T24/#103: the MySQL ResolveObject path reads
// information_schema.RESOURCE_GROUPS through the real provider query —
// the bounded NAME/TYPE/ENABLED/VCPU_IDS/THREAD_PRIORITY shape, a
// different surface from the TiDB RU catalog. No fake resolver is
// substituted.
const testMySQLResourceGroupFragment = "information_schema.resource_groups"

var testMySQLResourceGroupColumns = []string{
	"RESOURCE_GROUP_NAME", "RESOURCE_GROUP_TYPE", "RESOURCE_GROUP_ENABLED", "VCPU_IDS", "THREAD_PRIORITY",
}

func t24Resolve(t *testing.T, results map[string]testQueryResult, dialect spec.Dialect, req spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
	t.Helper()
	db := openTestDB(t, results)
	provider := NewProvider(db)
	return provider.ResolveObject(context.Background(), dialect, req)
}

// A well-formed catalog row proves existence and publishes every observed
// member — affinity effect is never modeled.
func TestT24MySQLResourceGroupLoaderPublishesDefinition(t *testing.T) {
	snapshot, err := t24Resolve(t, map[string]testQueryResult{
		testMySQLResourceGroupFragment: {
			columns: testMySQLResourceGroupColumns,
			rows: [][]driver.Value{{
				"rg_cpu", "USER", "1", "0-3,6", "10",
			}},
		},
	}, spec.DialectMySQL, spec.ObjectLookupRequest{Type: "mysql_resource_group", Name: "rg_cpu"})
	if err != nil {
		t.Fatalf("ResolveObject: %v", err)
	}
	if !snapshot.IsConfirmed() || !snapshot.Exists {
		t.Fatalf("snapshot = %#v, want confirmed present", snapshot)
	}
	def := snapshot.MySQLResourceGroup
	if def == nil || def.GroupType != spec.MySQLResourceGroupTypeUser ||
		def.Enabled == nil || !*def.Enabled ||
		def.VCPUIDs == nil || *def.VCPUIDs != "0-3,6" ||
		def.ThreadPriority == nil || *def.ThreadPriority != 10 {
		t.Fatalf("definition = %+v, want every catalog member published", def)
	}
}

// NULL and unrecognized members stay unproven — never a guessed default,
// never a silently mapped truth.
func TestT24MySQLResourceGroupLoaderNullsStayUnproven(t *testing.T) {
	snapshot, err := t24Resolve(t, map[string]testQueryResult{
		testMySQLResourceGroupFragment: {
			columns: testMySQLResourceGroupColumns,
			rows:    [][]driver.Value{{"rg", "SURPRISING", "MAYBE", nil, "abc"}},
		},
	}, spec.DialectMySQL, spec.ObjectLookupRequest{Type: "mysql_resource_group", Name: "rg"})
	if err != nil {
		t.Fatalf("ResolveObject: %v", err)
	}
	def := snapshot.MySQLResourceGroup
	if def == nil {
		t.Fatal("definition missing on a confirmed row")
	}
	if def.GroupType != "" || def.Enabled != nil || def.VCPUIDs != nil || def.ThreadPriority != nil {
		t.Fatalf("definition = %+v, want every unproven member nil", def)
	}
}

// A complete successful zero-row read stays unknown — unlike the scoped
// TiDB fact, no documented guarantee says the MySQL catalog emits every
// group unfiltered, so absence is never fabricated.
func TestT24MySQLResourceGroupLoaderZeroRowsStaysUnknown(t *testing.T) {
	snapshot, err := t24Resolve(t, map[string]testQueryResult{
		testMySQLResourceGroupFragment: {columns: testMySQLResourceGroupColumns},
	}, spec.DialectMySQL, spec.ObjectLookupRequest{Type: "mysql_resource_group", Name: "ghost"})
	if err != nil {
		t.Fatalf("ResolveObject: %v", err)
	}
	if !snapshot.IsUnavailable() || snapshot.Exists {
		t.Fatalf("snapshot = %#v, want unavailable — zero rows never proves absence", snapshot)
	}
}

// Catalog errors propagate wrapped with identity context — never degraded
// to unavailable, never cached as absence.
func TestT24MySQLResourceGroupLoaderErrorPropagates(t *testing.T) {
	want := errors.New("resource_groups select denied")
	_, err := t24Resolve(t, map[string]testQueryResult{
		testMySQLResourceGroupFragment: {columns: testMySQLResourceGroupColumns, queryErr: want},
	}, spec.DialectMySQL, spec.ObjectLookupRequest{Type: "mysql_resource_group", Name: "rg"})
	if err == nil {
		t.Fatal("want propagated error")
	}
	if !errors.Is(err, want) {
		t.Fatalf("err = %v, want errors.Is(%v)", err, want)
	}
}

// The TiDB dialect never emits the MySQL catalog query — the families
// stay separate under the wrong dialect.
func TestT24MySQLResourceGroupLoaderDialectGated(t *testing.T) {
	db := openTestDB(t, map[string]testQueryResult{
		testMySQLResourceGroupFragment: {
			columns: testMySQLResourceGroupColumns,
			rows:    [][]driver.Value{{"rg", "USER", "1", nil, "0"}},
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
}
