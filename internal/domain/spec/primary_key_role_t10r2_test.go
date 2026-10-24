// Package spec tests the T10-R2 corrections to the shared primary-role
// resolution and member-legality assessment: the four-state role answer
// (explicit primary / implicit holder / no candidate / unknown), pointer
// identity for unnamed members, TiDB PKIsHandle three-state knownness,
// kind-aware member legality (FULLTEXT/SPATIAL vs ordinary kinds), and the
// prefix-length and multi-valued cast checks pinned against MySQL 8.0.46
// and TiDB v8.5.0 (2026-10 native probes).
// input: constructed snapshots and declared statements exercising every
// state the resolution and member checks can reach
// output: determined issues, bounded gaps, and the UnresolvedTransition
// publication signal — never a fabricated refusal or pass
// pos: spec-level T10-R2 decision-table coverage beside the audit-level
// chain tests
// note: if this file changes, update this header and module README.md.
package spec

import "testing"

// t10r2Snapshot builds an existing-table snapshot whose member facts are all
// vouched: declared columns plus optional primary key and index members.
func t10r2Snapshot(columns []Column, pk *Index, indexes ...Index) *TableSnapshot {
	return &TableSnapshot{
		Exists:     true,
		Table:      &Table{Schema: "golden", Name: "t"},
		Columns:    columns,
		PrimaryKey: pk,
		Indexes:    indexes,
	}
}

// t10r2AddUnique builds an ALTER TABLE ADD UNIQUE statement for a snapshot.
func t10r2AddUniqueStatement(dialect Dialect, index Index, snapshot *TableSnapshot, version *VersionIdentity) Statement {
	return Statement{
		Dialect:  dialect,
		Kind:     KindDDL,
		Metadata: &Metadata{Version: version, TargetTable: snapshot},
		DDL: &DDL{
			Operation: DDLOperationAlterTable,
			Table:     &Table{Schema: "golden", Name: "t"},
			Alter: []Alter{{
				Action: "add_index",
				Index:  &AlterIndex{Definition: &index},
			}},
		},
	}
}

// t10r2CreateTable builds a CREATE TABLE statement from the declared shape.
func t10r2CreateTable(dialect Dialect, columns []Column, pk *Index, version *VersionIdentity, indexes ...Index) Statement {
	return Statement{
		Dialect:  dialect,
		Kind:     KindDDL,
		Metadata: &Metadata{Version: version},
		DDL: &DDL{
			Operation:  DDLOperationCreateTable,
			Table:      &Table{Schema: "golden", Name: "t"},
			Columns:    columns,
			PrimaryKey: pk,
			Indexes:    indexes,
		},
	}
}

func t10r2ColumnMember(name string, ordinal int) IndexKeyPart {
	return IndexKeyPart{Ordinal: ordinal, Kind: IndexKeyPartColumn, Column: name}
}

// B1: an explicit primary key exists — a later UNIQUE index can never hold
// the implicit role, so INVISIBLE on it is a legal no-issue statement on
// both entry points (native rc:0 on MySQL 8.0/8.4 and TiDB 8.5).
func TestT10R2ExplicitPrimaryNewUniqueInvisibleAllowed(t *testing.T) {
	t.Parallel()
	for _, dialect := range []Dialect{DialectMySQL, DialectTiDB} {
		snapshot := t10r2Snapshot(
			[]Column{{Name: "id", Type: "int", NotNull: true}, {Name: "a", Type: "int", NotNull: true}},
			&Index{Name: "primary", Kind: IndexKindPrimary, Columns: []string{"id"}},
		)
		idx := Index{
			Name: "u_a", Kind: IndexKindUnique, Columns: []string{"a"},
			Visibility: ColumnVisibilityInvisible, VisibilityExplicit: true,
		}
		idx.SetKeyParts([]IndexKeyPart{t10r2ColumnMember("a", 1)})
		a := AssessAdvancedIndexes(t10r2AddUniqueStatement(dialect, idx, snapshot,
			t10Version(versionProduct(dialect), "8.0.46", true)))
		if t10IssueReasons(a)[AdvancedIndexReasonPrimaryVisibility] {
			t.Fatalf("%s: explicit PK must make new unique invisible legal, got issues=%+v", dialect, a.Issues)
		}
		if len(a.Gaps) != 0 {
			t.Fatalf("%s: fully vouched shape must leave no gaps, got %+v", dialect, a.Gaps)
		}
	}
}

// B1 negative control: no explicit PK and no earlier candidate — the declared
// UNIQUE INVISIBLE itself becomes the holder and is refused (native 3522).
func TestT10R2NoPrimaryFirstCandidateStillRefused(t *testing.T) {
	t.Parallel()
	for _, dialect := range []Dialect{DialectMySQL, DialectTiDB} {
		snapshot := t10r2Snapshot(
			[]Column{{Name: "a", Type: "int", NotNull: true}},
			nil,
		)
		idx := Index{
			Name: "u_a", Kind: IndexKindUnique, Columns: []string{"a"},
			Visibility: ColumnVisibilityInvisible, VisibilityExplicit: true,
		}
		idx.SetKeyParts([]IndexKeyPart{t10r2ColumnMember("a", 1)})
		a := AssessAdvancedIndexes(t10r2AddUniqueStatement(dialect, idx, snapshot,
			t10Version(versionProduct(dialect), "8.0.46", true)))
		if !t10IssueReasons(a)[AdvancedIndexReasonPrimaryVisibility] || !a.InvalidTransition {
			t.Fatalf("%s: first qualifying unique must stay refused, got issues=%+v", dialect, a.Issues)
		}
	}
}

// B1 companion: with an existing implicit holder (earlier qualifying UNIQUE),
// a new UNIQUE INVISIBLE is also legal — the holder keeps the role.
func TestT10R2ExistingHolderNewUniqueInvisibleAllowed(t *testing.T) {
	t.Parallel()
	for _, dialect := range []Dialect{DialectMySQL, DialectTiDB} {
		first := Index{Name: "u_a", Kind: IndexKindUnique, Columns: []string{"a"}}
		first.SetKeyParts([]IndexKeyPart{t10r2ColumnMember("a", 1)})
		snapshot := t10r2Snapshot(
			[]Column{{Name: "a", Type: "int", NotNull: true}, {Name: "b", Type: "int", NotNull: true}},
			nil, first,
		)
		snapshot.IndexOrderCatalog = true
		idx := Index{
			Name: "u_b", Kind: IndexKindUnique, Columns: []string{"b"},
			Visibility: ColumnVisibilityInvisible, VisibilityExplicit: true,
		}
		idx.SetKeyParts([]IndexKeyPart{t10r2ColumnMember("b", 1)})
		a := AssessAdvancedIndexes(t10r2AddUniqueStatement(dialect, idx, snapshot,
			t10Version(versionProduct(dialect), "8.0.46", true)))
		if t10IssueReasons(a)[AdvancedIndexReasonPrimaryVisibility] {
			t.Fatalf("%s: earlier holder keeps the role — new unique must be legal, got %+v", dialect, a.Issues)
		}
	}
}

// B2: two unnamed UNIQUE members in one declared shape must not collapse
// into one holder — only the first qualifying member carries the role, so
// INVISIBLE on the second is legal while INVISIBLE on the first refuses.
// Holder name alone (both "") can never decide this.
func TestT10R2UnnamedUniqueMembersDistinctHolders(t *testing.T) {
	t.Parallel()
	for _, dialect := range []Dialect{DialectMySQL, DialectTiDB} {
		second := Index{Name: "", Kind: IndexKindUnique, Columns: []string{"b"},
			Visibility: ColumnVisibilityInvisible, VisibilityExplicit: true}
		second.SetKeyParts([]IndexKeyPart{t10r2ColumnMember("b", 1)})
		stmt := t10r2CreateTable(dialect,
			[]Column{{Name: "a", Type: "int", NotNull: true}, {Name: "b", Type: "int", NotNull: true}},
			nil,
			t10Version(versionProduct(dialect), "8.0.46", true),
			Index{Name: "", Kind: IndexKindUnique, Columns: []string{"a"}},
			second,
		)
		stmt.DDL.Indexes[0].SetKeyParts([]IndexKeyPart{t10r2ColumnMember("a", 1)})
		a := AssessAdvancedIndexes(stmt)
		if t10IssueReasons(a)[AdvancedIndexReasonPrimaryVisibility] {
			t.Fatalf("%s: the second unnamed unique must not inherit the first's role, got %+v", dialect, a.Issues)
		}
	}
}

// B2 negative control: INVISIBLE declared on the first unnamed member is
// still the role-holder refusal.
func TestT10R2UnnamedFirstUniqueInvisibleRefused(t *testing.T) {
	t.Parallel()
	for _, dialect := range []Dialect{DialectMySQL, DialectTiDB} {
		first := Index{Name: "", Kind: IndexKindUnique, Columns: []string{"a"},
			Visibility: ColumnVisibilityInvisible, VisibilityExplicit: true}
		first.SetKeyParts([]IndexKeyPart{t10r2ColumnMember("a", 1)})
		stmt := t10r2CreateTable(dialect,
			[]Column{{Name: "a", Type: "int", NotNull: true}, {Name: "b", Type: "int", NotNull: true}},
			nil,
			t10Version(versionProduct(dialect), "8.0.46", true),
			first,
			Index{Name: "", Kind: IndexKindUnique, Columns: []string{"b"}},
		)
		stmt.DDL.Indexes[1].SetKeyParts([]IndexKeyPart{t10r2ColumnMember("b", 1)})
		a := AssessAdvancedIndexes(stmt)
		if !t10IssueReasons(a)[AdvancedIndexReasonPrimaryVisibility] || !a.InvalidTransition {
			t.Fatalf("%s: the first unnamed unique holds the role — must refuse, got %+v", dialect, a.Issues)
		}
	}
}

// B3: an unvouched primary-key collection leaves the role unknown — the
// invisibility assessment emits the bounded gap AND marks the transition
// unresolved so no post-state may claim the member landed.
func TestT10R2UnknownPrimaryBlocksPublication(t *testing.T) {
	t.Parallel()
	for _, dialect := range []Dialect{DialectMySQL, DialectTiDB} {
		snapshot := t10r2Snapshot(
			[]Column{{Name: "a", Type: "int", NotNull: true}},
			nil,
		)
		snapshot.PrimaryKeyUnknown = true
		idx := Index{
			Name: "u_a", Kind: IndexKindUnique, Columns: []string{"a"},
			Visibility: ColumnVisibilityInvisible, VisibilityExplicit: true,
		}
		idx.SetKeyParts([]IndexKeyPart{t10r2ColumnMember("a", 1)})
		a := AssessAdvancedIndexes(t10r2AddUniqueStatement(dialect, idx, snapshot,
			t10Version(versionProduct(dialect), "8.0.46", true)))
		if !t10GapReasons(a)[AdvancedReasonUnknownTableState] {
			t.Fatalf("%s: unvouched primary key must surface the bounded gap, got %+v", dialect, a.Gaps)
		}
		if !a.UnresolvedTransition {
			t.Fatalf("%s: an unresolved role must block post-state publication, got %+v", dialect, a)
		}
		if t10IssueReasons(a)[AdvancedIndexReasonPrimaryVisibility] {
			t.Fatalf("%s: unknown must never fabricate the refusal, got %+v", dialect, a.Issues)
		}
	}
}

// B4: the TiDB handle answer is three-state — explicit CLUSTERED/declared
// int decides, NONCLUSTERED is a real object, and an unwritten type follows
// the observed tidb_enable_clustered_index default (on → handle, off → real
// object, unread → unknown; int_only additionally defers to the
// alter-primary-key fact — T10-R3).
func TestT10R2PrimaryKeyHandleTriState(t *testing.T) {
	t.Parallel()
	columns := []Column{{Name: "id", Type: "int(11)", NotNull: true}}
	on, off, intOnly := "on", "off", "int_only"
	alterFalse := false
	cases := []struct {
		name   string
		pkType string
		mode   *string
		apk    *bool
		want   *bool
	}{
		{name: "explicit clustered single-int", pkType: "clustered", want: boolPtr(true)},
		{name: "explicit nonclustered single-int", pkType: "nonclustered", want: boolPtr(false)},
		{name: "unstated + mode on", mode: &on, want: boolPtr(true)},
		{name: "unstated + mode off", mode: &off, want: boolPtr(false)},
		{name: "unstated + mode int_only + alter_pk off", mode: &intOnly, apk: &alterFalse, want: boolPtr(true)},
		{name: "unstated + mode int_only + alter_pk unobserved", mode: &intOnly, want: nil},
		{name: "unstated + mode unknown", mode: nil, want: nil},
	}
	for _, tc := range cases {
		pk := &Index{Kind: IndexKindPrimary, Columns: []string{"id"}, PrimaryKeyType: tc.pkType}
		got := PrimaryKeyHandle(DialectTiDB, pk, columns, tc.mode, tc.apk)
		if (got == nil) != (tc.want == nil) || (got != nil && tc.want != nil && *got != *tc.want) {
			t.Fatalf("%s: PrimaryKeyHandle = %v, want %v", tc.name, got, tc.want)
		}
	}
	// Non-single-int PKs are never the handle regardless of mode.
	pk := &Index{Kind: IndexKindPrimary, Columns: []string{"id", "id"}}
	if got := PrimaryKeyHandle(DialectTiDB, pk, columns, &on, nil); got == nil || *got {
		t.Fatalf("composite PK is never the row handle, got %v", got)
	}
	varcharColumns := []Column{{Name: "id", Type: "varchar(10)", NotNull: true}}
	one := &Index{Kind: IndexKindPrimary, Columns: []string{"id"}}
	if got := PrimaryKeyHandle(DialectTiDB, one, varcharColumns, &on, nil); got == nil || *got {
		t.Fatalf("non-integer PK is never the row handle, got %v", got)
	}
	// MySQL never produces a handle.
	if got := PrimaryKeyHandle(DialectMySQL, one, columns, &on, nil); got == nil || *got {
		t.Fatalf("MySQL always keeps a real PRIMARY object, got %v", got)
	}
}

// B4 rule consumer: an unknown handle must not resolve the PRIMARY object —
// ResolveIndexPrimaryObject answers unresolved so callers emit the gap
// instead of asserting a real index object exists.
func TestT10R2PrimaryObjectUnknownHandle(t *testing.T) {
	t.Parallel()
	snapshot := t10r2Snapshot(
		[]Column{{Name: "id", Type: "int", NotNull: true}},
		&Index{Name: "primary", Kind: IndexKindPrimary, Columns: []string{"id"}},
	)
	snapshot.PrimaryKeyHandleUnknown = true
	exists, resolved := ResolveIndexPrimaryObject(snapshot, DialectTiDB)
	if resolved || exists {
		t.Fatalf("unknown handle must not resolve the PRIMARY object, got exists=%v resolved=%v", exists, resolved)
	}
}

// C1: a legal FULLTEXT member on TEXT must not hit prefix_required — the
// kind-specific family check exempts the prefix-length contract (native
// rc:0 while a plain index on the same column fails 1170).
func TestT10R2FulltextTextNoPrefixRequired(t *testing.T) {
	t.Parallel()
	snapshot := t10r2Snapshot(
		[]Column{{Name: "body", Type: "text", NotNull: true}},
		nil,
	)
	idx := Index{Name: "ft", Kind: IndexKindFulltext, Columns: []string{"body"}}
	idx.SetKeyParts([]IndexKeyPart{t10r2ColumnMember("body", 1)})
	a := AssessAdvancedIndexes(t10r2AddUniqueStatement(DialectMySQL, idx, snapshot,
		t10Version("mysql", "8.0.46", true)))
	if t10IssueReasons(a)[AdvancedIndexReasonPrefixRequired] || t10IssueReasons(a)[AdvancedIndexReasonFulltextColumnType] {
		t.Fatalf("legal FULLTEXT(TEXT) must stay clean, got issues=%+v", a.Issues)
	}
	if len(a.Gaps) != 0 {
		t.Fatalf("vouched member must leave no gaps, got %+v", a.Gaps)
	}
}

// C2 negative control: a plain secondary index on TEXT keeps prefix_required
// (native 1170), and FULLTEXT on INT keeps fulltext_column_type (1283).
func TestT10R2MemberLegalityKindRouting(t *testing.T) {
	t.Parallel()
	snapshot := t10r2Snapshot(
		[]Column{{Name: "body", Type: "text", NotNull: true}, {Name: "n", Type: "int", NotNull: true}},
		nil,
	)
	plain := Index{Name: "i_body", Kind: IndexKindSecondary, Columns: []string{"body"}}
	plain.SetKeyParts([]IndexKeyPart{t10r2ColumnMember("body", 1)})
	a := AssessAdvancedIndexes(t10r2AddUniqueStatement(DialectMySQL, plain, snapshot,
		t10Version("mysql", "8.0.46", true)))
	if !t10IssueReasons(a)[AdvancedIndexReasonPrefixRequired] {
		t.Fatalf("plain TEXT member must keep prefix_required, got %+v", a.Issues)
	}
	ft := Index{Name: "ft_n", Kind: IndexKindFulltext, Columns: []string{"n"}}
	ft.SetKeyParts([]IndexKeyPart{t10r2ColumnMember("n", 1)})
	a = AssessAdvancedIndexes(t10r2AddUniqueStatement(DialectMySQL, ft, snapshot,
		t10Version("mysql", "8.0.46", true)))
	if !t10IssueReasons(a)[AdvancedIndexReasonFulltextColumnType] {
		t.Fatalf("FULLTEXT(INT) must emit fulltext_column_type, got %+v", a.Issues)
	}
}

// C3: a FULLTEXT member whose column set is vouched but type is empty must
// attribute the withheld answer to fulltext — never a filtered-out
// prefix_length gap that leaves the enabled check gapless.
func TestT10R2FulltextUnknownTypeGapAttribution(t *testing.T) {
	t.Parallel()
	snapshot := t10r2Snapshot(
		[]Column{{Name: "body"}},
		nil,
	)
	ft := Index{Name: "ft", Kind: IndexKindFulltext, Columns: []string{"body"}}
	ft.SetKeyParts([]IndexKeyPart{t10r2ColumnMember("body", 1)})
	a := AssessAdvancedIndexes(t10r2AddUniqueStatement(DialectMySQL, ft, snapshot,
		t10Version("mysql", "8.0.46", true)))
	fulltextGap, prefixGap := false, false
	for _, gap := range a.Gaps {
		if gap.Feature == AdvancedIndexFeatureFulltext {
			fulltextGap = true
		}
		if gap.Feature == AdvancedIndexFeaturePrefixLength {
			prefixGap = true
		}
	}
	if !fulltextGap || prefixGap {
		t.Fatalf("unknown member type on FT must gap fulltext, not prefix_length, got %+v", a.Gaps)
	}
}

// A-prefix: the legality checks pinned natively — prefix on a non-string
// column (1089), prefix 0 (1391), prefix past the declared length (1089),
// and the legal varchar prefix staying clean.
func TestT10R2PrefixLegality(t *testing.T) {
	t.Parallel()
	snapshot := t10r2Snapshot(
		[]Column{
			{Name: "a", Type: "varchar(5)", Length: 5, NotNull: true},
			{Name: "n", Type: "int", NotNull: true},
		},
		nil,
	)
	prefix := func(n int) *int { return &n }
	cases := []struct {
		name       string
		member     IndexKeyPart
		wantReason string
	}{
		{name: "prefix on int column", member: IndexKeyPart{Ordinal: 1, Kind: IndexKeyPartColumn, Column: "n", PrefixLength: prefix(2)}, wantReason: AdvancedIndexReasonPrefixNotString},
		{name: "prefix zero", member: IndexKeyPart{Ordinal: 1, Kind: IndexKeyPartColumn, Column: "a", PrefixLength: prefix(0)}, wantReason: AdvancedIndexReasonPrefixZero},
		{name: "prefix beyond declared length", member: IndexKeyPart{Ordinal: 1, Kind: IndexKeyPartColumn, Column: "a", PrefixLength: prefix(20)}, wantReason: AdvancedIndexReasonPrefixOverLength},
		{name: "legal varchar prefix", member: IndexKeyPart{Ordinal: 1, Kind: IndexKeyPartColumn, Column: "a", PrefixLength: prefix(3)}},
	}
	for _, dialect := range []Dialect{DialectMySQL, DialectTiDB} {
		for _, tc := range cases {
			idx := Index{Name: "i", Kind: IndexKindSecondary}
			idx.SetKeyParts([]IndexKeyPart{tc.member})
			a := AssessAdvancedIndexes(t10r2AddUniqueStatement(dialect, idx, snapshot,
				t10Version(versionProduct(dialect), "8.0.46", true)))
			if tc.wantReason == "" {
				if len(a.Issues) != 0 {
					t.Fatalf("%s %s: legal prefix must stay clean, got %+v", dialect, tc.name, a.Issues)
				}
				continue
			}
			if !t10IssueReasons(a)[tc.wantReason] {
				t.Fatalf("%s %s: expected %s, got %+v", dialect, tc.name, tc.wantReason, a.Issues)
			}
		}
	}
}

// A-multi-valued: the cast-target legality is dialect-split — MySQL accepts
// decimal but not double, TiDB accepts double but not decimal; both refuse
// json/year/float and any char/binary without a length (native 1235).
func TestT10R2MultiValuedCastLegality(t *testing.T) {
	t.Parallel()
	snapshot := t10r2Snapshot(
		[]Column{{Name: "j", Type: "json"}},
		nil,
	)
	mvPart := func(family string, lenSet bool) IndexKeyPart {
		return IndexKeyPart{
			Ordinal: 1, Kind: IndexKeyPartExpression, MultiValued: true,
			MultiValuedCastType: family, MultiValuedCastLengthSet: lenSet,
			Expression: &ExpressionFacts{Known: true, References: []ColumnReference{{Name: "j"}}},
		}
	}
	cases := []struct {
		family  string
		lenSet  bool
		mysqlOK bool
		tidbOK  bool
	}{
		{"signed", false, true, true},
		{"unsigned", false, true, true},
		{"char", true, true, true},
		{"char", false, false, false},
		{"binary", true, true, true},
		{"binary", false, false, false},
		{"date", false, true, true},
		{"time", false, true, true},
		{"datetime", false, true, true},
		{"decimal", false, true, false},
		{"decimal", true, true, false},
		{"double", false, false, true},
		{"float", false, false, false},
		{"json", false, false, false},
		{"year", false, false, false},
	}
	for _, tc := range cases {
		for _, dialect := range []Dialect{DialectMySQL, DialectTiDB} {
			wantOK := tc.mysqlOK
			if dialect == DialectTiDB {
				wantOK = tc.tidbOK
			}
			idx := Index{Name: "mv", Kind: IndexKindSecondary}
			idx.SetKeyParts([]IndexKeyPart{mvPart(tc.family, tc.lenSet)})
			a := AssessAdvancedIndexes(t10r2AddUniqueStatement(dialect, idx, snapshot,
				t10Version(versionProduct(dialect), "8.0.46", true)))
			if wantOK && len(a.Issues) != 0 {
				t.Fatalf("%s mv %s(len=%v): must stay legal, got %+v", dialect, tc.family, tc.lenSet, a.Issues)
			}
			if !wantOK && !t10IssueReasons(a)[AdvancedIndexReasonMultiValuedCastType] {
				t.Fatalf("%s mv %s(len=%v): expected multi_valued_cast_type, got %+v", dialect, tc.family, tc.lenSet, a.Issues)
			}
		}
	}
	// An unclassified cast target is a bounded gap, never a guessed verdict.
	idx := Index{Name: "mv", Kind: IndexKindSecondary}
	idx.SetKeyParts([]IndexKeyPart{mvPart("vector", false)})
	a := AssessAdvancedIndexes(t10r2AddUniqueStatement(DialectMySQL, idx, snapshot,
		t10Version("mysql", "8.0.46", true)))
	if t10IssueReasons(a)[AdvancedIndexReasonMultiValuedCastType] {
		t.Fatalf("unrecognized cast family must not fabricate a refusal, got %+v", a.Issues)
	}
	gapped := false
	for _, gap := range a.Gaps {
		if gap.Feature == AdvancedIndexFeatureMultiValued {
			gapped = true
		}
	}
	if !gapped {
		t.Fatalf("unrecognized cast family must gap multi_valued, got %+v", a.Gaps)
	}
}

// B-resolution shape: the four states surface distinctly — explicit primary,
// implicit holder (with pointer identity), no candidate, unknown.
func TestT10R2PrimaryRoleFourStates(t *testing.T) {
	t.Parallel()
	columns := []Column{{Name: "a", Type: "int", NotNull: true}, {Name: "b", Type: "int", NotNull: true}}
	holder := Index{Name: "u_a", Kind: IndexKindUnique, Columns: []string{"a"}}
	holder.SetKeyParts([]IndexKeyPart{t10r2ColumnMember("a", 1)})
	second := Index{Name: "", Kind: IndexKindUnique, Columns: []string{"b"}}
	second.SetKeyParts([]IndexKeyPart{t10r2ColumnMember("b", 1)})

	// Explicit primary.
	withPK := t10r2Snapshot(columns, &Index{Kind: IndexKindPrimary, Columns: []string{"a"}}, holder)
	role := ResolvePrimaryRole(DialectMySQL, withPK, true)
	if !role.Resolved || !role.ExplicitPrimary || role.HasHolder {
		t.Fatalf("explicit PK must resolve as explicit primary, got %+v", role)
	}

	// Implicit holder with pointer identity into the caller's slice.
	noPK := t10r2Snapshot(columns, nil, holder, second)
	role = ResolvePrimaryRole(DialectMySQL, noPK, true)
	if !role.Resolved || role.ExplicitPrimary || !role.HasHolder || role.Holder != "u_a" {
		t.Fatalf("first qualifying candidate must hold the role, got %+v", role)
	}
	if role.HolderIndex != &noPK.Indexes[0] {
		t.Fatalf("holder identity must point at the actual member, got %+v", role)
	}

	// No candidate: every unique member nullable.
	nullable := Index{Name: "u_a", Kind: IndexKindUnique, Columns: []string{"b"}}
	nullable.SetKeyParts([]IndexKeyPart{t10r2ColumnMember("b", 1)})
	none := t10r2Snapshot([]Column{{Name: "b", Type: "int"}}, nil, nullable)
	role = ResolvePrimaryRole(DialectMySQL, none, true)
	if !role.Resolved || role.ExplicitPrimary || role.HasHolder {
		t.Fatalf("nullable unique is no candidate, got %+v", role)
	}

	// Unknown: unvouched primary-key collection.
	unknown := t10r2Snapshot(columns, nil)
	unknown.PrimaryKeyUnknown = true
	role = ResolvePrimaryRole(DialectMySQL, unknown, true)
	if role.Resolved || len(role.GapFacts) == 0 {
		t.Fatalf("unvouched primary key must stay unresolved, got %+v", role)
	}
}

// helper: version product string for a dialect.
func versionProduct(dialect Dialect) string {
	if dialect == DialectTiDB {
		return "tidb"
	}
	return "mysql"
}

func boolPtr(v bool) *bool { return &v }
