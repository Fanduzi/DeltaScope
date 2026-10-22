// Package mysqlmeta verifies the T09-R1 provider member-fact mapping.
// input: synthetic information_schema.statistics member rows covering every
// plain-key knownness classification (plain ascending, prefix length,
// descending, expression key part, unclassifiable collation)
// output: the spec.Index non-plain marks each row must produce — provider
// member definitions carry the same knownness vocabulary declarations do,
// so an unqueried or unclassifiable attribute never reads as known-plain
// pos: provider fact-sufficiency coverage for issue #88 T09-R1; the marks
// feed indexPlainComparable in internal/domain/rule/ddl
// note: if this file changes, update this header and module README.md.
package mysqlmeta

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// The R1 classification table: each member row lands exactly the mark its
// catalog facts prove — nothing defaults to plain.
func TestT09R1ProviderMemberRowClassification(t *testing.T) {
	str := func(v string) sql.NullString { return sql.NullString{String: v, Valid: true} }
	int64n := func(v int64) sql.NullInt64 { return sql.NullInt64{Int64: v, Valid: true} }

	cases := []struct {
		name          string
		row           indexMemberRow
		wantColumns   []string
		wantPrefix    int
		wantDesc      int
		wantExprCount int
		wantExprFlag  bool
		wantUnmodeled int
	}{
		{
			name:        "plain ascending whole column",
			row:         indexMemberRow{columnName: str("c"), collation: str("A")},
			wantColumns: []string{"c"},
		},
		{
			name:        "prefix length marks PrefixParts but keeps the column",
			row:         indexMemberRow{columnName: str("c"), collation: str("A"), subPart: int64n(3)},
			wantColumns: []string{"c"},
			wantPrefix:  1,
		},
		{
			name:        "descending collation marks DescParts",
			row:         indexMemberRow{columnName: str("c"), collation: str("D")},
			wantColumns: []string{"c"},
			wantDesc:    1,
		},
		{
			name:          "NULL column_name is an expression key part",
			row:           indexMemberRow{collation: str("A")},
			wantExprCount: 1,
			wantExprFlag:  true,
		},
		{
			name:          "NULL collation on a named member is unclassifiable",
			row:           indexMemberRow{columnName: str("c")},
			wantColumns:   []string{"c"},
			wantUnmodeled: 1,
		},
		{
			name:          "unrecognized collation value is unclassifiable",
			row:           indexMemberRow{columnName: str("c"), collation: str("X")},
			wantColumns:   []string{"c"},
			wantUnmodeled: 1,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			indexes := make(map[string]*spec.Index)
			order := make([]string, 0)
			accumulateIndexRow(indexes, &order, "i", 1, "BTREE", tc.row, nil)
			index := indexes["i"]
			if index == nil {
				t.Fatalf("index not accumulated")
			}
			if len(index.Columns) != len(tc.wantColumns) ||
				(len(tc.wantColumns) > 0 && index.Columns[0] != tc.wantColumns[0]) {
				t.Fatalf("columns = %v, want %v", index.Columns, tc.wantColumns)
			}
			if index.PrefixParts != tc.wantPrefix {
				t.Fatalf("PrefixParts = %d, want %d", index.PrefixParts, tc.wantPrefix)
			}
			if index.DescParts != tc.wantDesc {
				t.Fatalf("DescParts = %d, want %d", index.DescParts, tc.wantDesc)
			}
			if index.ExpressionCount != tc.wantExprCount || index.HasExpressionKeys != tc.wantExprFlag {
				t.Fatalf("expression marks = %d/%v, want %d/%v",
					index.ExpressionCount, index.HasExpressionKeys, tc.wantExprCount, tc.wantExprFlag)
			}
			if index.UnmodeledKeyParts != tc.wantUnmodeled {
				t.Fatalf("UnmodeledKeyParts = %d, want %d", index.UnmodeledKeyParts, tc.wantUnmodeled)
			}
		})
	}
}

// A loaded index built from real-shaped rows must expose non-plain marks the
// comparability gate reads — a prefix row can never scan back in as a plain
// key definition.
func TestT09R1ProviderLoadIndexesMarksNonPlain(t *testing.T) {
	db := openTestDB(t, map[string]testQueryResult{
		"select version()": {
			columns: []string{"version()"},
			rows:    [][]driver.Value{{"8.0.46"}},
		},
		"from information_schema.tables": {
			columns: []string{"engine", "table_collation", "table_comment", "auto_increment", "row_format", "table_rows"},
			rows:    [][]driver.Value{{"InnoDB", "utf8mb4_general_ci", "", nil, "Dynamic", int64(0)}},
		},
		"from information_schema.statistics": {
			columns: []string{"index_name", "non_unique", "index_type", "column_name", "cardinality", "sub_part", "collation"},
			rows: [][]driver.Value{
				{"i_prefix", int64(1), "BTREE", "c", int64(30), int64(8), "A"},
				{"i_plain", int64(1), "BTREE", "c", int64(30), nil, "A"},
			},
		},
		"from information_schema.columns": {
			columns: []string{"column_name", "column_type", "character_set_name", "collation_name", "column_comment", "column_default", "is_nullable", "extra", "generation_expression"},
			rows:    [][]driver.Value{{"c", "int", nil, nil, "", nil, "YES", "", ""}},
		},
	})
	defer db.Close()

	provider := NewProvider(db)
	snapshot, err := provider.LoadTableSnapshot(context.Background(), spec.DialectMySQL, "app", "t")
	if err != nil {
		t.Fatalf("load snapshot: %v", err)
	}
	if len(snapshot.Indexes) != 2 {
		t.Fatalf("expected two indexes, got %#v", snapshot.Indexes)
	}
	var prefix, plain *spec.Index
	for i := range snapshot.Indexes {
		switch snapshot.Indexes[i].Name {
		case "i_prefix":
			prefix = &snapshot.Indexes[i]
		case "i_plain":
			plain = &snapshot.Indexes[i]
		}
	}
	if prefix == nil || prefix.PrefixParts != 1 || len(prefix.Columns) != 1 {
		t.Fatalf("prefix index must carry PrefixParts=1 with its column, got %#v", prefix)
	}
	if plain == nil || plain.PrefixParts != 0 || plain.DescParts != 0 ||
		plain.ExpressionCount != 0 || plain.UnmodeledKeyParts != 0 {
		t.Fatalf("plain index must carry no non-plain marks, got %#v", plain)
	}
}
