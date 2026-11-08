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

// The #101 first path through the official provider: capability ON and a
// complete successful zero-row catalog read. Fixed TiDB v8.5.0 emits
// every resource group with no row-level privilege filter, so zero rows
// prove absence — CREATE publishes the conditional definition (RU=100),
// the ordered state feeds ALTER, and both statements complete with the
// lifecycle notices firing and no gaps or blockers.
func TestT22GoldenPathThroughOfficialReader(t *testing.T) {
	db := openT22DB(t, t22Queries("ON", nil))
	provider := mysqlmeta.NewProvider(db)
	policy := t06a4Policy(t, map[string]string{
		t22ValidRule: "", t22BurstableRule: "",
		"ddl.create_resource_group.notice": "", "ddl.alter_resource_group.notice": "",
	})
	result, err := appaudit.AuditSQL(context.Background(), appaudit.Request{
		SQL:              "CREATE RESOURCE GROUP golden_rg RU_PER_SEC = 100;\nALTER RESOURCE GROUP golden_rg RU_PER_SEC = 200",
		Dialect:          spec.DialectTiDB,
		ConfigPath:       policy,
		MetadataProvider: provider,
	})
	if err != nil {
		t.Fatalf("AuditSQL: %v", err)
	}
	create, alter := result.Statements[0], result.Statements[1]
	if create.Kind != "ddl" || alter.Kind != "ddl" {
		t.Fatalf("kinds = %s/%s, want ddl/ddl", create.Kind, alter.Kind)
	}
	if create.Coverage.Status != "complete" || alter.Coverage.Status != "complete" {
		t.Fatalf("coverage = %s/%s, want complete/complete — zero-row read proves absence", create.Coverage.Status, alter.Coverage.Status)
	}
	for i, stmt := range result.Statements {
		if len(stmt.EvidenceGaps) != 0 {
			t.Fatalf("statement %d gaps = %+v, want none — every required premise is proven", i, stmt.EvidenceGaps)
		}
	}
	var createNotice, alterNotice bool
	for i, stmt := range result.Statements {
		for _, finding := range stmt.Findings {
			if finding.RuleID == t22ValidRule {
				t.Fatalf("statement %d valid finding = %#v, want none", i, finding)
			}
			if finding.RuleID == "ddl.create_resource_group.notice" {
				createNotice = true
			}
			if finding.RuleID == "ddl.alter_resource_group.notice" {
				alterNotice = true
			}
		}
	}
	if !createNotice || !alterNotice {
		t.Fatalf("notices = %v/%v, want create+alter lifecycle notices on the proven first path", createNotice, alterNotice)
	}
}

// The proven catalog BURSTABLE flag travels the official reader into the
// ordered pre-state: an ALTER declaring no burst member still sees the
// catalog's enabled mode and warns — only a real definition read can
// feed that premise. The TiDB 8.5 catalog emits YES/NO only.
func TestT22OfficialReaderFeedsBurstablePremise(t *testing.T) {
	db := openT22DB(t, t22Queries("ON", [][]driver.Value{{
		"rg", "500", "HIGH", "YES", nil,
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

// A complete successful zero-row catalog read proves absence through the
// public path: an unguarded ALTER on the proven-absent group is a refusal
// — the valid rule blocks, no state gap remains, and coverage completes.
// (Error and unavailable reads stay unknown — TestT22OfficialReaderErrorPropagates
// covers the error arm.)
func TestT22OfficialReaderZeroRowsProveAbsent(t *testing.T) {
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
	var blocker bool
	for _, finding := range result.Statements[0].Findings {
		if finding.RuleID == t22ValidRule {
			blocker = true
		}
	}
	if !blocker {
		t.Fatalf("findings = %#v, want the absent-target refusal on the proven-absent read", result.Statements[0].Findings)
	}
	for _, gap := range result.Statements[0].EvidenceGaps {
		if gap.RuleID == t22ValidRule && gap.ReasonCode == t22StateGap {
			t.Fatalf("state gap = %#v on a proven-absent read — existence is adjudicated", gap)
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
