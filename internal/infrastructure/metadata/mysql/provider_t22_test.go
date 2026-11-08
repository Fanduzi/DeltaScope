package mysqlmeta

import (
	"context"
	"database/sql/driver"
	"errors"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// T22/#101: the official ResolveObject resource-group path reads
// information_schema.RESOURCE_GROUPS through the real provider query — the
// bounded name/RU_PER_SEC/PRIORITY/BURSTABLE/QUERY_LIMIT shape (BACKGROUND
// stays outside the read). No fake resolver is substituted.
const testResourceGroupFragment = "information_schema.resource_groups"

var testResourceGroupColumns = []string{
	"NAME", "RU_PER_SEC", "PRIORITY", "BURSTABLE", "QUERY_LIMIT",
}

func t22Resolve(t *testing.T, results map[string]testQueryResult, dialect spec.Dialect, req spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
	t.Helper()
	db := openTestDB(t, results)
	provider := NewProvider(db)
	return provider.ResolveObject(context.Background(), dialect, req)
}

// A well-formed catalog row proves existence and publishes every observed
// definition member — workload effect is never modeled.
func TestT22ResourceGroupLoaderPublishesDefinition(t *testing.T) {
	snapshot, err := t22Resolve(t, map[string]testQueryResult{
		testResourceGroupFragment: {
			columns: testResourceGroupColumns,
			rows: [][]driver.Value{{
				"golden_rg", "1000", "HIGH", "MODERATED", "EXEC_ELAPSED='60s' ACTION=KILL",
			}},
		},
	}, spec.DialectTiDB, spec.ObjectLookupRequest{Type: "resource_group", Name: "golden_rg"})
	if err != nil {
		t.Fatalf("ResolveObject: %v", err)
	}
	if !snapshot.IsConfirmed() || !snapshot.Exists {
		t.Fatalf("snapshot = %#v, want confirmed present", snapshot)
	}
	def := snapshot.ResourceGroup
	if def == nil || def.RUPerSec == nil || *def.RUPerSec != 1000 ||
		def.RUUnlimited || def.Priority != "HIGH" ||
		def.Burstable == nil || !*def.Burstable ||
		def.QueryLimit == nil || *def.QueryLimit != "EXEC_ELAPSED='60s' ACTION=KILL" {
		t.Fatalf("definition = %+v, want every catalog member published", def)
	}
}

// UNLIMITED stays a distinct token from a numeric rate — never a conflated
// zero or a guessed bound.
func TestT22ResourceGroupLoaderUnlimitedRU(t *testing.T) {
	snapshot, err := t22Resolve(t, map[string]testQueryResult{
		testResourceGroupFragment: {
			columns: testResourceGroupColumns,
			rows:    [][]driver.Value{{"rg_u", "UNLIMITED", "MEDIUM", nil, nil}},
		},
	}, spec.DialectTiDB, spec.ObjectLookupRequest{Type: "resource_group", Name: "rg_u"})
	if err != nil {
		t.Fatalf("ResolveObject: %v", err)
	}
	def := snapshot.ResourceGroup
	if def == nil || !def.RUUnlimited || def.RUPerSec != nil {
		t.Fatalf("definition = %+v, want RUUnlimited without a numeric rate", def)
	}
}

// Zero rows stay unknown: RESOURCE_GROUPS filters by privilege, so an
// invisible group is never a proven absence.
func TestT22ResourceGroupLoaderZeroRowsStaysUnknown(t *testing.T) {
	snapshot, err := t22Resolve(t, map[string]testQueryResult{
		testResourceGroupFragment: {columns: testResourceGroupColumns},
	}, spec.DialectTiDB, spec.ObjectLookupRequest{Type: "resource_group", Name: "ghost"})
	if err != nil {
		t.Fatalf("ResolveObject: %v", err)
	}
	if !snapshot.IsUnavailable() || snapshot.IsNotFound() || snapshot.Exists {
		t.Fatalf("snapshot = %#v, want unavailable — never fabricated absence", snapshot)
	}
}

// NULL and unrecognized catalog members map to unproven fields — never a
// guessed zero, default, or cleared flag.
func TestT22ResourceGroupLoaderNullMembersStayUnproven(t *testing.T) {
	snapshot, err := t22Resolve(t, map[string]testQueryResult{
		testResourceGroupFragment: {
			columns: testResourceGroupColumns,
			rows:    [][]driver.Value{{"rg", "bogus", nil, "SURPRISING", nil}},
		},
	}, spec.DialectTiDB, spec.ObjectLookupRequest{Type: "resource_group", Name: "rg"})
	if err != nil {
		t.Fatalf("ResolveObject: %v", err)
	}
	def := snapshot.ResourceGroup
	if def == nil {
		t.Fatal("definition = nil on a confirmed row")
	}
	if def.RUPerSec != nil || def.RUUnlimited || def.Priority != "" || def.Burstable != nil || def.QueryLimit != nil {
		t.Fatalf("definition = %+v, want NULL/unrecognized members unproven", def)
	}
}

// Catalog errors propagate wrapped with identity context — they are never
// degraded to unavailable and never cached as absence.
func TestT22ResourceGroupLoaderErrorPropagates(t *testing.T) {
	want := errors.New("resource_groups select denied")
	_, err := t22Resolve(t, map[string]testQueryResult{
		testResourceGroupFragment: {columns: testResourceGroupColumns, queryErr: want},
	}, spec.DialectTiDB, spec.ObjectLookupRequest{Type: "resource_group", Name: "rg"})
	if err == nil {
		t.Fatal("want propagated error")
	}
	if !errors.Is(err, want) {
		t.Fatalf("err = %v, want errors.Is(%v)", err, want)
	}
}

// MySQL's CPU resource-group family is a different surface — the resolver
// must not emit the TiDB RU catalog query under the wrong dialect.
func TestT22ResourceGroupLoaderDialectGated(t *testing.T) {
	db := openTestDB(t, map[string]testQueryResult{
		testResourceGroupFragment: {
			columns: testResourceGroupColumns,
			rows:    [][]driver.Value{{"rg", "100", "LOW", nil, nil}},
		},
	})
	provider := NewProvider(db)
	snapshot, err := provider.ResolveObject(context.Background(), spec.DialectMySQL, spec.ObjectLookupRequest{
		Type: "resource_group", Name: "rg",
	})
	if err != nil {
		t.Fatalf("ResolveObject: %v", err)
	}
	if !snapshot.IsUnavailable() || snapshot.Exists {
		t.Fatalf("snapshot = %#v, want unavailable — MySQL must not read the TiDB catalog", snapshot)
	}
}

// The tidb_enable_resource_control instance fact is observed through
// SHOW VARIABLES and carries an explicit known bit — an absent row stays
// unknown rather than a guessed default.
func TestT22ResourceControlFactPublished(t *testing.T) {
	db := openTestDB(t, map[string]testQueryResult{
		"show variables": {
			columns: []string{"Variable_name", "Value"},
			rows:    [][]driver.Value{{"tidb_enable_resource_control", "OFF"}},
		},
	})
	facts, err := NewProvider(db).LoadInstanceFacts(context.Background(), spec.DialectTiDB, "app")
	if err != nil {
		t.Fatalf("LoadInstanceFacts: %v", err)
	}
	if !facts.TiDBEnableResourceControlKnown || facts.TiDBEnableResourceControl {
		t.Fatalf("resource-control fact = {known:%t on:%t}, want known OFF", facts.TiDBEnableResourceControlKnown, facts.TiDBEnableResourceControl)
	}
}
