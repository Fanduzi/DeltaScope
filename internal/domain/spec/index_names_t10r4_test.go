// Package spec verifies the T10-R4 fact-fidelity corrections at the shared
// determination layer: StoredIndexNames derives only natively provable
// stored names (MySQL 8.0.46 reserves `primary` and names any
// expression-bearing unnamed member `functional_index`; TiDB v8.5.0 names
// it `expression_index` and pre-occupies authored names before deriving),
// and PrimaryKeyClustered separates clustered storage (DROP PRIMARY KEY
// refusal 8200) from the PRIMARY-object addressability answer.
// input: declared index member lists across dialects and declared TiDB
// primary keys across member shapes, session modes, and config facts
// output: proven stored names or a withheld name space; a tri-state
// clustered-storage answer (nil = unprovable)
// pos: spec-side T10-R4 coverage beside primary_key_role_t10r3_test.go;
// upstream anchors — MySQL 8.0.46 sql/sql_table.cc make_unique_key_name
// (P1-P2/P9/P12-P14 probes), TiDB v8.5.0 pkg/ddl/create_table.go
// setEmptyConstraintName (P3-P4/P8/P10-P11 probes), pkg/ddl/executor.go
// IsCommonHandle||PKIsHandle drop refusal (P5-P7 probes)
// note: if this file changes, update this header and module README.md.
package spec

import (
	"testing"
)

func t10r4Col(name string) IndexKeyPart {
	return IndexKeyPart{Kind: IndexKeyPartColumn, Column: name}
}

func t10r4Expr() IndexKeyPart {
	return IndexKeyPart{Kind: IndexKeyPartExpression, Expression: &ExpressionFacts{Known: true}}
}

// B: the naming decision table is dialect-bound. Every name pinned below is
// backed by a recorded native probe on MySQL 8.0.46 / TiDB v8.5.0; cases the
// product itself cannot prove keep the name space unvouched instead of
// deriving a plausible-looking string.
func TestT10R4StoredIndexNamesDialectMatrix(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name         string
		dialect      Dialect
		indexes      []Index
		wantNames    []string
		wantProvable bool
	}{
		// MySQL positive derivations.
		{
			name:    "mysql reserved primary skips to primary_2 (P1)",
			dialect: DialectMySQL,
			indexes: []Index{
				{Kind: IndexKindSecondary, Columns: []string{"primary"}, KeyParts: []IndexKeyPart{t10r4Col("primary")}},
			},
			wantNames:    []string{"primary_2"},
			wantProvable: true,
		},
		{
			name:    "mysql primary collides through the used set to primary_3 (P2)",
			dialect: DialectMySQL,
			indexes: []Index{
				{Kind: IndexKindSecondary, Columns: []string{"primary_2"}, KeyParts: []IndexKeyPart{t10r4Col("primary_2")}},
				{Kind: IndexKindSecondary, Columns: []string{"primary"}, KeyParts: []IndexKeyPart{t10r4Col("primary")}},
			},
			wantNames:    []string{"primary_2", "primary_3"},
			wantProvable: true,
		},
		{
			name:    "mysql expression member names functional_index (P12)",
			dialect: DialectMySQL,
			indexes: []Index{
				{Kind: IndexKindUnique, ExpressionCount: 1, HasExpressionKeys: true, KeyParts: []IndexKeyPart{t10r4Expr()}},
			},
			wantNames:    []string{"functional_index"},
			wantProvable: true,
		},
		{
			name:    "mysql expression in any position names functional_index (P13)",
			dialect: DialectMySQL,
			indexes: []Index{
				{Kind: IndexKindSecondary, Columns: []string{"a"}, ExpressionCount: 1, HasExpressionKeys: true,
					KeyParts: []IndexKeyPart{t10r4Col("a"), t10r4Expr()}},
			},
			wantNames:    []string{"functional_index"},
			wantProvable: true,
		},
		{
			name:    "mysql two functional members suffix in order (P14)",
			dialect: DialectMySQL,
			indexes: []Index{
				{Kind: IndexKindSecondary, ExpressionCount: 1, HasExpressionKeys: true, KeyParts: []IndexKeyPart{t10r4Expr()}},
				{Kind: IndexKindSecondary, ExpressionCount: 1, HasExpressionKeys: true, KeyParts: []IndexKeyPart{t10r4Expr()}},
			},
			wantNames:    []string{"functional_index", "functional_index_2"},
			wantProvable: true,
		},
		{
			name:    "mysql unnamed-then-authored same derived name fails natively (P9)",
			dialect: DialectMySQL,
			indexes: []Index{
				{Kind: IndexKindSecondary, Columns: []string{"a"}, KeyParts: []IndexKeyPart{t10r4Col("a")}},
				{Name: "a", Kind: IndexKindSecondary, Columns: []string{"b"}, KeyParts: []IndexKeyPart{t10r4Col("b")}},
			},
			wantProvable: false,
		},
		// TiDB derivations — different literal base, different ordering.
		{
			name:    "tidb reserved primary skips to primary_2 (M8g)",
			dialect: DialectTiDB,
			indexes: []Index{
				{Kind: IndexKindSecondary, Columns: []string{"primary"}, KeyParts: []IndexKeyPart{t10r4Col("primary")}},
			},
			wantNames:    []string{"primary_2"},
			wantProvable: true,
		},
		{
			name:    "tidb authored primary fails natively (M8h)",
			dialect: DialectTiDB,
			indexes: []Index{
				{Name: "primary", Kind: IndexKindSecondary, Columns: []string{"a"}, KeyParts: []IndexKeyPart{t10r4Col("a")}},
			},
			wantProvable: false,
		},
		{
			name:    "mysql authored primary fails natively (M8b)",
			dialect: DialectMySQL,
			indexes: []Index{
				{Name: "primary", Kind: IndexKindSecondary, Columns: []string{"a"}, KeyParts: []IndexKeyPart{t10r4Col("a")}},
			},
			wantProvable: false,
		},
		{
			name:    "tidb expression member names expression_index (P3)",
			dialect: DialectTiDB,
			indexes: []Index{
				{Kind: IndexKindSecondary, Columns: []string{"a"}, ExpressionCount: 1, HasExpressionKeys: true,
					KeyParts: []IndexKeyPart{t10r4Col("a"), t10r4Expr()}},
			},
			wantNames:    []string{"expression_index"},
			wantProvable: true,
		},
		{
			name:    "tidb leading expression names expression_index (P4)",
			dialect: DialectTiDB,
			indexes: []Index{
				{Kind: IndexKindSecondary, Columns: []string{"b"}, ExpressionCount: 1, HasExpressionKeys: true,
					KeyParts: []IndexKeyPart{t10r4Expr(), t10r4Col("b")}},
			},
			wantNames:    []string{"expression_index"},
			wantProvable: true,
		},
		{
			name:    "tidb two expression members suffix in order (P11)",
			dialect: DialectTiDB,
			indexes: []Index{
				{Kind: IndexKindSecondary, ExpressionCount: 1, HasExpressionKeys: true, KeyParts: []IndexKeyPart{t10r4Expr(), t10r4Col("b")}},
				{Kind: IndexKindSecondary, ExpressionCount: 1, HasExpressionKeys: true, KeyParts: []IndexKeyPart{t10r4Expr(), t10r4Col("b")}},
			},
			wantNames:    []string{"expression_index", "expression_index_2"},
			wantProvable: true,
		},
		{
			name:    "tidb authored names pre-occupy before derivation (P8)",
			dialect: DialectTiDB,
			indexes: []Index{
				{Kind: IndexKindSecondary, Columns: []string{"a"}, KeyParts: []IndexKeyPart{t10r4Col("a")}},
				{Name: "a", Kind: IndexKindSecondary, Columns: []string{"b"}, KeyParts: []IndexKeyPart{t10r4Col("b")}},
			},
			wantNames:    []string{"a_2", "a"},
			wantProvable: true,
		},
		{
			name:    "tidb authored first column stays sequential (P10)",
			dialect: DialectTiDB,
			indexes: []Index{
				{Name: "a", Kind: IndexKindSecondary, Columns: []string{"a"}, KeyParts: []IndexKeyPart{t10r4Col("a")}},
				{Kind: IndexKindSecondary, Columns: []string{"b"}, KeyParts: []IndexKeyPart{t10r4Col("b")}},
			},
			wantNames:    []string{"a", "b"},
			wantProvable: true,
		},
		{
			name:    "tidb authored expression_index pre-occupies the base",
			dialect: DialectTiDB,
			indexes: []Index{
				{Name: "expression_index", Kind: IndexKindSecondary, Columns: []string{"a"}, KeyParts: []IndexKeyPart{t10r4Col("a")}},
				{Kind: IndexKindSecondary, Columns: []string{"b"}, ExpressionCount: 1, HasExpressionKeys: true,
					KeyParts: []IndexKeyPart{t10r4Col("b"), t10r4Expr()}},
			},
			wantNames:    []string{"expression_index", "expression_index_2"},
			wantProvable: true,
		},
		// Shared unprovable boundaries.
		{
			name:    "unmodeled member leaves the name unproven",
			dialect: DialectTiDB,
			indexes: []Index{
				{Kind: IndexKindSecondary, Columns: []string{"a"}, UnmodeledKeyParts: 1,
					KeyParts: []IndexKeyPart{t10r4Col("a"), {Kind: IndexKeyPartUnmodeled}}},
			},
			wantProvable: false,
		},
		{
			name:    "empty member list leaves the name unproven",
			dialect: DialectTiDB,
			indexes: []Index{
				{Kind: IndexKindUnique},
			},
			wantProvable: false,
		},
		{
			name:    "duplicate authored names stay unprovable on either dialect",
			dialect: DialectTiDB,
			indexes: []Index{
				{Name: "a", Kind: IndexKindSecondary, Columns: []string{"a"}, KeyParts: []IndexKeyPart{t10r4Col("a")}},
				{Name: "a", Kind: IndexKindSecondary, Columns: []string{"b"}, KeyParts: []IndexKeyPart{t10r4Col("b")}},
			},
			wantProvable: false,
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			resolved, provable := StoredIndexNames(tc.dialect, tc.indexes)
			if provable != tc.wantProvable {
				t.Fatalf("provable = %v, want %v", provable, tc.wantProvable)
			}
			if !tc.wantProvable {
				return
			}
			if len(resolved) != len(tc.wantNames) {
				t.Fatalf("resolved len = %d, want %d", len(resolved), len(tc.wantNames))
			}
			for i, want := range tc.wantNames {
				if resolved[i].Name != want {
					t.Fatalf("resolved[%d].Name = %q, want %q", i, resolved[i].Name, want)
				}
			}
		})
	}
}

// The 64-char boundary: a base at the identifier limit cannot gain a suffix
// without the product truncating — that shape is unprovable rather than
// derived wrongly.
func TestT10R4StoredIndexNamesLengthBoundary(t *testing.T) {
	t.Parallel()
	long := make([]byte, 64)
	for i := range long {
		long[i] = 'c'
	}
	base := string(long)
	indexes := []Index{
		{Name: base, Kind: IndexKindSecondary, Columns: []string{"a"}, KeyParts: []IndexKeyPart{t10r4Col("a")}},
		{Kind: IndexKindSecondary, Columns: []string{base}, KeyParts: []IndexKeyPart{t10r4Col(base)}},
	}
	resolved, provable := StoredIndexNames(DialectMySQL, indexes)
	if provable {
		t.Fatalf("a suffix needing truncation must stay unproven, got %+v", resolved)
	}
}

// C: clustered storage is the DROP PRIMARY KEY premise — TiDB refuses the
// drop on PKIsHandle and IsCommonHandle alike (8200, probed P5-P7), while a
// nonclustered PK drops cleanly. Addressability is a different question and
// stays with PrimaryKeyHandle.
func TestT10R4PrimaryKeyClustered(t *testing.T) {
	t.Parallel()
	on := "on"
	off := "off"
	intOnly := "int_only"
	weird := "bogus"
	apkTrue := true
	apkFalse := false
	columns := []Column{{Name: "id", Type: "int"}, {Name: "v", Type: "varchar(32)"}}
	cases := []struct {
		name    string
		dialect Dialect
		pk      *Index
		columns []Column
		mode    *string
		apk     *bool
		want    *bool
	}{
		{name: "mysql always droppable", dialect: DialectMySQL,
			pk: &Index{Kind: IndexKindPrimary, Columns: []string{"id"}}, columns: columns, want: boolPtr(false)},
		{name: "tidb declared nonclustered", dialect: DialectTiDB,
			pk: &Index{Kind: IndexKindPrimary, Columns: []string{"v"}, PrimaryKeyType: "nonclustered"}, columns: columns, want: boolPtr(false)},
		{name: "tidb declared clustered varchar (P5)", dialect: DialectTiDB,
			pk: &Index{Kind: IndexKindPrimary, Columns: []string{"v"}, PrimaryKeyType: "clustered"}, columns: columns, want: boolPtr(true)},
		{name: "tidb declared clustered composite (P6)", dialect: DialectTiDB,
			pk: &Index{Kind: IndexKindPrimary, Columns: []string{"id", "v"}, PrimaryKeyType: "clustered"}, columns: columns, want: boolPtr(true)},
		{name: "tidb unstated mode on clusters any shape (P7)", dialect: DialectTiDB,
			pk: &Index{Kind: IndexKindPrimary, Columns: []string{"v"}}, columns: columns, mode: &on, want: boolPtr(true)},
		{name: "tidb unstated mode off stays nonclustered", dialect: DialectTiDB,
			pk: &Index{Kind: IndexKindPrimary, Columns: []string{"v"}}, columns: columns, mode: &off, want: boolPtr(false)},
		{name: "tidb int_only single-int without alter-pk clusters", dialect: DialectTiDB,
			pk: &Index{Kind: IndexKindPrimary, Columns: []string{"id"}}, columns: columns, mode: &intOnly, apk: &apkFalse, want: boolPtr(true)},
		{name: "tidb int_only single-int with alter-pk stays real", dialect: DialectTiDB,
			pk: &Index{Kind: IndexKindPrimary, Columns: []string{"id"}}, columns: columns, mode: &intOnly, apk: &apkTrue, want: boolPtr(false)},
		{name: "tidb int_only single-int unobserved alter-pk stays unknown", dialect: DialectTiDB,
			pk: &Index{Kind: IndexKindPrimary, Columns: []string{"id"}}, columns: columns, mode: &intOnly, want: nil},
		{name: "tidb int_only non-int is nonclustered", dialect: DialectTiDB,
			pk: &Index{Kind: IndexKindPrimary, Columns: []string{"v"}}, columns: columns, mode: &intOnly, apk: &apkFalse, want: boolPtr(false)},
		{name: "tidb int_only composite is nonclustered", dialect: DialectTiDB,
			pk: &Index{Kind: IndexKindPrimary, Columns: []string{"id", "v"}}, columns: columns, mode: &intOnly, apk: &apkFalse, want: boolPtr(false)},
		{name: "tidb int_only unprovable member stays unknown", dialect: DialectTiDB,
			pk: &Index{Kind: IndexKindPrimary, Columns: []string{"id"}}, columns: nil, mode: &intOnly, apk: &apkFalse, want: nil},
		{name: "tidb unobserved mode stays unknown", dialect: DialectTiDB,
			pk: &Index{Kind: IndexKindPrimary, Columns: []string{"id"}}, columns: columns, want: nil},
		{name: "tidb unrecognized mode stays unknown", dialect: DialectTiDB,
			pk: &Index{Kind: IndexKindPrimary, Columns: []string{"id"}}, columns: columns, mode: &weird, want: nil},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := PrimaryKeyClustered(tc.dialect, tc.pk, tc.columns, tc.mode, tc.apk)
			if tc.want == nil {
				if got != nil {
					t.Fatalf("clustered = %v, want unprovable", *got)
				}
				return
			}
			if got == nil || *got != *tc.want {
				t.Fatalf("clustered = %v, want %v", got, *tc.want)
			}
		})
	}
}
