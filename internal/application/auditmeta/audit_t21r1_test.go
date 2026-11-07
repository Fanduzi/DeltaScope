// Package auditmeta verifies the T21-R1 B3 chain end to end: the real
// mysqlmeta.Provider reads information_schema.SEQUENCES definition columns
// through its ResolveObject loader, and the facts reach AuditSQL's ordered
// pre-state — catalog CYCLE feeds the cycle warning, the bounded read
// leaves the inbound-reference set unvouched so the drop reports the
// sequence.references gap, and a catalog error propagates through the
// public path. This is "real provider + constructed DB rows", not a live
// anchor.
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
	t21r1ValidRule = "ddl.tidb.sequence.valid"
	t21r1CycleRule = "ddl.tidb.sequence.cycle.warn"
	t21r1RefsRule  = "ddl.tidb.sequence.references.warn"
)

var (
	t21r1Once  sync.Once
	t21r1State sync.Map
)

type t21r1Driver struct{}
type t21r1Conn struct{ queries map[string]t21r1Result }
type t21r1Rows struct {
	cols  []string
	rows  [][]driver.Value
	index int
}
type t21r1Result struct {
	cols []string
	rows [][]driver.Value
	err  error
}

var t21r1SequenceColumns = []string{
	"SEQUENCE_SCHEMA", "SEQUENCE_NAME", "INCREMENT", "START",
	"MIN_VALUE", "MAX_VALUE", "CACHE", "CACHE_VALUE", "CYCLE",
}

func openT21R1DB(t *testing.T, queries map[string]t21r1Result) *sql.DB {
	t.Helper()
	t21r1Once.Do(func() { sql.Register("t21r1-mysqlmeta", t21r1Driver{}) })
	name := "t21r1-" + strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	t21r1State.Store(name, queries)
	db, err := sql.Open("t21r1-mysqlmeta", name)
	if err != nil {
		t.Fatalf("open driver db: %v", err)
	}
	t.Cleanup(func() {
		db.Close()
		t21r1State.Delete(name)
	})
	return db
}

func (t21r1Driver) Open(name string) (driver.Conn, error) {
	value, _ := t21r1State.Load(name)
	queries, _ := value.(map[string]t21r1Result)
	return t21r1Conn{queries: queries}, nil
}

func (t21r1Conn) Prepare(string) (driver.Stmt, error) { return nil, driver.ErrSkip }
func (t21r1Conn) Close() error                        { return nil }
func (t21r1Conn) Begin() (driver.Tx, error)           { return nil, driver.ErrSkip }

func (c t21r1Conn) QueryContext(_ context.Context, query string, _ []driver.NamedValue) (driver.Rows, error) {
	var result *t21r1Result
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
	return &t21r1Rows{cols: result.cols, rows: result.rows}, nil
}

func (r *t21r1Rows) Columns() []string { return r.cols }
func (r *t21r1Rows) Close() error      { return nil }
func (r *t21r1Rows) Next(dest []driver.Value) error {
	if r.index >= len(r.rows) {
		return io.EOF
	}
	copy(dest, r.rows[r.index])
	r.index++
	return nil
}

func t21r1Queries(rows [][]driver.Value) map[string]t21r1Result {
	return map[string]t21r1Result{
		"show variables": {
			cols: []string{"Variable_name", "Value"},
			rows: [][]driver.Value{{"version", "8.0.11-TiDB-v8.5.0"}},
		},
		"show config": {
			cols: []string{"Type", "Instance", "Name", "Value"},
		},
		"information_schema.sequences": {
			cols: t21r1SequenceColumns,
			rows: rows,
		},
	}
}

// The proven catalog CYCLE flag travels the official reader into the
// ordered pre-state: an ALTER declaring no cycle member still sees the
// catalog's enabled flag and warns — only a real definition read can feed
// that premise.
func TestT21R1OfficialReaderFeedsCyclePremise(t *testing.T) {
	db := openT21R1DB(t, t21r1Queries([][]driver.Value{{
		"t21", "seq", int64(1), int64(1), int64(1), int64(9223372036854775806), "YES", int64(1000), "YES",
	}}))
	provider := mysqlmeta.NewProvider(db)
	policy := t06a4Policy(t, map[string]string{t21r1ValidRule: "", t21r1CycleRule: ""})
	result, err := appaudit.AuditSQL(context.Background(), appaudit.Request{
		SQL:              "ALTER SEQUENCE seq INCREMENT BY 5",
		Dialect:          spec.DialectTiDB,
		Schema:           "t21",
		ConfigPath:       policy,
		MetadataProvider: provider,
	})
	if err != nil {
		t.Fatalf("AuditSQL: %v", err)
	}
	found := false
	for _, finding := range result.Statements[0].Findings {
		if finding.RuleID == t21r1CycleRule {
			found = true
		}
		if finding.RuleID == t21r1ValidRule {
			t.Fatalf("unexpected valid blocker on confirmed sequence: %#v", finding)
		}
	}
	if !found {
		t.Fatalf("no cycle warning — the catalog CYCLE flag did not reach the rule: %+v", result.Statements[0].Findings)
	}
}

// The bounded SEQUENCES read carries no inbound-edge set: the drop reports
// the sequence.references gap, never silence and never a fabricated empty
// set.
func TestT21R1OfficialReaderLeavesReferencesUnvouched(t *testing.T) {
	db := openT21R1DB(t, t21r1Queries([][]driver.Value{{
		"t21", "seq", int64(1), int64(1), int64(1), int64(9223372036854775806), "YES", int64(1000), "NO",
	}}))
	provider := mysqlmeta.NewProvider(db)
	policy := t06a4Policy(t, map[string]string{t21r1ValidRule: "", t21r1RefsRule: ""})
	result, err := appaudit.AuditSQL(context.Background(), appaudit.Request{
		SQL:              "DROP SEQUENCE seq",
		Dialect:          spec.DialectTiDB,
		Schema:           "t21",
		ConfigPath:       policy,
		MetadataProvider: provider,
	})
	if err != nil {
		t.Fatalf("AuditSQL: %v", err)
	}
	gotGap := false
	for _, gap := range result.Statements[0].EvidenceGaps {
		if gap.RuleID == t21r1RefsRule && gap.ReasonCode == "unknown_sequence_references" {
			gotGap = true
		}
	}
	if !gotGap {
		t.Fatalf("gaps = %#v, want unknown_sequence_references on the unvouched catalog read", result.Statements[0].EvidenceGaps)
	}
}

// A failed catalog read is an error through the public path — never a
// degraded unavailable, never a cached absence.
func TestT21R1OfficialReaderErrorPropagates(t *testing.T) {
	want := errors.New("sequences read refused")
	queries := t21r1Queries(nil)
	queries["information_schema.sequences"] = t21r1Result{cols: t21r1SequenceColumns, err: want}
	db := openT21R1DB(t, queries)
	provider := mysqlmeta.NewProvider(db)
	policy := t06a4Policy(t, map[string]string{t21r1ValidRule: ""})
	_, err := appaudit.AuditSQL(context.Background(), appaudit.Request{
		SQL:              "ALTER SEQUENCE seq START WITH 5",
		Dialect:          spec.DialectTiDB,
		Schema:           "t21",
		ConfigPath:       policy,
		MetadataProvider: provider,
	})
	if err == nil || !errors.Is(err, want) {
		t.Fatalf("err = %v, want propagated %v", err, want)
	}
}

// Zero catalog rows stay unknown through the public path: the existence
// premise reports the bounded state gap — never a proven absence, never a
// blocker on an unprovable target.
func TestT21R1OfficialReaderZeroRowsStayUnknown(t *testing.T) {
	db := openT21R1DB(t, t21r1Queries(nil))
	provider := mysqlmeta.NewProvider(db)
	policy := t06a4Policy(t, map[string]string{t21r1ValidRule: "", t21r1RefsRule: ""})
	result, err := appaudit.AuditSQL(context.Background(), appaudit.Request{
		SQL:              "DROP SEQUENCE ghost",
		Dialect:          spec.DialectTiDB,
		Schema:           "t21",
		ConfigPath:       policy,
		MetadataProvider: provider,
	})
	if err != nil {
		t.Fatalf("AuditSQL: %v", err)
	}
	gotStateGap := false
	for _, gap := range result.Statements[0].EvidenceGaps {
		if gap.RuleID == t21r1ValidRule && gap.ReasonCode == "unknown_sequence_state" {
			gotStateGap = true
		}
	}
	if !gotStateGap {
		t.Fatalf("gaps = %#v, want unknown_sequence_state on a zero-row catalog read", result.Statements[0].EvidenceGaps)
	}
	for _, finding := range result.Statements[0].Findings {
		if finding.RuleID == t21r1ValidRule {
			t.Fatalf("unexpected blocker on an unproven target: %#v", finding)
		}
	}
}
