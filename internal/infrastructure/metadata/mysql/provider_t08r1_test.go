// Package mysqlmeta verifies the T08-R1 comparable literal identity the
// provider publishes: catalog COLUMN_DEFAULT bytes are already decoded
// literal values, and the column's own base type fixes the comparison
// domain — keyword-looking bytes on a non-temporal column stay literal.
// The tests run the real Provider.LoadTableSnapshot/loadColumns against
// constructed database/sql rows — real provider, constructed DB rows.
// input: synthetic information_schema rows per (column_type, column_default, extra)
// output: DefaultLiteral kind+value, temporal flag, expression-fact routing
// pos: infrastructure metadata adapter regression tests for issue #87 T08-R1
// note: if this file changes, update this header and module README.md.
package mysqlmeta

import (
	"context"
	"database/sql/driver"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// t08r1ColumnRows renders full-shape information_schema.columns rows where
// each entry carries its own column_type, column_default, and extra — the
// provider must read all three to classify the comparable identity.
func t08r1Results(columns [][3]string) map[string]testQueryResult {
	rows := make([][]driver.Value, 0, len(columns)+1)
	rows = append(rows, []driver.Value{"id", "int", nil, nil, "", nil, "NO", "", ""})
	for i, col := range columns {
		var def driver.Value
		if col[1] != "<sql-null>" {
			def = col[1]
		}
		rows = append(rows, []driver.Value{string(rune('a' + i)), col[0], nil, nil, "", def, "YES", col[2], ""})
	}
	return map[string]testQueryResult{
		"from information_schema.tables": {
			columns: []string{"engine", "table_collation", "table_comment", "auto_increment", "row_format", "table_rows"},
			rows:    [][]driver.Value{{"InnoDB", "utf8mb4_general_ci", "", nil, "Dynamic", int64(0)}},
		},
		"from information_schema.columns": {
			columns: []string{"column_name", "column_type", "character_set_name", "collation_name", "column_comment", "column_default", "is_nullable", "extra", "generation_expression"},
			rows:    rows,
		},
		"from information_schema.statistics": {
			columns: []string{"index_name", "non_unique", "index_type", "column_name", "cardinality"},
			rows:    [][]driver.Value{{"PRIMARY", int64(0), "BTREE", "id", nil}},
		},
	}
}

// TestT08R1ProviderLiteralIdentity pins the identity-fact domain: the
// stored bytes are the literal identity verbatim and the base type —
// parameters and attribute text stripped — names the kind. No quoting,
// unescaping, or content guessing applies. A recorded numeric fact like
// 1.50 is the catalog's own encoding; whether a pair shares a proven
// encoding is the comparison's domain check (T08-R2), not this seam.
func TestT08R1ProviderLiteralIdentity(t *testing.T) {
	cases := []struct {
		name     string
		colType  string
		def      string // "<sql-null>" models a NULL row value
		extra    string
		wantKind string // "" = expect nil DefaultLiteral
		wantVal  string
	}{
		{"varchar_literal", "varchar(16)", "x", "", spec.LiteralDefaultString, "x"},
		{"varchar_empty", "varchar(16)", "", "", spec.LiteralDefaultString, ""},
		{"varchar_text_null", "varchar(16)", "NULL", "", spec.LiteralDefaultString, "NULL"},
		{"varchar_content_quotes", "varchar(16)", "'x'", "", spec.LiteralDefaultString, "'x'"},
		{"varchar_quote_in_content", "varchar(16)", "O'Reilly", "", spec.LiteralDefaultString, "O'Reilly"},
		{"int_numeric", "int", "7", "", spec.LiteralDefaultNumeric, "7"},
		{"bigint_unsigned_numeric", "bigint unsigned", "18446744073709551615", "", spec.LiteralDefaultNumeric, "18446744073709551615"},
		{"decimal_param_numeric", "decimal(10,2)", "1.50", "", spec.LiteralDefaultNumeric, "1.50"},
		{"char_string", "char(4)", "ab", "", spec.LiteralDefaultString, "ab"},
		{"enum_param_string", "enum('a','b')", "b", "", spec.LiteralDefaultString, "b"},
		{"sql_null_row", "varchar(16)", "<sql-null>", "", "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			db := openTestDB(t, t08r1Results([][3]string{{tc.colType, tc.def, tc.extra}}))
			defer db.Close()
			snapshot, err := NewProvider(db).LoadTableSnapshot(context.Background(), spec.DialectMySQL, "golden", "t")
			if err != nil {
				t.Fatalf("LoadTableSnapshot: %v", err)
			}
			if len(snapshot.Columns) != 2 {
				t.Fatalf("columns = %#v, want id + one column", snapshot.Columns)
			}
			got := snapshot.Columns[1]
			if tc.wantKind == "" {
				if got.DefaultLiteral != nil {
					t.Fatalf("column %q DefaultLiteral = %+v, want nil (SQL NULL row)", got.Name, got.DefaultLiteral)
				}
				return
			}
			if got.DefaultLiteral == nil {
				t.Fatalf("column %q DefaultLiteral = nil, want {%s %q}", got.Name, tc.wantKind, tc.wantVal)
			}
			if got.DefaultLiteral.Kind != tc.wantKind || got.DefaultLiteral.Value != tc.wantVal {
				t.Fatalf("column %q DefaultLiteral = {%q %q}, want {%q %q}",
					got.Name, got.DefaultLiteral.Kind, got.DefaultLiteral.Value, tc.wantKind, tc.wantVal)
			}
			// The display contract is untouched: DefaultValue keeps the
			// same verbatim catalog bytes the identity decodes from.
			if got.DefaultValue != tc.def {
				t.Fatalf("column %q DefaultValue = %q, want verbatim %q", got.Name, got.DefaultValue, tc.def)
			}
		})
	}
}

// TestT08R1ProviderTemporalGate pins the CURRENT_TIMESTAMP classification:
// only temporal base types promote the stored bytes to the temporal
// marker, and the marker is not a literal. Parameterized temporal types
// (datetime(6)) must still resolve through the base type.
func TestT08R1ProviderTemporalGate(t *testing.T) {
	cases := []struct {
		name          string
		colType       string
		wantTemporal  bool
		wantKindIsSet bool
	}{
		{"datetime_is_temporal", "datetime", true, false},
		{"datetime6_is_temporal", "datetime(6)", true, false},
		{"timestamp_is_temporal", "timestamp", true, false},
		{"varchar_ct_is_string_literal", "varchar(32)", false, true},
		{"char_ct_is_string_literal", "char(17)", false, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			db := openTestDB(t, t08r1Results([][3]string{{tc.colType, "current_timestamp", ""}}))
			defer db.Close()
			snapshot, err := NewProvider(db).LoadTableSnapshot(context.Background(), spec.DialectMySQL, "golden", "t")
			if err != nil {
				t.Fatalf("LoadTableSnapshot: %v", err)
			}
			got := snapshot.Columns[1]
			if got.DefaultIsCurrentTimestamp != tc.wantTemporal {
				t.Fatalf("column %q(%s) DefaultIsCurrentTimestamp = %v, want %v", got.Name, tc.colType, got.DefaultIsCurrentTimestamp, tc.wantTemporal)
			}
			if tc.wantTemporal {
				if got.DefaultLiteral != nil {
					t.Fatalf("temporal %s carried literal identity %+v", tc.colType, got.DefaultLiteral)
				}
				return
			}
			if got.DefaultLiteral == nil || got.DefaultLiteral.Kind != spec.LiteralDefaultString || got.DefaultLiteral.Value != "current_timestamp" {
				t.Fatalf("non-temporal %s literal identity = %+v, want {string current_timestamp}", tc.colType, got.DefaultLiteral)
			}
		})
	}
}

// TestT08R1ProviderExpressionRoutesOffLiteral pins the DEFAULT_GENERATED
// boundary: an expression default carries the parsed expression fact and
// never a literal identity, while undecodable/absent expression text keeps
// the explicit unknown fact instead of falling into a literal.
func TestT08R1ProviderExpressionRoutesOffLiteral(t *testing.T) {
	cases := []struct {
		name       string
		def        string
		wantExpr   bool
		wantExprOK bool
	}{
		{"expression_generated", "(uuid())", true, true},
		{"expression_undecodable", "", true, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			db := openTestDB(t, t08r1Results([][3]string{{"int", tc.def, "DEFAULT_GENERATED"}}))
			defer db.Close()
			snapshot, err := NewProvider(db).LoadTableSnapshot(context.Background(), spec.DialectMySQL, "golden", "t")
			if err != nil {
				t.Fatalf("LoadTableSnapshot: %v", err)
			}
			got := snapshot.Columns[1]
			if (got.DefaultExpression != nil) != tc.wantExpr {
				t.Fatalf("DefaultExpression = %+v, want present=%v", got.DefaultExpression, tc.wantExpr)
			}
			if got.DefaultExpression != nil && got.DefaultExpression.Known != tc.wantExprOK {
				t.Fatalf("DefaultExpression.Known = %v, want %v", got.DefaultExpression.Known, tc.wantExprOK)
			}
			if got.DefaultLiteral != nil {
				t.Fatalf("expression default carried literal identity %+v", got.DefaultLiteral)
			}
		})
	}
}
