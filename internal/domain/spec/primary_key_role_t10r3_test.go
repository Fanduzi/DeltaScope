// Package spec tests the T10-R3 identity corrections: TiDB int_only
// primary-key handle determination must consult the alter-primary-key
// configuration fact (upstream pin: TiDB v8.5.0 pkg/ddl/create_table.go
// L1582-1593 — int_only clusters a single-int PK only when
// alter-primary-key is disabled), and product-derived stored index names
// must follow the native auto-naming rule (MySQL 8.0.46
// mysql-test/r/invisible_indexes.result L274-285 — an unnamed index member
// takes its first column member's name, with a numeric suffix on
// collision).
// input: constructed primary-key/index declarations exercising the
// handle and naming decision tables
// output: tri-state handle answers, provable stored names, and bounded
// unprovable answers instead of guessed identities
// pos: spec-level T10-R3 decision-table coverage beside the audit-level
// chain tests
// note: if this file changes, update this header and module README.md.
package spec

import "testing"

// B3: int_only reads the alter-primary-key config fact before answering the
// handle question — true means a real PRIMARY object, false means the
// handle, and an unobserved fact keeps the answer unknown rather than
// collapsing to handle=true.
func TestT10R3PrimaryKeyHandleIntOnlyAlterPrimaryKey(t *testing.T) {
	t.Parallel()
	columns := []Column{{Name: "id", Type: "int(11)", NotNull: true}}
	intOnly := "int_only"
	alterTrue, alterFalse := true, false
	cases := []struct {
		name string
		apk  *bool
		want *bool
	}{
		{name: "int_only + alter_primary_key true → real object", apk: &alterTrue, want: boolPtr(false)},
		{name: "int_only + alter_primary_key false → handle", apk: &alterFalse, want: boolPtr(true)},
		{name: "int_only + alter_primary_key unobserved → unknown", apk: nil, want: nil},
	}
	for _, tc := range cases {
		pk := &Index{Kind: IndexKindPrimary, Columns: []string{"id"}}
		got := PrimaryKeyHandle(DialectTiDB, pk, columns, &intOnly, tc.apk)
		if (got == nil) != (tc.want == nil) || (got != nil && tc.want != nil && *got != *tc.want) {
			t.Fatalf("%s: PrimaryKeyHandle = %v, want %v", tc.name, got, tc.want)
		}
	}
}

// B3 controls: on/off/explicit types answer without consulting the config
// fact — a present or absent alter-primary-key value changes nothing there.
func TestT10R3PrimaryKeyHandleModeControls(t *testing.T) {
	t.Parallel()
	columns := []Column{{Name: "id", Type: "int(11)", NotNull: true}}
	on, off := "on", "off"
	alterTrue, alterFalse := true, false
	cases := []struct {
		name   string
		pkType string
		mode   *string
		apk    *bool
		want   *bool
	}{
		{name: "on + alter-pk true still handle", mode: &on, apk: &alterTrue, want: boolPtr(true)},
		{name: "off + alter-pk false still real object", mode: &off, apk: &alterFalse, want: boolPtr(false)},
		{name: "clustered + alter-pk true still handle", pkType: "clustered", apk: &alterTrue, want: boolPtr(true)},
		{name: "nonclustered + alter-pk false still real", pkType: "nonclustered", apk: &alterFalse, want: boolPtr(false)},
		{name: "unread mode + alter-pk known still unknown", apk: &alterFalse, want: nil},
	}
	for _, tc := range cases {
		pk := &Index{Kind: IndexKindPrimary, Columns: []string{"id"}, PrimaryKeyType: tc.pkType}
		got := PrimaryKeyHandle(DialectTiDB, pk, columns, tc.mode, tc.apk)
		if (got == nil) != (tc.want == nil) || (got != nil && tc.want != nil && *got != *tc.want) {
			t.Fatalf("%s: PrimaryKeyHandle = %v, want %v", tc.name, got, tc.want)
		}
	}
}

// B2: StoredIndexNames applies the product's auto-naming to a CREATE TABLE's
// declared index list — an unnamed member takes its first column member's
// name, collisions gain a numeric suffix in declaration order, and anything
// the name rule cannot prove keeps the name space unvouched.
func TestT10R3StoredIndexNames(t *testing.T) {
	t.Parallel()
	colPart := func(name string) IndexKeyPart {
		return IndexKeyPart{Kind: IndexKeyPartColumn, Column: name}
	}
	exprPart := func() IndexKeyPart {
		return IndexKeyPart{Kind: IndexKeyPartExpression, Expression: &ExpressionFacts{Known: true}}
	}
	cases := []struct {
		name         string
		indexes      []Index
		wantNames    []string
		wantProvable bool
	}{
		{
			name: "authored names pass through verbatim",
			indexes: []Index{
				{Name: "ux_b", Kind: IndexKindUnique, Columns: []string{"b"}, KeyParts: []IndexKeyPart{colPart("b")}},
			},
			wantNames:    []string{"ux_b"},
			wantProvable: true,
		},
		{
			name: "unnamed member takes first column member name",
			indexes: []Index{
				{Kind: IndexKindUnique, Columns: []string{"b"}, KeyParts: []IndexKeyPart{colPart("b")}},
			},
			wantNames:    []string{"b"},
			wantProvable: true,
		},
		{
			name: "two unnamed members sharing a first column suffix in order",
			indexes: []Index{
				{Kind: IndexKindUnique, Columns: []string{"a"}, KeyParts: []IndexKeyPart{colPart("a")}},
				{Kind: IndexKindUnique, Columns: []string{"a", "b"}, KeyParts: []IndexKeyPart{colPart("a"), colPart("b")}},
			},
			wantNames:    []string{"a", "a_2"},
			wantProvable: true,
		},
		{
			name: "auto name against an authored collision suffixes",
			indexes: []Index{
				{Name: "a", Kind: IndexKindSecondary, Columns: []string{"b"}, KeyParts: []IndexKeyPart{colPart("b")}},
				{Kind: IndexKindUnique, Columns: []string{"a"}, KeyParts: []IndexKeyPart{colPart("a")}},
			},
			wantNames:    []string{"a", "a_2"},
			wantProvable: true,
		},
		{
			name: "prefix member still names its column",
			indexes: func() []Index {
				length := 3
				return []Index{{
					Kind: IndexKindSecondary, Columns: []string{"c"},
					KeyParts:    []IndexKeyPart{{Kind: IndexKeyPartColumn, Column: "c", PrefixLength: &length}},
					PrefixParts: 1,
				}}
			}(),
			wantNames:    []string{"c"},
			wantProvable: true,
		},
		{
			name: "expression first member leaves the name unproven",
			indexes: []Index{
				{Kind: IndexKindUnique, ExpressionCount: 1, HasExpressionKeys: true,
					KeyParts: []IndexKeyPart{exprPart()}},
			},
			wantProvable: false,
		},
		{
			name: "authored duplicate name is unprovable (native 1061)",
			indexes: []Index{
				{Kind: IndexKindUnique, Columns: []string{"a"}, KeyParts: []IndexKeyPart{colPart("a")}},
				{Name: "a", Kind: IndexKindSecondary, Columns: []string{"a"}, KeyParts: []IndexKeyPart{colPart("a")}},
			},
			wantProvable: false,
		},
		{
			name: "member projection without key parts falls back to columns",
			indexes: []Index{
				{Kind: IndexKindUnique, Columns: []string{"b"}},
			},
			wantNames:    []string{"b"},
			wantProvable: true,
		},
		{
			name: "empty member list leaves the name unproven",
			indexes: []Index{
				{Kind: IndexKindUnique},
			},
			wantProvable: false,
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			resolved, provable := StoredIndexNames(tc.indexes)
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
