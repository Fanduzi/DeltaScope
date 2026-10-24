// Package mysql tests the T10-R3 provider member-fidelity correction: a
// STATISTICS row whose COLUMN_NAME is NULL and whose COLLATION is 'D' is a
// DESCENDING expression member — the catalog records the authored direction
// on functional key parts (MySQL 8.0.46 functional_index.result L186-193
// proves descending functional indexes are real objects). The expression
// branch must read the same collation fact the column branch does; a
// missing or unrecognized direction follows the existing unknown-member
// semantics, never a guessed ascending.
// input: sqlmock-shaped member rows covering the collation matrix on
// expression members plus the named-member controls
// output: KeyParts with honest kind/direction/expression facts and the
// derived DescParts count
// pos: provider-side T10-R3 member-fidelity coverage beside the spec
// resolution tests
// note: if this file changes, update this header and module README.md.
package mysqlmeta

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// C: the expression member collation matrix — 'D' keeps descending, 'A'
// stays ascending, and unproven directions keep the member unmodeled while
// its expression identity stays attached.
func TestT10R3ExpressionMemberCollationMatrix(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name          string
		collation     sql.NullString
		wantKind      spec.IndexKeyPartKind
		wantDesc      bool
		wantDescParts int
	}{
		{name: "D keeps descending", collation: t10r2Str("D"), wantKind: spec.IndexKeyPartExpression, wantDesc: true, wantDescParts: 1},
		{name: "A stays ascending", collation: t10r2Str("A"), wantKind: spec.IndexKeyPartExpression, wantDesc: false, wantDescParts: 0},
		{name: "NULL collation is unmodeled", collation: t10r2NullStr(), wantKind: spec.IndexKeyPartUnmodeled, wantDesc: false, wantDescParts: 0},
		{name: "unrecognized collation is unmodeled", collation: t10r2Str("X"), wantKind: spec.IndexKeyPartUnmodeled, wantDesc: false, wantDescParts: 0},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			indexes := make(map[string]*spec.Index)
			order := make([]string, 0)
			accumulateIndexRow(indexes, &order, "i_e", 1, "BTREE",
				indexMemberRow{
					columnName: t10r2NullStr(), collation: tc.collation,
					expression: t10r2Str("`a`+1"),
				}, nil, nil)
			index := indexes["i_e"]
			index.SetKeyParts(index.KeyParts)
			if len(index.KeyParts) != 1 {
				t.Fatalf("KeyParts = %d, want 1", len(index.KeyParts))
			}
			part := index.KeyParts[0]
			if part.Kind != tc.wantKind {
				t.Fatalf("part.Kind = %s, want %s", part.Kind, tc.wantKind)
			}
			if part.Descending != tc.wantDesc {
				t.Fatalf("part.Descending = %v, want %v", part.Descending, tc.wantDesc)
			}
			if index.DescParts != tc.wantDescParts {
				t.Fatalf("DescParts = %d, want %d", index.DescParts, tc.wantDescParts)
			}
		})
	}
}

// C: the descending expression member keeps its expression identity,
// dependencies, and multi-valued cast marks — direction fidelity must not
// launder the member into a column or drop its expression facts.
func TestT10R3DescendingExpressionKeepsIdentity(t *testing.T) {
	t.Parallel()
	indexes := make(map[string]*spec.Index)
	order := make([]string, 0)
	accumulateIndexRow(indexes, &order, "i_mv", 1, "BTREE",
		indexMemberRow{
			columnName: t10r2NullStr(), collation: t10r2Str("D"),
			expression: t10r2Str("cast(`a` as unsigned array)"),
		}, nil, nil)
	index := indexes["i_mv"]
	index.SetKeyParts(index.KeyParts)
	part := index.KeyParts[0]
	if part.Kind != spec.IndexKeyPartExpression || !part.Descending {
		t.Fatalf("descending multi-valued member = %+v, want expression+desc", part)
	}
	if part.Expression == nil {
		t.Fatalf("expression identity must survive direction classification, got %+v", part)
	}
	if !part.MultiValued || index.MultiValuedParts != 1 {
		t.Fatalf("multi-valued marks must survive direction classification, got %+v", index)
	}
	if len(index.Columns) != 0 {
		t.Fatalf("expression member must never invent a column name, got %v", index.Columns)
	}
}

// C: TiDB's literal-'NULL' expression row carries the same direction fact —
// the literal name path must not lose it either.
func TestT10R3LiteralNullExpressionKeepsDirection(t *testing.T) {
	t.Parallel()
	indexes := make(map[string]*spec.Index)
	order := make([]string, 0)
	accumulateIndexRow(indexes, &order, "i_e", 1, "BTREE",
		indexMemberRow{
			columnName: t10r2Str("NULL"), collation: t10r2Str("D"),
			expression: t10r2Str("`a`+1"),
		}, nil, nil)
	index := indexes["i_e"]
	index.SetKeyParts(index.KeyParts)
	part := index.KeyParts[0]
	if part.Kind != spec.IndexKeyPartExpression || !part.Descending {
		t.Fatalf("literal-NULL descending expression = %+v, want expression+desc", part)
	}
	if index.DescParts != 1 {
		t.Fatalf("DescParts = %d, want 1", index.DescParts)
	}
}

// B3: the alter-primary-key fact loads through SHOW CONFIG under the same
// all-rows-verification contract as max-index-length — a consistent value
// becomes known, disagreeing/unparsable/absent values stay unknown, and
// read failures propagate as provider errors rather than degrading.
func TestT10R3TiDBAlterPrimaryKeyFactMatrix(t *testing.T) {
	t.Parallel()
	tidbVariables := testQueryResult{
		columns: []string{"Variable_name", "Value"},
		rows:    [][]driver.Value{{"version", "8.0.11-TiDB-v8.5.0"}},
	}
	configColumns := []string{"Type", "Instance", "Name", "Value"}
	configRow := func(instance, value string) []driver.Value {
		return []driver.Value{"tidb", instance, "alter-primary-key", value}
	}
	cases := []struct {
		name      string
		config    testQueryResult
		wantValue bool
		wantKnown bool
		wantErr   bool
	}{
		{
			name:      "true across instances becomes known",
			config:    testQueryResult{columns: configColumns, rows: [][]driver.Value{configRow("n1:4000", "true"), configRow("n2:4000", "true")}},
			wantValue: true, wantKnown: true,
		},
		{
			name:      "false becomes known",
			config:    testQueryResult{columns: configColumns, rows: [][]driver.Value{configRow("n1:4000", "false")}},
			wantValue: false, wantKnown: true,
		},
		{
			name: "disagreeing instances stay unknown",
			config: testQueryResult{columns: configColumns, rows: [][]driver.Value{
				configRow("n1:4000", "true"), configRow("n2:4000", "false"),
			}},
			wantKnown: false,
		},
		{
			name:      "unparsable value stays unknown",
			config:    testQueryResult{columns: configColumns, rows: [][]driver.Value{configRow("n1:4000", "auto")}},
			wantKnown: false,
		},
		{
			name:      "zero rows stays unknown",
			config:    testQueryResult{columns: configColumns, rows: [][]driver.Value{}},
			wantKnown: false,
		},
		{
			name:    "query error propagates as provider error",
			config:  testQueryResult{queryErr: errors.New("SHOW CONFIG denied")},
			wantErr: true,
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			db := openTestDB(t, map[string]testQueryResult{
				"show variables":      tidbVariables,
				"'max-index-length'":  {columns: configColumns, rows: [][]driver.Value{{"tidb", "n1:4000", "max-index-length", "3072"}}},
				"'alter-primary-key'": tc.config,
			})
			defer db.Close()

			facts, err := NewProvider(db).LoadInstanceFacts(context.Background(), spec.DialectTiDB, "app")
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected provider error, got facts=%#v", facts)
				}
				return
			}
			if err != nil {
				t.Fatalf("LoadInstanceFacts: %v", err)
			}
			if facts.TiDBAlterPrimaryKeyKnown != tc.wantKnown {
				t.Fatalf("alter-primary-key known = %t, want %t", facts.TiDBAlterPrimaryKeyKnown, tc.wantKnown)
			}
			if tc.wantKnown && facts.TiDBAlterPrimaryKey != tc.wantValue {
				t.Fatalf("alter-primary-key = %t, want %t", facts.TiDBAlterPrimaryKey, tc.wantValue)
			}
		})
	}
}

// C control: a real column literally named `NULL` with an ascending
// collation still classifies as a plain column member — direction handling
// must not regress the T10-R2 reserved-name split.
func TestT10R3LiteralNullColumnStillColumn(t *testing.T) {
	t.Parallel()
	columns := []spec.Column{{Name: "NULL", Type: "int", NotNull: true}}
	indexes := make(map[string]*spec.Index)
	order := make([]string, 0)
	accumulateIndexRow(indexes, &order, "i_null", 1, "BTREE",
		indexMemberRow{columnName: t10r2Str("NULL"), collation: t10r2Str("A")},
		nil, columns)
	index := indexes["i_null"]
	index.SetKeyParts(index.KeyParts)
	part := index.KeyParts[0]
	if part.Kind != spec.IndexKeyPartColumn || part.Column != "NULL" {
		t.Fatalf("real NULL column = %+v, want column member", part)
	}
	if index.DescParts != 0 || index.ExpressionCount != 0 {
		t.Fatalf("real NULL column must not gain expression/desc marks, got %+v", index)
	}
}
