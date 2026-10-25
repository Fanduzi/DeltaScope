// Package mysql verifies the T10-R4 provider fact corrections: a STATISTICS
// expression member whose COLLATION is NULL or unrecognized is still an
// expression key part — kind, identity, dependencies, and multi-valued
// marks survive while only the direction answer is withheld — the
// TIDB_INDEXES clustered row answers the clustered-storage fact separately
// from the handle answer (a common-handle PK is a clustered real object),
// and the table_constraints pk-presence matrix binds a `primary`-fold
// member row to the PK identity only under a proven PRIMARY KEY constraint
// with exactly one fold (proven-absent keeps folds secondary, zero folds
// prove no PK object, unanswerable or multi-fold states withhold all three
// primary facts).
// input: sqlmock-shaped member rows across the direction matrix,
// TIDB_INDEXES clustered rows across PK member shapes, and the
// table_constraints PRIMARY KEY count matrix
// output: KeyParts kind=expression + DirectionUnknown and derived
// aggregates, PrimaryKeyClustered/PrimaryKeyClusteredUnknown facts, and
// `primary`-fold binding verdicts (snapshot.PrimaryKey promotion or the
// three primary unknowns withheld together)
// pos: provider-side T10-R4 member/catalog fidelity tests (issue #89)
// note: if this file changes, update this header and module README.md.
package mysqlmeta

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// D: an unproven direction does not demote the member — the catalog row is
// still an expression key part whose sort direction the model cannot
// assert, so kind stays expression and DirectionUnknown records the
// withheld half. Identity, references, and multi-valued marks are
// unaffected by the missing direction.
func TestT10R4DirectionUnknownExpressionKeepsIdentity(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name                 string
		collation            sql.NullString
		wantDesc             bool
		wantDirectionUnknown bool
	}{
		{name: "NULL collation", collation: t10r2NullStr(), wantDirectionUnknown: true},
		{name: "unrecognized collation", collation: t10r2Str("X"), wantDirectionUnknown: true},
		{name: "A ascending proven", collation: t10r2Str("A"), wantDirectionUnknown: false},
		{name: "D descending proven", collation: t10r2Str("D"), wantDesc: true, wantDirectionUnknown: false},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			indexes := make(map[string]*spec.Index)
			order := make([]string, 0)
			accumulateIndexRow(indexes, &order, spec.DialectMySQL, "i_e", 1, "BTREE",
				indexMemberRow{
					columnName: t10r2NullStr(), collation: tc.collation,
					expression: t10r2Str("`b`+1"),
				}, nil, nil)
			index := indexes["i_e"]
			index.SetKeyParts(index.KeyParts)
			if len(index.KeyParts) != 1 {
				t.Fatalf("KeyParts = %d, want 1", len(index.KeyParts))
			}
			part := index.KeyParts[0]
			if part.Kind != spec.IndexKeyPartExpression {
				t.Fatalf("an unproven direction keeps kind=expression, got %s", part.Kind)
			}
			if part.DirectionUnknown != tc.wantDirectionUnknown {
				t.Fatalf("DirectionUnknown = %v, want %v", part.DirectionUnknown, tc.wantDirectionUnknown)
			}
			if part.Descending != tc.wantDesc {
				t.Fatalf("Descending = %v, want %v", part.Descending, tc.wantDesc)
			}
			if part.Expression == nil {
				t.Fatalf("expression identity must survive the missing direction, got %+v", part)
			}
			if index.ExpressionCount != 1 || index.UnmodeledKeyParts != 0 {
				t.Fatalf("aggregate marks must count the expression member, got %+v", index)
			}
			wantDirParts := 0
			if tc.wantDirectionUnknown {
				wantDirParts = 1
			}
			if index.DirectionUnknownParts != wantDirParts {
				t.Fatalf("DirectionUnknownParts = %d, want %d", index.DirectionUnknownParts, wantDirParts)
			}
		})
	}
}

// D: multi-valued marks survive across the whole direction matrix — a
// cast-array expression keeps its MV aspect and counts whether the
// direction is proven (A/D) or withheld (NULL/unrecognized).
func TestT10R4DirectionUnknownMultiValuedKeepsMarks(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name                 string
		collation            sql.NullString
		wantDirectionUnknown bool
	}{
		{name: "NULL collation", collation: t10r2NullStr(), wantDirectionUnknown: true},
		{name: "unrecognized collation", collation: t10r2Str("X"), wantDirectionUnknown: true},
		{name: "A ascending proven", collation: t10r2Str("A"), wantDirectionUnknown: false},
		{name: "D descending proven", collation: t10r2Str("D"), wantDirectionUnknown: false},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			indexes := make(map[string]*spec.Index)
			order := make([]string, 0)
			accumulateIndexRow(indexes, &order, spec.DialectMySQL, "i_mv", 1, "BTREE",
				indexMemberRow{
					columnName: t10r2NullStr(), collation: tc.collation,
					expression: t10r2Str("cast(`a` as unsigned array)"),
				}, nil, nil)
			index := indexes["i_mv"]
			index.SetKeyParts(index.KeyParts)
			part := index.KeyParts[0]
			if part.Kind != spec.IndexKeyPartExpression {
				t.Fatalf("multi-valued member kind = %s, want expression", part.Kind)
			}
			if part.DirectionUnknown != tc.wantDirectionUnknown {
				t.Fatalf("DirectionUnknown = %v, want %v", part.DirectionUnknown, tc.wantDirectionUnknown)
			}
			if !part.MultiValued || index.MultiValuedParts != 1 || part.MultiValuedCastType == "" {
				t.Fatalf("multi-valued marks must survive the missing direction, got %+v", index)
			}
			if index.ExpressionCount != 1 || index.UnmodeledKeyParts != 0 {
				t.Fatalf("aggregates must stay consistent with the member facts, got %+v", index)
			}
			wantDirParts := 0
			if tc.wantDirectionUnknown {
				wantDirParts = 1
			}
			if index.DirectionUnknownParts != wantDirParts {
				t.Fatalf("DirectionUnknownParts = %d, want %d", index.DirectionUnknownParts, wantDirParts)
			}
		})
	}
}

// D control: a named member with a NULL collation still stays unmodeled —
// the direction-unknown fix reaches expression members only, never
// relabels an unclassifiable named row as an expression.
func TestT10R4NamedUnmodeledMemberStaysUnmodeled(t *testing.T) {
	t.Parallel()
	columns := []spec.Column{{Name: "b", Type: "int", NotNull: true}}
	indexes := make(map[string]*spec.Index)
	order := make([]string, 0)
	accumulateIndexRow(indexes, &order, spec.DialectMySQL, "i_b", 1, "BTREE",
		indexMemberRow{columnName: t10r2Str("b"), collation: t10r2NullStr()},
		nil, columns)
	index := indexes["i_b"]
	index.SetKeyParts(index.KeyParts)
	part := index.KeyParts[0]
	if part.Kind != spec.IndexKeyPartUnmodeled || part.Column != "b" {
		t.Fatalf("named NULL-collation member = %+v, want unmodeled column member", part)
	}
	if index.UnmodeledKeyParts != 1 || index.ExpressionCount != 0 || index.DirectionUnknownParts != 0 {
		t.Fatalf("named unmodeled member must not gain expression marks, got %+v", index)
	}
}

// C: the TIDB_INDEXES clustered row answers clustered storage separately
// from the handle question — a clustered non-single-int PK is a
// common-handle real object (clustered=true, handle=false), a clustered
// single-int is both, and a missing/NULL row leaves the clustered answer
// unknown exactly like the handle answer.
func TestT10R4TiDBClusteredFactMatrix(t *testing.T) {
	t.Parallel()
	base := func(columnRows, statRows [][]driver.Value) map[string]testQueryResult {
		return map[string]testQueryResult{
			"select engine, table_collation": {
				columns: []string{"engine", "table_collation", "table_comment", "auto_increment", "row_format", "table_rows"},
				rows:    [][]driver.Value{{"InnoDB", "utf8mb4_bin", "", nil, "", nil}},
			},
			"information_schema.columns": {
				columns: []string{"column_name", "column_type", "character_set_name", "collation_name", "column_comment", "column_default", "is_nullable", "extra", "generation_expression"},
				rows:    columnRows,
			},
			"column_name = 'IS_VISIBLE'": {
				columns: []string{"count"},
				rows:    [][]driver.Value{{1}},
			},
			"column_name = 'EXPRESSION'": {
				columns: []string{"count"},
				rows:    [][]driver.Value{{1}},
			},
			"information_schema.statistics": {
				columns: []string{"index_name", "non_unique", "index_type", "column_name", "cardinality", "sub_part", "collation", "is_visible", "expression"},
				rows:    statRows,
			},
			"information_schema.partitions": {
				columns: []string{"count"},
				rows:    [][]driver.Value{{0}},
			},
			"table_constraints": {
				columns: []string{"count"},
				rows:    [][]driver.Value{{1}},
			},
		}
	}
	clusteredRow := func(value driver.Value) testQueryResult {
		return testQueryResult{columns: []string{"clustered"}, rows: [][]driver.Value{{value}}}
	}
	varcharPK := func() (cols, stats [][]driver.Value) {
		return [][]driver.Value{{"id", "varchar(32)", nil, nil, "", nil, "NO", "", nil}},
			[][]driver.Value{{"PRIMARY", 0, "BTREE", "id", nil, nil, "A", "YES", nil}}
	}
	intPK := func() (cols, stats [][]driver.Value) {
		return [][]driver.Value{{"id", "int(11)", nil, nil, "", nil, "NO", "", nil}},
			[][]driver.Value{{"PRIMARY", 0, "BTREE", "id", nil, nil, "A", "YES", nil}}
	}
	compositePK := func() (cols, stats [][]driver.Value) {
		return [][]driver.Value{
				{"id", "int(11)", nil, nil, "", nil, "NO", "", nil},
				{"b", "varchar(32)", nil, nil, "", nil, "NO", "", nil},
			}, [][]driver.Value{
				{"PRIMARY", 0, "BTREE", "id", nil, nil, "A", "YES", nil},
				{"PRIMARY", 0, "BTREE", "b", nil, nil, "A", "YES", nil},
			}
	}
	cases := []struct {
		name             string
		queries          map[string]testQueryResult
		wantClustered    bool
		wantClusteredUnk bool
		wantHandle       bool
		wantHandleUnk    bool
	}{
		{
			name:          "clustered yes + varchar PK is a common-handle real object (P5)",
			queries:       withQuery(base(varcharPK()), "information_schema.tidb_indexes", clusteredRow("YES")),
			wantClustered: true, wantHandle: false,
		},
		{
			name:          "clustered yes + composite PK is a common-handle real object (P6)",
			queries:       withQuery(base(compositePK()), "information_schema.tidb_indexes", clusteredRow("YES")),
			wantClustered: true, wantHandle: false,
		},
		{
			name:          "clustered yes + single int is the handle",
			queries:       withQuery(base(intPK()), "information_schema.tidb_indexes", clusteredRow("YES")),
			wantClustered: true, wantHandle: true,
		},
		{
			name: "clustered no is nonclustered storage",
			queries: withQuery(
				withQuery(base(varcharPK()), "information_schema.tidb_indexes", clusteredRow("NO")),
				"tidb_pk_type", testQueryResult{columns: []string{"tidb_pk_type"}, rows: [][]driver.Value{{"NONCLUSTERED"}}}),
			wantClustered: false, wantHandle: false,
		},
		{
			name:             "missing clustered row leaves both answers unknown",
			queries:          base(varcharPK()),
			wantClusteredUnk: true, wantHandleUnk: true,
		},
		{
			name:             "NULL clustered value leaves both answers unknown",
			queries:          withQuery(base(varcharPK()), "information_schema.tidb_indexes", clusteredRow(nil)),
			wantClusteredUnk: true, wantHandleUnk: true,
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			db := openTestDB(t, tc.queries)
			defer db.Close()
			snapshot, err := NewProvider(db).LoadTableSnapshot(context.Background(), spec.DialectTiDB, "app", "t")
			if err != nil {
				t.Fatalf("LoadTableSnapshot: %v", err)
			}
			if snapshot.PrimaryKeyClustered != tc.wantClustered {
				t.Fatalf("PrimaryKeyClustered = %v, want %v", snapshot.PrimaryKeyClustered, tc.wantClustered)
			}
			if snapshot.PrimaryKeyClusteredUnknown != tc.wantClusteredUnk {
				t.Fatalf("PrimaryKeyClusteredUnknown = %v, want %v",
					snapshot.PrimaryKeyClusteredUnknown, tc.wantClusteredUnk)
			}
			if snapshot.PrimaryKeyIsHandle != tc.wantHandle || snapshot.PrimaryKeyHandleUnknown != tc.wantHandleUnk {
				t.Fatalf("handle = %v unknown=%v, want %v/%v",
					snapshot.PrimaryKeyIsHandle, snapshot.PrimaryKeyHandleUnknown,
					tc.wantHandle, tc.wantHandleUnk)
			}
		})
	}
}

// withQuery overlays one stubbed query result onto a base map.
func withQuery(base map[string]testQueryResult, key string, result testQueryResult) map[string]testQueryResult {
	out := make(map[string]testQueryResult, len(base)+1)
	for k, v := range base {
		out[k] = v
	}
	out[key] = result
	return out
}

// Spec-review residual pinned: on TiDB a secondary can literally own
// `primary` (RENAME … TO `primary` is legal on PK-less and PKIsHandle
// tables), so the catalog name alone cannot promote — the PRIMARY KEY
// table_constraints row proves whether a PK object exists (tidb_pk_type
// cannot: NONCLUSTERED covers both PK-less tables and real nonclustered
// primaries), and only a single `primary`-fold row under a proven PK
// binds to the constraint object.
func TestT10R4TiDBPrimaryNameBindingMatrix(t *testing.T) {
	t.Parallel()
	base := func(columnRows, statRows [][]driver.Value) map[string]testQueryResult {
		return map[string]testQueryResult{
			"select engine, table_collation": {
				columns: []string{"engine", "table_collation", "table_comment", "auto_increment", "row_format", "table_rows"},
				rows:    [][]driver.Value{{"InnoDB", "utf8mb4_bin", "", nil, "", nil}},
			},
			"information_schema.columns": {
				columns: []string{"column_name", "column_type", "character_set_name", "collation_name", "column_comment", "column_default", "is_nullable", "extra", "generation_expression"},
				rows:    columnRows,
			},
			"column_name = 'IS_VISIBLE'": {
				columns: []string{"count"},
				rows:    [][]driver.Value{{1}},
			},
			"column_name = 'EXPRESSION'": {
				columns: []string{"count"},
				rows:    [][]driver.Value{{1}},
			},
			"information_schema.statistics": {
				columns: []string{"index_name", "non_unique", "index_type", "column_name", "cardinality", "sub_part", "collation", "is_visible", "expression"},
				rows:    statRows,
			},
			"information_schema.partitions": {
				columns: []string{"count"},
				rows:    [][]driver.Value{{0}},
			},
		}
	}
	constraintRow := func(count driver.Value) testQueryResult {
		return testQueryResult{columns: []string{"count"}, rows: [][]driver.Value{{count}}}
	}
	colsID := [][]driver.Value{
		{"id", "int(11)", nil, nil, "", nil, "NO", "", nil},
		{"c", "int(11)", nil, nil, "", nil, "NO", "", nil},
	}
	pkRow := [][]driver.Value{{"PRIMARY", 0, "BTREE", "id", nil, nil, "A", "YES", nil}}
	memberPrimary := [][]driver.Value{
		{"primary", 1, "BTREE", "c", nil, nil, "A", "YES", nil},
	}
	cases := []struct {
		name           string
		queries        map[string]testQueryResult
		wantPK         bool
		wantMembers    int
		wantPKUnknown  bool
		wantHandleUnk  bool
		wantClusterUnk bool
		wantErr        bool
	}{
		{
			name:        "no PRIMARY KEY constraint: member primary stays a secondary (renamed member on a PK-less table)",
			queries:     withQuery(base(colsID, memberPrimary), "table_constraints", constraintRow(0)),
			wantMembers: 1,
		},
		{
			name: "PK constraint + single fold promotes to the constraint object",
			queries: withQuery(
				withQuery(base(colsID, pkRow), "table_constraints", constraintRow(1)),
				"information_schema.tidb_indexes",
				testQueryResult{columns: []string{"clustered"}, rows: [][]driver.Value{{"NO"}}}),
			wantPK:      true,
			wantMembers: 0,
		},
		{
			name: "PK constraint + unique PRIMARY fold plus member `primary` is the legal handle-table coexistence (verified v8.5.0: the member is dropped by name while the PK survives)",
			queries: withQuery(
				withQuery(base(colsID, append(append([][]driver.Value{}, pkRow...), memberPrimary...)), "table_constraints", constraintRow(1)),
				"information_schema.tidb_indexes",
				testQueryResult{columns: []string{"clustered"}, rows: [][]driver.Value{{"NO"}}}),
			wantPK:      true,
			wantMembers: 1,
		},
		{
			name:    "unproven constraint probe propagates as a provider error (issue #79: actual failures stay errors)",
			queries: base(colsID, pkRow),
			wantErr: true,
		},
		{
			name: "no primary-fold row and no PK constraint: provably PK-less member table",
			queries: withQuery(
				base(colsID, [][]driver.Value{{"s", 1, "BTREE", "c", nil, nil, "A", "YES", nil}}),
				"table_constraints", constraintRow(0)),
			wantMembers: 1,
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			db := openTestDB(t, tc.queries)
			defer db.Close()
			snapshot, err := NewProvider(db).LoadTableSnapshot(context.Background(), spec.DialectTiDB, "app", "t")
			if tc.wantErr {
				if err == nil {
					t.Fatalf("an unanswerable constraint probe must surface as a provider error, got %+v", snapshot)
				}
				return
			}
			if err != nil {
				t.Fatalf("LoadTableSnapshot: %v", err)
			}
			if (snapshot.PrimaryKey != nil) != tc.wantPK {
				t.Fatalf("PrimaryKey present = %v, want %v (pk=%+v)", snapshot.PrimaryKey != nil, tc.wantPK, snapshot.PrimaryKey)
			}
			if len(snapshot.Indexes) != tc.wantMembers {
				t.Fatalf("member count = %d, want %d (%+v)", len(snapshot.Indexes), tc.wantMembers, snapshot.Indexes)
			}
			if snapshot.PrimaryKeyUnknown != tc.wantPKUnknown ||
				snapshot.PrimaryKeyHandleUnknown != tc.wantHandleUnk ||
				snapshot.PrimaryKeyClusteredUnknown != tc.wantClusterUnk {
				t.Fatalf("unknowns = %v/%v/%v, want %v/%v/%v",
					snapshot.PrimaryKeyUnknown, snapshot.PrimaryKeyHandleUnknown, snapshot.PrimaryKeyClusteredUnknown,
					tc.wantPKUnknown, tc.wantHandleUnk, tc.wantClusterUnk)
			}
			if tc.wantPK && snapshot.PrimaryKey.Kind != spec.IndexKindPrimary {
				t.Fatalf("the promoted fold must carry primary kind, got %+v", snapshot.PrimaryKey)
			}
		})
	}
}
