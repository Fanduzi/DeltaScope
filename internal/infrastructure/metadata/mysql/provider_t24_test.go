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

// ── T24-R4: LoadMySQLResourceGroupFacts — the ordinary-client premise
// bundle on one session. Fragments are longest-match keys against the
// stub driver's literal queries.

const (
	t24FactsVarsFragment    = "show global variables"
	t24FactsRolesFragment   = "select current_role()"
	t24FactsGrantFragment   = "show grants for current_user()"
	t24FactsUsingFragment   = "show grants for current_user() using"
	t24FactsPFSFragment     = "@@global.performance_schema"
	t24FactsThreadsFragment = "performance_schema.threads"
	// The feature probe must out-score the object loader's
	// information_schema.resource_groups fragment (39 chars) — this
	// predicate appears only in the probe query.
	t24FactsFeatureFragment = "resource_group_name = 'usr_default'"
)

func t24LoadFacts(t *testing.T, results map[string]testQueryResult, dialect spec.Dialect) (*spec.InstanceFacts, error) {
	t.Helper()
	db := openTestDB(t, results)
	provider := NewProvider(db)
	return provider.LoadMySQLResourceGroupFacts(context.Background(), dialect)
}

// The complete wire: variables, grants (no active roles), a
// PROCESS-bearing grant set, an enabled performance_schema, and the
// builtin feature row — every premise answered.
func TestT24FactsLoaderCompleteWire(t *testing.T) {
	facts, err := t24LoadFacts(t, map[string]testQueryResult{
		t24FactsVarsFragment: {
			columns: []string{"Variable_name", "Value"},
			rows: [][]driver.Value{
				{"read_only", "OFF"},
				{"super_read_only", "OFF"},
			},
		},
		t24FactsRolesFragment: {columns: []string{"current_role()"}, rows: [][]driver.Value{{"NONE"}}},
		t24FactsGrantFragment: {
			columns: []string{"Grants for u@%"},
			rows: [][]driver.Value{
				{"GRANT USAGE ON *.* TO 'u'@'%'"},
				{"GRANT PROCESS, RESOURCE_GROUP_ADMIN, CONNECTION_ADMIN ON *.* TO 'u'@'%'"},
			},
		},
		t24FactsPFSFragment:     {columns: []string{"@@global.performance_schema"}, rows: [][]driver.Value{{"ON"}}},
		t24FactsThreadsFragment: {columns: []string{"resource_group"}, rows: [][]driver.Value{{"rg1"}, {"Other_RG"}}},
		t24FactsFeatureFragment: {columns: []string{"resource_group_name"}, rows: [][]driver.Value{{"usr_default"}}},
	}, spec.DialectMySQL)
	if err != nil {
		t.Fatalf("LoadMySQLResourceGroupFacts: %v", err)
	}
	if !facts.MySQLReadOnlyKnown || facts.MySQLReadOnly {
		t.Fatalf("read_only = known:%v value:%v, want known OFF", facts.MySQLReadOnlyKnown, facts.MySQLReadOnly)
	}
	if !facts.MySQLSuperReadOnlyKnown || facts.MySQLSuperReadOnly {
		t.Fatalf("super_read_only = known:%v value:%v, want known OFF", facts.MySQLSuperReadOnlyKnown, facts.MySQLSuperReadOnly)
	}
	if !facts.MySQLResourceGroupAdminKnown || !facts.MySQLResourceGroupAdmin {
		t.Fatalf("admin = known:%v value:%v, want known granted", facts.MySQLResourceGroupAdminKnown, facts.MySQLResourceGroupAdmin)
	}
	if !facts.MySQLReadOnlyExemptKnown || !facts.MySQLReadOnlyExempt {
		t.Fatalf("exempt = known:%v value:%v, want known via CONNECTION_ADMIN", facts.MySQLReadOnlyExemptKnown, facts.MySQLReadOnlyExempt)
	}
	if !facts.MySQLResourceGroupFeatureKnown || !facts.MySQLResourceGroupFeatureSupported {
		t.Fatalf("feature = known:%v value:%v, want proven supported by the builtin row", facts.MySQLResourceGroupFeatureKnown, facts.MySQLResourceGroupFeatureSupported)
	}
	if !facts.MySQLResourceGroupBound["rg1"] || !facts.MySQLResourceGroupBound["other_rg"] {
		t.Fatalf("bound = %v, want both observed groups proven occupied (names normalized lowercase)", facts.MySQLResourceGroupBound)
	}
	if !facts.MySQLResourceGroupBoundComplete {
		t.Fatal("bound set must be proven complete — PROCESS grants full thread visibility")
	}
	// Host CPU count and priority capability are never SQL-observable —
	// the provider must not fabricate them.
	if facts.MySQLHostVCPUsKnown || facts.MySQLResourceGroupPriorityCapKnown {
		t.Fatal("host vcpus/priority capability must stay unproven — no SQL channel observes them")
	}
}

// Active roles augment the effective set: the base grant shows USAGE
// while the role carries RESOURCE_GROUP_ADMIN — admin is proven, and
// the still-negative exemption answer is proven too because the read
// was complete.
func TestT24FactsLoaderRoleAugmented(t *testing.T) {
	facts, err := t24LoadFacts(t, map[string]testQueryResult{
		t24FactsVarsFragment: {
			columns: []string{"Variable_name", "Value"},
			rows:    [][]driver.Value{{"read_only", "ON"}},
		},
		t24FactsRolesFragment: {columns: []string{"current_role()"}, rows: [][]driver.Value{{"'rg_admin'@'%'"}}},
		t24FactsGrantFragment: {
			columns: []string{"Grants for u@%"},
			rows:    [][]driver.Value{{"GRANT USAGE ON *.* TO 'u'@'%'"}},
		},
		t24FactsUsingFragment: {
			columns: []string{"Grants for u@%"},
			rows:    [][]driver.Value{{"GRANT RESOURCE_GROUP_ADMIN, PROCESS ON *.* TO 'u'@'%'"}},
		},
	}, spec.DialectMySQL)
	if err != nil {
		t.Fatalf("LoadMySQLResourceGroupFacts: %v", err)
	}
	if !facts.MySQLResourceGroupAdminKnown || !facts.MySQLResourceGroupAdmin {
		t.Fatalf("admin = known:%v value:%v, want proven via the active role", facts.MySQLResourceGroupAdminKnown, facts.MySQLResourceGroupAdmin)
	}
	if !facts.MySQLReadOnlyExemptKnown || facts.MySQLReadOnlyExempt {
		t.Fatalf("exempt = known:%v value:%v, want proven-negative under a complete read", facts.MySQLReadOnlyExemptKnown, facts.MySQLReadOnlyExempt)
	}
}

// An unanswerable CURRENT_ROLE() (stubbed as no rows, the pre-8.0
// surface) degrades completeness: a base-grant positive still proves,
// but no negative answer is ever claimed — an unreadable role could
// carry the privilege.
func TestT24FactsLoaderRolesUnreadablePositiveOnly(t *testing.T) {
	facts, err := t24LoadFacts(t, map[string]testQueryResult{
		t24FactsGrantFragment: {
			columns: []string{"Grants for u@%"},
			rows:    [][]driver.Value{{"GRANT USAGE ON *.* TO 'u'@'%'"}},
		},
	}, spec.DialectMySQL)
	if err != nil {
		t.Fatalf("LoadMySQLResourceGroupFacts: %v", err)
	}
	if facts.MySQLResourceGroupAdminKnown || facts.MySQLResourceGroupAdmin {
		t.Fatalf("admin = known:%v value:%v, want unproven — active roles were never readable", facts.MySQLResourceGroupAdminKnown, facts.MySQLResourceGroupAdmin)
	}
	if facts.MySQLReadOnlyExemptKnown {
		t.Fatalf("exempt = known:%v value:%v, want unproven under partial visibility", facts.MySQLReadOnlyExemptKnown, facts.MySQLReadOnlyExempt)
	}
}

// RESOURCE_GROUP_USER is not RESOURCE_GROUP_ADMIN — bounded exact-token
// parsing never inflates the user privilege into the admin one.
func TestT24FactsLoaderResourceGroupUserIsNotAdmin(t *testing.T) {
	facts, err := t24LoadFacts(t, map[string]testQueryResult{
		t24FactsRolesFragment: {columns: []string{"current_role()"}, rows: [][]driver.Value{{"NONE"}}},
		t24FactsGrantFragment: {
			columns: []string{"Grants for u@%"},
			rows:    [][]driver.Value{{"GRANT USAGE, RESOURCE_GROUP_USER ON *.* TO 'u'@'%'"}},
		},
	}, spec.DialectMySQL)
	if err != nil {
		t.Fatalf("LoadMySQLResourceGroupFacts: %v", err)
	}
	if !facts.MySQLResourceGroupAdminKnown || facts.MySQLResourceGroupAdmin {
		t.Fatalf("admin = known:%v value:%v, want proven-negative — RESOURCE_GROUP_USER is not ADMIN", facts.MySQLResourceGroupAdminKnown, facts.MySQLResourceGroupAdmin)
	}
}

// Partial thread visibility — a bound row still proves its group's
// occupancy, but without PROCESS the missing keys stay unproven.
func TestT24FactsLoaderOccupancyPartialVisibility(t *testing.T) {
	facts, err := t24LoadFacts(t, map[string]testQueryResult{
		t24FactsRolesFragment:   {columns: []string{"current_role()"}, rows: [][]driver.Value{{"NONE"}}},
		t24FactsGrantFragment:   {columns: []string{"Grants for u@%"}, rows: [][]driver.Value{{"GRANT USAGE ON *.* TO 'u'@'%'"}}},
		t24FactsPFSFragment:     {columns: []string{"@@global.performance_schema"}, rows: [][]driver.Value{{"ON"}}},
		t24FactsThreadsFragment: {columns: []string{"resource_group"}, rows: [][]driver.Value{{"rg1"}}},
	}, spec.DialectMySQL)
	if err != nil {
		t.Fatalf("LoadMySQLResourceGroupFacts: %v", err)
	}
	if !facts.MySQLResourceGroupBound["rg1"] {
		t.Fatalf("bound = %v, want rg1 proven occupied", facts.MySQLResourceGroupBound)
	}
	if facts.MySQLResourceGroupBoundComplete {
		t.Fatal("bound completeness must not be claimed without PROCESS — hidden threads could be bound")
	}
}

// A disabled performance_schema leaves every occupancy fact unproven.
func TestT24FactsLoaderPerformanceSchemaOff(t *testing.T) {
	facts, err := t24LoadFacts(t, map[string]testQueryResult{
		t24FactsPFSFragment: {columns: []string{"@@global.performance_schema"}, rows: [][]driver.Value{{"OFF"}}},
	}, spec.DialectMySQL)
	if err != nil {
		t.Fatalf("LoadMySQLResourceGroupFacts: %v", err)
	}
	if facts.MySQLResourceGroupBound != nil || facts.MySQLResourceGroupBoundComplete {
		t.Fatalf("bound = %v complete:%v, want unproven under pfs OFF", facts.MySQLResourceGroupBound, facts.MySQLResourceGroupBoundComplete)
	}
}

// A zero-row feature probe stays unproven — catalog visibility is not
// documented as unfiltered, so emptiness never claims the feature
// absent.
func TestT24FactsLoaderFeatureProbeZeroRows(t *testing.T) {
	facts, err := t24LoadFacts(t, map[string]testQueryResult{
		t24FactsFeatureFragment: {columns: []string{"resource_group_name"}},
	}, spec.DialectMySQL)
	if err != nil {
		t.Fatalf("LoadMySQLResourceGroupFacts: %v", err)
	}
	if facts.MySQLResourceGroupFeatureKnown {
		t.Fatalf("feature = known:%v value:%v, want unproven on a zero-row probe", facts.MySQLResourceGroupFeatureKnown, facts.MySQLResourceGroupFeatureSupported)
	}
}

// A hard channel failure propagates — never degraded to unknown facts.
func TestT24FactsLoaderErrorPropagates(t *testing.T) {
	want := errors.New("variables read denied")
	_, err := t24LoadFacts(t, map[string]testQueryResult{
		t24FactsVarsFragment: {columns: []string{"Variable_name", "Value"}, queryErr: want},
	}, spec.DialectMySQL)
	if err == nil {
		t.Fatal("want propagated error")
	}
	if !errors.Is(err, want) {
		t.Fatalf("err = %v, want errors.Is(%v)", err, want)
	}
}

// The bundle is MySQL-scoped — TiDB never pays for or receives the
// MySQL premise reads.
func TestT24FactsLoaderDialectGated(t *testing.T) {
	facts, err := t24LoadFacts(t, map[string]testQueryResult{}, spec.DialectTiDB)
	if err != nil {
		t.Fatalf("LoadMySQLResourceGroupFacts: %v", err)
	}
	if facts != nil {
		t.Fatalf("facts = %#v, want nil — the MySQL bundle never cross-loads", facts)
	}
}
