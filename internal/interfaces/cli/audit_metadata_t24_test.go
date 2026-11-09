package cli

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"

	appaudit "github.com/Fanduzi/DeltaScope/internal/application/audit"
	auditmeta "github.com/Fanduzi/DeltaScope/internal/application/auditmeta"
	"github.com/Fanduzi/DeltaScope/internal/domain/report"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// T24-R13 D1 regression: the CLI metadata wrapper must forward the optional
// LoadMySQLResourceGroupFacts channel — the native run proved that without
// it the whole MySQL resource-group fact preload is silently skipped on the
// real CLI path (uuid never observed, online evidence never binds).

type t24FakeClient struct{ withRG bool }

func (t24FakeClient) LoadInstanceFacts(context.Context, spec.Dialect, string) (*spec.InstanceFacts, error) {
	return &spec.InstanceFacts{}, nil
}
func (t24FakeClient) LoadTableSnapshot(context.Context, spec.Dialect, string, string) (*spec.TableSnapshot, error) {
	return nil, nil
}
func (t24FakeClient) DetectDialect(context.Context) (spec.Dialect, error) {
	return spec.DialectMySQL, nil
}
func (t24FakeClient) FindSchemasForTable(context.Context, string) ([]string, error) {
	return nil, nil
}
func (t24FakeClient) Close() error { return nil }

func (c t24FakeClient) LoadMySQLResourceGroupFacts(ctx context.Context, dialect spec.Dialect) (*spec.InstanceFacts, error) {
	if !c.withRG {
		return nil, nil
	}
	return &spec.InstanceFacts{MySQLServerUUID: "srv-uuid-cli", MySQLServerUUIDKnown: true}, nil
}

// t24NoRGClient deliberately lacks the optional channel — a plain struct
// so the interface assertion inside the forwarder actually fails.
type t24NoRGClient struct{}

func (t24NoRGClient) LoadInstanceFacts(context.Context, spec.Dialect, string) (*spec.InstanceFacts, error) {
	return &spec.InstanceFacts{}, nil
}
func (t24NoRGClient) LoadTableSnapshot(context.Context, spec.Dialect, string, string) (*spec.TableSnapshot, error) {
	return nil, nil
}
func (t24NoRGClient) DetectDialect(context.Context) (spec.Dialect, error) {
	return spec.DialectMySQL, nil
}
func (t24NoRGClient) FindSchemasForTable(context.Context, string) ([]string, error) {
	return nil, nil
}
func (t24NoRGClient) Close() error { return nil }

func TestCLIMetadataProviderForwardsResourceGroupFacts(t *testing.T) {
	wrapped := cliMetadataProvider{client: t24FakeClient{withRG: true}}
	facts, err := wrapped.LoadMySQLResourceGroupFacts(context.Background(), spec.DialectMySQL)
	if err != nil {
		t.Fatalf("forward errored: %v", err)
	}
	if facts == nil || !facts.MySQLServerUUIDKnown || facts.MySQLServerUUID != "srv-uuid-cli" {
		t.Fatalf("forwarded facts lost: %+v", facts)
	}
}

func TestCLIMetadataProviderWithoutResourceGroupChannel(t *testing.T) {
	// A client lacking the optional channel degrades cleanly, never errors.
	wrapped := cliMetadataProvider{client: t24NoRGClient{}}
	facts, err := wrapped.LoadMySQLResourceGroupFacts(context.Background(), spec.DialectMySQL)
	if err != nil {
		t.Fatalf("absent channel errored: %v", err)
	}
	if facts != nil {
		t.Fatalf("absent channel must return nil facts, got %+v", facts)
	}
}

// ── T24-R15 D1: the real production composition end to end ──
// cliMetadataProvider{auditmeta.NewMySQLClient(stub db)} → public
// AuditSQL: the same wrapper order the CLI builds at audit.go — a stub
// database/sql driver answers every metadata read, so UUID and the
// resource-group premise facts must travel the full chain for a
// Target-bound evidence bundle to bind.
var (
	t24CliOnce  sync.Once
	t24CliState sync.Map
)

type t24CliResult struct {
	cols []string
	rows [][]driver.Value
	err  error
}
type t24CliDriver struct{}
type t24CliConn struct{ queries map[string]t24CliResult }
type t24CliRows struct {
	cols  []string
	rows  [][]driver.Value
	index int
}

func openT24CliDB(t *testing.T, queries map[string]t24CliResult) *sql.DB {
	t.Helper()
	t24CliOnce.Do(func() { sql.Register("t24cli-mysqlmeta", t24CliDriver{}) })
	name := "t24cli-" + strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	t24CliState.Store(name, queries)
	db, err := sql.Open("t24cli-mysqlmeta", name)
	if err != nil {
		t.Fatalf("open driver db: %v", err)
	}
	t.Cleanup(func() {
		db.Close()
		t24CliState.Delete(name)
	})
	return db
}

func (t24CliDriver) Open(name string) (driver.Conn, error) {
	value, _ := t24CliState.Load(name)
	queries, _ := value.(map[string]t24CliResult)
	return t24CliConn{queries: queries}, nil
}

func (t24CliConn) Prepare(string) (driver.Stmt, error) { return nil, driver.ErrSkip }
func (t24CliConn) Close() error                        { return nil }
func (t24CliConn) Begin() (driver.Tx, error)           { return nil, driver.ErrSkip }

func (c t24CliConn) QueryContext(_ context.Context, query string, _ []driver.NamedValue) (driver.Rows, error) {
	var result *t24CliResult
	longest := -1
	for fragment, candidate := range c.queries {
		if strings.Contains(query, fragment) && len(fragment) > longest {
			matched := candidate
			result = &matched
			longest = len(fragment)
		}
	}
	if result == nil {
		return nil, sql.ErrNoRows
	}
	if result.err != nil {
		return nil, result.err
	}
	return &t24CliRows{cols: result.cols, rows: result.rows}, nil
}

func (r *t24CliRows) Columns() []string { return r.cols }
func (r *t24CliRows) Close() error      { return nil }
func (r *t24CliRows) Next(dest []driver.Value) error {
	if r.index >= len(r.rows) {
		return io.EOF
	}
	copy(dest, r.rows[r.index])
	r.index++
	return nil
}

// t24CliCompleteQueries stubs a full premise bundle on a proven 8.0.46
// target: instance version, read-only OFF, RESOURCE_GROUP_ADMIN via
// base grants, the observed server uuid, a clean empty threads census
// with zero lost records, and the builtin feature row — while the
// resource_groups catalog itself answers zero rows (state unproven).
func t24CliCompleteQueries(uuid string) map[string]t24CliResult {
	return map[string]t24CliResult{
		"show variables": {
			cols: []string{"Variable_name", "Value"},
			rows: [][]driver.Value{{"version", "8.0.46"}},
		},
		"select version()": {cols: []string{"version()"}, rows: [][]driver.Value{{"8.0.46"}}},
		"show global variables": {
			cols: []string{"Variable_name", "Value"},
			rows: [][]driver.Value{{"read_only", "OFF"}, {"super_read_only", "OFF"}},
		},
		"select current_role()": {cols: []string{"current_role()"}, rows: [][]driver.Value{{"NONE"}}},
		"show grants for current_user()": {
			cols: []string{"Grants for u@%"},
			rows: [][]driver.Value{{"GRANT RESOURCE_GROUP_ADMIN ON *.* TO 'u'@'%'"}},
		},
		"@@server_uuid":               {cols: []string{"@@server_uuid"}, rows: [][]driver.Value{{uuid}}},
		"@@global.performance_schema": {cols: []string{"@@global.performance_schema"}, rows: [][]driver.Value{{"ON"}}},
		"performance_schema.threads":  {cols: []string{"resource_group"}},
		"performance_schema_thread_instances_lost": {
			cols: []string{"Variable_name", "Value"},
			rows: [][]driver.Value{{"Performance_schema_thread_instances_lost", "0"}},
		},
		"resource_group_name = 'usr_default'": {cols: []string{"resource_group_name"}, rows: [][]driver.Value{{"usr_default"}}},
	}
}

func t24CliGapCodes(stmt report.StatementResult) []string {
	var codes []string
	for _, gap := range stmt.EvidenceGaps {
		codes = append(codes, gap.ReasonCode)
	}
	return codes
}

// The full production composition: a Target-bound evidence bundle can
// only fill the host-VCPU premise when the observed @@server_uuid
// travelled cliMetadataProvider → mysqlClient → real Provider → public
// Audit — every layer must forward the optional channel for the bind
// to happen, and a foreign target must never bind.
func TestT24CLICompositionBindsEvidenceThroughRealChain(t *testing.T) {
	db := openT24CliDB(t, t24CliCompleteQueries("srv-uuid-observe"))
	client := auditmeta.NewMySQLClient(db)
	defer client.Close()
	provider := cliMetadataProvider{client: client}

	hostVCPUs := uint64(4)
	auditWith := func(target string) report.Result {
		result, err := appaudit.AuditSQL(context.Background(), appaudit.Request{
			SQL:              "CREATE RESOURCE GROUP rg TYPE=USER VCPU=0-3;",
			Dialect:          spec.DialectMySQL,
			MetadataProvider: provider,
			DeploymentEvidence: &spec.DeploymentEvidence{
				Source: "t24-r15-probe", Product: "mysql", Version: "8.0.46",
				Target: target, HostVCPUs: &hostVCPUs,
			},
		})
		if err != nil {
			t.Fatalf("AuditSQL(target=%q): %v", target, err)
		}
		return result
	}

	bound := auditWith("srv-uuid-observe")
	gaps := t24CliGapCodes(bound.Statements[0])
	for _, gap := range gaps {
		if gap == "unknown_host_vcpu_count" {
			t.Fatalf("bound gaps = %v, want the evidence-bound host fact applied through the real chain", gaps)
		}
	}
	// The catalog answers zero rows — existence stays honestly unproven.
	foundState := false
	for _, gap := range gaps {
		if gap == "unknown_resource_group_state" {
			foundState = true
		}
	}
	if !foundState {
		t.Fatalf("bound gaps = %v, want the honest state gap kept", gaps)
	}

	foreign := auditWith("srv-uuid-foreign")
	foundHost := false
	for _, gap := range t24CliGapCodes(foreign.Statements[0]) {
		if gap == "unknown_host_vcpu_count" {
			foundHost = true
		}
	}
	if !foundHost {
		t.Fatal("a foreign-target bundle must not fill the host premise")
	}
}

// A failed metadata read propagates through the whole composition —
// the wrappers never launder a query error into a quiet answer.
func TestT24CLICompositionPropagatesErrors(t *testing.T) {
	denied := errors.New("readonly vars denied")
	queries := t24CliCompleteQueries("srv-uuid-observe")
	queries["show global variables"] = t24CliResult{err: denied}
	db := openT24CliDB(t, queries)
	client := auditmeta.NewMySQLClient(db)
	defer client.Close()
	provider := cliMetadataProvider{client: client}

	_, err := appaudit.AuditSQL(context.Background(), appaudit.Request{
		SQL:              "DROP RESOURCE GROUP rg;",
		Dialect:          spec.DialectMySQL,
		MetadataProvider: provider,
	})
	if err == nil {
		t.Fatal("want the metadata read failure propagated through the composition")
	}
	if !strings.Contains(err.Error(), denied.Error()) {
		t.Fatalf("err = %v, want the original read failure", err)
	}
}
