// Package auditmeta verifies the T22 chain end to end: the real
// mysqlmeta.Provider reads information_schema.RESOURCE_GROUPS through its
// ResolveObject loader plus the tidb_enable_resource_control instance
// fact, and both facts reach AuditSQL's ordered pre-state — a proven
// catalog BURSTABLE feeds the burst warning, the known-OFF gate refuses
// before existence adjudication, zero rows owe the bounded state gap,
// and a catalog error propagates through the public path. This is "real
// provider + constructed DB rows", not a live anchor.
package auditmeta

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
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
	mysqlmeta "github.com/Fanduzi/DeltaScope/internal/infrastructure/metadata/mysql"
)

const (
	t22ValidRule     = "ddl.tidb.resource_group.valid"
	t22BurstableRule = "ddl.tidb.resource_group.burstable.warn"
	t22StateGap      = "unknown_resource_group_state"
)

var (
	t22Once  sync.Once
	t22State sync.Map
)

type t22Driver struct{}
type t22Conn struct{ queries map[string]t22Result }
type t22Rows struct {
	cols  []string
	rows  [][]driver.Value
	index int
}
type t22Result struct {
	cols []string
	rows [][]driver.Value
	err  error
}

var t22ResourceGroupColumns = []string{
	"NAME", "RU_PER_SEC", "PRIORITY", "BURSTABLE", "QUERY_LIMIT",
}

func openT22DB(t *testing.T, queries map[string]t22Result) *sql.DB {
	t.Helper()
	t22Once.Do(func() { sql.Register("t22-mysqlmeta", t22Driver{}) })
	name := "t22-" + strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	t22State.Store(name, queries)
	db, err := sql.Open("t22-mysqlmeta", name)
	if err != nil {
		t.Fatalf("open driver db: %v", err)
	}
	t.Cleanup(func() {
		db.Close()
		t22State.Delete(name)
	})
	return db
}

func (t22Driver) Open(name string) (driver.Conn, error) {
	value, _ := t22State.Load(name)
	queries, _ := value.(map[string]t22Result)
	return t22Conn{queries: queries}, nil
}

func (t22Conn) Prepare(string) (driver.Stmt, error) { return nil, driver.ErrSkip }
func (t22Conn) Close() error                        { return nil }
func (t22Conn) Begin() (driver.Tx, error)           { return nil, driver.ErrSkip }

func (c t22Conn) QueryContext(_ context.Context, query string, _ []driver.NamedValue) (driver.Rows, error) {
	var result *t22Result
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
	return &t22Rows{cols: result.cols, rows: result.rows}, nil
}

func (r *t22Rows) Columns() []string { return r.cols }
func (r *t22Rows) Close() error      { return nil }
func (r *t22Rows) Next(dest []driver.Value) error {
	if r.index >= len(r.rows) {
		return io.EOF
	}
	copy(dest, r.rows[r.index])
	r.index++
	return nil
}

func t22Queries(gate string, rows [][]driver.Value) map[string]t22Result {
	return map[string]t22Result{
		"show variables": {
			cols: []string{"Variable_name", "Value"},
			rows: [][]driver.Value{
				{"version", "8.0.11-TiDB-v8.5.0"},
				{"tidb_enable_resource_control", gate},
			},
		},
		"show config": {
			cols: []string{"Type", "Instance", "Name", "Value"},
		},
		"information_schema.resource_groups": {
			cols: t22ResourceGroupColumns,
			rows: rows,
		},
	}
}

// The proven catalog BURSTABLE flag travels the official reader into the
// ordered pre-state: an ALTER declaring no burst member still sees the
// catalog's enabled mode and warns — only a real definition read can
// feed that premise.
func TestT22OfficialReaderFeedsBurstablePremise(t *testing.T) {
	db := openT22DB(t, t22Queries("ON", [][]driver.Value{{
		"rg", "500", "HIGH", "MODERATED", nil,
	}}))
	provider := mysqlmeta.NewProvider(db)
	policy := t06a4Policy(t, map[string]string{t22ValidRule: "", t22BurstableRule: ""})
	result, err := appaudit.AuditSQL(context.Background(), appaudit.Request{
		SQL:              "ALTER RESOURCE GROUP rg RU_PER_SEC=800",
		Dialect:          spec.DialectTiDB,
		ConfigPath:       policy,
		MetadataProvider: provider,
	})
	if err != nil {
		t.Fatalf("AuditSQL: %v", err)
	}
	found := false
	for _, finding := range result.Statements[0].Findings {
		if finding.RuleID == t22BurstableRule {
			found = true
		}
		if finding.RuleID == t22ValidRule {
			t.Fatalf("unexpected valid blocker on confirmed group: %#v", finding)
		}
	}
	if !found {
		t.Fatalf("no burstable warning — the catalog mode did not reach the rule: %+v", result.Statements[0].Findings)
	}
}

// The resource-control gate is a proven-refusal premise: a known-OFF
// observation blocks the lifecycle statement before existence
// adjudication — even on a confirmed-present group.
func TestT22CapabilityOffRefusesBeforeExistence(t *testing.T) {
	db := openT22DB(t, t22Queries("OFF", [][]driver.Value{{
		"rg", "500", "HIGH", nil, nil,
	}}))
	provider := mysqlmeta.NewProvider(db)
	policy := t06a4Policy(t, map[string]string{t22ValidRule: ""})
	result, err := appaudit.AuditSQL(context.Background(), appaudit.Request{
		SQL:              "DROP RESOURCE GROUP rg",
		Dialect:          spec.DialectTiDB,
		ConfigPath:       policy,
		MetadataProvider: provider,
	})
	if err != nil {
		t.Fatalf("AuditSQL: %v", err)
	}
	found := false
	for _, finding := range result.Statements[0].Findings {
		if finding.RuleID == t22ValidRule && strings.Contains(finding.Message, "tidb_enable_resource_control") {
			found = true
		}
	}
	if !found {
		t.Fatalf("findings = %+v, want the capability refusal", result.Statements[0].Findings)
	}
}

// Zero catalog rows stay unknown through the public path: the existence
// premise reports the bounded state gap — never a proven absence, never
// a blocker on an unprovable target.
func TestT22OfficialReaderZeroRowsStayUnknown(t *testing.T) {
	db := openT22DB(t, t22Queries("ON", nil))
	provider := mysqlmeta.NewProvider(db)
	policy := t06a4Policy(t, map[string]string{t22ValidRule: ""})
	result, err := appaudit.AuditSQL(context.Background(), appaudit.Request{
		SQL:              "ALTER RESOURCE GROUP ghost RU_PER_SEC=10",
		Dialect:          spec.DialectTiDB,
		ConfigPath:       policy,
		MetadataProvider: provider,
	})
	if err != nil {
		t.Fatalf("AuditSQL: %v", err)
	}
	gotStateGap := false
	for _, gap := range result.Statements[0].EvidenceGaps {
		if gap.RuleID == t22ValidRule && gap.ReasonCode == t22StateGap {
			gotStateGap = true
		}
	}
	if !gotStateGap {
		t.Fatalf("gaps = %#v, want %s on the unvouched catalog read", result.Statements[0].EvidenceGaps, t22StateGap)
	}
	for _, finding := range result.Statements[0].Findings {
		if finding.RuleID == t22ValidRule {
			t.Fatalf("unexpected blocker on an unproven target: %#v", finding)
		}
	}
}

// A failed catalog read is an error through the public path — never a
// degraded unavailable, never a cached absence.
func TestT22OfficialReaderErrorPropagates(t *testing.T) {
	want := errors.New("resource_groups read refused")
	queries := t22Queries("ON", nil)
	queries["information_schema.resource_groups"] = t22Result{cols: t22ResourceGroupColumns, err: want}
	db := openT22DB(t, queries)
	provider := mysqlmeta.NewProvider(db)
	policy := t06a4Policy(t, map[string]string{t22ValidRule: ""})
	_, err := appaudit.AuditSQL(context.Background(), appaudit.Request{
		SQL:              "ALTER RESOURCE GROUP rg RU_PER_SEC=10",
		Dialect:          spec.DialectTiDB,
		ConfigPath:       policy,
		MetadataProvider: provider,
	})
	if err == nil || !errors.Is(err, want) {
		t.Fatalf("err = %v, want propagated %v", err, want)
	}
}
