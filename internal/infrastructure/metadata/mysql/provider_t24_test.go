package mysqlmeta

import (
	"context"
	"database/sql/driver"
	"errors"
	"testing"

	gomysql "github.com/go-sql-driver/mysql"

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
	// R6 additions: the observed server identity, the occupancy channel's
	// own version read, and the dropped-records counter that bounds
	// census completeness.
	t24FactsUUIDFragment    = "@@server_uuid"
	t24FactsVersionFragment = "select version()"
	t24FactsLostFragment    = "performance_schema_thread_instances_lost"
)

// t24FactsVersionRow stubs the occupancy channel's version read with a
// proven resource-group-capable banner.
func t24FactsVersionRow(banner string) testQueryResult {
	return testQueryResult{columns: []string{"version()"}, rows: [][]driver.Value{{banner}}}
}

// t24FactsLostRow stubs the dropped-thread-records counter.
func t24FactsLostRow(lost int64) testQueryResult {
	return testQueryResult{columns: []string{"Variable_name", "Value"}, rows: [][]driver.Value{{"Performance_schema_thread_instances_lost", lost}}}
}

func t24LoadFacts(t *testing.T, results map[string]testQueryResult, dialect spec.Dialect) (*spec.InstanceFacts, error) {
	t.Helper()
	db := openTestDB(t, results)
	provider := NewProvider(db)
	return provider.LoadMySQLResourceGroupFacts(context.Background(), dialect)
}

// The complete wire: variables, grants (no active roles — and NO
// PROCESS, proving the threads privilege model needs none), an enabled
// performance_schema, a proven-capable banner, a clean bound-set read
// with zero dropped records, an observed server uuid, and the builtin
// feature row — every premise answered.
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
				{"GRANT RESOURCE_GROUP_ADMIN, CONNECTION_ADMIN ON *.* TO 'u'@'%'"},
			},
		},
		t24FactsUUIDFragment:    {columns: []string{"@@server_uuid"}, rows: [][]driver.Value{{"aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"}}},
		t24FactsPFSFragment:     {columns: []string{"@@global.performance_schema"}, rows: [][]driver.Value{{"ON"}}},
		t24FactsVersionFragment: t24FactsVersionRow("8.0.36"),
		t24FactsThreadsFragment: {columns: []string{"resource_group"}, rows: [][]driver.Value{{"rg1"}, {"Other_RG"}}},
		t24FactsLostFragment:    t24FactsLostRow(0),
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
	if facts.MySQLServerUUID != "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee" || !facts.MySQLServerUUIDKnown {
		t.Fatalf("server uuid = %q known:%v, want the observed identity", facts.MySQLServerUUID, facts.MySQLServerUUIDKnown)
	}
	if !facts.MySQLResourceGroupBound["rg1"] || !facts.MySQLResourceGroupBound["other_rg"] {
		t.Fatalf("bound = %v, want both observed groups proven occupied (names normalized lowercase)", facts.MySQLResourceGroupBound)
	}
	if !facts.MySQLResourceGroupBoundComplete {
		t.Fatal("bound set must be proven complete — a clean read plus zero dropped records, no PROCESS required")
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

// A nonzero dropped-records counter means thread instances existed
// without a table row — observed bound names still prove occupancy, but
// a missing key is never a proven "unoccupied".
func TestT24FactsLoaderOccupancyPartialVisibility(t *testing.T) {
	facts, err := t24LoadFacts(t, map[string]testQueryResult{
		t24FactsRolesFragment:   {columns: []string{"current_role()"}, rows: [][]driver.Value{{"NONE"}}},
		t24FactsGrantFragment:   {columns: []string{"Grants for u@%"}, rows: [][]driver.Value{{"GRANT USAGE ON *.* TO 'u'@'%'"}}},
		t24FactsPFSFragment:     {columns: []string{"@@global.performance_schema"}, rows: [][]driver.Value{{"ON"}}},
		t24FactsVersionFragment: t24FactsVersionRow("8.0.36"),
		t24FactsThreadsFragment: {columns: []string{"resource_group"}, rows: [][]driver.Value{{"rg1"}}},
		t24FactsLostFragment:    t24FactsLostRow(3),
	}, spec.DialectMySQL)
	if err != nil {
		t.Fatalf("LoadMySQLResourceGroupFacts: %v", err)
	}
	if !facts.MySQLResourceGroupBound["rg1"] {
		t.Fatalf("bound = %v, want rg1 proven occupied", facts.MySQLResourceGroupBound)
	}
	if facts.MySQLResourceGroupBoundComplete {
		t.Fatal("bound completeness must not be claimed while thread records were dropped — hidden threads could be bound")
	}
}

// An unanswered dropped-records counter also keeps completeness
// unproven — the clean read alone cannot exclude lost rows — while a
// real counter failure propagates like every other read error rather
// than silently degrading into a partial census.
func TestT24FactsLoaderOccupancyLostCounterContract(t *testing.T) {
	facts, err := t24LoadFacts(t, map[string]testQueryResult{
		t24FactsPFSFragment:     {columns: []string{"@@global.performance_schema"}, rows: [][]driver.Value{{"ON"}}},
		t24FactsVersionFragment: t24FactsVersionRow("8.0.36"),
		t24FactsThreadsFragment: {columns: []string{"resource_group"}},
	}, spec.DialectMySQL)
	if err != nil {
		t.Fatalf("LoadMySQLResourceGroupFacts: %v", err)
	}
	if facts.MySQLResourceGroupBound == nil || len(facts.MySQLResourceGroupBound) != 0 {
		t.Fatalf("bound = %v, want the proven empty bound set", facts.MySQLResourceGroupBound)
	}
	if facts.MySQLResourceGroupBoundComplete {
		t.Fatal("bound completeness must not be claimed when the lost-records counter is unobserved")
	}
	want := errors.New("counter read denied")
	if _, err := t24LoadFacts(t, map[string]testQueryResult{
		t24FactsPFSFragment:     {columns: []string{"@@global.performance_schema"}, rows: [][]driver.Value{{"ON"}}},
		t24FactsVersionFragment: t24FactsVersionRow("8.0.36"),
		t24FactsThreadsFragment: {columns: []string{"resource_group"}},
		t24FactsLostFragment:    {queryErr: want},
	}, spec.DialectMySQL); !errors.Is(err, want) {
		t.Fatalf("err = %v, want errors.Is(%v) — a counter failure is a read error, not a normal missing value", err, want)
	}
}

// A missing threads.RESOURCE_GROUP column is a legitimate degradation
// only while the server could lack the feature — on a proven 8.0.3+
// banner the column must exist and the error propagates; on a pre-8.0.3
// banner it stays unproven.
func TestT24FactsLoaderOccupancyColumnVersionGated(t *testing.T) {
	missingColumn := &gomysql.MySQLError{Number: 1054, Message: "Unknown column 'resource_group' in 'field list'"}
	mustNotQuery := errors.New("threads query must not be issued")
	facts, err := t24LoadFacts(t, map[string]testQueryResult{
		t24FactsPFSFragment:     {columns: []string{"@@global.performance_schema"}, rows: [][]driver.Value{{"ON"}}},
		t24FactsVersionFragment: t24FactsVersionRow("5.7.44"),
		// A proven pre-8.0.3 server can never hold the column — the
		// doomed query is skipped, and a sentinel error proves it never
		// ran. The bound set stays unproven rather than "observed empty".
		t24FactsThreadsFragment: {queryErr: mustNotQuery},
	}, spec.DialectMySQL)
	if err != nil {
		t.Fatalf("5.7: LoadMySQLResourceGroupFacts: %v, want the occupancy channel skipped", err)
	}
	if facts.MySQLResourceGroupBound != nil {
		t.Fatalf("5.7: bound = %v, want unproven — the channel is inapplicable", facts.MySQLResourceGroupBound)
	}
	if _, err := t24LoadFacts(t, map[string]testQueryResult{
		t24FactsPFSFragment:     {columns: []string{"@@global.performance_schema"}, rows: [][]driver.Value{{"ON"}}},
		t24FactsVersionFragment: t24FactsVersionRow("8.0.36"),
		t24FactsThreadsFragment: {queryErr: missingColumn},
	}, spec.DialectMySQL); err == nil {
		t.Fatal("8.0.36: a missing RESOURCE_GROUP column must propagate — the feature guarantees it")
	}
}

// A disabled performance_schema leaves every occupancy fact unproven —
// and the threads/lost queries are never issued on an inapplicable
// channel (sentinel errors prove none ran). The base version read stays
// legal: the feature channel needs it to gate its own probe, so it is
// answered normally rather than sentinel-blocked.
func TestT24FactsLoaderPerformanceSchemaOff(t *testing.T) {
	mustNotQuery := errors.New("occupancy channel query must not be issued")
	facts, err := t24LoadFacts(t, map[string]testQueryResult{
		t24FactsPFSFragment:     {columns: []string{"@@global.performance_schema"}, rows: [][]driver.Value{{"OFF"}}},
		t24FactsVersionFragment: t24FactsVersionRow("8.0.36"),
		t24FactsThreadsFragment: {queryErr: mustNotQuery},
		t24FactsLostFragment:    {queryErr: mustNotQuery},
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
// The observed-identity read follows the same contract: a failed
// @@server_uuid query fails the bundle, so an online identity lookup
// error can never be laundered into a declared-target match.
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
	wantUUID := errors.New("uuid read denied")
	if _, err := t24LoadFacts(t, map[string]testQueryResult{
		t24FactsUUIDFragment: {queryErr: wantUUID},
	}, spec.DialectMySQL); !errors.Is(err, wantUUID) {
		t.Fatalf("err = %v, want errors.Is(%v) — a failed identity observation is an error, not a declared-target fallback", err, wantUUID)
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

// T24-R13 D2 contract: the per-object snapshot degrades a missing-object
// answer (pre-8.0 catalog absent — the version premise adjudicates) but
// never swallows a real query error on a supported server.
func TestT24ResourceGroupLoaderMissingObjectDegrades(t *testing.T) {
	snapshot, err := t24Resolve(t, map[string]testQueryResult{
		testResourceGroupFragment: {
			columns:  testMySQLResourceGroupColumns,
			queryErr: &gomysql.MySQLError{Number: 1109, Message: "Unknown table 'resource_groups' in information_schema"},
		},
	}, spec.DialectMySQL, spec.ObjectLookupRequest{Type: "mysql_resource_group", Name: "rg57"})
	if err != nil {
		t.Fatalf("missing table must degrade to unproven, got %v", err)
	}
	if snapshot == nil || snapshot.IsConfirmed() {
		t.Fatalf("snapshot = %#v, want unproven", snapshot)
	}
}

func TestT24ResourceGroupLoaderRealErrorPropagates(t *testing.T) {
	denied := &gomysql.MySQLError{Number: 1142, Message: "SELECT command denied"}
	if _, err := t24Resolve(t, map[string]testQueryResult{
		testResourceGroupFragment: {
			columns:  testMySQLResourceGroupColumns,
			queryErr: denied,
		},
	}, spec.DialectMySQL, spec.ObjectLookupRequest{Type: "mysql_resource_group", Name: "rg84"}); !errors.Is(err, denied) {
		t.Fatalf("err = %v, want errors.Is(1142)", err)
	}
}

// ── T24-R15 D2: one version contract gates every dedicated read ──
// Sentinel errors on the dedicated fragments prove the query was never
// issued; the observed version row is what arms the gate (a base read
// that is always legal). The same contract binds the bundle's feature
// probe, the occupancy channel (already gated), and the per-object
// snapshot reader.

// A proven pre-8.0.3 server must never see the information_schema
// resource-group reads — not the feature probe, not the threads census,
// not the per-object snapshot. Each dedicated fragment carries a
// sentinel so an issued query fails loudly instead of degrading.
func TestT24R15VersionGateSkipsDedicatedQueries(t *testing.T) {
	mustNotQuery := errors.New("dedicated resource-group query must not be issued")
	facts, err := t24LoadFacts(t, map[string]testQueryResult{
		t24FactsPFSFragment:     {columns: []string{"@@global.performance_schema"}, rows: [][]driver.Value{{"ON"}}},
		t24FactsVersionFragment: t24FactsVersionRow("5.7.44"),
		t24FactsThreadsFragment: {queryErr: mustNotQuery},
		t24FactsLostFragment:    {queryErr: mustNotQuery},
		t24FactsFeatureFragment: {queryErr: mustNotQuery},
	}, spec.DialectMySQL)
	if err != nil {
		t.Fatalf("5.7: LoadMySQLResourceGroupFacts: %v, want every dedicated query gated out", err)
	}
	if facts.MySQLResourceGroupFeatureKnown || facts.MySQLResourceGroupBound != nil {
		t.Fatalf("5.7: facts = %+v, want feature/bound unproven — never queried, never fabricated", facts)
	}
	snapshot, err := t24Resolve(t, map[string]testQueryResult{
		t24FactsVersionFragment:   t24FactsVersionRow("5.7.44"),
		testResourceGroupFragment: {columns: testMySQLResourceGroupColumns, queryErr: mustNotQuery},
	}, spec.DialectMySQL, spec.ObjectLookupRequest{Type: "mysql_resource_group", Name: "rg57"})
	if err != nil {
		t.Fatalf("5.7: ResolveObject: %v, want the catalog query gated out", err)
	}
	if snapshot.IsConfirmed() {
		t.Fatalf("5.7: snapshot = %#v, want unproven", snapshot)
	}
}

// On a proven supported server the catalog objects must exist — a
// missing-object answer is a real failure and propagates at both the
// snapshot and the feature-probe levels, on both supported majors.
func TestT24R15SupportedVersionMissingObjectPropagates(t *testing.T) {
	missing := &gomysql.MySQLError{Number: 1109, Message: "Unknown table 'resource_groups'"}
	for _, banner := range []string{"8.0.46", "8.4.10"} {
		if _, err := t24Resolve(t, map[string]testQueryResult{
			t24FactsVersionFragment:   t24FactsVersionRow(banner),
			testResourceGroupFragment: {columns: testMySQLResourceGroupColumns, queryErr: missing},
		}, spec.DialectMySQL, spec.ObjectLookupRequest{Type: "mysql_resource_group", Name: "rg"}); !errors.Is(err, missing) {
			t.Fatalf("%s: err = %v, want errors.Is(1109) — a supported server never degrades a real catalog error", banner, err)
		}
	}
	if _, err := t24LoadFacts(t, map[string]testQueryResult{
		t24FactsPFSFragment:     {columns: []string{"@@global.performance_schema"}, rows: [][]driver.Value{{"ON"}}},
		t24FactsVersionFragment: t24FactsVersionRow("8.0.46"),
		t24FactsFeatureFragment: {queryErr: missing},
	}, spec.DialectMySQL); !errors.Is(err, missing) {
		t.Fatalf("8.0.46 feature probe: err = %v, want errors.Is(1109)", err)
	}
}

// An unproven version keeps the narrow missing-object degrade (the
// channel may not exist at all); a failed version observation is itself
// an error and propagates — it is never laundered into "unknown".
func TestT24R15VersionGateUnknownAndError(t *testing.T) {
	// Version unobservable (zero rows) → missing-object answers degrade.
	snapshot, err := t24Resolve(t, map[string]testQueryResult{
		testResourceGroupFragment: {columns: testMySQLResourceGroupColumns,
			queryErr: &gomysql.MySQLError{Number: 1146, Message: "Table 'resource_groups' doesn't exist"}},
	}, spec.DialectMySQL, spec.ObjectLookupRequest{Type: "mysql_resource_group", Name: "rg"})
	if err != nil || snapshot == nil || snapshot.IsConfirmed() {
		t.Fatalf("unknown-version degrade: snapshot=%#v err=%v, want unproven nil error", snapshot, err)
	}
	denied := errors.New("version read denied")
	if _, err := t24Resolve(t, map[string]testQueryResult{
		t24FactsVersionFragment:   {queryErr: denied},
		testResourceGroupFragment: {columns: testMySQLResourceGroupColumns},
	}, spec.DialectMySQL, spec.ObjectLookupRequest{Type: "mysql_resource_group", Name: "rg"}); !errors.Is(err, denied) {
		t.Fatalf("version-read failure: err = %v, want errors.Is(denied) — never laundered into unknown", err)
	}
	if _, err := t24LoadFacts(t, map[string]testQueryResult{
		t24FactsVersionFragment: {queryErr: denied},
	}, spec.DialectMySQL); !errors.Is(err, denied) {
		t.Fatalf("bundle version failure: err = %v, want errors.Is(denied)", err)
	}
}

// Driver-level cancellation and permission answers propagate under any
// version — they are never missing-object answers and never degrade.
func TestT24R15NonMissingErrorsPropagate(t *testing.T) {
	if _, err := t24Resolve(t, map[string]testQueryResult{
		t24FactsVersionFragment:   t24FactsVersionRow("8.0.46"),
		testResourceGroupFragment: {columns: testMySQLResourceGroupColumns, queryErr: context.Canceled},
	}, spec.DialectMySQL, spec.ObjectLookupRequest{Type: "mysql_resource_group", Name: "rg"}); !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want errors.Is(context.Canceled)", err)
	}
	denied := &gomysql.MySQLError{Number: 1142, Message: "SELECT command denied"}
	if _, err := t24LoadFacts(t, map[string]testQueryResult{
		t24FactsPFSFragment:     {columns: []string{"@@global.performance_schema"}, rows: [][]driver.Value{{"ON"}}},
		t24FactsVersionFragment: t24FactsVersionRow("8.0.46"),
		t24FactsFeatureFragment: {queryErr: denied},
	}, spec.DialectMySQL); !errors.Is(err, denied) {
		t.Fatalf("feature probe 1142: err = %v, want errors.Is(1142)", err)
	}
}
