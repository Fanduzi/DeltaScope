// Package tidbparser verifies T06-A12 ordinary CREATE TABLE non-PK-column
// nullability extraction: the declared facts and the AST option shapes the
// NOT NULL policy consumes.
// input: the frozen A12 CREATE TABLE inputs, an explicit-NULL parser-only
// control, and a reordered mixed-column control, parsed through the real
// TiDB parser and extractor under both dialects
// output: Column.Name/Type/NotNull ordering, absent primary key, absent
// default/on-update flags, and the AST-level distinction between an omitted
// nullability clause, ColumnOptionNull, and ColumnOptionNotNull
// pos: infrastructure parser extractor regression for issue #85 T06-A12
// note: if this file changes, update this header and module README.md.
package tidbparser

import (
	"context"
	"testing"

	"github.com/pingcap/tidb/pkg/parser/ast"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// parseExtractT06A12 returns both the raw AST statement and the extracted
// spec so a test can pin the declaration provenance the spec model carries —
// and the clause shape it deliberately collapses.
func parseExtractT06A12(t *testing.T, dialect spec.Dialect, sql string) (*ast.CreateTableStmt, spec.Statement) {
	t.Helper()

	parser := New()
	result, err := parser.Parse(context.Background(), sql)
	if err != nil {
		t.Fatalf("parse sql: %v", err)
	}
	if len(result.Statements) != 1 {
		t.Fatalf("expected 1 ast statement, got %d", len(result.Statements))
	}
	create, ok := result.Statements[0].(*ast.CreateTableStmt)
	if !ok {
		t.Fatalf("statement is %T, want *ast.CreateTableStmt", result.Statements[0])
	}
	wrapped := WrapStatements(result.Statements, result.Warnings)
	statement, err := wrapped[0].Extractor.Extract(dialect, wrapped[0].RawSQL)
	if err != nil {
		t.Fatalf("extract statement: %v", err)
	}
	return create, statement
}

// columnOptionTypes lists the declared column option kind per column, in
// declaration order — an omitted nullability clause is an absent option, not
// a ColumnOptionNull.
func columnOptionTypes(create *ast.CreateTableStmt, columnIndex int) []ast.ColumnOptionType {
	types := []ast.ColumnOptionType{}
	for _, option := range create.Cols[columnIndex].Options {
		types = append(types, option.Tp)
	}
	return types
}

// t06A12Column is the extracted-fact want for one declared column. Type is
// the parser-rendered text verbatim: `int(11)` is the parser's default
// display-width rendering for an undeclared width (Length stays -1), and a
// bare `datetime` renders with Length 19 — neither is a user-declared fsp
// and neither is a server COLUMN_TYPE oracle.
type t06A12Column struct {
	name     string
	typeText string
	notNull  bool
	length   int
}

var t06A12Inputs = []struct {
	name    string
	sql     string
	columns []t06A12Column
}{
	{name: "ordinary-nullable",
		sql: "CREATE TABLE t (c INT);",
		columns: []t06A12Column{
			{name: "c", typeText: "int(11)", notNull: false, length: -1}}},
	{name: "ordinary-not-null",
		sql: "CREATE TABLE t (c INT NOT NULL);",
		columns: []t06A12Column{
			{name: "c", typeText: "int(11)", notNull: true, length: -1}}},
	{name: "time-exempt",
		sql: "CREATE TABLE t (c DATETIME NULL);",
		columns: []t06A12Column{
			{name: "c", typeText: "datetime", notNull: false, length: 19}}},
	{name: "text-exempt",
		sql: "CREATE TABLE t (payload TEXT);",
		columns: []t06A12Column{
			{name: "payload", typeText: "text", notNull: false, length: -1}}},
	{name: "mixed-attribution",
		sql: "CREATE TABLE t (ordinary INT, time_c DATETIME NULL, payload TEXT);",
		columns: []t06A12Column{
			{name: "ordinary", typeText: "int(11)", notNull: false, length: -1},
			{name: "time_c", typeText: "datetime", notNull: false, length: 19},
			{name: "payload", typeText: "text", notNull: false, length: -1}}},
	{name: "reordered-mixed",
		sql: "CREATE TABLE t (time_c DATETIME NULL, ordinary INT);",
		columns: []t06A12Column{
			{name: "time_c", typeText: "datetime", notNull: false, length: 19},
			{name: "ordinary", typeText: "int(11)", notNull: false, length: -1}}},
}

// TestExtractorT06A12NullabilityFacts pins the extracted per-column facts the
// NOT NULL policy consumes: name/order/Type/NotNull, an absent primary key,
// and no default or on-update semantics riding along.
func TestExtractorT06A12NullabilityFacts(t *testing.T) {
	t.Parallel()
	for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
		for _, tc := range t06A12Inputs {
			tc := tc
			t.Run(string(dialect)+"/"+tc.name, func(t *testing.T) {
				t.Parallel()
				_, statement := parseExtractT06A12(t, dialect, tc.sql)
				if statement.Kind != spec.KindDDL || statement.DDL == nil {
					t.Fatalf("statement kind = %v, want ddl", statement.Kind)
				}
				if len(statement.DDL.Columns) != len(tc.columns) {
					t.Fatalf("columns = %#v, want %d", statement.DDL.Columns, len(tc.columns))
				}
				for i, want := range tc.columns {
					column := statement.DDL.Columns[i]
					if column.Name != want.name || column.Type != want.typeText ||
						column.NotNull != want.notNull || column.Length != want.length {
						t.Fatalf("column[%d] = %#v, want %s %s notNull=%v length=%d",
							i, column, want.name, want.typeText, want.notNull, want.length)
					}
					if column.HasDefault || column.DefaultIsCurrentTimestamp ||
						column.OnUpdateCurrentTimestamp {
						t.Fatalf("column[%d] = %#v carries default/update semantics", i, column)
					}
				}
				if statement.DDL.PrimaryKey != nil || len(statement.DDL.Constraints) != 0 {
					t.Fatalf("pk/constraints = %#v/%#v, want none",
						statement.DDL.PrimaryKey, statement.DDL.Constraints)
				}
			})
		}
	}
}

// TestExtractorT06A12NullClauseASTShape pins the clause-level distinction the
// spec model collapses: an omitted nullability clause carries no option,
// explicit NULL carries ColumnOptionNull, and NOT NULL carries
// ColumnOptionNotNull — while extracted NotNull is false for both the omitted
// and the explicit-NULL form and true only for NOT NULL. This records the
// current representational boundary; it does not claim a new provenance
// field.
func TestExtractorT06A12NullClauseASTShape(t *testing.T) {
	t.Parallel()
	inputs := []struct {
		name        string
		sql         string
		wantOptions []ast.ColumnOptionType
		wantNotNull bool
	}{
		{name: "omitted-nullability", sql: "CREATE TABLE t (c INT);",
			wantOptions: []ast.ColumnOptionType{}, wantNotNull: false},
		{name: "explicit-null", sql: "CREATE TABLE t (c INT NULL);",
			wantOptions: []ast.ColumnOptionType{ast.ColumnOptionNull}, wantNotNull: false},
		{name: "not-null", sql: "CREATE TABLE t (c INT NOT NULL);",
			wantOptions: []ast.ColumnOptionType{ast.ColumnOptionNotNull}, wantNotNull: true},
	}
	for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
		for _, tc := range inputs {
			tc := tc
			t.Run(string(dialect)+"/"+tc.name, func(t *testing.T) {
				t.Parallel()
				create, statement := parseExtractT06A12(t, dialect, tc.sql)
				gotOptions := columnOptionTypes(create, 0)
				if len(gotOptions) != len(tc.wantOptions) {
					t.Fatalf("options = %v, want %v", gotOptions, tc.wantOptions)
				}
				for i, want := range tc.wantOptions {
					if gotOptions[i] != want {
						t.Fatalf("option[%d] = %v, want %v", i, gotOptions[i], want)
					}
				}
				if len(statement.DDL.Columns) != 1 ||
					statement.DDL.Columns[0].NotNull != tc.wantNotNull {
					t.Fatalf("extracted columns = %#v, want one column notNull=%v",
						statement.DDL.Columns, tc.wantNotNull)
				}
			})
		}
	}
}
