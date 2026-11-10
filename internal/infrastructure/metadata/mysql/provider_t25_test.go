package mysqlmeta

import (
	"context"
	"database/sql/driver"
	"errors"
	"testing"

	gomysql "github.com/go-sql-driver/mysql"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// T25/#104: the official ResolveObject mysql_tablespace path reads the
// InnoDB catalog — the bounded INNODB_TABLESPACES identity row plus the
// INNODB_TABLES bound-table count, behind the same shared version gate
// contract as the resource-group channels. No fake resolver is
// substituted.
var (
	testTablespaceIdentityFragment = "information_schema.innodb_tablespaces"
	testTablespaceBoundFragment    = "information_schema.innodb_tables"
	testTablespaceVersionFragment  = "select version()"
)

var testTablespaceIdentityColumns = []string{"SPACE", "SPACE_TYPE"}

func t25Resolve(t *testing.T, results map[string]testQueryResult, dialect spec.Dialect, req spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
	t.Helper()
	db := openTestDB(t, results)
	provider := NewProvider(db)
	return provider.ResolveObject(context.Background(), dialect, req)
}

func t25VersionRow(banner string) testQueryResult {
	return testQueryResult{columns: []string{"version()"}, rows: [][]driver.Value{{banner}}}
}

// A well-formed catalog row proves existence, maps the SPACE_TYPE kind,
// and a positive bound-table count proves occupancy.
func TestT25TablespaceLoaderOccupiedRow(t *testing.T) {
	snapshot, err := t25Resolve(t, map[string]testQueryResult{
		testTablespaceVersionFragment:  t25VersionRow("8.0.46"),
		testTablespaceIdentityFragment: {columns: testTablespaceIdentityColumns, rows: [][]driver.Value{{int64(42), "General"}}},
		testTablespaceBoundFragment:    {columns: []string{"count(*)"}, rows: [][]driver.Value{{int64(3)}}},
	}, spec.DialectMySQL, spec.ObjectLookupRequest{Type: "mysql_tablespace", Name: "ts1"})
	if err != nil {
		t.Fatalf("ResolveObject: %v", err)
	}
	if !snapshot.IsConfirmed() || !snapshot.Exists {
		t.Fatalf("snapshot = %#v, want confirmed present", snapshot)
	}
	state := snapshot.Tablespace
	if state == nil || state.Kind != spec.TablespaceKindGeneral ||
		state.Occupied == nil || !*state.Occupied {
		t.Fatalf("tablespace state = %+v, want general + proven occupied", state)
	}
}

// The undo kind maps; a zero bound-table count proves unoccupied —
// both directions of the occupancy fact are honest.
func TestT25TablespaceLoaderUnoccupiedUndo(t *testing.T) {
	snapshot, err := t25Resolve(t, map[string]testQueryResult{
		testTablespaceVersionFragment:  t25VersionRow("8.4.10"),
		testTablespaceIdentityFragment: {columns: testTablespaceIdentityColumns, rows: [][]driver.Value{{int64(7), "Undo"}}},
		testTablespaceBoundFragment:    {columns: []string{"count(*)"}, rows: [][]driver.Value{{int64(0)}}},
	}, spec.DialectMySQL, spec.ObjectLookupRequest{Type: "mysql_tablespace", Name: "undo_001"})
	if err != nil {
		t.Fatalf("ResolveObject: %v", err)
	}
	state := snapshot.Tablespace
	if state == nil || state.Kind != spec.TablespaceKindUndo ||
		state.Occupied == nil || *state.Occupied {
		t.Fatalf("tablespace state = %+v, want undo + proven unoccupied", state)
	}
}

// An unrecognized SPACE_TYPE stays an unproven kind — the row still
// proves existence, never a fabricated family.
func TestT25TablespaceLoaderUnrecognizedKind(t *testing.T) {
	snapshot, err := t25Resolve(t, map[string]testQueryResult{
		testTablespaceVersionFragment:  t25VersionRow("8.0.46"),
		testTablespaceIdentityFragment: {columns: testTablespaceIdentityColumns, rows: [][]driver.Value{{int64(5), "Single"}}},
		testTablespaceBoundFragment:    {columns: []string{"count(*)"}, rows: [][]driver.Value{{int64(1)}}},
	}, spec.DialectMySQL, spec.ObjectLookupRequest{Type: "mysql_tablespace", Name: "ts1"})
	if err != nil {
		t.Fatalf("ResolveObject: %v", err)
	}
	if !snapshot.IsConfirmed() || !snapshot.Exists {
		t.Fatalf("snapshot = %#v, want confirmed present", snapshot)
	}
	if snapshot.Tablespace == nil || snapshot.Tablespace.Kind != "" {
		t.Fatalf("kind = %+v, want unproven — never silently mapped", snapshot.Tablespace)
	}
}

// A zero-row answer stays unknown — never a fabricated absence: the
// InnoDB catalog is privilege-gated, so an invisible object is not a
// proven-missing one.
func TestT25TablespaceLoaderZeroRowsStaysUnknown(t *testing.T) {
	snapshot, err := t25Resolve(t, map[string]testQueryResult{
		testTablespaceVersionFragment:  t25VersionRow("8.0.46"),
		testTablespaceIdentityFragment: {columns: testTablespaceIdentityColumns},
	}, spec.DialectMySQL, spec.ObjectLookupRequest{Type: "mysql_tablespace", Name: "ghost"})
	if err != nil {
		t.Fatalf("ResolveObject: %v", err)
	}
	if snapshot.IsNotFound() || !snapshot.IsUnavailable() || snapshot.Exists {
		t.Fatalf("snapshot = %#v, want unavailable — zero rows never fabricate absence", snapshot)
	}
}

// A failed bound-table read propagates — it is never laundered into an
// unoccupied answer.
func TestT25TablespaceLoaderOccupancyErrorPropagates(t *testing.T) {
	want := errors.New("innodb_tables select denied")
	_, err := t25Resolve(t, map[string]testQueryResult{
		testTablespaceVersionFragment:  t25VersionRow("8.0.46"),
		testTablespaceIdentityFragment: {columns: testTablespaceIdentityColumns, rows: [][]driver.Value{{int64(42), "General"}}},
		testTablespaceBoundFragment:    {queryErr: want},
	}, spec.DialectMySQL, spec.ObjectLookupRequest{Type: "mysql_tablespace", Name: "ts1"})
	if !errors.Is(err, want) {
		t.Fatalf("err = %v, want errors.Is(%v)", err, want)
	}
}

// Identity query and version-read failures propagate wrapped — never
// degraded to unavailable and never cached as absence.
func TestT25TablespaceLoaderErrorPropagates(t *testing.T) {
	want := errors.New("innodb_tablespaces select denied")
	_, err := t25Resolve(t, map[string]testQueryResult{
		testTablespaceVersionFragment:  t25VersionRow("8.0.46"),
		testTablespaceIdentityFragment: {columns: testTablespaceIdentityColumns, queryErr: want},
	}, spec.DialectMySQL, spec.ObjectLookupRequest{Type: "mysql_tablespace", Name: "ts1"})
	if !errors.Is(err, want) {
		t.Fatalf("err = %v, want errors.Is(%v)", err, want)
	}
	denied := errors.New("version read denied")
	_, err = t25Resolve(t, map[string]testQueryResult{
		testTablespaceVersionFragment: {queryErr: denied},
	}, spec.DialectMySQL, spec.ObjectLookupRequest{Type: "mysql_tablespace", Name: "ts1"})
	if !errors.Is(err, denied) {
		t.Fatalf("version failure: err = %v, want errors.Is(denied)", err)
	}
}

// The InnoDB catalog is an 8.0 surface — a proven 5.7 server never
// issues the dedicated queries; an unproven version keeps the narrow
// missing-object degrade; on a proven 8.0+ server every catalog error
// propagates, missing-object codes included.
func TestT25TablespaceLoaderVersionGate(t *testing.T) {
	mustNotQuery := errors.New("dedicated tablespace query must not be issued")
	snapshot, err := t25Resolve(t, map[string]testQueryResult{
		testTablespaceVersionFragment:  t25VersionRow("5.7.44"),
		testTablespaceIdentityFragment: {queryErr: mustNotQuery},
	}, spec.DialectMySQL, spec.ObjectLookupRequest{Type: "mysql_tablespace", Name: "ts1"})
	if err != nil {
		t.Fatalf("5.7: ResolveObject: %v, want the catalog query gated out", err)
	}
	if snapshot.IsConfirmed() {
		t.Fatalf("5.7: snapshot = %#v, want unproven", snapshot)
	}

	snapshot, err = t25Resolve(t, map[string]testQueryResult{
		testTablespaceIdentityFragment: {columns: testTablespaceIdentityColumns,
			queryErr: &gomysql.MySQLError{Number: 1146, Message: "Table 'innodb_tablespaces' doesn't exist"}},
	}, spec.DialectMySQL, spec.ObjectLookupRequest{Type: "mysql_tablespace", Name: "ts1"})
	if err != nil || snapshot == nil || snapshot.IsConfirmed() {
		t.Fatalf("unknown-version degrade: snapshot=%#v err=%v, want unproven nil error", snapshot, err)
	}

	missing := &gomysql.MySQLError{Number: 1109, Message: "Unknown table 'innodb_tablespaces'"}
	if _, err = t25Resolve(t, map[string]testQueryResult{
		testTablespaceVersionFragment:  t25VersionRow("8.0.46"),
		testTablespaceIdentityFragment: {queryErr: missing},
	}, spec.DialectMySQL, spec.ObjectLookupRequest{Type: "mysql_tablespace", Name: "ts1"}); !errors.Is(err, missing) {
		t.Fatalf("proven 8.0 missing-object: err = %v, want errors.Is(1109)", err)
	}
}

// The InnoDB tablespace catalog is a MySQL surface — under TiDB the
// resolver never emits the MySQL catalog queries.
func TestT25TablespaceLoaderDialectGated(t *testing.T) {
	mustNotQuery := errors.New("mysql tablespace query must not be issued under tidb")
	snapshot, err := t25Resolve(t, map[string]testQueryResult{
		testTablespaceVersionFragment:  {queryErr: mustNotQuery},
		testTablespaceIdentityFragment: {queryErr: mustNotQuery},
	}, spec.DialectTiDB, spec.ObjectLookupRequest{Type: "mysql_tablespace", Name: "ts1"})
	if err != nil {
		t.Fatalf("tidb: ResolveObject: %v, want gated out before any query", err)
	}
	if snapshot.IsConfirmed() {
		t.Fatalf("tidb: snapshot = %#v, want unproven", snapshot)
	}
}
