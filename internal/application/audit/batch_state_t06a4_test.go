// Package audit verifies the T06-A4 internal-state contract behind the
// provider default-identity fix: once loadColumns stops flagging stored
// 'NULL' text as a SQL NULL default, the ordered state must keep that
// sibling inside the A6 conservative drop boundary, while a stored SQL NULL
// keeps the precise path. The real mysqlmeta.Provider itself is exercised
// end-to-end in auditmeta's T06A4 test (the provider cannot be imported
// here — mysqlmeta already asserts the audit.MetadataProvider interface);
// this file pins the state-machine half with provider-shaped stubs whose
// fields mirror the post-fix production output.
// input: two-statement ALTER DROP COLUMN + CREATE INDEX batches over stored-shape snapshots
// output: conservative unknown_table_state for literal siblings; precise post-drop state for stored SQL NULL
// pos: application-layer internal-state contract tests for issue #85 T06-A4
// note: if this file changes, update this header and module README.md.
package audit

import (
	"context"
	"reflect"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// t06a4StoredProvider mirrors the fields the real provider emits for the
// frozen fixture after the fix: a stored SQL NULL (a, b, and C-variant c)
// surfaces as HasDefault=false — the catalog cannot distinguish it from an
// omitted clause — and a stored byte string surfaces unquoted with
// DefaultIsNull=false. cText="NULL" is the A/B shape (DEFAULT 'NULL');
// cText=nil is the C shape (DEFAULT NULL).
type t06a4StoredProvider struct {
	calls []string
	cText *string
}

func (p *t06a4StoredProvider) LoadInstanceFacts(context.Context, spec.Dialect, string) (*spec.InstanceFacts, error) {
	return &spec.InstanceFacts{Version: "8.4.10"}, nil
}

func (p *t06a4StoredProvider) LoadTableSnapshot(_ context.Context, _ spec.Dialect, schema, table string) (*spec.TableSnapshot, error) {
	p.calls = append(p.calls, schema+"."+table)
	c := spec.Column{Name: "c", Type: "varchar", Length: 8}
	if p.cText != nil {
		c.HasDefault = true
		c.DefaultValue = *p.cText
	}
	return &spec.TableSnapshot{
		Exists: true,
		Schema: schema,
		Table:  &spec.Table{Schema: schema, Name: table},
		Columns: []spec.Column{
			{Name: "id", Type: "int", NotNull: true},
			{Name: "a", Type: "varchar", Length: 8},
			{Name: "b", Type: "varchar", Length: 8},
			c,
			{Name: "d", Type: "varchar", Length: 8, HasDefault: true, DefaultValue: "<nil>"},
		},
		PrimaryKey: &spec.Index{Name: "PRIMARY", Kind: spec.IndexKindPrimary, Columns: []string{"id"}},
	}, nil
}

// TestT06A4NullPostDropStateInternal pins the C contract on the ordered
// state: the post-drop column order is [id a b c], PRIMARY(id) survives, and
// the INDEX statement's pre-state carries it — proving the fix does not
// invalidate every metadata-backed DROP.
func TestT06A4NullPostDropStateInternal(t *testing.T) {
	provider := &t06a4StoredProvider{}
	sqlText := "ALTER TABLE t06_a4_defaults DROP COLUMN d;\nCREATE INDEX idx_b ON t06_a4_defaults(b);"
	enriched := enrichA3(t, sqlText, spec.DialectMySQL, provider)
	preIndex := enriched[1].Metadata.TargetTable
	if preIndex == nil || len(preIndex.Columns) != 4 {
		t.Fatalf("pre-index snapshot = %+v, want [id a b c]", preIndex)
	}
	for i, name := range []string{"id", "a", "b", "c"} {
		if preIndex.Columns[i].Name != name {
			t.Fatalf("pre-index column %d = %q, want %q", i, preIndex.Columns[i].Name, name)
		}
	}
	if preIndex.PrimaryKey == nil || !reflect.DeepEqual(preIndex.PrimaryKey.Columns, []string{"id"}) {
		t.Fatalf("primary key post-drop = %+v, want PRIMARY(id)", preIndex.PrimaryKey)
	}
	if len(provider.calls) != 1 || provider.calls[0] != "golden.t06_a4_defaults" {
		t.Fatalf("provider calls = %#v, want exactly one golden.t06_a4_defaults load", provider.calls)
	}
}

// TestT06A4TextNullInvalidatesStateInternal pins the A contract on the
// ordered state: with c carrying the stored 'NULL' literal (the post-fix
// provider shape: HasDefault=true, DefaultIsNull=false), the drop must
// invalidate rather than publish a precise post-state.
func TestT06A4TextNullInvalidatesStateInternal(t *testing.T) {
	text := "NULL"
	provider := &t06a4StoredProvider{cText: &text}
	sqlText := "ALTER TABLE t06_a4_defaults DROP COLUMN d;\nCREATE INDEX idx_b ON t06_a4_defaults(b);"
	enriched := enrichA3(t, sqlText, spec.DialectMySQL, provider)
	t05A4R1WantUnknown(t, enriched[1])
}
