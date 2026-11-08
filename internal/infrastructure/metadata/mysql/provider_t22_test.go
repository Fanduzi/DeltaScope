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

func boolPtr(v bool) *bool { return &v }

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
				"golden_rg", "1000", "HIGH", "YES", "EXEC_ELAPSED='60s' ACTION=KILL",
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
		def.QueryLimit == nil || *def.QueryLimit != "EXEC_ELAPSED='60s' ACTION=KILL" ||
		def.QueryLimitState != spec.ResourceGroupQueryLimitConfigured {
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

// The TiDB 8.5 catalog emits the burst flag as YES/NO only — a NO row is a
// proven-disabled flag, and any other token (the later-version MODERATED /
// UNLIMITED vocabulary never appears on 8.5) stays unproven rather than a
// silently mapped truth.
func TestT22ResourceGroupLoaderBurstableYesNo(t *testing.T) {
	for _, tc := range []struct {
		token string
		want  *bool
	}{
		{"YES", boolPtr(true)},
		{"NO", boolPtr(false)},
		{"MODERATED", nil},
		{"UNLIMITED", nil},
		{"SURPRISING", nil},
	} {
		snapshot, err := t22Resolve(t, map[string]testQueryResult{
			testResourceGroupFragment: {
				columns: testResourceGroupColumns,
				rows:    [][]driver.Value{{"rg", "100", "LOW", tc.token, nil}},
			},
		}, spec.DialectTiDB, spec.ObjectLookupRequest{Type: "resource_group", Name: "rg"})
		if err != nil {
			t.Fatalf("ResolveObject(%q): %v", tc.token, err)
		}
		def := snapshot.ResourceGroup
		switch {
		case tc.want == nil && def.Burstable != nil:
			t.Fatalf("burstable = %v for token %q, want unproven", *def.Burstable, tc.token)
		case tc.want != nil && (def.Burstable == nil || *def.Burstable != *tc.want):
			t.Fatalf("burstable = %+v for token %q, want %v", def.Burstable, tc.token, *tc.want)
		}
	}
}

// QUERY_LIMIT on a complete RU-mode row distinguishes three real catalog
// states (v8.5 setDataFromResourceGroups emits SQL NULL only when no
// runaway rule is configured): NULL = confirmed unset, non-NULL text =
// configured with unmodeled member detail, and an all-NULL non-applicable
// row proves nothing about the column.
func TestT22ResourceGroupLoaderQueryLimitStates(t *testing.T) {
	t.Run("null on a complete row is confirmed absent", func(t *testing.T) {
		snapshot, err := t22Resolve(t, map[string]testQueryResult{
			testResourceGroupFragment: {
				columns: testResourceGroupColumns,
				rows:    [][]driver.Value{{"rg", "100", "MEDIUM", "NO", nil}},
			},
		}, spec.DialectTiDB, spec.ObjectLookupRequest{Type: "resource_group", Name: "rg"})
		if err != nil {
			t.Fatalf("ResolveObject: %v", err)
		}
		def := snapshot.ResourceGroup
		if def.QueryLimitState != spec.ResourceGroupQueryLimitConfirmedAbsent || def.QueryLimit != nil {
			t.Fatalf("query_limit = {%s %v}, want confirmed_absent on a NULL complete-row member", def.QueryLimitState, def.QueryLimit)
		}
	})
	t.Run("incomplete row proves nothing", func(t *testing.T) {
		snapshot, err := t22Resolve(t, map[string]testQueryResult{
			testResourceGroupFragment: {
				columns: testResourceGroupColumns,
				rows:    [][]driver.Value{{"rg", nil, nil, nil, nil}},
			},
		}, spec.DialectTiDB, spec.ObjectLookupRequest{Type: "resource_group", Name: "rg"})
		if err != nil {
			t.Fatalf("ResolveObject: %v", err)
		}
		def := snapshot.ResourceGroup
		if def.QueryLimitState != spec.ResourceGroupQueryLimitUnread {
			t.Fatalf("query_limit state = %s on an all-NULL row, want unread", def.QueryLimitState)
		}
	})
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
