// Package audit: T26-SRS-DROP-EXECUTION-01 R1 — the audited MySQL DROP
// SPATIAL REFERENCE SYSTEM slice's row-level tests (accepted contract
// docs/verification/t26-srs-drop-contract-01, candidate d68f4ab7).
// Read counters sit at the real seams: R = objectResolverFor forward,
// Qexist/Qdeps/Qo = SQL through database/sql into the stub driver,
// L = the SERVER privilege loader, Vd = instance-facts discovery.
package audit

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"reflect"
	"strings"
	"sync"
	"testing"

	"fmt"
	"strconv"

	"github.com/Fanduzi/DeltaScope/internal/domain/report"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// --- stub driver ------------------------------------------------------------

type srsDepRow struct {
	schema, table, column string
	srid                  uint64
}

type srsSQLFixture struct {
	version   string
	qoVersion string            // select version() answer inside loadSRS — when set, diverges from version (the LoadInstanceFacts/observed banner)
	srs       map[uint32]string // srid -> srs_name (existence rows)
	deps      []srsDepRow
	// nameOwners maps an SRS_NAME spelling to the SRID holding it —
	// the CREATE family's name-conflict read witnesses
	// (T26-SRS-CREATE-PUBLICATION-01).
	nameOwners map[string]uint32
	// orgOwners maps an "ORGANIZATION<ID>" pair to the SRID holding it —
	// the org+id conflict read witnesses.
	orgOwners map[string]uint32
	servers   map[string]bool
	queries   []string
	failOn    map[string]error // query-substring -> sentinel
	closeOn   map[string]error // query-substring -> rows.Close() sentinel
}

var (
	srsSQLDriverOnce sync.Once
	srsSQLDatasets   sync.Map
)

func openSRSSQLDB(t *testing.T, fixture *srsSQLFixture) *sql.DB {
	t.Helper()
	srsSQLDriverOnce.Do(func() { sql.Register("t26srs-sql", srsSQLDriver{}) })
	name := "t26srs-" + strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	srsSQLDatasets.Store(name, fixture)
	db, err := sql.Open("t26srs-sql", name)
	if err != nil {
		t.Fatalf("open sql db: %v", err)
	}
	t.Cleanup(func() {
		db.Close()
		srsSQLDatasets.Delete(name)
	})
	return db
}

type srsSQLDriver struct{}
type srsSQLConn struct{ fixture *srsSQLFixture }
type srsSQLRows struct {
	cols     []string
	rows     [][]driver.Value
	index    int
	closeErr error
}

func (srsSQLDriver) Open(name string) (driver.Conn, error) {
	fixture, _ := srsSQLDatasets.Load(name)
	return &srsSQLConn{fixture: fixture.(*srsSQLFixture)}, nil
}

func (c *srsSQLConn) Prepare(string) (driver.Stmt, error) { return nil, driver.ErrSkip }
func (c *srsSQLConn) Close() error                        { return nil }
func (c *srsSQLConn) Begin() (driver.Tx, error)           { return nil, driver.ErrSkip }

func (c *srsSQLConn) QueryContext(_ context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	c.fixture.queries = append(c.fixture.queries, query)
	for mark, sentinel := range c.fixture.failOn {
		if strings.Contains(query, mark) {
			return nil, sentinel
		}
	}
	closeErr := func() error {
		for mark, sentinel := range c.fixture.closeOn {
			if strings.Contains(query, mark) {
				return sentinel
			}
		}
		return nil
	}()
	switch {
	case strings.Contains(query, "version()"):
		banner := c.fixture.version
		if c.fixture.qoVersion != "" {
			banner = c.fixture.qoVersion
		}
		return &srsSQLRows{cols: []string{"version()"}, rows: [][]driver.Value{{banner}}, closeErr: closeErr}, nil
	case strings.Contains(query, "st_spatial_reference_systems"):
		switch {
		case strings.Contains(query, "srs_name = ?"):
			// NAME conflict read — args[0] is the authored name
			// spelling (T26-SRS-CREATE-PUBLICATION-01).
			name, _ := args[0].Value.(string)
			var rows [][]driver.Value
			if owner, ok := c.fixture.nameOwners[name]; ok {
				rows = append(rows, []driver.Value{int64(owner)})
			}
			return &srsSQLRows{cols: []string{"srs_id"}, rows: rows, closeErr: closeErr}, nil
		case strings.Contains(query, "organization = ?"):
			// ORGANIZATION + ORGANIZATION_COORDSYS_ID conflict read —
			// args[0]/args[1] are the authored pair.
			org, _ := args[0].Value.(string)
			id, _ := args[1].Value.(int64)
			var rows [][]driver.Value
			if owner, ok := c.fixture.orgOwners[fmt.Sprintf("%s%d", org, id)]; ok {
				rows = append(rows, []driver.Value{int64(owner)})
			}
			return &srsSQLRows{cols: []string{"srs_id"}, rows: rows, closeErr: closeErr}, nil
		default:
			srid := args[0].Value.(int64)
			if name, ok := c.fixture.srs[uint32(srid)]; ok {
				return &srsSQLRows{cols: []string{"srs_id", "srs_name"}, rows: [][]driver.Value{{srid, name}}, closeErr: closeErr}, nil
			}
			return &srsSQLRows{cols: []string{"srs_id", "srs_name"}, closeErr: closeErr}, nil
		}
	case strings.Contains(query, "st_geometry_columns"):
		srid := args[0].Value.(int64)
		var rows [][]driver.Value
		for _, dep := range c.fixture.deps {
			if dep.srid == uint64(srid) {
				rows = append(rows, []driver.Value{dep.schema, dep.table, dep.column})
			}
		}
		return &srsSQLRows{cols: []string{"table_schema", "table_name", "column_name"}, rows: rows, closeErr: closeErr}, nil
	case strings.Contains(query, "mysql.servers"):
		name, _ := args[0].Value.(string)
		if c.fixture.servers[name] {
			return &srsSQLRows{cols: []string{"Server_name"}, rows: [][]driver.Value{{name}}, closeErr: closeErr}, nil
		}
		return &srsSQLRows{cols: []string{"Server_name"}, closeErr: closeErr}, nil
	}
	return &srsSQLRows{cols: []string{"x"}, closeErr: closeErr}, nil
}

func (r *srsSQLRows) Columns() []string { return r.cols }
func (r *srsSQLRows) Close() error      { return r.closeErr }
func (r *srsSQLRows) Next(dest []driver.Value) error {
	if r.index >= len(r.rows) {
		return io.EOF
	}
	copy(dest, r.rows[r.index])
	r.index++
	return nil
}

func srsQueryCount(fixture *srsSQLFixture, mark string) int {
	n := 0
	for _, q := range fixture.queries {
		if strings.Contains(q, mark) {
			n++
		}
	}
	return n
}

// srsSQLProvider mirrors the real mysqlmeta loader's SQL sequence over
// the stub driver — mysqlmeta cannot be imported here (it asserts the
// audit.MetadataProvider interface — an import cycle), so the same
// query order and state construction runs in-test: Qo = select
// version(), Qexist = st_spatial_reference_systems, Qdeps =
// st_geometry_columns. Table snapshots come from the fixture map
// (tbl-t style prefixes carry N/A reads by design).
type srsSQLProvider struct {
	db              *sql.DB
	version         string
	tables          map[string]*spec.TableSnapshot
	forwards        []spec.ObjectLookupRequest // R seam
	privCalls       int                        // L seam
	privObservation *spec.MySQLServerDropPrivilegeObservation
	discoverErr     error
	resolveErr      error
}

func (p *srsSQLProvider) LoadInstanceFacts(context.Context, spec.Dialect, string) (*spec.InstanceFacts, error) {
	if p.discoverErr != nil {
		return nil, p.discoverErr
	}
	return &spec.InstanceFacts{Version: p.version}, nil
}

func (p *srsSQLProvider) LoadTableSnapshot(_ context.Context, _ spec.Dialect, schema, table string) (*spec.TableSnapshot, error) {
	key := strings.ToLower(strings.TrimSpace(schema) + "." + strings.TrimSpace(table))
	return p.tables[key], nil
}

func (p *srsSQLProvider) ResolveObject(ctx context.Context, dialect spec.Dialect, req spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
	p.forwards = append(p.forwards, req)
	if p.resolveErr != nil {
		return nil, p.resolveErr
	}
	switch req.Type {
	case "mysql_spatial_reference_system":
		return p.loadSRS(ctx, dialect, req)
	case "mysql_server":
		return p.loadServer(ctx, req)
	}
	return &spec.ObjectSnapshot{Schema: req.Schema, Type: req.Type, Name: req.Name, Status: spec.MetadataStatusUnavailable}, nil
}

// loadSRS mirrors loadMySQLSRSObject: parse the SRID, qualify the
// catalog surface through select version(), then the explicit-Rows
// existence read whose zero-observed-row answer supplies Absence only
// under the same Qo observation's D-proof, and the always-issued
// dependency witness read. Errors propagate wrapped exactly as the
// real loader wraps them.
func (p *srsSQLProvider) loadSRS(ctx context.Context, dialect spec.Dialect, req spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
	snapshot := &spec.ObjectSnapshot{Schema: req.Schema, Type: req.Type, Name: req.Name, Status: spec.MetadataStatusUnavailable}
	if dialect != spec.DialectMySQL {
		return snapshot, nil
	}
	state := &spec.SpatialReferenceSystemState{}
	snapshot.SpatialReference = state
	srid, err := strconv.ParseUint(strings.TrimSpace(req.Name), 10, 32)
	if err != nil {
		state.UnreadReason = spec.MetadataUnreadIdentityUnproven
		return snapshot, nil
	}
	state.SRID = uint32(srid)
	var banner string
	if err := p.db.QueryRowContext(ctx, `select version()`).Scan(&banner); err != nil {
		if err != sql.ErrNoRows {
			return nil, fmt.Errorf("query server version for srs channels: %w", err)
		}
	}
	version := spec.ParseObservedVersion(banner)
	if version != nil && version.ValidatedRange && version.Product == spec.VersionProductMySQL && version.Major < 8 {
		state.UnreadReason = spec.MetadataUnreadVersionInapplicable
		return snapshot, nil
	}
	var matchedID sql.NullInt64
	var matchedName sql.NullString
	existRows, err := p.db.QueryContext(ctx, `
		select srs_id, srs_name
		from information_schema.st_spatial_reference_systems
		where srs_id = ?
	`, int64(srid))
	if err != nil {
		return nil, fmt.Errorf("query mysql srs snapshot: %w", err)
	}
	observed := false
	for existRows.Next() {
		observed = true
		if err := existRows.Scan(&matchedID, &matchedName); err != nil {
			existRows.Close()
			return nil, fmt.Errorf("scan mysql srs snapshot: %w", err)
		}
	}
	if err := existRows.Err(); err != nil {
		existRows.Close()
		return nil, fmt.Errorf("iterate mysql srs snapshot: %w", err)
	}
	if err := existRows.Close(); err != nil {
		return nil, fmt.Errorf("close mysql srs snapshot: %w", err)
	}
	if observed {
		state.Read = true
		state.Exists = true
		state.Status = spec.MetadataStatusConfirmed
		if matchedName.Valid {
			state.CatalogName = matchedName.String
		}
		snapshot.Status = spec.MetadataStatusConfirmed
		snapshot.Exists = true
	} else {
		state.Read = true
		state.Absence = spec.MySQLSRSDropAuditedDomain(version)
	}
	// The conflict reads fire only on provable request identities —
	// mirroring loadMySQLSRSObject: a qualifier-less request (DROP)
	// never issues them. An empty answer is a fact with zero matched
	// SRIDs, never a proven "no conflict".
	if name := req.Qualifiers[spec.ObjectLookupQualifierSRSName]; strings.TrimSpace(name) != "" {
		fact, err := p.srsConflictRead(ctx, `
			select srs_id
			from information_schema.st_spatial_reference_systems
			where srs_name = ?
		`, name)
		if err != nil {
			return nil, err
		}
		state.NameConflict = fact
	}
	if org := req.Qualifiers[spec.ObjectLookupQualifierSRSOrg]; strings.TrimSpace(org) != "" {
		if idText := req.Qualifiers[spec.ObjectLookupQualifierSRSOrgID]; strings.TrimSpace(idText) != "" {
			if orgID, parseErr := strconv.ParseUint(idText, 10, 32); parseErr == nil {
				fact, err := p.srsConflictRead(ctx, `
					select srs_id
					from information_schema.st_spatial_reference_systems
					where organization = ? and organization_coordsys_id = ?
				`, org, int64(orgID))
				if err != nil {
					return nil, err
				}
				state.OrgConflict = fact
			}
		}
	}
	rows, err := p.db.QueryContext(ctx, `
		select table_schema, table_name, column_name
		from information_schema.st_geometry_columns
		where srs_id = ?
	`, int64(srid))
	if err != nil {
		return nil, fmt.Errorf("query mysql srs column dependencies: %w", err)
	}
	defer rows.Close()
	fact := &spec.SRSDependencyFact{}
	for rows.Next() {
		var schemaName, tableName, columnName sql.NullString
		if err := rows.Scan(&schemaName, &tableName, &columnName); err != nil {
			return nil, fmt.Errorf("scan mysql srs column dependency: %w", err)
		}
		fact.Columns = append(fact.Columns, spec.SRSDependentColumn{
			Schema: schemaName.String,
			Table:  tableName.String,
			Column: columnName.String,
			SRID:   uint32(srid),
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate mysql srs column dependencies: %w", err)
	}
	state.Dependencies = fact
	return snapshot, nil
}

// srsConflictRead mirrors loadMySQLSRSConflict — the shared
// scan/iteration/close contract for the NAME and ORGANIZATION witness
// reads. Errors propagate wrapped; a zero-row answer is a fact with
// empty MatchedSRIDs, never a proven "no conflict".
func (p *srsSQLProvider) srsConflictRead(ctx context.Context, query string, args ...any) (*spec.SRSConflictFact, error) {
	rows, err := p.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query mysql srs conflict read: %w", err)
	}
	fact := &spec.SRSConflictFact{}
	for rows.Next() {
		var id sql.NullInt64
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return nil, fmt.Errorf("scan mysql srs conflict witness: %w", err)
		}
		if id.Valid && id.Int64 >= 0 && id.Int64 <= 4294967295 {
			fact.MatchedSRIDs = append(fact.MatchedSRIDs, uint32(id.Int64))
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, fmt.Errorf("iterate mysql srs conflict witnesses: %w", err)
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("close mysql srs conflict read: %w", err)
	}
	return fact, nil
}

// loadServer mirrors loadMySQLServerObject's existence read — enough
// for the sibling-family crosstalk rows.
func (p *srsSQLProvider) loadServer(ctx context.Context, req spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
	snapshot := &spec.ObjectSnapshot{Schema: req.Schema, Type: req.Type, Name: req.Name, Status: spec.MetadataStatusUnavailable}
	state := &spec.ServerState{Name: req.Name}
	snapshot.Server = state
	var name sql.NullString
	row := p.db.QueryRowContext(ctx, `select server_name from mysql.servers where server_name = ?`, req.Name)
	if err := row.Scan(&name); err != nil {
		if err == sql.ErrNoRows {
			state.Read = true
			return snapshot, nil
		}
		return nil, fmt.Errorf("query mysql server snapshot: %w", err)
	}
	state.Read = true
	state.Exists = true
	state.Status = spec.MetadataStatusConfirmed
	snapshot.Status = spec.MetadataStatusConfirmed
	snapshot.Exists = true
	return snapshot, nil
}

// srsBareProvider is the no-resolver shape for E3.3 — a provider exists
// but carries no ObjectResolver, so the SRS bind stays unissued with the
// CURRENT empty UnreadReason.
type srsBareProvider struct{ version string }

func (p *srsBareProvider) LoadInstanceFacts(context.Context, spec.Dialect, string) (*spec.InstanceFacts, error) {
	return &spec.InstanceFacts{Version: p.version}, nil
}

func (p *srsBareProvider) LoadTableSnapshot(context.Context, spec.Dialect, string, string) (*spec.TableSnapshot, error) {
	return nil, nil
}

func (p *srsSQLProvider) LoadMySQLServerDropPrivilege(_ context.Context, dialect spec.Dialect) (*spec.MySQLServerDropPrivilegeObservation, error) {
	p.privCalls++
	if dialect != spec.DialectMySQL {
		return nil, nil
	}
	if p.privObservation != nil {
		return p.privObservation, nil
	}
	return &spec.MySQLServerDropPrivilegeObservation{}, nil
}

func newSRSSQLHarness(t *testing.T, fixture *srsSQLFixture, tables map[string]*spec.TableSnapshot) (*srsSQLFixture, *srsSQLProvider) {
	t.Helper()
	if fixture == nil {
		fixture = &srsSQLFixture{version: "8.0.36"}
	}
	db := openSRSSQLDB(t, fixture)
	provider := &srsSQLProvider{db: db, version: fixture.version, tables: tables}
	return fixture, provider
}

// --- harness -----------------------------------------------------------------

var srsGoldenVersion = &spec.VersionIdentity{Product: spec.VersionProductMySQL, Version: "8.0.36", Major: 8, Minor: 0, Patch: 36, ValidatedRange: true, Source: spec.VersionSourceObserved}

func newSRSState(provider MetadataProvider) *batchState {
	state := newBatchState(spec.DialectMySQL, "golden", provider)
	state.resolvedVersion = srsGoldenVersion
	state.instanceFacts = &spec.InstanceFacts{Version: "8.0.36"}
	return state
}

func enrichSRS(t *testing.T, state *batchState, provider MetadataProvider, sqlText string) []spec.Statement {
	t.Helper()
	statements, failures := parseT26(t, sqlText)
	enriched, err := state.enrichStatements(context.Background(),
		&MetadataRequest{Schema: "golden", Provider: provider},
		statements, failures, state.instanceFacts, auditLimits{orderedStatements: defaultOrderedStatementLimit})
	if err != nil {
		t.Fatalf("enrich %q: %v", sqlText, err)
	}
	return enriched
}

func srsArm(t *testing.T, statement spec.Statement) spec.SRSDropArm {
	t.Helper()
	if statement.Metadata == nil || statement.Metadata.SRSDrop == nil {
		t.Fatalf("no shared adjudication attached: %+v", statement.Metadata)
	}
	return statement.Metadata.SRSDrop.Arm
}

func srsOwed(statement spec.Statement) []string {
	if statement.Metadata == nil || statement.Metadata.SRSDrop == nil {
		return nil
	}
	return statement.Metadata.SRSDrop.Owed
}

func assertReads(t *testing.T, provider *srsSQLProvider, fixture *srsSQLFixture, wantR, wantExist, wantDeps, wantQo, wantL int) {
	t.Helper()
	// R counts the SRS resolver seam only — schema/tablespace lookups
	// for touching statements ride the same channel but are a
	// different fact family's read.
	got := 0
	for _, req := range provider.forwards {
		if req.Type == "mysql_spatial_reference_system" {
			got++
		}
	}
	if got != wantR {
		t.Fatalf("R=%d want %d (forwards %+v)", got, wantR, provider.forwards)
	}
	if fixture == nil {
		return
	}
	if got := srsQueryCount(fixture, "st_spatial_reference_systems"); got != wantExist {
		t.Fatalf("Qexist=%d want %d", got, wantExist)
	}
	if got := srsQueryCount(fixture, "st_geometry_columns"); got != wantDeps {
		t.Fatalf("Qdeps=%d want %d", got, wantDeps)
	}
	if got := srsQueryCount(fixture, "version()"); got != wantQo {
		t.Fatalf("Qo=%d want %d", got, wantQo)
	}
	if got := provider.privCalls; got != wantL {
		t.Fatalf("L=%d want %d", got, wantL)
	}
}

// readsVector returns the current R/Qexist/Qdeps/Qo/L counters as an
// ordered array — used for per-call deltas where assertReads' fatal
// comparisons are too coarse.
func readsVector(provider *srsSQLProvider, fixture *srsSQLFixture) [5]int {
	var v [5]int
	for _, req := range provider.forwards {
		if req.Type == "mysql_spatial_reference_system" {
			v[0]++
		}
	}
	if fixture != nil {
		v[1] = srsQueryCount(fixture, "st_spatial_reference_systems")
		v[2] = srsQueryCount(fixture, "st_geometry_columns")
		v[3] = srsQueryCount(fixture, "version()")
	}
	v[4] = provider.privCalls
	return v
}

// assertReservedAdvisoryText is the R2-A discriminating check: the
// advisory explains reserved-range membership only — it must never
// upgrade membership into a native refusal prediction.
func assertReservedAdvisoryText(t *testing.T, finding rule.Finding) {
	t.Helper()
	if finding.Explanation == nil {
		t.Fatalf("reserved notice lost its Explanation")
	}
	for _, field := range []string{finding.Explanation.Why, finding.Explanation.Risk} {
		if strings.Contains(field, "refused") && !strings.Contains(field, "no claim") {
			t.Fatalf("reserved advisory claims native refusal: %q", field)
		}
	}
	if !strings.Contains(finding.Explanation.Why, "reserved range") {
		t.Fatalf("Why must state reserved-range membership: %q", finding.Explanation.Why)
	}
}

// assertSRSPrivilegeNotice is the strict conditional-marker check the
// privilege contract fixes (T26-SRS-DROP-PRIVILEGE-CONTRACT-01 §3):
// exactly one ddl.mysql.srs.drop.privilege finding on the statement —
// notice level, the exact discharged-premise message, and typed
// metadata carrying EXACTLY {srid:uint32, conditional_audit:string}.
// A missing key, nil, non-string, empty, or wrong token all fail;
// "!= \"\"" is never the criterion.
func assertSRSPrivilegeNotice(t *testing.T, stmt report.StatementResult, srid uint32) {
	t.Helper()
	var notice *rule.Finding
	for i := range stmt.Findings {
		if stmt.Findings[i].RuleID == "ddl.mysql.srs.drop.privilege" {
			if notice != nil {
				t.Fatalf("duplicate privilege notice: %+v", stmt.Findings)
			}
			notice = &stmt.Findings[i]
		}
	}
	if notice == nil {
		t.Fatalf("no ddl.mysql.srs.drop.privilege finding: %+v", stmt.Findings)
	}
	if notice.Level != rule.LevelNotice {
		t.Fatalf("privilege notice level=%v want notice", notice.Level)
	}
	wantMessage := fmt.Sprintf("spatial reference system %d drop privilege premise discharged under the observed_account_new_session conditional audit", srid)
	if notice.Message != wantMessage {
		t.Fatalf("privilege message=%q want exact %q", notice.Message, wantMessage)
	}
	if len(notice.Metadata) != 2 {
		t.Fatalf("privilege metadata=%+v want exactly {srid, conditional_audit}", notice.Metadata)
	}
	gotSRID, ok := notice.Metadata["srid"].(uint32)
	if !ok || gotSRID != srid {
		t.Fatalf("privilege metadata srid=%v(%T) want typed uint32 %d", notice.Metadata["srid"], notice.Metadata["srid"], srid)
	}
	marker, ok := notice.Metadata["conditional_audit"].(string)
	if !ok || marker != string(spec.PrivilegeAuditModeObservedAccountNewSession) {
		t.Fatalf("conditional_audit=%v(%T) want exact string observed_account_new_session",
			notice.Metadata["conditional_audit"], notice.Metadata["conditional_audit"])
	}
}

func tblTFixture() map[string]*spec.TableSnapshot {
	return map[string]*spec.TableSnapshot{
		"s.t": {Schema: "s", Table: &spec.Table{Schema: "s", Name: "t"}, Exists: true,
			Columns: []spec.Column{{Name: "g", Type: "geometry"}}},
		"s.u": {Schema: "s", Table: &spec.Table{Schema: "s", Name: "u"}, Exists: true,
			Columns: []spec.Column{{Name: "g", Type: "geometry"}}},
		"s.t_new": {Schema: "s", Table: &spec.Table{Schema: "s", Name: "t_new"}, Exists: false},
		"golden.t": {Schema: "golden", Table: &spec.Table{Schema: "golden", Name: "t"}, Exists: true,
			Columns: []spec.Column{{Name: "g", Type: "geometry"}}},
		"golden.t_new": {Schema: "golden", Table: &spec.Table{Schema: "golden", Name: "t_new"}, Exists: false},
	}
}

// --- E1/E4/E5.11: refused_dependency -----------------------------------------

func TestT26SRSR1RefusedDependency(t *testing.T) {
	fixture, provider := newSRSSQLHarness(t, &srsSQLFixture{
		version: "8.0.36",
		srs:     map[uint32]string{4326: "EPSG 4326"},
		deps:    []srsDepRow{{schema: "s", table: "t", column: "g", srid: 4326}},
	}, nil)
	state := newSRSState(provider)
	enriched := enrichSRS(t, state, provider, "DROP SPATIAL REFERENCE SYSTEM 4326")
	if arm := srsArm(t, enriched[0]); arm != spec.SRSDropArmRefusedDependency {
		t.Fatalf("arm=%v want refused_dependency", arm)
	}
	if len(srsOwed(enriched[0])) != 0 {
		t.Fatalf("owed=%v want empty for refusal", srsOwed(enriched[0]))
	}
	assertReads(t, provider, fixture, 1, 1, 1, 1, 0)

	result := evaluateEnriched(t, enriched)
	if len(result.Statements) != 1 || len(result.Statements[0].Findings) != 1 {
		t.Fatalf("findings=%+v want exactly one", result.Statements[0].Findings)
	}
	finding := result.Statements[0].Findings[0]
	if finding.RuleID != "ddl.mysql.srs.drop.dependency" || finding.Level != rule.LevelBlocker {
		t.Fatalf("finding=%+v want dependency blocker", finding)
	}
	if finding.Message != "spatial reference system 4326 is used by 1 visible geometry column(s)" {
		t.Fatalf("message=%q", finding.Message)
	}
	if finding.Metadata["srid"] != uint32(4326) || finding.Metadata["outcome"] != "refused_dependency" || finding.Metadata["dependency_witness_count"] != 1 {
		t.Fatalf("metadata=%+v", finding.Metadata)
	}
	for _, f := range result.Statements[0].Findings {
		if f.RuleID == "ddl.mysql.srs.drop.reserved" {
			t.Fatalf("reserved advisory must not fire inside the refusal arm even though 4326 is reserved: %+v", f)
		}
	}
	if len(result.Statements[0].EvidenceGaps) != 0 || result.Statements[0].Coverage.Status != report.CoverageComplete {
		t.Fatalf("gaps=%+v coverage=%+v", result.Statements[0].EvidenceGaps, result.Statements[0].Coverage)
	}
	if result.Verdict != report.VerdictReject {
		t.Fatalf("verdict=%v want reject", result.Verdict)
	}
}

// E1.2/E5.2: IF EXISTS never reaches adjudication — same arm.
func TestT26SRSR1IfExistsSameArm(t *testing.T) {
	fixture, provider := newSRSSQLHarness(t, &srsSQLFixture{
		version: "8.0.36",
		srs:     map[uint32]string{4326: "EPSG 4326"},
		deps:    []srsDepRow{{schema: "s", table: "t", column: "g", srid: 4326}},
	}, nil)
	enriched := enrichSRS(t, newSRSState(provider), provider, "DROP SPATIAL REFERENCE SYSTEM IF EXISTS 4326")
	if arm := srsArm(t, enriched[0]); arm != spec.SRSDropArmRefusedDependency {
		t.Fatalf("arm=%v want refused_dependency under IF EXISTS", arm)
	}
	result := evaluateEnriched(t, enriched)
	if got := result.Statements[0].NormalizedSQL; got != "DROP SPATIAL REFERENCE SYSTEM IF EXISTS 4326" {
		t.Fatalf("normalized_sql=%q", got)
	}
	for _, f := range result.Statements[0].Findings {
		if _, bad := f.Metadata["if_exists"]; bad {
			t.Fatalf("if_exists must never appear in finding metadata: %+v", f.Metadata)
		}
	}
	assertReads(t, provider, fixture, 1, 1, 1, 1, 0)
}

// E4.1: zero-row existence + valid positive dependency still refuses.
func TestT26SRSR1ZeroRowExistencePositiveDeps(t *testing.T) {
	_, provider := newSRSSQLHarness(t, &srsSQLFixture{
		version: "8.0.36",
		srs:     map[uint32]string{},
		deps:    []srsDepRow{{schema: "s", table: "t", column: "g", srid: 4326}},
	}, nil)
	enriched := enrichSRS(t, newSRSState(provider), provider, "DROP SPATIAL REFERENCE SYSTEM 4326")
	if arm := srsArm(t, enriched[0]); arm != spec.SRSDropArmRefusedDependency {
		t.Fatalf("arm=%v want refused_dependency — zero-row existence never discharges a positive witness", arm)
	}
}

// --- E2/E3/E5.3: unproven -----------------------------------------------------

func TestT26SRSR1UnprovenPresentEmptyDeps(t *testing.T) {
	fixture, provider := newSRSSQLHarness(t, &srsSQLFixture{
		version: "8.0.36",
		srs:     map[uint32]string{4326: "EPSG 4326"},
		deps:    nil,
	}, nil)
	enriched := enrichSRS(t, newSRSState(provider), provider, "DROP SPATIAL REFERENCE SYSTEM 4326")
	if arm := srsArm(t, enriched[0]); arm != spec.SRSDropArmUnproven {
		t.Fatalf("arm=%v want unproven", arm)
	}
	if got := srsOwed(enriched[0]); len(got) != 2 || got[0] != spec.SRSFactColumnDependencies || got[1] != spec.SRSFactDropPrivilege {
		t.Fatalf("owed=%v want [D,P] — existence proven", got)
	}
	assertReads(t, provider, fixture, 1, 1, 1, 1, 0)

	result := evaluateEnriched(t, enriched)
	stmt := result.Statements[0]
	if len(stmt.Findings) != 1 || stmt.Findings[0].RuleID != "ddl.mysql.srs.drop.reserved" ||
		stmt.Findings[0].Level != rule.LevelNotice || stmt.Findings[0].Metadata["srid"] != uint32(4326) {
		t.Fatalf("findings=%+v want one reserved notice {srid:4326}", stmt.Findings)
	}
	if len(stmt.Findings[0].Metadata) != 1 {
		t.Fatalf("reserved metadata carries only {srid}: %+v", stmt.Findings[0].Metadata)
	}
	if len(stmt.EvidenceGaps) != 1 || stmt.EvidenceGaps[0].ReasonCode != "unknown_srs_state" ||
		stmt.EvidenceGaps[0].RuleID != "ddl.mysql.srs.drop.dependency" ||
		len(stmt.EvidenceGaps[0].RequiredFacts) != 2 {
		t.Fatalf("gaps=%+v want one dependency gap [D,P]", stmt.EvidenceGaps)
	}
	if stmt.Coverage.Status != report.CoverageUnverified {
		t.Fatalf("coverage=%v want unverified", stmt.Coverage.Status)
	}
	if result.Verdict != report.VerdictReview {
		t.Fatalf("verdict=%v want review", result.Verdict)
	}
}

func TestT26SRSR1UnprovenProviderless(t *testing.T) {
	// E3.1: providerless — no resolver, no reads, full owed set.
	state := newSRSState(nil)
	enriched := enrichSRS(t, state, nil, "DROP SPATIAL REFERENCE SYSTEM 100001")
	if arm := srsArm(t, enriched[0]); arm != spec.SRSDropArmUnproven {
		t.Fatalf("arm=%v want unproven", arm)
	}
	if got := srsOwed(enriched[0]); len(got) != 3 || got[0] != spec.SRSFactExistence ||
		got[1] != spec.SRSFactColumnDependencies || got[2] != spec.SRSFactDropPrivilege {
		t.Fatalf("owed=%v want [E,D,P]", got)
	}
	result := evaluateEnriched(t, enriched)
	if len(result.Statements[0].Findings) != 0 {
		t.Fatalf("findings=%+v want absent (100001 not reserved)", result.Statements[0].Findings)
	}
}

// E3.3: provider present without a usable resolver — CURRENT shape:
// empty UnreadReason, unproven, [E,D,P], zero reads.
func TestT26SRSR1UnprovenNoResolver(t *testing.T) {
	provider := &srsBareProvider{version: "8.0.36"}
	enriched := enrichSRS(t, newSRSState(provider), provider, "DROP SPATIAL REFERENCE SYSTEM 100001")
	if arm := srsArm(t, enriched[0]); arm != spec.SRSDropArmUnproven {
		t.Fatalf("arm=%v want unproven", arm)
	}
	if st := enriched[0].Metadata.SpatialReference; st == nil || st.UnreadReason != "" {
		t.Fatalf("state=%+v want bound with empty UnreadReason (CURRENT shape)", st)
	}
	if got := srsOwed(enriched[0]); len(got) != 3 {
		t.Fatalf("owed=%v want [E,D,P]", got)
	}
}

// --- E5: SRID0 / reserved endpoints / numeric bounds --------------------------

func TestT26SRSR1RefusedSRID0(t *testing.T) {
	// E5.1: reads still fire — the contract keeps the normal provider-
	// backed read path; only the arm refuses.
	fixture, provider := newSRSSQLHarness(t, &srsSQLFixture{
		version: "8.0.36",
		srs:     map[uint32]string{0: "0"},
	}, nil)
	enriched := enrichSRS(t, newSRSState(provider), provider, "DROP SPATIAL REFERENCE SYSTEM 0")
	if arm := srsArm(t, enriched[0]); arm != spec.SRSDropArmRefusedSRID0 {
		t.Fatalf("arm=%v want refused_srid0", arm)
	}
	assertReads(t, provider, fixture, 1, 1, 1, 1, 0)

	result := evaluateEnriched(t, enriched)
	stmt := result.Statements[0]
	if len(stmt.Findings) != 1 || stmt.Findings[0].RuleID != "ddl.mysql.srs.drop.srid0" ||
		stmt.Findings[0].Level != rule.LevelBlocker ||
		stmt.Findings[0].Message != "dropping spatial reference system SRID 0 is not allowed" ||
		stmt.Findings[0].Metadata["srid"] != 0 || stmt.Findings[0].Metadata["outcome"] != "refused_srid0" {
		t.Fatalf("findings=%+v want the fixed srid0 blocker", stmt.Findings)
	}
	for _, f := range stmt.Findings {
		if f.RuleID == "ddl.mysql.srs.drop.reserved" {
			t.Fatalf("reserved advisory must not fire inside the refusal arm: %+v", f)
		}
	}
	if len(stmt.EvidenceGaps) != 0 || stmt.Coverage.Status != report.CoverageComplete || result.Verdict != report.VerdictReject {
		t.Fatalf("gaps=%+v coverage=%v verdict=%v", stmt.EvidenceGaps, stmt.Coverage.Status, result.Verdict)
	}
}

func TestT26SRSR1ReservedEndpoints(t *testing.T) {
	reserved := map[uint32]bool{
		0: true, 32767: true, 32768: false,
		59999999: false, 60000000: true, 69999999: true, 70000000: false,
		1999999999: false, 2000000000: true, 2147483647: true, 2147483648: false,
		4294967295: false,
	}
	for srid, want := range reserved {
		if got := spec.SRSDropInReservedRange(srid); got != want {
			t.Fatalf("SRSDropInReservedRange(%d)=%v want %v", srid, got, want)
		}
	}
	// E5.4 shape: endpoint advisory fires as the lone notice finding.
	// Qo outside the D-proof keeps the zero-row answer unproven → S2.
	fixture, provider := newSRSSQLHarness(t, &srsSQLFixture{version: "8.0.36", qoVersion: "8.0.3"}, nil)
	enriched := enrichSRS(t, newSRSState(provider), provider, "DROP SPATIAL REFERENCE SYSTEM 32767")
	if arm := srsArm(t, enriched[0]); arm != spec.SRSDropArmUnproven {
		t.Fatalf("32767 arm=%v want unproven before checking the advisory text", arm)
	}
	result := evaluateEnriched(t, enriched)
	stmt := result.Statements[0]
	if len(stmt.Findings) != 1 || stmt.Findings[0].RuleID != "ddl.mysql.srs.drop.reserved" ||
		stmt.Findings[0].Message != "spatial reference system 32767 is in a reserved SRID range" {
		t.Fatalf("findings=%+v want the reserved notice", stmt.Findings)
	}
	assertReservedAdvisoryText(t, stmt.Findings[0])
	assertReads(t, provider, fixture, 1, 1, 1, 1, 0)

	// E5.5 shape: 32768 — no advisory at all.
	fixture, provider = newSRSSQLHarness(t, &srsSQLFixture{version: "8.0.36", qoVersion: "8.0.3"}, nil)
	enriched = enrichSRS(t, newSRSState(provider), provider, "DROP SPATIAL REFERENCE SYSTEM 32768")
	result = evaluateEnriched(t, enriched)
	if n := len(result.Statements[0].Findings); n != 0 {
		t.Fatalf("32768 findings=%d want 0", n)
	}
}

// --- E5.7–E5.10: unclaimed forms stay parser diagnostics -----------------------

func TestT26SRSR1UnclaimedForms(t *testing.T) {
	for _, sql := range []string{
		"DROP SPATIAL REFERENCE SYSTEM -1",
		"DROP SPATIAL REFERENCE SYSTEM 4294967296",
		"DROP SPATIAL REFERENCE SYSTEM 'x'",
		"DROP SPATIAL REFERENCE SYSTEM foo",
		"DROP SPATIAL REFERENCE SYSTEM 1 EXTRA",
		"DROP SPATIAL REFERENCE SYSTEM IF EXISTS 1 X",
	} {
		parsed, _ := parseSQL(context.Background(), sql, spec.DialectMySQL)
		statements, err := Extract(context.Background(), parsed)
		if err != nil {
			t.Fatalf("%q: extract err %v", sql, err)
		}
		for _, stmt := range statements {
			if stmt.DDL != nil && stmt.DDL.SpatialReference != nil {
				t.Fatalf("%q: claimed as SRS statement", sql)
			}
		}
		if len(parsed.failures) == 0 {
			t.Fatalf("%q: want a parse failure record", sql)
		}
	}
}

// --- E6/E7: repeat-SRID, cross-call, boundary lifecycle -----------------------

// E6.1: S2 mints same-SRID existence stale — stmt2 blocks before the
// cache (reads 0). Qo 8.0.3 keeps the zero-row answer outside the
// proven-absence domain → the unproven arm still governs.
func TestT26SRSR1S2MintsExistenceStale(t *testing.T) {
	fixture, provider := newSRSSQLHarness(t, &srsSQLFixture{version: "8.0.36", qoVersion: "8.0.3"}, nil)
	enriched := enrichSRS(t, newSRSState(provider), provider,
		"DROP SPATIAL REFERENCE SYSTEM 4326; DROP SPATIAL REFERENCE SYSTEM 4326")
	if arm := srsArm(t, enriched[0]); arm != spec.SRSDropArmUnproven {
		t.Fatalf("stmt1 arm=%v want unproven", arm)
	}
	if arm := srsArm(t, enriched[1]); arm != spec.SRSDropArmBlocked {
		t.Fatalf("stmt2 arm=%v want blocked — stale flag outranks the cache", arm)
	}
	if got := srsOwed(enriched[1]); len(got) != 1 || got[0] != spec.SRSFactBatchEffects {
		t.Fatalf("stmt2 owed=%v want exactly [srs.batch_effects]", got)
	}
	if st := enriched[1].Metadata.SpatialReference; st == nil || st.UnreadReason != spec.MetadataUnreadBatchEffects {
		t.Fatalf("stmt2 state=%+v want batch_effects unread", st)
	}
	assertReads(t, provider, fixture, 1, 1, 1, 1, 0)
}

// E6.2: S0 mints nothing — the repeat still reads and refuses.
func TestT26SRSR1S0RepeatReads(t *testing.T) {
	fixture, provider := newSRSSQLHarness(t, &srsSQLFixture{
		version: "8.0.36", srs: map[uint32]string{0: "0"},
	}, nil)
	enriched := enrichSRS(t, newSRSState(provider), provider,
		"DROP SPATIAL REFERENCE SYSTEM 0; DROP SPATIAL REFERENCE SYSTEM 0")
	if arm := srsArm(t, enriched[0]); arm != spec.SRSDropArmRefusedSRID0 {
		t.Fatalf("stmt1 arm=%v want refused_srid0", arm)
	}
	// Second statement hits the same identity through the once-per-request
	// object cache — the clean channel keeps it consumable.
	if arm := srsArm(t, enriched[1]); arm != spec.SRSDropArmRefusedSRID0 {
		t.Fatalf("stmt2 arm=%v want refused_srid0 — S0 mints no stale", arm)
	}
	assertReads(t, provider, fixture, 1, 1, 1, 1, 0)
}

// E6.3: S1 mints nothing — repeat re-refuses on the cached witness.
func TestT26SRSR1S1RepeatRefuses(t *testing.T) {
	fixture, provider := newSRSSQLHarness(t, &srsSQLFixture{
		version: "8.0.36",
		srs:     map[uint32]string{4326: "EPSG 4326"},
		deps:    []srsDepRow{{schema: "s", table: "t", column: "g", srid: 4326}},
	}, nil)
	enriched := enrichSRS(t, newSRSState(provider), provider,
		"DROP SPATIAL REFERENCE SYSTEM 4326; DROP SPATIAL REFERENCE SYSTEM 4326")
	for i, stmt := range enriched {
		if arm := srsArm(t, stmt); arm != spec.SRSDropArmRefusedDependency {
			t.Fatalf("stmt%d arm=%v want refused_dependency", i+1, arm)
		}
	}
	assertReads(t, provider, fixture, 1, 1, 1, 1, 0)
}

// E7.1: an audited CREATE OR REPLACE carrying unresolved premises
// mints existence stale — the later same-SRID DROP blocks with zero
// reads.
func TestT26SRSR1BoundaryLifecycleMintsStale(t *testing.T) {
	fixture, provider := newSRSSQLHarness(t, &srsSQLFixture{version: "8.0.36"}, nil)
	enriched := enrichSRS(t, newSRSState(provider), provider,
		"CREATE OR REPLACE SPATIAL REFERENCE SYSTEM 4326 NAME 'r' DEFINITION 'x'; DROP SPATIAL REFERENCE SYSTEM 4326")
	if enriched[0].Unsupported != nil {
		t.Fatalf("stmt1 unsupported=%+v want nil — MySQL CREATE OR REPLACE is audited", enriched[0].Unsupported)
	}
	if enriched[0].Metadata == nil || enriched[0].Metadata.SRSCreate == nil ||
		enriched[0].Metadata.SRSCreate.Arm != spec.SRSCreateArmCreate ||
		len(enriched[0].Metadata.SRSCreate.Owed) == 0 {
		t.Fatalf("stmt1 adj=%+v want create arm with owed premises (stale-minting write)", enriched[0].Metadata)
	}
	if arm := srsArm(t, enriched[1]); arm != spec.SRSDropArmBlocked {
		t.Fatalf("stmt2 arm=%v want blocked", arm)
	}
	if got := srsOwed(enriched[1]); len(got) != 1 || got[0] != spec.SRSFactBatchEffects {
		t.Fatalf("stmt2 owed=%v want [B]", got)
	}
	assertReads(t, provider, fixture, 1, 2, 1, 1, 0)
	result := evaluateEnriched(t, enriched)
	if n := len(result.Statements[1].Findings); n != 1 || result.Statements[1].Findings[0].RuleID != "ddl.mysql.srs.drop.reserved" {
		t.Fatalf("stmt2 findings=%+v want only the reserved co-fire", result.Statements[1].Findings)
	}
	assertReservedAdvisoryText(t, result.Statements[1].Findings[0])
}

// E7.2: a different SRID stays unaffected by the boundary write.
// Qo 8.0.3 keeps the zero-row answer unproven → S2, not the absent arm.
func TestT26SRSR1BoundaryDifferentSRID(t *testing.T) {
	fixture, provider := newSRSSQLHarness(t, &srsSQLFixture{version: "8.0.36", qoVersion: "8.0.3"}, nil)
	enriched := enrichSRS(t, newSRSState(provider), provider,
		"CREATE OR REPLACE SPATIAL REFERENCE SYSTEM 4326 DEFINITION 'x'; DROP SPATIAL REFERENCE SYSTEM 100001")
	if arm := srsArm(t, enriched[1]); arm != spec.SRSDropArmUnproven {
		t.Fatalf("stmt2 arm=%v want unproven — per-SRID staleness only", arm)
	}
	assertReads(t, provider, fixture, 2, 2, 2, 2, 0)
}

// --- E8: spatialTouching ------------------------------------------------------

// E8.1: DROP TABLE t is touching — depsStale demotes the witness; a
// still-valid positive existence keeps consumption → S2 [D,P].
func TestT26SRSR1DropTableTouchesDeps(t *testing.T) {
	fixture, provider := newSRSSQLHarness(t, &srsSQLFixture{
		version: "8.0.36",
		srs:     map[uint32]string{4326: "EPSG 4326"},
		deps:    []srsDepRow{{schema: "s", table: "t", column: "g", srid: 4326}},
	}, tblTFixture())
	enriched := enrichSRS(t, newSRSState(provider), provider,
		"DROP TABLE s.t; DROP SPATIAL REFERENCE SYSTEM 4326")
	if arm := srsArm(t, enriched[1]); arm != spec.SRSDropArmUnproven {
		t.Fatalf("stmt2 arm=%v want unproven", arm)
	}
	if got := srsOwed(enriched[1]); len(got) != 2 || got[0] != spec.SRSFactColumnDependencies || got[1] != spec.SRSFactDropPrivilege {
		t.Fatalf("owed=%v want exactly [D,P] — existence consumed, dependency stale", got)
	}
	assertReads(t, provider, fixture, 1, 1, 1, 1, 0)
}

// E8.2a: complete inert CREATE keeps the witness consumable → S1.
func TestT26SRSR1InertCreateKeepsWitness(t *testing.T) {
	fixture, provider := newSRSSQLHarness(t, &srsSQLFixture{
		version: "8.0.36",
		srs:     map[uint32]string{4326: "EPSG 4326"},
		deps:    []srsDepRow{{schema: "s", table: "t", column: "g", srid: 4326}},
	}, tblTFixture())
	enriched := enrichSRS(t, newSRSState(provider), provider,
		"CREATE TABLE s.t_new (i INT); DROP SPATIAL REFERENCE SYSTEM 4326")
	if arm := srsArm(t, enriched[1]); arm != spec.SRSDropArmRefusedDependency {
		t.Fatalf("stmt2 arm=%v want refused_dependency — inert CREATE mints nothing", arm)
	}
	assertReads(t, provider, fixture, 1, 1, 1, 1, 0)
}

// E8.4: vendor-boundary COMPACT touches → S2 [D,P] — bound-target
// invalidation without contamination.
func TestT26SRSR1CompactTouchesWithoutContamination(t *testing.T) {
	fixture, provider := newSRSSQLHarness(t, &srsSQLFixture{
		version: "8.0.36",
		srs:     map[uint32]string{4326: "EPSG 4326"},
		deps:    []srsDepRow{{schema: "s", table: "t", column: "g", srid: 4326}},
	}, tblTFixture())
	state := newSRSState(provider)
	enriched := enrichSRS(t, state, provider,
		"ALTER TABLE s.t COMPACT TIFLASH REPLICA; DROP SPATIAL REFERENCE SYSTEM 4326")
	if state.contaminated {
		t.Fatal("COMPACT contaminated the batch — vendor boundary must stay clean")
	}
	if arm := srsArm(t, enriched[1]); arm != spec.SRSDropArmUnproven {
		t.Fatalf("stmt2 arm=%v want unproven", arm)
	}
	if got := srsOwed(enriched[1]); len(got) != 2 || got[0] != spec.SRSFactColumnDependencies || got[1] != spec.SRSFactDropPrivilege {
		t.Fatalf("owed=%v want [D,P]", got)
	}
	assertReads(t, provider, fixture, 1, 1, 1, 1, 0)
}

// E8.5: the flag survives across enrich calls on the same batchState —
// per-call read deltas plus the exact public projection.
func TestT26SRSR1DepsStaleCrossCall(t *testing.T) {
	fixture, provider := newSRSSQLHarness(t, &srsSQLFixture{
		version: "8.0.36",
		srs:     map[uint32]string{4326: "EPSG 4326"},
		deps:    []srsDepRow{{schema: "s", table: "t", column: "g", srid: 4326}},
	}, tblTFixture())
	state := newSRSState(provider)
	enrichSRS(t, state, provider, "DROP TABLE s.t")
	if !state.srsDepsStale {
		t.Fatal("DROP TABLE t must mint srsDepsStale")
	}
	// Call-1 delta: zero SRS reads — the table write never binds SRS.
	call1 := readsVector(provider, fixture)
	if call1 != [5]int{0, 0, 0, 0, 0} {
		t.Fatalf("call1 SRS reads=%v want all zero", call1)
	}
	// Per-call counters: the once-per-request object cache is per
	// enrich call, so call2 re-forwards the SRID.
	provider.forwards = nil
	fixture.queries = nil
	enriched := enrichSRS(t, state, provider, "DROP SPATIAL REFERENCE SYSTEM 4326")
	if arm := srsArm(t, enriched[0]); arm != spec.SRSDropArmUnproven {
		t.Fatalf("call2 arm=%v want unproven — flag persists across calls", arm)
	}
	wantOwed := []string{spec.SRSFactColumnDependencies, spec.SRSFactDropPrivilege}
	if got := srsOwed(enriched[0]); !reflect.DeepEqual(got, wantOwed) {
		t.Fatalf("owed=%v want exactly %v", got, wantOwed)
	}
	call2 := readsVector(provider, fixture)
	if call2 != [5]int{1, 1, 1, 1, 0} {
		t.Fatalf("call2 delta=%v want 1/1/1/1/0", call2)
	}
	result := evaluateEnriched(t, enriched)
	stmt := result.Statements[0]
	if len(stmt.EvidenceGaps) != 1 || !reflect.DeepEqual(stmt.EvidenceGaps[0].RequiredFacts, wantOwed) {
		t.Fatalf("public gap=%+v want exactly %v", stmt.EvidenceGaps, wantOwed)
	}
	reserved := 0
	for _, f := range stmt.Findings {
		if f.RuleID == "ddl.mysql.srs.drop.reserved" {
			reserved++
		}
	}
	if reserved != 1 || stmt.Coverage.Status != report.CoverageUnverified {
		t.Fatalf("reserved=%d coverage=%v want 1/unverified", reserved, stmt.Coverage.Status)
	}
}

// E8.6: refused S1 mints nothing; the later touching write is what
// demotes stmt3's witness.
func TestT26SRSR1RefusedThenTouching(t *testing.T) {
	fixture, provider := newSRSSQLHarness(t, &srsSQLFixture{
		version: "8.0.36",
		srs:     map[uint32]string{4326: "EPSG 4326"},
		deps:    []srsDepRow{{schema: "s", table: "t", column: "g", srid: 4326}},
	}, tblTFixture())
	state := newSRSState(provider)
	enriched := enrichSRS(t, state, provider,
		"DROP SPATIAL REFERENCE SYSTEM 4326; DROP TABLE s.t; DROP SPATIAL REFERENCE SYSTEM 4326")
	if arm := srsArm(t, enriched[0]); arm != spec.SRSDropArmRefusedDependency {
		t.Fatalf("stmt1 arm=%v want refused_dependency", arm)
	}
	// Stmt3 reaching unproven — not blocked — proves stmt1's refusal
	// minted no existence stale (a minted flag would block the bind).
	if arm := srsArm(t, enriched[2]); arm != spec.SRSDropArmUnproven {
		t.Fatalf("stmt3 arm=%v want unproven — deps stale, existence consumed", arm)
	}
	if got := srsOwed(enriched[2]); len(got) != 2 {
		t.Fatalf("stmt3 owed=%v want [D,P]", got)
	}
	assertReads(t, provider, fixture, 1, 1, 1, 1, 0)
}

// E8.7: ALTER TABLE DROP COLUMN g touches.
func TestT26SRSR1AlterDropColumnTouches(t *testing.T) {
	fixture, provider := newSRSSQLHarness(t, &srsSQLFixture{
		version: "8.0.36",
		srs:     map[uint32]string{4326: "EPSG 4326"},
		deps:    []srsDepRow{{schema: "s", table: "t", column: "g", srid: 4326}},
	}, tblTFixture())
	enriched := enrichSRS(t, newSRSState(provider), provider,
		"DROP SPATIAL REFERENCE SYSTEM 4326; ALTER TABLE s.t DROP COLUMN g; DROP SPATIAL REFERENCE SYSTEM 4326")
	if arm := srsArm(t, enriched[0]); arm != spec.SRSDropArmRefusedDependency {
		t.Fatalf("stmt1 arm=%v want refused_dependency", arm)
	}
	if arm := srsArm(t, enriched[2]); arm != spec.SRSDropArmUnproven {
		t.Fatalf("stmt3 arm=%v want unproven", arm)
	}
	assertReads(t, provider, fixture, 1, 1, 1, 1, 0)
}

// E8.8: complete ENGINE-only inert ALTER keeps the witness consumable.
func TestT26SRSR1InertAlterKeepsWitness(t *testing.T) {
	fixture, provider := newSRSSQLHarness(t, &srsSQLFixture{
		version: "8.0.36",
		srs:     map[uint32]string{4326: "EPSG 4326"},
		deps:    []srsDepRow{{schema: "s", table: "t", column: "g", srid: 4326}},
	}, tblTFixture())
	enriched := enrichSRS(t, newSRSState(provider), provider,
		"ALTER TABLE s.t ENGINE=InnoDB; DROP SPATIAL REFERENCE SYSTEM 4326")
	if arm := srsArm(t, enriched[1]); arm != spec.SRSDropArmRefusedDependency {
		t.Fatalf("stmt2 arm=%v want refused_dependency — ENGINE-only ALTER is inert", arm)
	}
	assertReads(t, provider, fixture, 1, 1, 1, 1, 0)
}

// E8.9: DROP SCHEMA touches; E8.10: CREATE SCHEMA is inert.
func TestT26SRSR1SchemaOps(t *testing.T) {
	fixture, provider := newSRSSQLHarness(t, &srsSQLFixture{
		version: "8.0.36",
		srs:     map[uint32]string{4326: "EPSG 4326"},
		deps:    []srsDepRow{{schema: "s", table: "t", column: "g", srid: 4326}},
	}, tblTFixture())
	enriched := enrichSRS(t, newSRSState(provider), provider,
		"DROP SCHEMA s2; DROP SPATIAL REFERENCE SYSTEM 4326")
	if arm := srsArm(t, enriched[1]); arm != spec.SRSDropArmUnproven {
		t.Fatalf("DROP SCHEMA: stmt2 arm=%v want unproven", arm)
	}
	assertReads(t, provider, fixture, 1, 1, 1, 1, 0)

	fixture, provider = newSRSSQLHarness(t, &srsSQLFixture{
		version: "8.0.36",
		srs:     map[uint32]string{4326: "EPSG 4326"},
		deps:    []srsDepRow{{schema: "s", table: "t", column: "g", srid: 4326}},
	}, tblTFixture())
	enriched = enrichSRS(t, newSRSState(provider), provider,
		"CREATE SCHEMA s2_new; DROP SPATIAL REFERENCE SYSTEM 4326")
	if arm := srsArm(t, enriched[1]); arm != spec.SRSDropArmRefusedDependency {
		t.Fatalf("CREATE SCHEMA: stmt2 arm=%v want refused_dependency — empty container is inert", arm)
	}
	assertReads(t, provider, fixture, 1, 1, 1, 1, 0)
}

// E8.11: CREATE TABLE ... LIKE fails the inert predicate → touching.
func TestT26SRSR1CreateLikeTouches(t *testing.T) {
	fixture, provider := newSRSSQLHarness(t, &srsSQLFixture{
		version: "8.0.36",
		srs:     map[uint32]string{4326: "EPSG 4326"},
		deps:    []srsDepRow{{schema: "s", table: "t", column: "g", srid: 4326}},
	}, tblTFixture())
	enriched := enrichSRS(t, newSRSState(provider), provider,
		"CREATE TABLE s.t_new LIKE s.t; DROP SPATIAL REFERENCE SYSTEM 4326")
	if arm := srsArm(t, enriched[1]); arm != spec.SRSDropArmUnproven {
		t.Fatalf("stmt2 arm=%v want unproven — LIKE is touching", arm)
	}
	if got := srsOwed(enriched[1]); len(got) != 2 {
		t.Fatalf("owed=%v want [D,P]", got)
	}
	assertReads(t, provider, fixture, 1, 1, 1, 1, 0)
}

// D3 overlap checks on the classifier itself — synthetic spec-level
// statements only (no-producer inputs stay at the spec/model layer).
func TestT26SRSR1D3ClassifierEdges(t *testing.T) {
	state := newSRSState(nil)
	inert := func(sql string, build func() *spec.DDL) {
		stmt := spec.Statement{Dialect: spec.DialectMySQL, Kind: spec.KindDDL, DDL: build()}
		if state.spatialTouching(stmt) {
			t.Fatalf("%s: want inert", sql)
		}
	}
	touching := func(sql string, build func() *spec.DDL) {
		stmt := spec.Statement{Dialect: spec.DialectMySQL, Kind: spec.KindDDL, DDL: build()}
		if !state.spatialTouching(stmt) {
			t.Fatalf("%s: want touching", sql)
		}
	}
	// Inert ops with non-empty TableTargets stay inert (index/region ops).
	inert("create_index", func() *spec.DDL {
		return &spec.DDL{Operation: spec.DDLOperationCreateIndex, Table: &spec.Table{Name: "t"}}
	})
	inert("split_region", func() *spec.DDL {
		return &spec.DDL{Operation: spec.DDLOperationSplitRegion, Table: &spec.Table{Name: "t"}}
	})
	// SERVER/SRS families exit by marker, never by op enumeration.
	inert("server", func() *spec.DDL {
		return &spec.DDL{Operation: spec.DDLOperationDropServer, Server: &spec.ServerDeclaration{}}
	})
	inert("srs", func() *spec.DDL {
		return &spec.DDL{Operation: spec.DDLOperationDropSpatialReferenceSystem, SpatialReference: &spec.SpatialReferenceDeclaration{Kind: spec.SRSKindDrop}}
	})
	// Inert CREATE edge cases: LIKE/CTAS/spatial type/unextracted all fail.
	touching("create_like", func() *spec.DDL {
		return &spec.DDL{Operation: spec.DDLOperationCreateTable, HasReferTable: true,
			ReferTable: &spec.Table{Name: "t"}, Columns: []spec.Column{{Name: "i", Type: "int"}}}
	})
	// The discriminator flags alone decide — a true flag with a nil
	// payload or empty source list is still touching.
	touching("create_refer_flag_nil", func() *spec.DDL {
		return &spec.DDL{Operation: spec.DDLOperationCreateTable, HasReferTable: true,
			Columns: []spec.Column{{Name: "i", Type: "int"}}}
	})
	touching("create_select_flag_empty", func() *spec.DDL {
		return &spec.DDL{Operation: spec.DDLOperationCreateTable, HasSelect: true,
			SelectSources: nil, Columns: []spec.Column{{Name: "i", Type: "int"}}}
	})
	touching("create_select_sources", func() *spec.DDL {
		return &spec.DDL{Operation: spec.DDLOperationCreateTable, HasSelect: true,
			SelectSources: []spec.Table{{Name: "src"}}, Columns: []spec.Column{{Name: "i", Type: "int"}}}
	})
	touching("create_spatial_col", func() *spec.DDL {
		return &spec.DDL{Operation: spec.DDLOperationCreateTable, Columns: []spec.Column{{Name: "g", Type: "geometry"}}}
	})
	// Position inside the column list never matters: a clean head column
	// followed by a spatial tail is still touching.
	touching("create_clean_then_spatial", func() *spec.DDL {
		return &spec.DDL{Operation: spec.DDLOperationCreateTable, Columns: []spec.Column{
			{Name: "i", Type: "int"}, {Name: "g", Type: "point"}}}
	})
	// Same for a clean head + unextracted tail — member-level options
	// defeat the column-list completeness clause.
	touching("create_clean_then_unextracted", func() *spec.DDL {
		return &spec.DDL{Operation: spec.DDLOperationCreateTable, Columns: []spec.Column{
			{Name: "i", Type: "int"}, {Name: "v", Type: "varchar", UnextractedOptions: []string{"charset"}}}}
	})
	inert("create_plain", func() *spec.DDL {
		return &spec.DDL{Operation: spec.DDLOperationCreateTable, Columns: []spec.Column{{Name: "i", Type: "int"}, {Name: "v", Type: "varchar"}}}
	})
	// Inert ALTER edge cases.
	touching("alter_two_options", func() *spec.DDL {
		return &spec.DDL{Operation: spec.DDLOperationAlterTable, Table: &spec.Table{Name: "t"},
			Alter: []spec.Alter{{Action: "table_option", Options: map[string]string{"engine": "x", "auto_increment": "1"}}}}
	})
	touching("alter_unknown_action", func() *spec.DDL {
		return &spec.DDL{Operation: spec.DDLOperationAlterTable, Table: &spec.Table{Name: "t"},
			Alter: []spec.Alter{{Action: "alter_99"}}}
	})
	touching("alter_unextracted", func() *spec.DDL {
		return &spec.DDL{Operation: spec.DDLOperationAlterTable, Table: &spec.Table{Name: "t"},
			Alter: []spec.Alter{{Action: "table_option", Options: map[string]string{"engine": "x"}, UnextractedOptions: []string{"compression"}}}}
	})
	touching("alter_empty", func() *spec.DDL {
		return &spec.DDL{Operation: spec.DDLOperationAlterTable, Table: &spec.Table{Name: "t"}}
	})
	// Fallback: unknown op + boundary statement shapes resolve touching.
	touching("unknown_op", func() *spec.DDL {
		return &spec.DDL{Operation: spec.DDLOperationUnknown}
	})

	// Coverage-aspect triggers — the completeness clause fires on a real
	// aspect gap, not just on field shape. Each fixture first proves the
	// named aspect is actually produced, then asserts the classifier
	// resolves touching (R3-D).
	// CREATE: an AUTO_RANDOM column under MySQL produces an
	// aspect:"column" gap while every other inert predicate holds —
	// the aspect clause alone defeats inertness.
	autoRandom := spec.Statement{Dialect: spec.DialectMySQL, Kind: spec.KindDDL,
		DDL: &spec.DDL{Operation: spec.DDLOperationCreateTable, Columns: []spec.Column{
			{Name: "i", Type: "bigint", AutoRandom: true}}}}
	if !srsHasCoverageAspect(spec.DialectMySQL, autoRandom, "column") {
		t.Fatal("AUTO_RANDOM fixture must produce the column aspect gap it tests")
	}
	if !state.spatialTouching(autoRandom) {
		t.Fatal("CREATE with a column-aspect gap must be touching")
	}
	// ALTER: a DDL-level unextracted option produces aspect:"option"
	// while every member action is a clean engine-only table_option —
	// the aspect clause catches what member checks miss. (Member-level
	// UnextractedOptions overlaps the action exclusion and is covered
	// by alter_unextracted above.)
	alterOptGap := spec.Statement{Dialect: spec.DialectMySQL, Kind: spec.KindDDL,
		DDL: &spec.DDL{Operation: spec.DDLOperationAlterTable, Table: &spec.Table{Name: "t"},
			UnextractedOptions: []string{"compression"},
			Alter:              []spec.Alter{{Action: "table_option", Options: map[string]string{"engine": "x"}}}}}
	if !srsHasCoverageAspect(spec.DialectMySQL, alterOptGap, "option") {
		t.Fatal("DDL-level unextracted option must produce the option aspect gap")
	}
	if !state.spatialTouching(alterOptGap) {
		t.Fatal("ALTER with an option-aspect gap must be touching")
	}
}

// --- E9: authored ordering + cross-call contamination -------------------------

func TestT26SRSR1ParseFailureBlocks(t *testing.T) {
	// E9.1/E9.2: a failure record authored before the SRS statement
	// contaminates it — line/column order governs, not list position.
	fixture, provider := newSRSSQLHarness(t, &srsSQLFixture{version: "8.0.36"}, nil)
	parsed, _ := parseSQL(context.Background(), "GARBAGE; DROP SPATIAL REFERENCE SYSTEM 4326", spec.DialectMySQL)
	if len(parsed.failures) != 1 || len(parsed.Statements) != 1 {
		t.Fatalf("failures=%d statements=%d", len(parsed.failures), len(parsed.Statements))
	}
	failure := parsed.failures[0]
	if failure.Line != 1 || failure.Column < 1 || failure.Column > 7 {
		t.Fatalf("failure at (%d,%d) want line1 col1-7", failure.Line, failure.Column)
	}
	statements, err := Extract(context.Background(), parsed)
	if err != nil || len(statements) != 1 {
		t.Fatalf("extract: %v %d", err, len(statements))
	}
	// E9.2 authored coordinates: the failure record sits at (1,1) and
	// the DROP was authored at (1,10) on the same line — those exact
	// positions belong to this GARBAGE case; the GEOMETRY rows keep
	// their own authored positions (R3 erratum E-9.2-attribution).
	if statements[0].Line != 1 || statements[0].Column != 10 {
		t.Fatalf("DROP authored at (%d,%d) want (1,10)", statements[0].Line, statements[0].Column)
	}
	state := newSRSState(provider)
	enriched, err := state.enrichStatements(context.Background(),
		&MetadataRequest{Schema: "golden", Provider: provider},
		statements, parsed.failures, state.instanceFacts, auditLimits{orderedStatements: defaultOrderedStatementLimit})
	if err != nil {
		t.Fatalf("enrich: %v", err)
	}
	if arm := srsArm(t, enriched[0]); arm != spec.SRSDropArmBlocked {
		t.Fatalf("arm=%v want blocked — authored-order contamination", arm)
	}
	if got := srsOwed(enriched[0]); len(got) != 1 || got[0] != spec.SRSFactBatchEffects {
		t.Fatalf("owed=%v want [B]", got)
	}
	assertReads(t, provider, fixture, 0, 0, 0, 0, 0)
}

// E9.3: two real enrich calls share the batchState — call1's S0 reads
// fire, the trailing failure contaminates via the tail sweep, call2
// enters B with zero reads.
func TestT26SRSR1TwoCallContamination(t *testing.T) {
	fixture, provider := newSRSSQLHarness(t, &srsSQLFixture{
		version: "8.0.36", srs: map[uint32]string{0: "0", 4326: "EPSG 4326"},
	}, nil)
	state := newSRSState(provider)
	parsed1, _ := parseSQL(context.Background(), "DROP SPATIAL REFERENCE SYSTEM 0; GARBAGE;", spec.DialectMySQL)
	statements1, err := Extract(context.Background(), parsed1)
	if err != nil || len(statements1) != 1 {
		t.Fatalf("call1 extract: %v %d", err, len(statements1))
	}
	enriched1, err := state.enrichStatements(context.Background(),
		&MetadataRequest{Schema: "golden", Provider: provider},
		statements1, parsed1.failures, state.instanceFacts, auditLimits{orderedStatements: defaultOrderedStatementLimit})
	if err != nil {
		t.Fatalf("call1 enrich: %v", err)
	}
	if arm := srsArm(t, enriched1[0]); arm != spec.SRSDropArmRefusedSRID0 {
		t.Fatalf("call1 arm=%v want refused_srid0", arm)
	}
	// Call-1 delta: the S0 bind really fired all four reads.
	call1 := readsVector(provider, fixture)
	if call1 != [5]int{1, 1, 1, 1, 0} {
		t.Fatalf("call1 reads=%v want 1/1/1/1/0 — S0's reads must not be erased", call1)
	}
	if !state.contaminated {
		t.Fatal("trailing failure must contaminate via the tail sweep")
	}
	if len(state.srsExistenceStale) != 0 {
		t.Fatalf("S0 mints no stale: %+v", state.srsExistenceStale)
	}
	result1 := evaluateEnriched(t, enriched1)
	stmt1 := result1.Statements[0]
	if len(stmt1.Findings) != 1 || stmt1.Findings[0].RuleID != "ddl.mysql.srs.drop.srid0" {
		t.Fatalf("call1 findings=%+v want only the srid0 blocker", stmt1.Findings)
	}
	if stmt1.Coverage.Status != report.CoverageComplete {
		t.Fatalf("call1 coverage=%v want complete", stmt1.Coverage.Status)
	}

	parsed2, _ := parseSQL(context.Background(), "DROP SPATIAL REFERENCE SYSTEM 4326", spec.DialectMySQL)
	statements2, err := Extract(context.Background(), parsed2)
	if err != nil || len(statements2) != 1 {
		t.Fatalf("call2 extract: %v %d", err, len(statements2))
	}
	enriched2, err := state.enrichStatements(context.Background(),
		&MetadataRequest{Schema: "golden", Provider: provider},
		statements2, parsed2.failures, state.instanceFacts, auditLimits{orderedStatements: defaultOrderedStatementLimit})
	if err != nil {
		t.Fatalf("call2 enrich: %v", err)
	}
	if arm := srsArm(t, enriched2[0]); arm != spec.SRSDropArmBlocked {
		t.Fatalf("call2 arm=%v want blocked", arm)
	}
	if got := srsOwed(enriched2[0]); !reflect.DeepEqual(got, []string{spec.SRSFactBatchEffects}) {
		t.Fatalf("call2 owed=%v want exactly [B]", got)
	}
	// Call-2 delta: zero — the fresh per-call cache cannot launder the
	// batchState contamination.
	call2 := readsVector(provider, fixture)
	if call2 != [5]int{1, 1, 1, 1, 0} {
		t.Fatalf("call2 cumulative=%v want 1/1/1/1/0 — call2 delta zero", call2)
	}
	result2 := evaluateEnriched(t, enriched2)
	stmt2 := result2.Statements[0]
	if len(stmt2.EvidenceGaps) != 1 ||
		!reflect.DeepEqual(stmt2.EvidenceGaps[0].RequiredFacts, []string{spec.SRSFactBatchEffects}) {
		t.Fatalf("call2 public gap=%+v want exactly [B]", stmt2.EvidenceGaps)
	}
	reserved := 0
	for _, f := range stmt2.Findings {
		if f.RuleID == "ddl.mysql.srs.drop.reserved" {
			reserved++
		}
	}
	if reserved != 1 {
		t.Fatalf("call2 reserved=%d want 1 — reserved co-fires on B", reserved)
	}
	if stmt2.Coverage.Status != report.CoverageUnverified {
		t.Fatalf("call2 coverage=%v want unverified", stmt2.Coverage.Status)
	}
}

// B priority: contamination outranks S0/S1 in the first-match order.
func TestT26SRSR1BlockedOutranksRefusals(t *testing.T) {
	_, provider := newSRSSQLHarness(t, &srsSQLFixture{
		version: "8.0.36",
		srs:     map[uint32]string{0: "0"},
		deps:    []srsDepRow{{schema: "s", table: "t", column: "g", srid: 0}},
	}, nil)
	state := newSRSState(provider)
	state.contaminated = true
	enriched := enrichSRS(t, state, provider, "DROP SPATIAL REFERENCE SYSTEM 0")
	if arm := srsArm(t, enriched[0]); arm != spec.SRSDropArmBlocked {
		t.Fatalf("arm=%v want blocked — B outranks S0", arm)
	}
	if len(provider.forwards) != 0 {
		t.Fatalf("blocked bind consumed the resolver: %+v", provider.forwards)
	}
}

// B sources stay distinguishable: stale-only vs contamination-only.
func TestT26SRSR1BlockedSourceDistinction(t *testing.T) {
	// stale-only: uncontaminated batch, minted per-SRID flag.
	_, provider := newSRSSQLHarness(t, &srsSQLFixture{version: "8.0.36"}, nil)
	state := newSRSState(provider)
	state.srsExistenceStale[4326] = struct{}{}
	enriched := enrichSRS(t, state, provider, "DROP SPATIAL REFERENCE SYSTEM 4326")
	if arm := srsArm(t, enriched[0]); arm != spec.SRSDropArmBlocked {
		t.Fatalf("stale-only arm=%v want blocked", arm)
	}
	if state.contaminated {
		t.Fatal("stale-only B must not mint contamination")
	}
	if len(provider.forwards) != 0 {
		t.Fatalf("stale-only consumed resolver: %+v", provider.forwards)
	}

	// contamination-only: no per-SRID flag at all.
	_, provider = newSRSSQLHarness(t, &srsSQLFixture{version: "8.0.36"}, nil)
	state = newSRSState(provider)
	state.contaminated = true
	enriched = enrichSRS(t, state, provider, "DROP SPATIAL REFERENCE SYSTEM 4326")
	if arm := srsArm(t, enriched[0]); arm != spec.SRSDropArmBlocked {
		t.Fatalf("contamination arm=%v want blocked", arm)
	}
	if len(state.srsExistenceStale) != 0 {
		t.Fatalf("contamination-only B must not mint existence stale: %+v", state.srsExistenceStale)
	}
}

// --- E10: version gate ---------------------------------------------------------

func TestT26SRSR1VersionGate(t *testing.T) {
	gate := func(version *spec.VersionIdentity, want bool) {
		if got := spec.MySQLSRSDropAuditedDomain(version); got != want {
			t.Fatalf("domain(%+v)=%v want %v", version, got, want)
		}
	}
	gate(nil, false)
	gate(&spec.VersionIdentity{Product: spec.VersionProductMySQL, Version: "8.0.3", Major: 8, Minor: 0, Patch: 3, ValidatedRange: true, Source: spec.VersionSourceObserved}, false)
	gate(&spec.VersionIdentity{Product: spec.VersionProductMySQL, Version: "8.0.4", Major: 8, Minor: 0, Patch: 4, ValidatedRange: true, Source: spec.VersionSourceObserved}, true)
	gate(&spec.VersionIdentity{Product: spec.VersionProductMySQL, Version: "5.7.44", Major: 5, Minor: 7, Patch: 44, ValidatedRange: true, Source: spec.VersionSourceObserved}, false)
	gate(&spec.VersionIdentity{Product: "tidb", Version: "8.5.0", Major: 8, Minor: 5, Patch: 0, ValidatedRange: true, Source: spec.VersionSourceObserved}, false)
	gate(&spec.VersionIdentity{Product: spec.VersionProductMySQL, Version: "8.0.36", Major: 8, Minor: 0, Patch: 36, ValidatedRange: false, Source: spec.VersionSourceObserved}, false)
	// ER1: a caller pin (Source=target) is never an observation — even
	// a valid in-domain pin fails the domain check without a live banner.
	gate(&spec.VersionIdentity{Product: spec.VersionProductMySQL, Version: "8.0.46", Major: 8, Minor: 0, Patch: 46, ValidatedRange: true, Source: spec.VersionSourceTarget}, false)

	// E10.3c/E10.4: providerless pin → gate boundary (ER1: the pin is a
	// conflict check only; with no observation the identity is unproven).
	for _, pin := range []string{"8.0.3", "5.7.44"} {
		result, err := AuditSQL(context.Background(), Request{
			SQL: "DROP SPATIAL REFERENCE SYSTEM 100001", Dialect: spec.DialectMySQL, TargetVersion: pin,
		})
		if err == nil {
			t.Fatalf("pin %s: want unsupported statement error", pin)
		}
		if len(result.Unsupported) != 1 {
			t.Fatalf("pin %s unsupported=%+v", pin, result.Unsupported)
		}
		u := result.Unsupported[0]
		if u.Feature != "drop_spatial_reference_system" || u.Reason != spec.UnsupportedUnauditedReason ||
			u.Metadata["boundary"] != "unaudited" || u.Metadata["object"] != "spatial_reference_system" || u.Metadata["gate"] != "version" {
			t.Fatalf("pin %s unsupported=%+v want the frozen gate entry", pin, u)
		}
		if result.Verdict != report.VerdictReview {
			t.Fatalf("pin %s verdict=%v want review", pin, result.Verdict)
		}
	}

	// E10.3d (ER1): even a valid in-domain pin cannot substitute for an
	// observation — providerless 8.0.4 stays boundary, version_unproven.
	result, err := AuditSQL(context.Background(), Request{
		SQL: "DROP SPATIAL REFERENCE SYSTEM 4326", Dialect: spec.DialectMySQL, TargetVersion: "8.0.4",
	})
	if err == nil || len(result.Unsupported) != 1 {
		t.Fatalf("8.0.4 pin err=%v unsupported=%+v want the version boundary", err, result.Unsupported)
	}
	if got := result.Unsupported[0].Metadata["version_reason"]; got != spec.MetadataUnreadVersionUnproven {
		t.Fatalf("8.0.4 pin version_reason=%v want %v", got, spec.MetadataUnreadVersionUnproven)
	}

	// E10.2-shape: providerless + no pin → gate (resolved version
	// unproven is a boundary, not feature absence).
	result, err = AuditSQL(context.Background(), Request{
		SQL: "DROP SPATIAL REFERENCE SYSTEM 100001", Dialect: spec.DialectMySQL,
	})
	if err == nil || len(result.Unsupported) != 1 || result.Unsupported[0].Metadata["gate"] != "version" {
		t.Fatalf("no-pin result err=%v unsupported=%+v want the version gate", err, result.Unsupported)
	}
}

// --- E11: error paths ----------------------------------------------------------

func TestT26SRSR1ErrorPaths(t *testing.T) {
	// E11.1: resolver forward sentinel propagates as metadataObjectReadError.
	sentinel := errors.New("srs-sentinel-7")
	fixture, provider := newSRSSQLHarness(t, &srsSQLFixture{version: "8.0.36"}, nil)
	provider.resolveErr = sentinel
	state := newSRSState(provider)
	statements, failures := parseT26(t, "DROP SPATIAL REFERENCE SYSTEM 4326")
	_, err := state.enrichStatements(context.Background(),
		&MetadataRequest{Schema: "golden", Provider: provider},
		statements, failures, state.instanceFacts, auditLimits{orderedStatements: defaultOrderedStatementLimit})
	if err == nil || !errors.Is(err, sentinel) {
		t.Fatalf("enrich err=%v want errors.Is(srs-sentinel-7)", err)
	}
	if len(provider.forwards) != 1 {
		t.Fatalf("R=%d want 1(att)", len(provider.forwards))
	}
	if srsQueryCount(fixture, "st_spatial_reference_systems") != 0 {
		t.Fatal("resolver failure must issue no later queries")
	}

	// E11.4: select version() inside the SRS loader fails first.
	sentinelV := errors.New("srs-ver-fail-4")
	fixture, provider = newSRSSQLHarness(t, &srsSQLFixture{
		version: "8.0.36", failOn: map[string]error{"version()": sentinelV},
	}, nil)
	state = newSRSState(provider)
	statements, _ = parseT26(t, "DROP SPATIAL REFERENCE SYSTEM 4326")
	_, err = state.enrichStatements(context.Background(),
		&MetadataRequest{Schema: "golden", Provider: provider},
		statements, failures, state.instanceFacts, auditLimits{orderedStatements: defaultOrderedStatementLimit})
	if err == nil || !errors.Is(err, sentinelV) {
		t.Fatalf("enrich err=%v want errors.Is(srs-ver-fail-4)", err)
	}
	if srsQueryCount(fixture, "st_spatial_reference_systems") != 0 || srsQueryCount(fixture, "st_geometry_columns") != 0 {
		t.Fatal("version failure must stop before any catalog read")
	}

	// E11.2: existence query fails — deps never issued.
	sentinelE := errors.New("srs-exist-fail-2")
	fixture, provider = newSRSSQLHarness(t, &srsSQLFixture{
		version: "8.0.36", failOn: map[string]error{"st_spatial_reference_systems": sentinelE},
	}, nil)
	state = newSRSState(provider)
	_, err = state.enrichStatements(context.Background(),
		&MetadataRequest{Schema: "golden", Provider: provider},
		statements, failures, state.instanceFacts, auditLimits{orderedStatements: defaultOrderedStatementLimit})
	if err == nil || !errors.Is(err, sentinelE) {
		t.Fatalf("enrich err=%v want errors.Is(srs-exist-fail-2)", err)
	}
	if srsQueryCount(fixture, "st_geometry_columns") != 0 {
		t.Fatal("existence failure must stop before the deps read")
	}

	// E11.3: deps query fails after a clean existence answer.
	sentinelD := errors.New("srs-deps-fail-3")
	fixture, provider = newSRSSQLHarness(t, &srsSQLFixture{
		version: "8.0.36", failOn: map[string]error{"st_geometry_columns": sentinelD},
	}, nil)
	state = newSRSState(provider)
	_, err = state.enrichStatements(context.Background(),
		&MetadataRequest{Schema: "golden", Provider: provider},
		statements, failures, state.instanceFacts, auditLimits{orderedStatements: defaultOrderedStatementLimit})
	if err == nil || !errors.Is(err, sentinelD) {
		t.Fatalf("enrich err=%v want errors.Is(srs-deps-fail-3)", err)
	}
}

// E11.5: request discovery error propagates %w — zero result + error,
// enrichment never entered.
func TestT26SRSR1DiscoveryError(t *testing.T) {
	sentinel := errors.New("srs-discover-fail-5")
	_, provider := newSRSSQLHarness(t, &srsSQLFixture{version: "8.0.36"}, nil)
	provider.discoverErr = sentinel
	result, err := AuditSQL(context.Background(), Request{
		SQL: "DROP SPATIAL REFERENCE SYSTEM 4326", Dialect: spec.DialectMySQL,
		MetadataProvider: provider,
	})
	if err == nil || !errors.Is(err, sentinel) {
		t.Fatalf("err=%v want errors.Is(srs-discover-fail-5)", err)
	}
	if len(result.Statements) != 0 {
		t.Fatalf("result=%+v want zero statements — discovery precedes enrichment", result.Statements)
	}
}

// --- E12: public shape + fidelity ---------------------------------------------

func TestT26SRSR1PublicShape(t *testing.T) {
	// E12.1: raw and normalized forms carry the fixed redacted skeleton.
	fixture, provider := newSRSSQLHarness(t, &srsSQLFixture{
		version: "8.0.36",
		srs:     map[uint32]string{4326: "EPSG 4326"},
		deps:    []srsDepRow{{schema: "s", table: "t", column: "g", srid: 4326}},
	}, nil)
	enriched := enrichSRS(t, newSRSState(provider), provider, "DROP SPATIAL REFERENCE SYSTEM 4326")
	result := evaluateEnriched(t, enriched)
	stmt := result.Statements[0]
	if stmt.RawSQL != "DROP SPATIAL REFERENCE SYSTEM 4326" || stmt.NormalizedSQL != "DROP SPATIAL REFERENCE SYSTEM 4326" {
		t.Fatalf("raw=%q normalized=%q", stmt.RawSQL, stmt.NormalizedSQL)
	}
	// E12.2 shape: secret identities never reach the serialized surface —
	// finding metadata carries only the bounded fields.
	f := stmt.Findings[0]
	for _, key := range []string{"schema", "table", "column", "arm", "gap"} {
		if _, bad := f.Metadata[key]; bad {
			t.Fatalf("metadata must never carry %q: %+v", key, f.Metadata)
		}
	}
	assertReads(t, provider, fixture, 1, 1, 1, 1, 0)
}

// E12.3: B preserves the entry state exactly — full fidelity snapshot
// through the own apply.
func TestT26SRSR1BlockedPreservesState(t *testing.T) {
	_, provider := newSRSSQLHarness(t, &srsSQLFixture{version: "8.0.36"}, tblTFixture())
	state := newSRSState(provider)
	state.srsExistenceStale[4326] = struct{}{}
	enrichSRS(t, state, provider, "DROP TABLE s.t")
	if len(state.srsExistenceStale) != 1 {
		t.Fatalf("existenceStale=%+v want the seeded entry only", state.srsExistenceStale)
	}
	beforeStale := map[uint32]bool{}
	for srid := range state.srsExistenceStale {
		beforeStale[srid] = true
	}
	beforeDeps := state.srsDepsStale
	enriched := enrichSRS(t, state, provider, "DROP SPATIAL REFERENCE SYSTEM 4326")
	if arm := srsArm(t, enriched[0]); arm != spec.SRSDropArmBlocked {
		t.Fatalf("arm=%v want blocked", arm)
	}
	if len(state.srsExistenceStale) != len(beforeStale) || state.srsDepsStale != beforeDeps {
		t.Fatal("B own apply must preserve the entry state exactly")
	}
}

// E12.4: the touching write mints before the SRS statement binds —
// observation point is the table statement's own apply.
func TestT26SRSR1TouchingWriteOwnApply(t *testing.T) {
	_, provider := newSRSSQLHarness(t, &srsSQLFixture{
		version: "8.0.36",
		srs:     map[uint32]string{4326: "EPSG 4326"},
	}, tblTFixture())
	state := newSRSState(provider)
	var depsAfterStmt1 bool
	state.afterApplyObserve = func(i int, stmt *spec.Statement) {
		if i == 0 {
			depsAfterStmt1 = state.srsDepsStale
		}
	}
	enrichSRS(t, state, provider, "DROP TABLE s.t; DROP SPATIAL REFERENCE SYSTEM 4326")
	if !depsAfterStmt1 {
		t.Fatal("DROP TABLE t's own apply must mint srsDepsStale before stmt2 binds")
	}
}

// E12.5: the SERVER and SRS families share the resolver seam but never
// each other's premise state — an SRS lifecycle mint shadows nothing in
// the SERVER channel and vice versa.
func TestT26SRSR1SiblingFamilyNoCrosstalk(t *testing.T) {
	fixture, provider := newSRSSQLHarness(t, &srsSQLFixture{
		version: "8.0.36",
		srs:     map[uint32]string{4326: "EPSG 4326"},
		servers: map[string]bool{"s1": true},
	}, nil)
	state := newSRSState(provider)
	enriched := enrichSRS(t, state, provider,
		"DROP SPATIAL REFERENCE SYSTEM 4326; DROP SERVER s1")
	if arm := srsArm(t, enriched[0]); arm != spec.SRSDropArmUnproven {
		t.Fatalf("stmt1 arm=%v want unproven", arm)
	}
	// The SRS S2 mint is scoped to srsExistenceStale — the SERVER
	// statement's own ordered channel stays unshadowed.
	server := enriched[1].Metadata.Server
	if server == nil || server.UnreadReason == spec.MetadataUnreadBatchEffects {
		t.Fatalf("server state=%+v — SRS staleness must never shadow the server channel", server)
	}
	if server.OrderedAbsent || server.OrderedPresent {
		t.Fatalf("server ordered state=%+v want neither — the SRS write is not a server-channel entry", server)
	}

	// Reverse order: the SERVER audited drop mints the server
	// channel's own unproven barrier — that barrier is a SERVER
	// premise and never enters srsExistenceStale, so the SRS arm
	// stays S2, not B.
	enriched = enrichSRS(t, newSRSState(provider), provider,
		"DROP SERVER s1; DROP SPATIAL REFERENCE SYSTEM 4326")
	if st := enriched[0].Metadata.Server; st == nil {
		t.Fatalf("server state nil: %+v", enriched[0].Metadata)
	}
	if arm := srsArm(t, enriched[1]); arm != spec.SRSDropArmUnproven {
		t.Fatalf("stmt2 arm=%v want unproven — the server barrier never shadows the SRS premise", arm)
	}
	_ = fixture
}

// --- adjudication-unit layer -------------------------------------------------

func TestT26SRSR1AdjudicationUnit(t *testing.T) {
	// B: batch-effects marker → exact owed.
	adj := spec.AdjudicateSRSDrop(&spec.SpatialReferenceSystemState{SRID: 1, UnreadReason: spec.MetadataUnreadBatchEffects}, 1, false, spec.SRSDropPrivilegeUnproven, false)
	if adj.Arm != spec.SRSDropArmBlocked || len(adj.Owed) != 1 || adj.Owed[0] != spec.SRSFactBatchEffects {
		t.Fatalf("B adj=%+v", adj)
	}
	// S0: SRID 0 even with a positive witness — B did not match.
	adj = spec.AdjudicateSRSDrop(&spec.SpatialReferenceSystemState{
		SRID: 0, Read: true, Exists: true,
		Dependencies: &spec.SRSDependencyFact{Columns: []spec.SRSDependentColumn{{SRID: 0}}},
	}, 0, false, spec.SRSDropPrivilegeUnproven, false)
	if adj.Arm != spec.SRSDropArmRefusedSRID0 || len(adj.Owed) != 0 {
		t.Fatalf("S0 adj=%+v", adj)
	}
	// S1: valid positive witness — zero-row existence allowed.
	adj = spec.AdjudicateSRSDrop(&spec.SpatialReferenceSystemState{
		SRID: 4326, Read: true, Exists: false,
		Dependencies: &spec.SRSDependencyFact{Columns: []spec.SRSDependentColumn{{Schema: "s", Table: "t", Column: "g", SRID: 4326}}},
	}, 4326, false, spec.SRSDropPrivilegeUnproven, false)
	if adj.Arm != spec.SRSDropArmRefusedDependency {
		t.Fatalf("S1 adj=%+v", adj)
	}
	// Wrong-SRID rows never witness.
	adj = spec.AdjudicateSRSDrop(&spec.SpatialReferenceSystemState{
		SRID: 4326, Read: true, Exists: true,
		Dependencies: &spec.SRSDependencyFact{Columns: []spec.SRSDependentColumn{{Schema: "s", Table: "t", Column: "g", SRID: 999}}},
	}, 4326, false, spec.SRSDropPrivilegeUnproven, false)
	if adj.Arm != spec.SRSDropArmUnproven {
		t.Fatalf("wrong-SRID witness adj=%+v want unproven", adj)
	}
	// Missing read source (nil Dependencies) never witnesses.
	adj = spec.AdjudicateSRSDrop(&spec.SpatialReferenceSystemState{SRID: 4326, Read: true, Exists: true}, 4326, false, spec.SRSDropPrivilegeUnproven, false)
	if adj.Arm != spec.SRSDropArmUnproven || len(adj.Owed) != 2 {
		t.Fatalf("nil-deps adj=%+v want unproven [D,P]", adj)
	}
	// Stale deps demote an otherwise valid witness; existence stays
	// consumed → [D,P] exactly.
	adj = spec.AdjudicateSRSDrop(&spec.SpatialReferenceSystemState{
		SRID: 4326, Read: true, Exists: true,
		Dependencies: &spec.SRSDependencyFact{Columns: []spec.SRSDependentColumn{{Schema: "s", Table: "t", Column: "g", SRID: 4326}}},
	}, 4326, true, spec.SRSDropPrivilegeUnproven, false)
	if adj.Arm != spec.SRSDropArmUnproven || len(adj.Owed) != 2 || adj.Owed[0] != spec.SRSFactColumnDependencies {
		t.Fatalf("stale-deps adj=%+v want unproven [D,P]", adj)
	}
	// Unproven existence → [E,D,P] ordered.
	adj = spec.AdjudicateSRSDrop(nil, 100001, false, spec.SRSDropPrivilegeUnproven, false)
	if adj.Arm != spec.SRSDropArmUnproven || len(adj.Owed) != 3 ||
		adj.Owed[0] != spec.SRSFactExistence || adj.Owed[1] != spec.SRSFactColumnDependencies || adj.Owed[2] != spec.SRSFactDropPrivilege {
		t.Fatalf("nil-state adj=%+v want unproven [E,D,P]", adj)
	}
}
