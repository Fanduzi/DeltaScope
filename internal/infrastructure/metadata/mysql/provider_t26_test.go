package mysqlmeta

import (
	"context"
	"database/sql/driver"
	"errors"
	"strings"
	"testing"

	gomysql "github.com/go-sql-driver/mysql"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// T26/#105 provider round: the official ResolveObject mysql_server and
// mysql_spatial_reference_system paths read the real catalog surfaces —
// mysql.servers, information_schema.ST_SPATIAL_REFERENCE_SYSTEMS, and
// ST_GEOMETRY_COLUMNS — through the same provider entry, gated on
// identity provability, dialect, and the shared 8.0 version contract.
// No fake resolver is substituted; the stub driver records every issued
// query so tests assert which reads actually fired.
var (
	testServerIdentityFragment = "from mysql.servers"
	testSRSIdentityFragment    = "srs_id, srs_name"
	testSRSNameFragment        = "where srs_name = ?"
	testSRSOrgFragment         = "organization_coordsys_id = ?"
	testSRSDepFragment         = "from information_schema.st_geometry_columns"
	testSRSVersionFragment     = "select version()"
)

func t26Resolve(t *testing.T, results map[string]testQueryResult, dialect spec.Dialect, req spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
	t.Helper()
	db := openTestDB(t, results)
	provider := NewProvider(db)
	return provider.ResolveObject(context.Background(), dialect, req)
}

func t26Calls(t *testing.T, fragment string) []testQueryCall {
	t.Helper()
	var out []testQueryCall
	for _, call := range testQueryCallsByName(testStubName(t)) {
		if strings.Contains(call.query, fragment) {
			out = append(out, call)
		}
	}
	return out
}

func t26VersionRow(banner string) testQueryResult {
	return testQueryResult{columns: []string{"version()"}, rows: [][]driver.Value{{banner}}}
}

func t26IdentLookup(name string) spec.ObjectLookupRequest {
	return spec.ObjectLookupRequest{
		Type: "mysql_server",
		Name: name,
		Qualifiers: map[string]string{
			spec.ObjectLookupQualifierNameForm: spec.ObjectLookupNameFormIdentifier,
		},
	}
}

// A provable identifier name reads mysql.servers once — the catalog row
// proves presence and returns the stored spelling as the witness.
func TestT26ServerIdentityRowConfirms(t *testing.T) {
	snapshot, err := t26Resolve(t, map[string]testQueryResult{
		testServerIdentityFragment: {columns: []string{"SERVER_NAME"}, rows: [][]driver.Value{{"srv_a"}}},
	}, spec.DialectMySQL, t26IdentLookup("srv_a"))
	if err != nil {
		t.Fatalf("ResolveObject: %v", err)
	}
	if !snapshot.IsConfirmed() || !snapshot.Exists {
		t.Fatalf("snapshot = %#v, want confirmed present", snapshot)
	}
	state := snapshot.Server
	if state == nil || !state.Read || !state.Exists || state.Status != spec.MetadataStatusConfirmed || state.Name != "srv_a" {
		t.Fatalf("server state = %+v, want read+confirmed+exists with catalog name", state)
	}
	calls := t26Calls(t, testServerIdentityFragment)
	if len(calls) != 1 {
		t.Fatalf("server queries = %d, want exactly one", len(calls))
	}
	if !strings.Contains(calls[0].query, "where server_name = ?") {
		t.Fatalf("server predicate %q must be parameterized on the identity", calls[0].query)
	}
	if len(calls[0].args) != 1 || calls[0].args[0].Value != "srv_a" {
		t.Fatalf("server args = %+v, want exactly the authored name", calls[0].args)
	}
	if strings.Contains(strings.ToLower(calls[0].query), "password") ||
		strings.Contains(calls[0].query, "definition") {
		t.Fatalf("server query must never select connection payloads: %q", calls[0].query)
	}
}

// A clean zero-row answer stays unknown — never proven-absent: the
// read ran, but no approved completeness contract exists for
// mysql.servers visibility. The bound state keeps the verbatim decoded
// name — zero rows never rewrite the authored identity.
func TestT26ServerZeroRowsStaysUnknown(t *testing.T) {
	snapshot, err := t26Resolve(t, map[string]testQueryResult{
		testServerIdentityFragment: {columns: []string{"SERVER_NAME"}},
	}, spec.DialectMySQL, t26IdentLookup(" ghost"))
	if err != nil {
		t.Fatalf("ResolveObject: %v", err)
	}
	if snapshot.IsNotFound() || !snapshot.IsUnavailable() || snapshot.Exists {
		t.Fatalf("snapshot = %#v, want unavailable — zero rows never fabricate absence", snapshot)
	}
	state := snapshot.Server
	if state == nil || !state.Read || state.Exists {
		t.Fatalf("server state = %+v, want issued read + unproven answer", state)
	}
	if state.Name != " ghost" {
		t.Fatalf("server state name = %q, want verbatim decoded %q — zero rows never normalize identity", state.Name, " ghost")
	}
}

// An unprovable name (no name_form=identifier qualifier — a
// string-literal spelling or an absent qualifier) issues no query at
// all, keeps the state bound-but-unread, and caches nothing.
func TestT26ServerUnprovableIdentityNeverQueries(t *testing.T) {
	snapshot, err := t26Resolve(t, map[string]testQueryResult{
		testServerIdentityFragment: {queryErr: errors.New("server query must not be issued")},
	}, spec.DialectMySQL, spec.ObjectLookupRequest{Type: "mysql_server", Name: "spelled 'name"})
	if err != nil {
		t.Fatalf("ResolveObject: %v, want silent unproven bind", err)
	}
	state := snapshot.Server
	if state == nil || state.Read || state.UnreadReason != spec.MetadataUnreadIdentityUnproven {
		t.Fatalf("server state = %+v, want bound + identity_unproven", state)
	}
	if calls := t26Calls(t, testServerIdentityFragment); len(calls) != 0 {
		t.Fatalf("unprovable identity issued %d queries — spelling must never normalize", len(calls))
	}
	// A qualifier carrying a different value is equally unprovable.
	snapshot, err = t26Resolve(t, map[string]testQueryResult{
		testServerIdentityFragment: {queryErr: errors.New("server query must not be issued")},
	}, spec.DialectMySQL, spec.ObjectLookupRequest{
		Type: "mysql_server", Name: "x",
		Qualifiers: map[string]string{spec.ObjectLookupQualifierNameForm: "spelling"},
	})
	if err != nil || snapshot.Server == nil || snapshot.Server.Read {
		t.Fatalf("spelling qualifier must stay unissued: %+v err=%v", snapshot, err)
	}
	if calls := t26Calls(t, testServerIdentityFragment); len(calls) != 0 {
		t.Fatalf("non-identifier form issued %d queries", len(calls))
	}
}

// The catalog read failure propagates — never laundered into an
// unproven or absent answer, never cached.
func TestT26ServerQueryErrorPropagates(t *testing.T) {
	want := errors.New("mysql.servers select denied")
	_, err := t26Resolve(t, map[string]testQueryResult{
		testServerIdentityFragment: {queryErr: want},
	}, spec.DialectMySQL, t26IdentLookup("srv_a"))
	if !errors.Is(err, want) {
		t.Fatalf("err = %v, want errors.Is(%v)", err, want)
	}
}

// The SERVER catalog is a MySQL surface — under TiDB the resolver never
// emits the query, and no fact member is bound.
func TestT26ServerDialectGated(t *testing.T) {
	snapshot, err := t26Resolve(t, map[string]testQueryResult{
		testServerIdentityFragment: {queryErr: errors.New("server query must not be issued under tidb")},
	}, spec.DialectTiDB, t26IdentLookup("srv_a"))
	if err != nil {
		t.Fatalf("tidb: ResolveObject: %v, want gated out before any query", err)
	}
	if snapshot.Server != nil {
		t.Fatalf("tidb: server state = %+v, want unbound (nil)", snapshot.Server)
	}
	if calls := t26Calls(t, "mysql.servers"); len(calls) != 0 {
		t.Fatalf("tidb issued %d server queries", len(calls))
	}
}

// A proven SRS row confirms existence keyed on the authored SRID — the
// catalog name lands as witness and the dependency read fires on the
// same independently-provable identity.
func TestT26SRSIdentityRowConfirms(t *testing.T) {
	snapshot, err := t26Resolve(t, map[string]testQueryResult{
		testSRSVersionFragment:  t26VersionRow("8.0.46"),
		testSRSIdentityFragment: {columns: []string{"SRS_ID", "SRS_NAME"}, rows: [][]driver.Value{{int64(4326), "WGS 84"}}},
		testSRSDepFragment: {
			columns: []string{"TABLE_SCHEMA", "TABLE_NAME", "COLUMN_NAME"},
			rows:    [][]driver.Value{{"geo", "cities", "pt"}, {"gis", "roads", "path"}},
		},
	}, spec.DialectMySQL, spec.ObjectLookupRequest{Type: "mysql_spatial_reference_system", Name: "4326", RequestVersion: spec.ParseObservedVersion("8.0.46")})
	if err != nil {
		t.Fatalf("ResolveObject: %v", err)
	}
	if !snapshot.IsConfirmed() || !snapshot.Exists {
		t.Fatalf("snapshot = %#v, want confirmed present", snapshot)
	}
	state := snapshot.SpatialReference
	if state == nil || !state.Read || !state.Exists || state.SRID != 4326 || state.CatalogName != "WGS 84" {
		t.Fatalf("srs state = %+v, want confirmed 4326 with name witness", state)
	}
	if state.Dependencies == nil || len(state.Dependencies.Columns) != 2 {
		t.Fatalf("dependencies = %+v, want two cross-schema witnesses", state.Dependencies)
	}
	dep := state.Dependencies.Columns[0]
	if dep.Schema != "geo" || dep.Table != "cities" || dep.Column != "pt" || dep.SRID != 4326 {
		t.Fatalf("dep witness = %+v, want geo.cities.pt on 4326", dep)
	}
	if state.Dependencies.Complete {
		t.Fatalf("dependency read is never complete under filtered visibility")
	}
	// The SRID parameter is numeric and parameterized — never interpolated.
	calls := t26Calls(t, testSRSIdentityFragment)
	if len(calls) != 1 || len(calls[0].args) != 1 || calls[0].args[0].Value != int64(4326) {
		t.Fatalf("srs identity args = %+v, want one numeric 4326 parameter", calls)
	}
}

// O1=8.0.3 is outside the registered audited domain — under the R3
// single-authority binding the request-level identity closes the call
// before any query: no witness, no existence read, no deps read.
func TestT26SRSZeroRowsUnknownButDepsRead(t *testing.T) {
	snapshot, err := t26Resolve(t, map[string]testQueryResult{
		testSRSVersionFragment:  t26VersionRow("8.0.3"),
		testSRSIdentityFragment: {columns: []string{"SRS_ID", "SRS_NAME"}},
		testSRSDepFragment:      {columns: []string{"TABLE_SCHEMA", "TABLE_NAME", "COLUMN_NAME"}},
	}, spec.DialectMySQL, spec.ObjectLookupRequest{Type: "mysql_spatial_reference_system", Name: "9999", RequestVersion: spec.ParseObservedVersion("8.0.3")})
	if err != nil {
		t.Fatalf("ResolveObject: %v", err)
	}
	state := snapshot.SpatialReference
	if state == nil || state.Read || state.UnreadReason != spec.MetadataUnreadVersionInapplicable {
		t.Fatalf("srs state = %+v, want unread version_inapplicable", state)
	}
	if calls := t26Calls(t, testSRSDepFragment); len(calls) != 0 {
		t.Fatalf("dependency read issued %d times, want zero under an off-domain O1", len(calls))
	}
}

// A non-decimal Name is an unprovable SRID identity — no query at all,
// not even the version probe.
func TestT26SRSUnprovableIdentityNeverQueries(t *testing.T) {
	snapshot, err := t26Resolve(t, map[string]testQueryResult{
		testSRSVersionFragment:  {queryErr: errors.New("must not be issued")},
		testSRSIdentityFragment: {queryErr: errors.New("must not be issued")},
	}, spec.DialectMySQL, spec.ObjectLookupRequest{Type: "mysql_spatial_reference_system", Name: "WGS 84"})
	if err != nil {
		t.Fatalf("ResolveObject: %v, want silent unproven bind", err)
	}
	state := snapshot.SpatialReference
	if state == nil || state.Read || state.UnreadReason != spec.MetadataUnreadIdentityUnproven {
		t.Fatalf("srs state = %+v, want identity_unproven", state)
	}
	if total := len(testQueryCallsByName(testStubName(t))); total != 0 {
		t.Fatalf("unprovable srid issued %d queries — spelling never keys a read", total)
	}
}

// The NAME-conflict read fires only on the provable srs_name qualifier —
// matched SRIDs (the target's own and another) are both witnesses.
func TestT26SRSNameConflictWitnesses(t *testing.T) {
	snapshot, err := t26Resolve(t, map[string]testQueryResult{
		testSRSVersionFragment:  t26VersionRow("8.0.46"),
		testSRSIdentityFragment: {columns: []string{"SRS_ID", "SRS_NAME"}},
		testSRSNameFragment:     {columns: []string{"SRS_ID"}, rows: [][]driver.Value{{int64(4326)}, {int64(7777)}}},
		testSRSDepFragment:      {columns: []string{"TABLE_SCHEMA", "TABLE_NAME", "COLUMN_NAME"}},
	}, spec.DialectMySQL, spec.ObjectLookupRequest{
		Type: "mysql_spatial_reference_system",
		Name: "4326",
		Qualifiers: map[string]string{
			spec.ObjectLookupQualifierSRSName: "WGS 84",
		},
		RequestVersion: spec.ParseObservedVersion("8.0.46"),
	})
	if err != nil {
		t.Fatalf("ResolveObject: %v", err)
	}
	fact := snapshot.SpatialReference.NameConflict
	if fact == nil || len(fact.MatchedSRIDs) != 2 || fact.MatchedSRIDs[0] != 4326 || fact.MatchedSRIDs[1] != 7777 {
		t.Fatalf("name conflict = %+v, want witness SRIDs [4326 7777] — self and other both raw", fact)
	}
	if fact.Complete {
		t.Fatalf("conflict completeness is never claimed without an approved contract")
	}
	calls := t26Calls(t, testSRSNameFragment)
	if len(calls) != 1 || len(calls[0].args) != 1 || calls[0].args[0].Value != "WGS 84" {
		t.Fatalf("name conflict args = %+v, want the provable name as one parameter", calls)
	}
}

// Without the qualifier the NAME-conflict read never fires — an empty
// matched set when it does fire is "read, zero rows, unknown", not a
// proven no-conflict.
func TestT26SRSNameConflictZeroRowsAndAbsent(t *testing.T) {
	snapshot, err := t26Resolve(t, map[string]testQueryResult{
		testSRSVersionFragment:  t26VersionRow("8.0.46"),
		testSRSIdentityFragment: {columns: []string{"SRS_ID", "SRS_NAME"}},
		testSRSDepFragment:      {columns: []string{"TABLE_SCHEMA", "TABLE_NAME", "COLUMN_NAME"}},
	}, spec.DialectMySQL, spec.ObjectLookupRequest{Type: "mysql_spatial_reference_system", Name: "4326", RequestVersion: spec.ParseObservedVersion("8.0.46")})
	if err != nil {
		t.Fatalf("ResolveObject: %v", err)
	}
	state := snapshot.SpatialReference
	if state.NameConflict != nil {
		t.Fatalf("name conflict = %+v, want nil (never issued) without the qualifier", state.NameConflict)
	}
	if calls := t26Calls(t, testSRSNameFragment); len(calls) != 0 {
		t.Fatalf("unqualified name conflict issued %d queries", len(calls))
	}
	// With the qualifier and zero matching rows: issued, empty, unknown.
	snapshot, err = t26Resolve(t, map[string]testQueryResult{
		testSRSVersionFragment:  t26VersionRow("8.0.46"),
		testSRSIdentityFragment: {columns: []string{"SRS_ID", "SRS_NAME"}},
		testSRSNameFragment:     {columns: []string{"SRS_ID"}},
		testSRSDepFragment:      {columns: []string{"TABLE_SCHEMA", "TABLE_NAME", "COLUMN_NAME"}},
	}, spec.DialectMySQL, spec.ObjectLookupRequest{
		Type: "mysql_spatial_reference_system", Name: "4326",
		Qualifiers: map[string]string{spec.ObjectLookupQualifierSRSName: "Free Name"}, RequestVersion: spec.ParseObservedVersion("8.0.46"),
	})
	if err != nil {
		t.Fatalf("ResolveObject: %v", err)
	}
	fact := snapshot.SpatialReference.NameConflict
	if fact == nil || len(fact.MatchedSRIDs) != 0 || fact.Complete {
		t.Fatalf("zero-row name conflict = %+v, want issued+empty+incomplete — never proven none", fact)
	}
}

// The ORGANIZATION + ORGANIZATION_COORDSYS_ID conflict reads as one
// pair: both qualifiers provable fires the combined predicate; a
// partially-known pair issues nothing.
func TestT26SRSOrgConflictPairContract(t *testing.T) {
	snapshot, err := t26Resolve(t, map[string]testQueryResult{
		testSRSVersionFragment:  t26VersionRow("8.0.46"),
		testSRSIdentityFragment: {columns: []string{"SRS_ID", "SRS_NAME"}},
		testSRSOrgFragment:      {columns: []string{"SRS_ID"}, rows: [][]driver.Value{{int64(4326)}}},
		testSRSDepFragment:      {columns: []string{"TABLE_SCHEMA", "TABLE_NAME", "COLUMN_NAME"}},
	}, spec.DialectMySQL, spec.ObjectLookupRequest{
		Type: "mysql_spatial_reference_system", Name: "4326",
		Qualifiers: map[string]string{
			spec.ObjectLookupQualifierSRSOrg:   "EPSG",
			spec.ObjectLookupQualifierSRSOrgID: "4326",
		}, RequestVersion: spec.ParseObservedVersion("8.0.46"),
	})
	if err != nil {
		t.Fatalf("ResolveObject: %v", err)
	}
	fact := snapshot.SpatialReference.OrgConflict
	if fact == nil || len(fact.MatchedSRIDs) != 1 || fact.MatchedSRIDs[0] != 4326 {
		t.Fatalf("org conflict = %+v, want self-SRID witness", fact)
	}
	calls := t26Calls(t, testSRSOrgFragment)
	if len(calls) != 1 || !strings.Contains(calls[0].query, "organization = ? and organization_coordsys_id = ?") {
		t.Fatalf("org predicate = %d calls %+v, want the combined pair", len(calls), calls)
	}
	if len(calls[0].args) != 2 || calls[0].args[0].Value != "EPSG" || calls[0].args[1].Value != int64(4326) {
		t.Fatalf("org args = %+v, want (EPSG, 4326) as parameters", calls[0].args)
	}
}

// A partially-known org pair — name without a provable id, or an
// unparseable id — issues no query and leaves the fact unbound.
func TestT26SRSOrgConflictPartialNeverQueries(t *testing.T) {
	for _, qualifiers := range []map[string]string{
		{spec.ObjectLookupQualifierSRSOrg: "EPSG"},
		{spec.ObjectLookupQualifierSRSOrgID: "4326"},
		{spec.ObjectLookupQualifierSRSOrg: "EPSG", spec.ObjectLookupQualifierSRSOrgID: "abc"},
	} {
		snapshot, err := t26Resolve(t, map[string]testQueryResult{
			testSRSVersionFragment:  t26VersionRow("8.0.46"),
			testSRSIdentityFragment: {columns: []string{"SRS_ID", "SRS_NAME"}},
			testSRSOrgFragment:      {queryErr: errors.New("org query must not be issued")},
			testSRSDepFragment:      {columns: []string{"TABLE_SCHEMA", "TABLE_NAME", "COLUMN_NAME"}},
		}, spec.DialectMySQL, spec.ObjectLookupRequest{
			Type: "mysql_spatial_reference_system", Name: "4326", Qualifiers: qualifiers,
			RequestVersion: spec.ParseObservedVersion("8.0.46"),
		})
		if err != nil {
			t.Fatalf("qualifiers %+v: %v, want silent unissued", qualifiers, err)
		}
		if snapshot.SpatialReference.OrgConflict != nil {
			t.Fatalf("qualifiers %+v: org conflict = %+v, want nil", qualifiers, snapshot.SpatialReference.OrgConflict)
		}
	}
	if calls := t26Calls(t, testSRSOrgFragment); len(calls) != 0 {
		t.Fatalf("partial org pairs issued %d queries", len(calls))
	}
}

// Identity-query, dependency-read, and conflict-read failures all
// propagate — never degraded into empty sets or absence.
func TestT26SRSErrorPropagation(t *testing.T) {
	qualifiers := map[string]string{
		spec.ObjectLookupQualifierSRSName:  "N",
		spec.ObjectLookupQualifierSRSOrg:   "O",
		spec.ObjectLookupQualifierSRSOrgID: "1",
	}
	want := errors.New("srs identity denied")
	if _, err := t26Resolve(t, map[string]testQueryResult{
		testSRSVersionFragment:  t26VersionRow("8.0.46"),
		testSRSIdentityFragment: {queryErr: want},
	}, spec.DialectMySQL, spec.ObjectLookupRequest{Type: "mysql_spatial_reference_system", Name: "1", Qualifiers: qualifiers, RequestVersion: spec.ParseObservedVersion("8.0.46")}); !errors.Is(err, want) {
		t.Fatalf("identity error: %v, want errors.Is", err)
	}
	depErr := errors.New("geometry columns denied")
	if _, err := t26Resolve(t, map[string]testQueryResult{
		testSRSVersionFragment:  t26VersionRow("8.0.46"),
		testSRSIdentityFragment: {columns: []string{"SRS_ID", "SRS_NAME"}},
		testSRSNameFragment:     {columns: []string{"SRS_ID"}},
		testSRSOrgFragment:      {columns: []string{"SRS_ID"}},
		testSRSDepFragment:      {queryErr: depErr},
	}, spec.DialectMySQL, spec.ObjectLookupRequest{Type: "mysql_spatial_reference_system", Name: "1", Qualifiers: qualifiers, RequestVersion: spec.ParseObservedVersion("8.0.46")}); !errors.Is(err, depErr) {
		t.Fatalf("dep error: %v, want errors.Is", err)
	}
	nameErr := errors.New("name conflict denied")
	if _, err := t26Resolve(t, map[string]testQueryResult{
		testSRSVersionFragment:  t26VersionRow("8.0.46"),
		testSRSIdentityFragment: {columns: []string{"SRS_ID", "SRS_NAME"}},
		testSRSNameFragment:     {queryErr: nameErr},
		testSRSOrgFragment:      {columns: []string{"SRS_ID"}},
		testSRSDepFragment:      {columns: []string{"TABLE_SCHEMA", "TABLE_NAME", "COLUMN_NAME"}},
	}, spec.DialectMySQL, spec.ObjectLookupRequest{Type: "mysql_spatial_reference_system", Name: "1", Qualifiers: qualifiers, RequestVersion: spec.ParseObservedVersion("8.0.46")}); !errors.Is(err, nameErr) {
		t.Fatalf("name conflict error: %v, want errors.Is", err)
	}
	orgErr := errors.New("org conflict denied")
	if _, err := t26Resolve(t, map[string]testQueryResult{
		testSRSVersionFragment:  t26VersionRow("8.0.46"),
		testSRSIdentityFragment: {columns: []string{"SRS_ID", "SRS_NAME"}},
		testSRSNameFragment:     {columns: []string{"SRS_ID"}},
		testSRSOrgFragment:      {queryErr: orgErr},
		testSRSDepFragment:      {columns: []string{"TABLE_SCHEMA", "TABLE_NAME", "COLUMN_NAME"}},
	}, spec.DialectMySQL, spec.ObjectLookupRequest{Type: "mysql_spatial_reference_system", Name: "1", Qualifiers: qualifiers, RequestVersion: spec.ParseObservedVersion("8.0.46")}); !errors.Is(err, orgErr) {
		t.Fatalf("org conflict error: %v, want errors.Is", err)
	}
	verErr := errors.New("version read denied")
	if _, err := t26Resolve(t, map[string]testQueryResult{
		testSRSVersionFragment: {queryErr: verErr},
	}, spec.DialectMySQL, spec.ObjectLookupRequest{Type: "mysql_spatial_reference_system", Name: "1", RequestVersion: spec.ParseObservedVersion("8.0.46")}); !errors.Is(err, verErr) {
		t.Fatalf("version error: %v, want errors.Is", err)
	}
}

// A mid-iteration rows.Err() propagates — a partial witness set is
// never published as the answer.
func TestT26SRSDependencyIterationErrorPropagates(t *testing.T) {
	want := errors.New("iteration died mid-rows")
	_, err := t26Resolve(t, map[string]testQueryResult{
		testSRSVersionFragment:  t26VersionRow("8.0.46"),
		testSRSIdentityFragment: {columns: []string{"SRS_ID", "SRS_NAME"}},
		testSRSDepFragment: {
			columns:  []string{"TABLE_SCHEMA", "TABLE_NAME", "COLUMN_NAME"},
			rows:     [][]driver.Value{{"geo", "t", "c"}},
			rowErrAt: 1, rowErr: want,
		},
	}, spec.DialectMySQL, spec.ObjectLookupRequest{Type: "mysql_spatial_reference_system", Name: "1", RequestVersion: spec.ParseObservedVersion("8.0.46")})
	if !errors.Is(err, want) {
		t.Fatalf("err = %v, want errors.Is(%v)", err, want)
	}
}

// A scan-shape mismatch propagates — column-count errors never degrade.
func TestT26SRSScanErrorPropagates(t *testing.T) {
	_, err := t26Resolve(t, map[string]testQueryResult{
		testSRSVersionFragment:  t26VersionRow("8.0.46"),
		testSRSIdentityFragment: {columns: []string{"SRS_ID"}, rows: [][]driver.Value{{int64(1)}}},
	}, spec.DialectMySQL, spec.ObjectLookupRequest{Type: "mysql_spatial_reference_system", Name: "1", RequestVersion: spec.ParseObservedVersion("8.0.46")})
	if err == nil {
		t.Fatalf("want scan error on one-column row into a two-column scan")
	}
}

// The SRS catalog pair is an 8.0 surface — a proven 5.7 server never
// issues the dedicated queries; an unproven version still issues them
// and propagates every issued-query failure, including missing-object
// codes; a proven 8.0+ propagates the same way.
func TestT26SRSVersionGate(t *testing.T) {
	mustNotQuery := errors.New("srs catalog query must not be issued")
	snapshot, err := t26Resolve(t, map[string]testQueryResult{
		testSRSVersionFragment:  t26VersionRow("5.7.44"),
		testSRSIdentityFragment: {queryErr: mustNotQuery},
		testSRSDepFragment:      {queryErr: mustNotQuery},
	}, spec.DialectMySQL, spec.ObjectLookupRequest{Type: "mysql_spatial_reference_system", Name: "4326", RequestVersion: spec.ParseObservedVersion("5.7.44")})
	if err != nil {
		t.Fatalf("5.7: ResolveObject: %v, want catalog queries gated out", err)
	}
	state := snapshot.SpatialReference
	if state == nil || state.Read || state.UnreadReason != spec.MetadataUnreadVersionInapplicable {
		t.Fatalf("5.7: srs state = %+v, want version_inapplicable", state)
	}
	if calls := t26Calls(t, "st_spatial_reference_systems"); len(calls) != 0 {
		t.Fatalf("5.7 issued %d catalog queries", len(calls))
	}
	if calls := t26Calls(t, "st_geometry_columns"); len(calls) != 0 {
		t.Fatalf("5.7 issued %d dep queries", len(calls))
	}

	// Missing O1 (R3): no authoritative version exists — the call closes
	// version_unproven with zero queries of any kind; even a scripted
	// catalog error is never reached.
	snapshot, err = t26Resolve(t, map[string]testQueryResult{
		testSRSVersionFragment:  {queryErr: mustNotQuery},
		testSRSIdentityFragment: {queryErr: mustNotQuery},
		testSRSDepFragment:      {queryErr: mustNotQuery},
	}, spec.DialectMySQL, spec.ObjectLookupRequest{Type: "mysql_spatial_reference_system", Name: "4326"})
	if err != nil {
		t.Fatalf("missing-O1: ResolveObject: %v, want silent unproven bind", err)
	}
	state = snapshot.SpatialReference
	if state == nil || state.Read || state.UnreadReason != spec.MetadataUnreadVersionUnproven {
		t.Fatalf("missing-O1: srs state = %+v, want version_unproven", state)
	}
	if total := len(testQueryCallsByName(testStubName(t))); total != 0 {
		t.Fatalf("missing O1 issued %d queries — zero reads is the contract", total)
	}

	// In-domain O1 + missing-object code → propagates like every error.
	missing := &gomysql.MySQLError{Number: 1146, Message: "Table 'x' doesn't exist"}
	if _, err = t26Resolve(t, map[string]testQueryResult{
		testSRSVersionFragment:  t26VersionRow("8.0.46"),
		testSRSIdentityFragment: {queryErr: missing},
	}, spec.DialectMySQL, spec.ObjectLookupRequest{Type: "mysql_spatial_reference_system", Name: "4326", RequestVersion: spec.ParseObservedVersion("8.0.46")}); !errors.Is(err, missing) {
		t.Fatalf("proven 8.0 missing-object: err = %v, want errors.Is(1146)", err)
	}

	// In-domain O1 + existence answered + dep error → propagates.
	// A successful existence read never launders a following dependency
	// failure into a partial normal return.
	depMissing := &gomysql.MySQLError{Number: 1146, Message: "Table 'st_geometry_columns' doesn't exist"}
	if _, err = t26Resolve(t, map[string]testQueryResult{
		testSRSVersionFragment: t26VersionRow("8.0.46"),
		testSRSIdentityFragment: {columns: []string{"SRS_ID", "SRS_NAME"},
			rows: [][]driver.Value{{int64(4326), "WGS 84"}}},
		testSRSDepFragment: {queryErr: depMissing},
	}, spec.DialectMySQL, spec.ObjectLookupRequest{Type: "mysql_spatial_reference_system", Name: "4326", RequestVersion: spec.ParseObservedVersion("8.0.46")}); !errors.Is(err, depMissing) {
		t.Fatalf("dep error: err = %v, want errors.Is propagation", err)
	}
}

// The SRS catalog pair is a MySQL surface — under TiDB no query is ever
// issued, not even the version probe, and no fact member is bound.
func TestT26SRSDialectGated(t *testing.T) {
	snapshot, err := t26Resolve(t, map[string]testQueryResult{
		testSRSVersionFragment:  {queryErr: errors.New("must not be issued under tidb")},
		testSRSIdentityFragment: {queryErr: errors.New("must not be issued under tidb")},
	}, spec.DialectTiDB, spec.ObjectLookupRequest{Type: "mysql_spatial_reference_system", Name: "4326"})
	if err != nil {
		t.Fatalf("tidb: ResolveObject: %v, want gated out", err)
	}
	if snapshot.SpatialReference != nil {
		t.Fatalf("tidb: srs state = %+v, want unbound", snapshot.SpatialReference)
	}
	if total := len(testQueryCallsByName(testStubName(t))); total != 0 {
		t.Fatalf("tidb issued %d queries", total)
	}
}

// Object families stay isolated at the provider level: a mysql_server
// request emits only the server read — never an SRS existence,
// dependency, or version query. Laziness for statements that name no
// T26 family is an application-layer contract, exercised by the audit
// package's T26 tests rather than here.
func TestT26FamilyIsolation(t *testing.T) {
	if _, err := t26Resolve(t, map[string]testQueryResult{
		testServerIdentityFragment: {columns: []string{"SERVER_NAME"}, rows: [][]driver.Value{{"s"}}},
	}, spec.DialectMySQL, t26IdentLookup("s")); err != nil {
		t.Fatalf("ResolveObject: %v", err)
	}
	for _, fragment := range []string{"st_spatial_reference_systems", "st_geometry_columns", "select version()"} {
		if calls := t26Calls(t, fragment); len(calls) != 0 {
			t.Fatalf("server lookup issued %d %q queries — families stay isolated", len(calls), fragment)
		}
	}
}

// R3-E witness matrix: O1=8.0.46 fixed — only an identical O2 banner
// may open the catalog channels. Every disagreeing or unreadable
// channel answer is the bounded consistency error (a raw query error
// propagates on the bounded path), the witness fires exactly once,
// and NO existence or dependency query is ever reached. O2 can never
// change the version verdict nor mint a fact.
func TestT26SRSR3ChannelWitnessMatrix(t *testing.T) {
	noCatalog := errors.New("catalog read must never issue past a failed witness")
	mustNotExist := testQueryResult{queryErr: noCatalog}
	mustNotDeps := testQueryResult{queryErr: noCatalog}
	sentinel := errors.New("channel observation query failed")

	cells := []struct {
		name        string
		version     testQueryResult
		wantErr     error
		wantIsIsErr bool // errors.Is against ErrSRSChannelVersionInconsistent
	}{
		{"o2-mismatch-8.4.10", t26VersionRow("8.4.10"), ErrSRSChannelVersionInconsistent, true},
		{"o2-mismatch-5.7.44", t26VersionRow("5.7.44"), ErrSRSChannelVersionInconsistent, true},
		{"o2-unparseable", t26VersionRow("bogus-banner"), ErrSRSChannelVersionInconsistent, true},
		{"o2-empty-answer", testQueryResult{columns: []string{"version()"}}, ErrSRSChannelVersionInconsistent, true},
		{"o2-query-error", testQueryResult{queryErr: sentinel}, sentinel, false},
	}
	for _, cell := range cells {
		t.Run(cell.name, func(t *testing.T) {
			snapshot, err := t26Resolve(t, map[string]testQueryResult{
				testSRSVersionFragment:  cell.version,
				testSRSIdentityFragment: mustNotExist,
				testSRSNameFragment:     mustNotDeps,
				testSRSOrgFragment:      mustNotDeps,
				testSRSDepFragment:      mustNotDeps,
			}, spec.DialectMySQL, spec.ObjectLookupRequest{
				Type: "mysql_spatial_reference_system", Name: "4326",
				RequestVersion: spec.ParseObservedVersion("8.0.46"),
			})
			if !errors.Is(err, cell.wantErr) {
				t.Fatalf("err=%v want errors.Is(%v)", err, cell.wantErr)
			}
			if isInconsistent := errors.Is(err, ErrSRSChannelVersionInconsistent); isInconsistent != cell.wantIsIsErr {
				t.Fatalf("errors.Is(ErrSRSChannelVersionInconsistent)=%v want %v", isInconsistent, cell.wantIsIsErr)
			}
			if snapshot != nil && snapshot.SpatialReference != nil && snapshot.SpatialReference.Read {
				t.Fatalf("srs state = %+v — a failed witness mints no existence fact", snapshot.SpatialReference)
			}
			if got := t26Attempt(t, testSRSVersionFragment); got != 1 {
				t.Fatalf("Qo attempts=%d want exactly one witness probe", got)
			}
			if got := t26Attempt(t, testSRSIdentityFragment); got != 0 {
				t.Fatalf("Qexist attempts=%d want zero — a failed witness mints nothing", got)
			}
			if got := t26Attempt(t, testSRSDepFragment); got != 0 {
				t.Fatalf("Qdeps attempts=%d want zero — a failed witness mints nothing", got)
			}
		})
	}

	// The identical banner is the only witness that opens the catalog.
	t.Run("o2-match-8.0.46", func(t *testing.T) {
		snapshot, err := t26Resolve(t, map[string]testQueryResult{
			testSRSVersionFragment: t26VersionRow("8.0.46"),
			testSRSIdentityFragment: {columns: []string{"SRS_ID", "SRS_NAME"},
				rows: [][]driver.Value{{int64(4326), "WGS 84"}}},
			testSRSDepFragment: {columns: []string{"TABLE_SCHEMA", "TABLE_NAME", "COLUMN_NAME"}},
		}, spec.DialectMySQL, spec.ObjectLookupRequest{
			Type: "mysql_spatial_reference_system", Name: "4326",
			RequestVersion: spec.ParseObservedVersion("8.0.46"),
		})
		if err != nil {
			t.Fatalf("ResolveObject: %v, want matching witness to admit", err)
		}
		state := snapshot.SpatialReference
		if state == nil || !state.Read || !state.Exists {
			t.Fatalf("srs state = %+v, want read+present under a matching witness", state)
		}
	})
}
