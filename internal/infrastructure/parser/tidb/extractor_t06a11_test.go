// Package tidbparser verifies T06-A11 single-member primary-key BIGINT/UNSIGNED
// extraction: the declared column facts and the ordered PK member binding the
// two policies consume.
// input: the eight frozen CREATE TABLE inputs, a table-level single-PK control,
// a non-PK-unsigned-precursor control, and a ZEROFILL parser-only observation,
// parsed through the real TiDB parser and extractor under both dialects
// output: Column.Type/Length/Unsigned/AutoIncrement and ordered DDL.PrimaryKey
// members pinned to the actual fields — never reconstructed from findings
// pos: infrastructure parser extractor regression for issue #85 T06-A11
// note: if this file changes, update this header and module README.md.
package tidbparser

import (
	"context"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// extractT06A11 parses and extracts with the given dialect so both the mysql
// and tidb stamps exercise the real parser/extractor path.
func extractT06A11(t *testing.T, dialect spec.Dialect, sql string) spec.Statement {
	t.Helper()

	parser := New()
	result, err := parser.Parse(context.Background(), sql)
	if err != nil {
		t.Fatalf("parse sql: %v", err)
	}
	wrapped := WrapStatements(result.Statements, result.Warnings)
	if len(wrapped) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(wrapped))
	}
	statement, err := wrapped[0].Extractor.Extract(dialect, wrapped[0].RawSQL)
	if err != nil {
		t.Fatalf("extract statement: %v", err)
	}
	return statement
}

// t06A11Column is the extracted-fact want for one declared column. Type is the
// parser-rendered text verbatim — its (11)/(20) width tokens are the parser's
// default rendering for an undeclared display width (Length=-1), never a
// numeric storage range and never a server COLUMN_TYPE oracle.
type t06A11Column struct {
	name          string
	typeText      string
	length        int
	unsigned      bool
	autoIncrement bool
}

// t06A11Inputs covers the eight frozen roles plus the two boundary controls.
// pkMembers is nil for the non-PK member-binding control.
var t06A11Inputs = []struct {
	name      string
	sql       string
	columns   []t06A11Column
	pkMembers []string
}{
	{name: "bigint-pass",
		sql: "CREATE TABLE t (id BIGINT PRIMARY KEY);",
		columns: []t06A11Column{
			{name: "id", typeText: "bigint(20)", length: -1},
		},
		pkMembers: []string{"id"}},
	{name: "bigint-reject",
		sql: "CREATE TABLE t (id INT PRIMARY KEY);",
		columns: []t06A11Column{
			{name: "id", typeText: "int(11)", length: -1},
		},
		pkMembers: []string{"id"}},
	{name: "bigint-composite-skip",
		sql: "CREATE TABLE t (id INT NOT NULL, tenant_id INT NOT NULL, PRIMARY KEY (id, tenant_id));",
		columns: []t06A11Column{
			{name: "id", typeText: "int(11)", length: -1},
			{name: "tenant_id", typeText: "int(11)", length: -1},
		},
		pkMembers: []string{"id", "tenant_id"}},
	{name: "bigint-off",
		sql: "CREATE TABLE t (id INT PRIMARY KEY);",
		columns: []t06A11Column{
			{name: "id", typeText: "int(11)", length: -1},
		},
		pkMembers: []string{"id"}},
	{name: "unsigned-pass",
		sql: "CREATE TABLE t (id BIGINT UNSIGNED PRIMARY KEY);",
		columns: []t06A11Column{
			{name: "id", typeText: "bigint(20) unsigned", length: -1, unsigned: true},
		},
		pkMembers: []string{"id"}},
	{name: "unsigned-reject",
		sql: "CREATE TABLE t (id BIGINT PRIMARY KEY);",
		columns: []t06A11Column{
			{name: "id", typeText: "bigint(20)", length: -1},
		},
		pkMembers: []string{"id"}},
	{name: "unsigned-composite-skip",
		sql: "CREATE TABLE t (id BIGINT NOT NULL, tenant_id INT NOT NULL, PRIMARY KEY (id, tenant_id));",
		columns: []t06A11Column{
			{name: "id", typeText: "bigint(20)", length: -1},
			{name: "tenant_id", typeText: "int(11)", length: -1},
		},
		pkMembers: []string{"id", "tenant_id"}},
	{name: "unsigned-off",
		sql: "CREATE TABLE t (id BIGINT PRIMARY KEY);",
		columns: []t06A11Column{
			{name: "id", typeText: "bigint(20)", length: -1},
		},
		pkMembers: []string{"id"}},
	// Table-level single-member control: PRIMARY KEY (id) binds the same id
	// column fact as the inline form — type, unsigned flag, and member list
	// are identical.
	{name: "table-level-single",
		sql: "CREATE TABLE t (id BIGINT UNSIGNED, PRIMARY KEY (id));",
		columns: []t06A11Column{
			{name: "id", typeText: "bigint(20) unsigned", length: -1, unsigned: true},
		},
		pkMembers: []string{"id"}},
	// Non-PK precursor control: an earlier BIGINT UNSIGNED column must not
	// stand in for the bound PK member — the rule reads the member named by
	// the PK list, not column order or "some unsigned column exists".
	{name: "non-pk-precursor",
		sql: "CREATE TABLE t (tag BIGINT UNSIGNED, id INT PRIMARY KEY);",
		columns: []t06A11Column{
			{name: "tag", typeText: "bigint(20) unsigned", length: -1, unsigned: true},
			{name: "id", typeText: "int(11)", length: -1},
		},
		pkMembers: []string{"id"}},
}

// TestExtractorT06A11PKMemberTypeAndUnsignedFacts pins the extracted column
// facts and the ordered PK member binding the two policies consume.
func TestExtractorT06A11PKMemberTypeAndUnsignedFacts(t *testing.T) {
	t.Parallel()
	for _, tc := range t06A11Inputs {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
				stmt := extractT06A11(t, dialect, tc.sql)
				if stmt.Dialect != dialect {
					t.Fatalf("dialect stamp = %q, want %q", stmt.Dialect, dialect)
				}
				if stmt.DDL == nil || stmt.DDL.Table == nil || stmt.DDL.Table.Name != "t" {
					t.Fatalf("[%s] not a create-table on t: %#v", dialect, stmt.DDL)
				}
				if len(stmt.DDL.Columns) != len(tc.columns) {
					t.Fatalf("[%s] columns = %#v, want %v", dialect, stmt.DDL.Columns, tc.columns)
				}
				for i, want := range tc.columns {
					column := stmt.DDL.Columns[i]
					if column.Name != want.name || column.Type != want.typeText ||
						column.Length != want.length || column.Unsigned != want.unsigned ||
						column.AutoIncrement != want.autoIncrement {
						t.Fatalf("[%s] column %d = %#v, want name=%s type=%q length=%d unsigned=%v auto_increment=%v",
							dialect, i, column, want.name, want.typeText, want.length, want.unsigned, want.autoIncrement)
					}
				}
				if stmt.DDL.PrimaryKey == nil {
					t.Fatalf("[%s] missing primary key in %#v", dialect, stmt.DDL)
				}
				if stmt.DDL.PrimaryKey.Kind != spec.IndexKindPrimary {
					t.Fatalf("[%s] pk kind = %q, want primary", dialect, stmt.DDL.PrimaryKey.Kind)
				}
				if len(stmt.DDL.PrimaryKey.Columns) != len(tc.pkMembers) {
					t.Fatalf("[%s] pk members = %#v, want %v", dialect, stmt.DDL.PrimaryKey.Columns, tc.pkMembers)
				}
				for i, name := range tc.pkMembers {
					if stmt.DDL.PrimaryKey.Columns[i] != name {
						t.Fatalf("[%s] pk member %d = %q, want %q", dialect, i, stmt.DDL.PrimaryKey.Columns[i], name)
					}
				}
			}
		})
	}
}

// TestExtractorT06A11PKMemberBindsDeclaredColumn pins that each PK member name
// resolves back to exactly one declared column — the fact the rule's
// primaryKeyColumnSpecs binding consumes — and that the non-PK precursor keeps
// its own unsigned fact without polluting the bound member.
func TestExtractorT06A11PKMemberBindsDeclaredColumn(t *testing.T) {
	t.Parallel()
	for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
		stmt := extractT06A11(t, dialect,
			"CREATE TABLE t (tag BIGINT UNSIGNED, id INT PRIMARY KEY);")
		bound := map[string]spec.Column{}
		for _, column := range stmt.DDL.Columns {
			bound[column.Name] = column
		}
		if len(bound) != len(stmt.DDL.Columns) {
			t.Fatalf("[%s] duplicate column names: %#v", dialect, stmt.DDL.Columns)
		}
		member := bound[stmt.DDL.PrimaryKey.Columns[0]]
		if member.Name != "id" || member.Type != "int(11)" || member.Unsigned {
			t.Fatalf("[%s] bound PK member = %#v, want signed int(11) id — the precursor's unsigned fact must not substitute",
				dialect, member)
		}
	}
}

// TestExtractorT06A11ZerofillParserObservation records the ZEROFILL parser
// boundary exactly as extracted: the parser flag set carries both UNSIGNED
// and ZEROFILL, so spec.Unsigned reads true and the type text carries the
// zerofill suffix. Parser acceptance plus the implied unsigned flag is a
// parser-layer fact only — it is not a four-anchor native-legality claim,
// a recommendation, or a version-governance verdict.
func TestExtractorT06A11ZerofillParserObservation(t *testing.T) {
	t.Parallel()
	for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
		stmt := extractT06A11(t, dialect,
			"CREATE TABLE t (id BIGINT ZEROFILL PRIMARY KEY);")
		if stmt.DDL == nil || len(stmt.DDL.Columns) != 1 {
			t.Fatalf("[%s] columns = %#v, want one", dialect, stmt.DDL)
		}
		column := stmt.DDL.Columns[0]
		if column.Name != "id" || column.Type != "bigint(20) unsigned zerofill" ||
			column.Length != -1 || !column.Unsigned || column.AutoIncrement {
			t.Fatalf("[%s] zerofill column = %#v, want unsigned+zerofill type text",
				dialect, column)
		}
		if stmt.DDL.PrimaryKey == nil ||
			len(stmt.DDL.PrimaryKey.Columns) != 1 ||
			stmt.DDL.PrimaryKey.Columns[0] != "id" {
			t.Fatalf("[%s] pk = %#v, want single member id", dialect, stmt.DDL.PrimaryKey)
		}
	}
}
