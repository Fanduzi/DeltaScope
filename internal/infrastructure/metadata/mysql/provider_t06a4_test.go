// Package mysqlmeta verifies the T06-A4 provider default-identity contract:
// a stored non-NULL COLUMN_DEFAULT byte string is a literal representation,
// never proof of a SQL NULL default. The tests run the real
// Provider.LoadTableSnapshot/loadColumns against constructed database/sql
// rows — this is "real provider + constructed DB rows", not a live anchor.
// input: synthetic information_schema row sets covering NULL/string literal defaults
// output: corrected HasDefault/DefaultValue/DefaultIsNull identity per stored value
// pos: infrastructure metadata adapter regression tests for issue #85 T06-A4
// note: if this file changes, update this header and module README.md.
package mysqlmeta

import (
	"context"
	"database/sql/driver"
	"errors"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// t06a4ColumnRows renders the information_schema.columns row shape the real
// loadColumns scans: name, type, charset, collation, comment, column_default,
// is_nullable, extra. column_default nil models a stored SQL NULL.
func t06a4ColumnRows(defaults []driver.Value) testQueryResult {
	rows := make([][]driver.Value, 0, len(defaults)+1)
	rows = append(rows, []driver.Value{"id", "int", nil, nil, "", nil, "NO", ""})
	for i, def := range defaults {
		name := string(rune('a' + i))
		rows = append(rows, []driver.Value{name, "varchar(8)", nil, nil, "", def, "YES", ""})
	}
	return testQueryResult{
		columns: []string{"column_name", "column_type", "character_set_name", "collation_name", "column_comment", "column_default", "is_nullable", "extra"},
		rows:    rows,
	}
}

// t06a4Results builds the canned catalog for a table whose columns carry the
// given column_default values in order (id first, then a..).
func t06a4Results(defaults []driver.Value) map[string]testQueryResult {
	return map[string]testQueryResult{
		"from information_schema.tables": {
			columns: []string{"engine", "table_collation", "table_comment", "auto_increment", "row_format", "table_rows"},
			rows:    [][]driver.Value{{"InnoDB", "utf8mb4_general_ci", "", nil, "Dynamic", int64(0)}},
		},
		"from information_schema.columns": t06a4ColumnRows(defaults),
		"from information_schema.statistics": {
			columns: []string{"index_name", "non_unique", "index_type", "column_name", "cardinality"},
			rows:    [][]driver.Value{{"PRIMARY", int64(0), "BTREE", "id", nil}},
		},
	}
}

// TestT06A4ProviderDefaultIdentityMatrix pins the stored-value contract:
// COLUMN_DEFAULT SQL NULL is the only NULL default — and it cannot be told
// from an omitted DEFAULT clause (catalog limitation, not a provider bug to
// paper over). Every non-NULL value is a literal byte representation:
// "NULL"/"null"/"NuLl" are string literals, never SQL NULL.
func TestT06A4ProviderDefaultIdentityMatrix(t *testing.T) {
	type want struct {
		hasDefault bool
		value      string
		isNull     bool
	}
	cases := []struct {
		name string
		raw  driver.Value
		want want
	}{
		{"sql_null_stores_null", nil, want{false, "", false}},
		{"text_NULL_is_literal", "NULL", want{true, "NULL", false}},
		{"text_null_lowercase", "null", want{true, "null", false}},
		{"text_NuLl_mixed", "NuLl", want{true, "NuLl", false}},
		{"text_nil_literal", "<nil>", want{true, "<nil>", false}},
		{"empty_string_literal", "", want{true, "", false}},
		{"zero_literal", "0", want{true, "0", false}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			db := openTestDB(t, t06a4Results([]driver.Value{tc.raw}))
			defer db.Close()
			snapshot, err := NewProvider(db).LoadTableSnapshot(context.Background(), spec.DialectMySQL, "golden", "t")
			if err != nil {
				t.Fatalf("LoadTableSnapshot: %v", err)
			}
			if len(snapshot.Columns) != 2 {
				t.Fatalf("columns = %#v, want id + one varchar", snapshot.Columns)
			}
			got := snapshot.Columns[1]
			if got.Name != "a" || got.HasDefault != tc.want.hasDefault ||
				got.DefaultValue != tc.want.value || got.DefaultIsNull != tc.want.isNull {
				t.Fatalf("column %q = {HasDefault:%v DefaultValue:%q DefaultIsNull:%v}, want {%v %q %v}",
					got.Name, got.HasDefault, got.DefaultValue, got.DefaultIsNull,
					tc.want.hasDefault, tc.want.value, tc.want.isNull)
			}
		})
	}
}

// TestT06A4ProviderDefaultsDoNotCrossRows guards per-row isolation: one
// column's stored NULL/literal can never bleed flags into its siblings, and
// the a/b catalog ambiguity is honestly preserved — a stored SQL NULL shows
// HasDefault=false on both `a` (no clause possible) and `b` (DEFAULT NULL).
func TestT06A4ProviderDefaultsDoNotCrossRows(t *testing.T) {
	db := openTestDB(t, t06a4Results([]driver.Value{nil, nil, "NULL", "<nil>"}))
	defer db.Close()
	snapshot, err := NewProvider(db).LoadTableSnapshot(context.Background(), spec.DialectMySQL, "golden", "t")
	if err != nil {
		t.Fatalf("LoadTableSnapshot: %v", err)
	}
	want := map[string][3]any{
		"id": {false, "", false},
		"a":  {false, "", false},
		"b":  {false, "", false},
		"c":  {true, "NULL", false},
		"d":  {true, "<nil>", false},
	}
	for _, column := range snapshot.Columns {
		exp, ok := want[column.Name]
		if !ok {
			t.Fatalf("unexpected column %q", column.Name)
		}
		got := [3]any{column.HasDefault, column.DefaultValue, column.DefaultIsNull}
		if got != exp {
			t.Fatalf("column %q = %v, want %v", column.Name, got, exp)
		}
	}
}

// TestT06A4ProviderColumnQueryErrorPropagates keeps the error channel
// honest: a real columns-query failure surfaces as a provider error through
// errors.Is, never as an unknown or absent snapshot.
func TestT06A4ProviderColumnQueryErrorPropagates(t *testing.T) {
	sentinel := errors.New("columns access denied")
	results := t06a4Results(nil)
	results["from information_schema.columns"] = testQueryResult{queryErr: sentinel}
	db := openTestDB(t, results)
	defer db.Close()
	_, err := NewProvider(db).LoadTableSnapshot(context.Background(), spec.DialectMySQL, "golden", "t")
	if err == nil || !errors.Is(err, sentinel) {
		t.Fatalf("err = %v, want wrapped %v", err, sentinel)
	}
}
