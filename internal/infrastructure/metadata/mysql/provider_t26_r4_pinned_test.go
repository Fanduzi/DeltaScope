package mysqlmeta

// T26-SRS-VERSION-APPLICABILITY-01 EXECUTION-01 R4 — pinned-channel
// connection identity proofs. *sql.DB is a pool: these rows assert the
// O2 witness and every catalog read inside one SRS read unit ride the
// SAME *sql.Conn (same recorded connection ID), that a gated O1 never
// even acquires a connection, and that the pinned connection is
// released on success and failure paths alike.

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// r4Connector hands out driver conns with monotonically increasing
// connection IDs — each conn is an independent "backend" for the
// identity proofs. The per-conn banner map lets a test simulate a
// topology split where different connections report different
// versions.
type r4Connector struct {
	mu      *sync.Mutex
	nextID  int
	conns   int
	closed  int
	banners map[int]string // connID → version() answer for that backend
	results map[string]testQueryResult
	calls   *[]r4QueryCall
}

type r4QueryCall struct {
	connID int
	query  string
}

func (c *r4Connector) Connect(context.Context) (driver.Conn, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.nextID++
	c.conns++
	return &r4Conn{id: c.nextID, owner: c}, nil
}

func (c *r4Connector) Driver() driver.Driver { return r4Driver{} }

type r4Driver struct{}

func (r4Driver) Open(name string) (driver.Conn, error) {
	return nil, errors.New("r4Driver.Open is unused — connector only")
}

type r4Conn struct {
	id     int
	owner  *r4Connector
	closed bool
}

func (c *r4Conn) Prepare(string) (driver.Stmt, error) { return nil, driver.ErrSkip }
func (c *r4Conn) Begin() (driver.Tx, error)           { return nil, driver.ErrSkip }

func (c *r4Conn) Close() error {
	c.owner.mu.Lock()
	defer c.owner.mu.Unlock()
	if !c.closed {
		c.closed = true
		c.owner.closed++
	}
	return nil
}

func (c *r4Conn) QueryContext(_ context.Context, query string, _ []driver.NamedValue) (driver.Rows, error) {
	c.owner.mu.Lock()
	*c.owner.calls = append(*c.owner.calls, r4QueryCall{connID: c.id, query: query})
	banner := c.owner.banners[c.id]
	c.owner.mu.Unlock()
	if strings.Contains(query, "select version()") {
		if banner == "" {
			return nil, errors.New("version query must not issue on this connection")
		}
		return &r4Rows{cols: []string{"version()"}, rows: [][]driver.Value{{banner}}}, nil
	}
	// longest-fragment match against the scripted catalog results
	var key string
	for fragment := range c.owner.results {
		if strings.Contains(query, fragment) && len(fragment) > len(key) {
			key = fragment
		}
	}
	result, ok := c.owner.results[key]
	if !ok || key == "" {
		return nil, errors.New("unscripted catalog query: " + query)
	}
	if result.queryErr != nil {
		return nil, result.queryErr
	}
	return &r4Rows{cols: result.columns, rows: result.rows}, nil
}

type r4Rows struct {
	cols  []string
	rows  [][]driver.Value
	index int
}

func (r *r4Rows) Columns() []string { return r.cols }
func (r *r4Rows) Close() error      { return nil }
func (r *r4Rows) Next(dest []driver.Value) error {
	if r.index >= len(r.rows) {
		return io.EOF
	}
	copy(dest, r.rows[r.index])
	r.index++
	return nil
}

func newR4Fixture(banners map[int]string, results map[string]testQueryResult) (*r4Connector, *[]r4QueryCall, *sql.DB) {
	calls := &[]r4QueryCall{}
	conn := &r4Connector{mu: &sync.Mutex{}, banners: banners, results: results, calls: calls}
	db := sql.OpenDB(conn)
	// Idle-free pooling: every released pinned conn is closed and the
	// next read unit acquires a fresh driver conn, so connection IDs
	// are stable identities per unit and the banner map can model a
	// different backend behind every connection.
	db.SetMaxIdleConns(0)
	return conn, calls, db
}

// r4ConnIDs folds the recorded calls into the ordered distinct
// connection IDs that served them.
func r4ConnIDs(calls []r4QueryCall) []int {
	var out []int
	seen := map[int]bool{}
	for _, c := range calls {
		if !seen[c.connID] {
			seen[c.connID] = true
			out = append(out, c.connID)
		}
	}
	return out
}

// R4-C1 (O2 match): witness + existence + conflict + dependency reads
// all record the SAME connection ID — the pinned conn is the proof
// that O2's backend IS the backend the catalog answers came from.
func TestT26SRSR4PinnedConnMatch(t *testing.T) {
	noDeps := testQueryResult{columns: []string{"TABLE_SCHEMA", "TABLE_NAME", "COLUMN_NAME"}}
	connector, calls, db := newR4Fixture(map[int]string{1: "8.0.46"}, map[string]testQueryResult{
		testSRSIdentityFragment: {columns: []string{"SRS_ID", "SRS_NAME"},
			rows: [][]driver.Value{{int64(4326), "WGS 84"}}},
		testSRSNameFragment: {columns: []string{"SRS_ID"}, rows: [][]driver.Value{{int64(4326)}}},
		testSRSDepFragment:  noDeps,
	})
	defer db.Close()
	snapshot, err := NewProvider(db).ResolveObject(context.Background(),
		spec.DialectMySQL, spec.ObjectLookupRequest{
			Type: "mysql_spatial_reference_system", Name: "4326",
			Qualifiers:     map[string]string{spec.ObjectLookupQualifierSRSName: "WGS 84"},
			RequestVersion: spec.ParseObservedVersion("8.0.46"),
		})
	if err != nil {
		t.Fatalf("ResolveObject: %v", err)
	}
	if state := snapshot.SpatialReference; state == nil || !state.Read || !state.Exists {
		t.Fatalf("srs state = %+v, want read+present", state)
	}
	ids := r4ConnIDs(*calls)
	if len(ids) != 1 {
		t.Fatalf("connection IDs = %v — witness and catalog reads MUST share one pinned connection", ids)
	}
	if len(*calls) != 4 {
		t.Fatalf("calls = %d want 4 (witness + existence + name conflict + deps)", len(*calls))
	}
	if connector.closed != 1 {
		t.Fatalf("closed conns = %d want 1 — the pinned connection is released on success", connector.closed)
	}
}

// R4-C2 (O2 mismatch on the pinned backend): conn 1 reports 5.7.44 —
// the witness fails closed on THAT connection and zero catalog queries
// are issued; the connection is still released.
func TestT26SRSR4PinnedConnMismatch(t *testing.T) {
	connector, calls, db := newR4Fixture(map[int]string{1: "5.7.44"}, map[string]testQueryResult{
		testSRSIdentityFragment: {queryErr: errors.New("must not issue")},
		testSRSDepFragment:      {queryErr: errors.New("must not issue")},
	})
	defer db.Close()
	_, err := NewProvider(db).ResolveObject(context.Background(),
		spec.DialectMySQL, spec.ObjectLookupRequest{
			Type: "mysql_spatial_reference_system", Name: "4326",
			RequestVersion: spec.ParseObservedVersion("8.0.46"),
		})
	if !errors.Is(err, ErrSRSChannelVersionInconsistent) {
		t.Fatalf("err=%v want ErrSRSChannelVersionInconsistent", err)
	}
	if len(*calls) != 1 || (*calls)[0].connID != 1 {
		t.Fatalf("calls = %+v want exactly the witness probe on conn 1", *calls)
	}
	if connector.closed != 1 {
		t.Fatalf("closed conns = %d want 1 — the pinned connection is released on witness failure", connector.closed)
	}
}

// R4-C3 (O1 gated): nil and off-domain O1 never acquire a connection —
// zero Connect(), zero SQL.
func TestT26SRSR4PinnedConnGated(t *testing.T) {
	for _, tc := range []struct {
		name    string
		version *spec.VersionIdentity
	}{
		{"o1-missing", nil},
		{"o1-off-domain", spec.ParseObservedVersion("5.7.44")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			connector, calls, db := newR4Fixture(map[int]string{1: "8.0.46"}, map[string]testQueryResult{
				testSRSIdentityFragment: {queryErr: errors.New("must not issue")},
			})
			defer db.Close()
			snapshot, err := NewProvider(db).ResolveObject(context.Background(),
				spec.DialectMySQL, spec.ObjectLookupRequest{
					Type: "mysql_spatial_reference_system", Name: "4326",
					RequestVersion: tc.version,
				})
			if err != nil {
				t.Fatalf("ResolveObject: %v, want silent gated bind", err)
			}
			if connector.conns != 0 {
				t.Fatalf("connections acquired = %d want 0 — a gated O1 never pins a connection", connector.conns)
			}
			if len(*calls) != 0 {
				t.Fatalf("calls = %+v want zero SQL", *calls)
			}
			state := snapshot.SpatialReference
			if state == nil || state.Read {
				t.Fatalf("srs state = %+v want gated unread", state)
			}
		})
	}
}

// R4-C4 (dependency reader): the witness and the deps query share one
// pinned conn; a second call gets a fresh conn with its own witness —
// never a cached result riding a previous backend.
func TestT26SRSR4PinnedConnDepsReader(t *testing.T) {
	connector, calls, db := newR4Fixture(map[int]string{1: "8.0.46", 2: "8.0.46"}, map[string]testQueryResult{
		testSRSDepFragment: {columns: []string{"TABLE_SCHEMA", "TABLE_NAME", "COLUMN_NAME"}},
	})
	defer db.Close()
	p := NewProvider(db)
	for i := 0; i < 2; i++ {
		fact, err := p.ResolveMySQLSRSDependencies(context.Background(), 100001, spec.ParseObservedVersion("8.0.46"))
		if err != nil {
			t.Fatalf("call %d ResolveMySQLSRSDependencies: %v", i, err)
		}
		if fact == nil {
			t.Fatalf("call %d: nil fact under a matching witness", i)
		}
	}
	// Each call = one pinned conn: witness + deps query share its ID.
	ids := r4ConnIDs(*calls)
	if len(ids) != 2 {
		t.Fatalf("connection IDs = %v want exactly 2 — each read unit pins its own conn", ids)
	}
	for i, id := range ids {
		unit := (*calls)[i*2 : i*2+2]
		if unit[0].connID != id || unit[1].connID != id {
			t.Fatalf("call %d conn IDs = %+v — witness and deps query must share conn %d", i, unit, id)
		}
		if !strings.Contains(unit[0].query, "select version()") || !strings.Contains(unit[1].query, "st_geometry_columns") {
			t.Fatalf("call %d queries = %+v want witness then deps", i, unit)
		}
	}
	if connector.closed != 2 {
		t.Fatalf("closed conns = %d want 2 — both read units released", connector.closed)
	}
}

// R4-C5 (topology split): conn 1 reports 8.0.46, conn 2 reports
// 5.7.44 — the second read unit's witness observes ITS OWN backend and
// fails closed; nothing from conn 1 is reused or laundered.
func TestT26SRSR4PinnedConnTopologySplit(t *testing.T) {
	connector, calls, db := newR4Fixture(map[int]string{1: "8.0.46", 2: "5.7.44"}, map[string]testQueryResult{
		testSRSDepFragment: {columns: []string{"TABLE_SCHEMA", "TABLE_NAME", "COLUMN_NAME"}},
	})
	defer db.Close()
	p := NewProvider(db)
	if _, err := p.ResolveMySQLSRSDependencies(context.Background(), 100001, spec.ParseObservedVersion("8.0.46")); err != nil {
		t.Fatalf("conn1: %v, want matching witness", err)
	}
	if _, err := p.ResolveMySQLSRSDependencies(context.Background(), 100001, spec.ParseObservedVersion("8.0.46")); !errors.Is(err, ErrSRSChannelVersionInconsistent) {
		t.Fatalf("conn2 err=%v want ErrSRSChannelVersionInconsistent — the witness observes its own backend", err)
	}
	if len(*calls) != 3 {
		t.Fatalf("calls = %d want 3 — conn2 ran the witness only", len(*calls))
	}
	if (*calls)[2].connID != 2 || !strings.Contains((*calls)[2].query, "select version()") {
		t.Fatalf("third call = %+v want conn2's own witness probe", (*calls)[2])
	}
	if connector.closed != 2 {
		t.Fatalf("closed conns = %d want 2", connector.closed)
	}
}
