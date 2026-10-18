// Package mysqlmeta verifies the T06-A17 catalog type-parameter fidelity
// contract: DECIMAL/FLOAT/DOUBLE parameter text and ENUM/SET member literals
// from COLUMN_TYPE survive the real Provider.LoadTableSnapshot — parameters
// are no longer truncated to the base keyword plus first integer. Rows are
// constructed catalog input (a live server may normalize differently; the
// provider must only preserve what it actually receives).
// input: constructed information_schema row sets for the five target types
// output: faithful Column.Type/Length/Unsigned per stored column_type
// pos: infrastructure metadata adapter regression tests for issue #85 T06-A17
// note: if this file changes, update this header and module README.md.
package mysqlmeta

import (
	"context"
	"database/sql/driver"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// t06a17ColumnRows renders the frozen A17 column_type inputs verbatim.
func t06a17Results() map[string]testQueryResult {
	columnTypes := [][2]string{
		{"d_dec122", "decimal(12,2)"},
		{"d_dec123", "decimal(12,3)"},
		{"d_dec120", "decimal(12,0)"},
		{"f_flt102", "float(10,2)"},
		{"f_flt103", "float(10,3)"},
		{"f_flt100", "float(10,0)"},
		{"x_dbl162", "double(16,2)"},
		{"x_dbl164", "double(16,4)"},
		{"x_dbl160", "double(16,0)"},
		{"e_upper", "enum('Ready','Closed')"},
		{"e_lower", "enum('ready','closed')"},
		{"e_order", "enum('Closed','Ready')"},
		{"s_upper", "set('Read','Write')"},
		{"s_lower", "set('read','write')"},
		{"s_order", "set('Write','Read')"},
		{"e_edge", "enum('','A,B','A)B','O''Reilly')"},
		// Attribute-boundary controls: ' unsigned'/'zerofill' inside member
		// literals are values, never type attributes.
		{"e_attrword", "enum('x unsigned','y zerofill')"},
		{"s_attrword", "set('a unsigned','b')"},
		{"c_int", "int"},
		{"c_varchar8", "varchar(8)"},
		{"c_bigint_u", "bigint unsigned"},
		{"d_dec122u", "decimal(12,2) unsigned"},
	}
	rows := make([][]driver.Value, 0, len(columnTypes))
	for _, pair := range columnTypes {
		rows = append(rows, []driver.Value{pair[0], pair[1], nil, nil, "", nil, "YES", ""})
	}
	return map[string]testQueryResult{
		"from information_schema.tables": {
			columns: []string{"engine", "table_collation", "table_comment", "auto_increment", "row_format", "table_rows"},
			rows:    [][]driver.Value{{"InnoDB", "utf8mb4_general_ci", "", nil, "Dynamic", int64(0)}},
		},
		"from information_schema.columns": {
			columns: []string{"column_name", "column_type", "character_set_name", "collation_name", "column_comment", "column_default", "is_nullable", "extra"},
			rows:    rows,
		},
		"from information_schema.statistics": {
			columns: []string{"index_name", "non_unique", "index_type", "column_name", "cardinality"},
		},
	}
}

func t06a17SnapshotColumns(t *testing.T) []spec.Column {
	t.Helper()
	db := openTestDB(t, t06a17Results())
	defer db.Close()
	snapshot, err := NewProvider(db).LoadTableSnapshot(context.Background(), spec.DialectMySQL, "golden", "t")
	if err != nil {
		t.Fatalf("LoadTableSnapshot: %v", err)
	}
	return snapshot.Columns
}

// TestT06A17ProviderParameterFidelity pins verbatim catalog parameter text:
// scale/second-parameter/member content survive; non-target types keep the
// established base-keyword contract.
func TestT06A17ProviderParameterFidelity(t *testing.T) {
	columns := t06a17SnapshotColumns(t)
	want := map[string]struct {
		typeText string
		length   int
		unsigned bool
	}{
		"d_dec122":   {"decimal(12,2)", 12, false},
		"d_dec123":   {"decimal(12,3)", 12, false},
		"d_dec120":   {"decimal(12,0)", 12, false},
		"f_flt102":   {"float(10,2)", 10, false},
		"f_flt103":   {"float(10,3)", 10, false},
		"f_flt100":   {"float(10,0)", 10, false},
		"x_dbl162":   {"double(16,2)", 16, false},
		"x_dbl164":   {"double(16,4)", 16, false},
		"x_dbl160":   {"double(16,0)", 16, false},
		"e_upper":    {"enum('Ready','Closed')", 0, false},
		"e_lower":    {"enum('ready','closed')", 0, false},
		"e_order":    {"enum('Closed','Ready')", 0, false},
		"s_upper":    {"set('Read','Write')", 0, false},
		"s_lower":    {"set('read','write')", 0, false},
		"s_order":    {"set('Write','Read')", 0, false},
		"e_edge":     {"enum('','A,B','A)B','O''Reilly')", 0, false},
		"e_attrword": {"enum('x unsigned','y zerofill')", 0, false},
		"s_attrword": {"set('a unsigned','b')", 0, false},
		"c_int":      {"int", 0, false},
		"c_varchar8": {"varchar", 8, false},
		"c_bigint_u": {"bigint", 0, true},
		"d_dec122u":  {"decimal(12,2) unsigned", 12, true},
	}
	for _, column := range columns {
		w, ok := want[column.Name]
		if !ok {
			t.Fatalf("unexpected column %q", column.Name)
		}
		if column.Type != w.typeText || column.Length != w.length || column.Unsigned != w.unsigned {
			t.Fatalf("%s = Type %q Length %d Unsigned %v, want %q/%d/%v",
				column.Name, column.Type, column.Length, column.Unsigned, w.typeText, w.length, w.unsigned)
		}
	}
}

// TestT06A17ProviderDistinctness asserts the merged groups stay distinguishable
// end-to-end: scale variants, case variants, and order variants never collapse.
func TestT06A17ProviderDistinctness(t *testing.T) {
	columns := t06a17SnapshotColumns(t)
	byName := map[string]spec.Column{}
	for _, column := range columns {
		byName[column.Name] = column
	}
	pairs := [][2]string{
		{"d_dec122", "d_dec123"}, {"d_dec122", "d_dec120"},
		{"f_flt102", "f_flt103"}, {"x_dbl162", "x_dbl164"},
		{"e_upper", "e_lower"}, {"e_upper", "e_order"},
		{"s_upper", "s_lower"}, {"s_upper", "s_order"},
	}
	for _, pair := range pairs {
		if byName[pair[0]].Type == byName[pair[1]].Type {
			t.Fatalf("%s and %s merged to %q", pair[0], pair[1], byName[pair[0]].Type)
		}
	}
}

// TestT06A17ProviderParamBoundary pins quote-aware boundary scanning: a member
// containing ")" or "," never ends the parameter list early, and attribute
// words inside members never set Unsigned.
func TestT06A17ProviderParamBoundary(t *testing.T) {
	columns := t06a17SnapshotColumns(t)
	byName := map[string]spec.Column{}
	for _, column := range columns {
		byName[column.Name] = column
	}
	edge := byName["e_edge"]
	if edge.Type != "enum('','A,B','A)B','O''Reilly')" {
		t.Fatalf("e_edge Type = %q — member delimiters broke the parameter span", edge.Type)
	}
	for _, name := range []string{"e_attrword", "s_attrword"} {
		if byName[name].Unsigned {
			t.Fatalf("%s Unsigned = true — attribute word inside member literal leaked", name)
		}
	}
}
