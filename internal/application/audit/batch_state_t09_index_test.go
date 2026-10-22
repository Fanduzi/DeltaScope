// Package audit verifies the T09 index state chain end to end.
// input: real MySQL/TiDB ALTER/CREATE/DROP/RENAME INDEX batches audited
// through AuditSQL with a stub provider whose table snapshots carry a
// known-empty index set
// output: statement-bound findings proving added, dropped and renamed
// indexes are visible to later statements — and never leak across tables
// pos: application-layer ordered-state regression for issue #88 T09; the
// batch-state transitions under test live in batch_state_index.go
// note: if this file changes, update this header and module README.md.
package audit

import (
	"context"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/report"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// t09Provider serves fixed per-table snapshots. Each table gets a
// nine-column shape with a vouched (possibly empty) index set — nil Indexes
// is never used here, so "known empty" and "unknown" stay distinct.
type t09Provider struct {
	tables map[string]*spec.TableSnapshot
	calls  []string
}

func (p *t09Provider) LoadInstanceFacts(context.Context, spec.Dialect, string) (*spec.InstanceFacts, error) {
	return &spec.InstanceFacts{}, nil
}

func (p *t09Provider) LoadTableSnapshot(_ context.Context, _ spec.Dialect, schema, table string) (*spec.TableSnapshot, error) {
	p.calls = append(p.calls, schema+"."+table)
	if snap, ok := p.tables[strings.ToLower(table)]; ok {
		return snap, nil
	}
	return &spec.TableSnapshot{Exists: false, Table: &spec.Table{Schema: schema, Name: table}}, nil
}

func t09Table(name string, indexes ...spec.Index) *spec.TableSnapshot {
	columns := make([]spec.Column, 0, 9)
	for _, col := range []string{"a", "b", "c", "d", "e", "f", "g", "h", "i"} {
		columns = append(columns, spec.Column{Name: col, Type: "int"})
	}
	return &spec.TableSnapshot{
		Exists:  true,
		Table:   &spec.Table{Name: name},
		Columns: columns,
		Indexes: indexes,
	}
}

func t09Audit(t *testing.T, dialect spec.Dialect, sql string, provider MetadataProvider, rules map[string]string) report.Result {
	t.Helper()
	result, err := AuditSQL(context.Background(), Request{
		SQL:              sql,
		Dialect:          dialect,
		Schema:           "golden",
		ConfigPath:       t05A7Policy(t, rules),
		MetadataProvider: provider,
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	return result
}

// ADD INDEX publishes the index; a later same-name standalone CREATE INDEX
// must hit ddl.alter.add_index.exists.forbid — proof the transition reached
// the ordered state.
func TestT09AddIndexVisibleToLaterCreateIndex(t *testing.T) {
	t.Parallel()
	for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
		dialect := dialect
		t.Run(string(dialect), func(t *testing.T) {
			t.Parallel()
			provider := &t09Provider{tables: map[string]*spec.TableSnapshot{"t": t09Table("t")}}
			result := t09Audit(t, dialect,
				"ALTER TABLE t ADD INDEX idx_dup(a);\nCREATE INDEX idx_dup ON t(a);",
				provider,
				map[string]string{"ddl.alter.add_index.exists.forbid": ""},
			)
			if got := t05FindingsByRule(result, 0, "ddl.alter.add_index.exists.forbid"); len(got) != 0 {
				t.Fatalf("stmt0 fresh add must not collide, got %+v", got)
			}
			findings := t05FindingsByRule(result, 1, "ddl.alter.add_index.exists.forbid")
			if len(findings) != 1 || findings[0].Metadata["name"] != "idx_dup" {
				t.Fatalf("stmt1 same-name CREATE INDEX must report the published index, got %+v", result.Statements[1].Findings)
			}
		})
	}
}

// DROP INDEX removes the published index; a later re-add of the freed name
// stays silent.
func TestT09AddDropReaddIndexChain(t *testing.T) {
	t.Parallel()
	for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
		dialect := dialect
		t.Run(string(dialect), func(t *testing.T) {
			t.Parallel()
			provider := &t09Provider{tables: map[string]*spec.TableSnapshot{"t": t09Table("t")}}
			result := t09Audit(t, dialect,
				"ALTER TABLE t ADD INDEX idx_a(a);\nDROP INDEX idx_a ON t;\nCREATE INDEX idx_a ON t(a);",
				provider,
				map[string]string{
					"ddl.alter.add_index.exists.forbid":   "",
					"ddl.alter.drop_index.exists.require": "",
				},
			)
			for i := 0; i < 3; i++ {
				if got := t05FindingsByRule(result, i, "ddl.alter.add_index.exists.forbid"); len(got) != 0 {
					t.Fatalf("stmt%d must not report a name collision after drop freed it, got %+v", i, got)
				}
			}
			if got := t05FindingsByRule(result, 1, "ddl.alter.drop_index.exists.require"); len(got) != 0 {
				t.Fatalf("stmt1 standalone DROP INDEX on the published index must see it, got %+v", got)
			}
		})
	}
}

// RENAME INDEX frees the old name and publishes the new one: a later CREATE
// with the old name stays silent, and a later DROP of the new name succeeds.
func TestT09RenameIndexPublishesNewName(t *testing.T) {
	t.Parallel()
	for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
		dialect := dialect
		t.Run(string(dialect), func(t *testing.T) {
			t.Parallel()
			provider := &t09Provider{tables: map[string]*spec.TableSnapshot{"t": t09Table("t")}}
			result := t09Audit(t, dialect,
				"ALTER TABLE t ADD INDEX idx_old(a);\nALTER TABLE t RENAME INDEX idx_old TO idx_new;\nDROP INDEX idx_new ON t;\nCREATE INDEX idx_old ON t(a);",
				provider,
				map[string]string{
					"ddl.alter.add_index.exists.forbid":     "",
					"ddl.alter.drop_index.exists.require":   "",
					"ddl.alter.rename_index.exists.require": "",
				},
			)
			if got := t05FindingsByRule(result, 1, "ddl.alter.rename_index.exists.require"); len(got) != 0 {
				t.Fatalf("rename premise must see the published idx_old, got %+v", got)
			}
			if got := t05FindingsByRule(result, 2, "ddl.alter.drop_index.exists.require"); len(got) != 0 {
				t.Fatalf("drop must see the renamed idx_new, got %+v", got)
			}
			if got := t05FindingsByRule(result, 3, "ddl.alter.add_index.exists.forbid"); len(got) != 0 {
				t.Fatalf("freed idx_old name must be addable again, got %+v", got)
			}
		})
	}
}

// A unique index added through CONSTRAINT syntax publishes to the shared
// index set, so a later ALTER ADD INDEX on the same ordered columns trips
// the previously deferred unique-overlap rule.
func TestT09ConstraintUniqueFeedsLaterOverlap(t *testing.T) {
	t.Parallel()
	for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
		dialect := dialect
		t.Run(string(dialect), func(t *testing.T) {
			t.Parallel()
			provider := &t09Provider{tables: map[string]*spec.TableSnapshot{"t": t09Table("t")}}
			result := t09Audit(t, dialect,
				"ALTER TABLE t ADD CONSTRAINT uq_ab UNIQUE(a,b);\nALTER TABLE t ADD INDEX s_ab(a,b);",
				provider,
				map[string]string{"ddl.alter.add_index.redundant_unique_overlap.forbid": ""},
			)
			findings := t05FindingsByRule(result, 1, "ddl.alter.add_index.redundant_unique_overlap.forbid")
			if len(findings) != 1 || findings[0].Metadata["index"] != "s_ab" {
				t.Fatalf("stmt1 secondary overlapping the published unique must fire, got %+v", result.Statements[1].Findings)
			}
		})
	}
}

// The same index name on two different tables never collides — state entries
// are keyed by table identity.
func TestT09IndexNamesAreTableScoped(t *testing.T) {
	t.Parallel()
	for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
		dialect := dialect
		t.Run(string(dialect), func(t *testing.T) {
			t.Parallel()
			provider := &t09Provider{tables: map[string]*spec.TableSnapshot{
				"t": t09Table("t", spec.Index{Name: "idx_a", Kind: spec.IndexKindSecondary, Columns: []string{"a"}}),
				"u": t09Table("u"),
			}}
			result := t09Audit(t, dialect,
				"ALTER TABLE u ADD INDEX idx_a(a);",
				provider,
				map[string]string{"ddl.alter.add_index.exists.forbid": ""},
			)
			if got := t05FindingsByRule(result, 0, "ddl.alter.add_index.exists.forbid"); len(got) != 0 {
				t.Fatalf("idx_a on t must not collide with idx_a on u, got %+v", got)
			}
		})
	}
}
