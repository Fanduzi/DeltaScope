// Package mysqlmeta verifies the T07 catalog column facts: GENERATION_EXPRESSION
// plus EXTRA tokens feed generated storage, expression defaults, and effective
// visibility on the same single ordered COLUMNS read.
package mysqlmeta

import (
	"context"
	"database/sql/driver"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// t07ColumnRows renders the nine-column COLUMNS row shape loadColumns scans.
func t07ColumnRows(rows [][]driver.Value) testQueryResult {
	return testQueryResult{
		columns: []string{"column_name", "column_type", "character_set_name", "collation_name", "column_comment", "column_default", "is_nullable", "extra", "generation_expression"},
		rows:    rows,
	}
}

func t07Results(columnRows [][]driver.Value) map[string]testQueryResult {
	return map[string]testQueryResult{
		"from information_schema.tables": {
			columns: []string{"engine", "table_collation", "table_comment", "auto_increment", "row_format", "table_rows"},
			rows:    [][]driver.Value{{"InnoDB", "utf8mb4_general_ci", "", nil, "Dynamic", int64(0)}},
		},
		"from information_schema.columns":    t07ColumnRows(columnRows),
		"from information_schema.statistics": {columns: []string{"index_name", "non_unique", "index_type", "column_name", "cardinality", "sub_part", "collation"}},
	}
}

func t07Column(t *testing.T, snapshot *spec.TableSnapshot, name string) spec.Column {
	t.Helper()
	for _, column := range snapshot.Columns {
		if column.Name == name {
			return column
		}
	}
	t.Fatalf("column %q missing from %#v", name, snapshot.Columns)
	return spec.Column{}
}

// t07ColumnsExactly asserts the complete ordered column collection — extra,
// duplicated, reordered, or missing members all fail.
func t07ColumnsExactly(t *testing.T, snapshot *spec.TableSnapshot, names ...string) {
	t.Helper()
	if len(snapshot.Columns) != len(names) {
		t.Fatalf("columns = %#v, want exactly %v", snapshot.Columns, names)
	}
	for i, want := range names {
		if snapshot.Columns[i].Name != want {
			t.Fatalf("column[%d] = %q, want %q (full set %#v)", i, snapshot.Columns[i].Name, want, snapshot.Columns)
		}
	}
}

// TestT07ProviderGeneratedColumnFacts pins catalog generated columns:
// storage comes from the EXTRA token, the expression facts come from
// GENERATION_EXPRESSION, and the default fields stay untouched.
func TestT07ProviderGeneratedColumnFacts(t *testing.T) {
	db := openTestDB(t, t07Results([][]driver.Value{
		{"a", "int", nil, nil, "", nil, "NO", "", nil},
		{"g", "int", nil, nil, "", nil, "YES", "VIRTUAL GENERATED", "(`a` + 1)"},
		{"h", "int", nil, nil, "", nil, "YES", "STORED GENERATED", "concat(`a`,_utf8mb4\\'x\\')"},
	}))
	defer db.Close()
	snapshot, err := NewProvider(db).LoadTableSnapshot(context.Background(), spec.DialectMySQL, "golden", "t")
	if err != nil {
		t.Fatalf("LoadTableSnapshot: %v", err)
	}
	t07ColumnsExactly(t, snapshot, "a", "g", "h")

	ordinary := t07Column(t, snapshot, "a")
	if ordinary.Generated != nil || ordinary.DefaultExpression != nil {
		t.Fatalf("ordinary column gained generated facts: %+v", ordinary)
	}
	if ordinary.Visibility != spec.ColumnVisibilityVisible || ordinary.VisibilityExplicit {
		t.Fatalf("ordinary column visibility = %q explicit=%v, want effective visible", ordinary.Visibility, ordinary.VisibilityExplicit)
	}

	virtual := t07Column(t, snapshot, "g")
	if virtual.Generated == nil || virtual.Generated.Storage != spec.GeneratedStorageVirtual {
		t.Fatalf("virtual generated column = %+v", virtual.Generated)
	}
	expr := virtual.Generated.Expression
	if !expr.Known || len(expr.References) != 1 || expr.References[0].Name != "a" {
		t.Fatalf("virtual generated expression = %+v", expr)
	}
	if expr.Fingerprint == "" {
		t.Fatalf("virtual generated expression missing fingerprint")
	}

	stored := t07Column(t, snapshot, "h")
	if stored.Generated == nil || stored.Generated.Storage != spec.GeneratedStorageStored {
		t.Fatalf("stored generated column = %+v", stored.Generated)
	}
	sexpr := stored.Generated.Expression
	if !sexpr.Known || len(sexpr.References) != 1 || sexpr.References[0].Name != "a" {
		t.Fatalf("stored generated refs = %+v", sexpr)
	}
	// The escaped catalog literal must decode through the recognized escape
	// layer: `concat(...)` collects the function and the `a` reference.
	foundConcat := false
	for _, fn := range sexpr.Functions {
		if fn == "concat" {
			foundConcat = true
		}
	}
	if !foundConcat {
		t.Fatalf("stored generated functions = %v, want concat", sexpr.Functions)
	}
}

// TestT07ProviderExpressionDefaultFacts pins DEFAULT_GENERATED handling: the
// raw catalog bytes stay in DefaultValue while parsed facts land in
// DefaultExpression — including escaped quote delimiters.
func TestT07ProviderExpressionDefaultFacts(t *testing.T) {
	db := openTestDB(t, t07Results([][]driver.Value{
		{"a", "int", nil, nil, "", nil, "NO", "", nil},
		{"b", "int", nil, nil, "", "( `a` + 1 )", "YES", "DEFAULT_GENERATED", nil},
		{"j", "json", nil, nil, "", "json_object(_utf8mb4\\'k\\',1)", "YES", "DEFAULT_GENERATED", nil},
		{"l", "varchar(8)", nil, nil, "", "plain", "YES", "", nil},
	}))
	defer db.Close()
	snapshot, err := NewProvider(db).LoadTableSnapshot(context.Background(), spec.DialectMySQL, "golden", "t")
	if err != nil {
		t.Fatalf("LoadTableSnapshot: %v", err)
	}

	t07ColumnsExactly(t, snapshot, "a", "b", "j", "l")
	expr := t07Column(t, snapshot, "b")
	if !expr.HasDefault || expr.DefaultValue != "( `a` + 1 )" {
		t.Fatalf("expression default raw bytes lost: %+v", expr)
	}
	if expr.DefaultExpression == nil || !expr.DefaultExpression.Known ||
		len(expr.DefaultExpression.References) != 1 || expr.DefaultExpression.References[0].Name != "a" {
		t.Fatalf("expression default facts = %+v", expr.DefaultExpression)
	}
	if expr.DefaultExpression.Fingerprint == "" {
		t.Fatalf("expression default missing fingerprint")
	}

	json := t07Column(t, snapshot, "j")
	if json.DefaultExpression == nil || !json.DefaultExpression.Known {
		t.Fatalf("json expression default unknown: %+v", json.DefaultExpression)
	}
	foundJSON := false
	for _, fn := range json.DefaultExpression.Functions {
		if fn == "json_object" {
			foundJSON = true
		}
	}
	if !foundJSON {
		t.Fatalf("json default functions = %v, want json_object", json.DefaultExpression.Functions)
	}
	if json.DefaultValue != "json_object(_utf8mb4\\'k\\',1)" {
		t.Fatalf("raw default bytes = %q, want verbatim catalog text", json.DefaultValue)
	}

	literal := t07Column(t, snapshot, "l")
	if !literal.HasDefault || literal.DefaultValue != "plain" || literal.DefaultExpression != nil {
		t.Fatalf("literal default = %+v, want plain literal only", literal)
	}
}

// TestT07ProviderUndecodableCatalogExpressions pins the failure boundary: a
// generated or expression-default marker with absent/unparseable expression
// text produces explicit unknown facts, never nil (absence) or known-empty.
func TestT07ProviderUndecodableCatalogExpressions(t *testing.T) {
	db := openTestDB(t, t07Results([][]driver.Value{
		{"g", "int", nil, nil, "", nil, "YES", "STORED GENERATED", nil},
		{"d", "int", nil, nil, "", nil, "YES", "DEFAULT_GENERATED", nil},
	}))
	defer db.Close()
	snapshot, err := NewProvider(db).LoadTableSnapshot(context.Background(), spec.DialectMySQL, "golden", "t")
	if err != nil {
		t.Fatalf("LoadTableSnapshot: %v", err)
	}
	t07ColumnsExactly(t, snapshot, "g", "d")
	g := t07Column(t, snapshot, "g")
	if g.Generated == nil || g.Generated.Expression.Known {
		t.Fatalf("generated column without expression text = %+v, want explicit unknown", g.Generated)
	}
	d := t07Column(t, snapshot, "d")
	if d.DefaultExpression == nil || d.DefaultExpression.Known {
		t.Fatalf("default-generated column without text = %+v, want explicit unknown", d.DefaultExpression)
	}
}

// TestT07ProviderInvisibleColumn pins effective visibility from EXTRA: the
// INVISIBLE token is the only marker; its absence on a complete row is
// effective-visible, and the catalog never proves explicit declaration.
func TestT07ProviderInvisibleColumn(t *testing.T) {
	db := openTestDB(t, t07Results([][]driver.Value{
		{"a", "int", nil, nil, "", nil, "NO", "", nil},
		{"i", "int", nil, nil, "", nil, "YES", "INVISIBLE", nil},
	}))
	defer db.Close()
	snapshot, err := NewProvider(db).LoadTableSnapshot(context.Background(), spec.DialectMySQL, "golden", "t")
	if err != nil {
		t.Fatalf("LoadTableSnapshot: %v", err)
	}
	t07ColumnsExactly(t, snapshot, "a", "i")
	a := t07Column(t, snapshot, "a")
	if a.Visibility != spec.ColumnVisibilityVisible || a.VisibilityExplicit {
		t.Fatalf("visible column = visibility %q explicit %v", a.Visibility, a.VisibilityExplicit)
	}
	i := t07Column(t, snapshot, "i")
	if i.Visibility != spec.ColumnVisibilityInvisible || i.VisibilityExplicit {
		t.Fatalf("invisible column = visibility %q explicit %v", i.Visibility, i.VisibilityExplicit)
	}
}
