// Package tidbparser verifies the T06-A2 CREATE TABLE primary-key member
// nullability normalization and the preserved default/alter fact contract.
// input: MySQL and TiDB CREATE TABLE inputs through the real parser and extractor
// output: normalized Column.NotNull/DDL.PrimaryKey member facts and unchanged default/alter extractions
// pos: infrastructure parser extractor test coverage for issue #85 T06-A2
// note: if this file changes, update this header and module README.md.
package tidbparser

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// t06a2Extract parses and extracts one statement with the requested dialect so
// a case table can cover the MySQL and TiDB paths symmetrically.
func t06a2Extract(t *testing.T, dialect spec.Dialect, sql string) spec.Statement {
	t.Helper()
	parser := New()
	result, err := parser.Parse(context.Background(), sql)
	if err != nil {
		t.Fatalf("parse %s %q: %v", dialect, sql, err)
	}
	wrapped := WrapStatements(result.Statements, result.Warnings)
	if len(wrapped) != 1 {
		t.Fatalf("parsed statements for %q = %d, want 1", sql, len(wrapped))
	}
	statement, err := wrapped[0].Extractor.Extract(dialect, wrapped[0].RawSQL)
	if err != nil {
		t.Fatalf("extract %s %q: %v", dialect, sql, err)
	}
	return statement
}

// t06a2CheckColumns asserts declaration order and per-column NotNull facts.
func t06a2CheckColumns(t *testing.T, statement spec.Statement, order []string, notNull map[string]bool) {
	t.Helper()
	ddl := statement.DDL
	if ddl == nil || len(ddl.Columns) != len(order) {
		t.Fatalf("columns = %+v, want %d", ddl, len(order))
	}
	for i, name := range order {
		column := ddl.Columns[i]
		if column.Name != name {
			t.Fatalf("column[%d] = %q, want %q — declaration order must be preserved", i, column.Name, name)
		}
		if column.NotNull != notNull[name] {
			t.Fatalf("column %q NotNull = %t, want %t", name, column.NotNull, notNull[name])
		}
	}
}

// t06a2CheckPK asserts the normalized primary-key identity.
func t06a2CheckPK(t *testing.T, statement spec.Statement, columns []string) {
	t.Helper()
	pk := statement.DDL.PrimaryKey
	if columns == nil {
		if pk != nil {
			t.Fatalf("PrimaryKey = %+v, want nil", pk)
		}
		return
	}
	if pk == nil || pk.Name != "primary" || pk.Kind != spec.IndexKindPrimary ||
		!reflect.DeepEqual(pk.Columns, columns) {
		t.Fatalf("PrimaryKey = %+v, want primary/%v", pk, columns)
	}
}

// TestT06A2PrimaryKeyMemberNullability pins that every primary-key member form
// carries the database-implied NOT NULL fact, while an explicit NULL
// declaration keeps the conflict visible to the policy layer.
func TestT06A2PrimaryKeyMemberNullability(t *testing.T) {
	cases := []struct {
		name    string
		sql     string
		order   []string
		notNull map[string]bool
		pk      []string
	}{
		{"inline", "CREATE TABLE t (id INT PRIMARY KEY);",
			[]string{"id"}, map[string]bool{"id": true}, []string{"id"}},
		{"table_single", "CREATE TABLE t (id INT, PRIMARY KEY (id));",
			[]string{"id"}, map[string]bool{"id": true}, []string{"id"}},
		{"table_explicit_not_null", "CREATE TABLE t (id INT NOT NULL, PRIMARY KEY (id));",
			[]string{"id"}, map[string]bool{"id": true}, []string{"id"}},
		{"table_composite_reordered", "CREATE TABLE t (a INT, spare INT, b INT, PRIMARY KEY (b,a));",
			[]string{"a", "spare", "b"}, map[string]bool{"a": true, "spare": false, "b": true}, []string{"b", "a"}},
		{"table_prefix_member", "CREATE TABLE t (id INT, PRIMARY KEY (id(4)));",
			[]string{"id"}, map[string]bool{"id": true}, []string{"id"}},
		{"explicit_null_table", "CREATE TABLE t (id INT NULL, PRIMARY KEY (id));",
			[]string{"id"}, map[string]bool{"id": false}, []string{"id"}},
		{"explicit_null_inline", "CREATE TABLE t (id INT NULL PRIMARY KEY);",
			[]string{"id"}, map[string]bool{"id": false}, []string{"id"}},
		{"no_pk", "CREATE TABLE t (id INT);",
			[]string{"id"}, map[string]bool{"id": false}, nil},
		{"secondary_key", "CREATE TABLE t (id INT, KEY k_id (id));",
			[]string{"id"}, map[string]bool{"id": false}, nil},
		{"unique_key", "CREATE TABLE t (id INT, UNIQUE KEY u_id (id));",
			[]string{"id"}, map[string]bool{"id": false}, nil},
		{"foreign_key_member", "CREATE TABLE t (id INT, FOREIGN KEY (id) REFERENCES p (id));",
			[]string{"id"}, map[string]bool{"id": false}, nil},
		{"mixed_case_binding", "CREATE TABLE t (UserID INT, PRIMARY KEY (UserID));",
			[]string{"userid"}, map[string]bool{"userid": true}, []string{"userid"}},
	}
	for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
		for _, tc := range cases {
			t.Run(fmt.Sprintf("%s/%s", dialect, tc.name), func(t *testing.T) {
				statement := t06a2Extract(t, dialect, tc.sql)
				t06a2CheckColumns(t, statement, tc.order, tc.notNull)
				t06a2CheckPK(t, statement, tc.pk)
			})
		}
	}
}

// TestT06A2PrimaryKeyIndexFactsPreserved pins that normalization only touches
// member-column NotNull: key-part facts recorded beside the index stay intact.
func TestT06A2PrimaryKeyIndexFactsPreserved(t *testing.T) {
	statement := t06a2Extract(t, spec.DialectMySQL,
		"CREATE TABLE t (id INT, tag VARCHAR(8), PRIMARY KEY (id(4), tag DESC));")
	pk := statement.DDL.PrimaryKey
	if pk == nil || pk.PrefixParts != 1 || pk.DescParts != 1 ||
		!reflect.DeepEqual(pk.Columns, []string{"id", "tag"}) {
		t.Fatalf("PrimaryKey = %+v, want [id tag] with prefix=1 desc=1", pk)
	}
	t06a2CheckColumns(t, statement, []string{"id", "tag"}, map[string]bool{"id": true, "tag": true})
}

// TestT06A2DefaultFactsPreserved pins that the nullability fix does not touch
// the modeled default contract: absent clause, explicit DEFAULT NULL, and
// literal defaults stay distinguishable.
func TestT06A2DefaultFactsPreserved(t *testing.T) {
	statement := t06a2Extract(t, spec.DialectMySQL,
		"CREATE TABLE t (a INT, b INT DEFAULT NULL, c INT DEFAULT 0, d VARCHAR(4) DEFAULT '');")
	columns := statement.DDL.Columns
	if columns[0].HasDefault || columns[0].DefaultIsNull {
		t.Fatalf("a = %+v, want no explicit DEFAULT clause modeled", columns[0])
	}
	// Explicit DEFAULT NULL satisfies "has a default" and records the typed
	// NULL literal fact (T06-A3): DefaultValue is the normalized text "NULL"
	// and DefaultIsNull=true.
	if !columns[1].HasDefault || !columns[1].DefaultIsNull || columns[1].DefaultValue != "NULL" {
		t.Fatalf("b = %+v, want typed DEFAULT NULL recorded", columns[1])
	}
	if !columns[2].HasDefault || columns[2].DefaultIsNull || columns[2].DefaultValue != "0" {
		t.Fatalf("c = %+v, want DEFAULT 0 distinct from an absent clause", columns[2])
	}
	if !columns[3].HasDefault || columns[3].DefaultIsNull || columns[3].DefaultValue != "''" {
		t.Fatalf("d = %+v, want DEFAULT '' distinct from an absent clause", columns[3])
	}

	// A PK member carrying DEFAULT NULL keeps its default fact; DEFAULT NULL is
	// not a NULL nullability declaration and must not mask implied NOT NULL.
	pkStatement := t06a2Extract(t, spec.DialectMySQL,
		"CREATE TABLE t (id INT DEFAULT NULL, PRIMARY KEY (id));")
	id := pkStatement.DDL.Columns[0]
	if !id.NotNull || !id.HasDefault || !id.DefaultIsNull || id.DefaultValue != "NULL" {
		t.Fatalf("id = %+v, want NotNull with the typed DEFAULT NULL fact preserved", id)
	}
}

// TestT06A2AlterColumnExtractionUnchanged pins that the CREATE-scoped
// normalization does not leak into shared column extraction used by ALTER.
func TestT06A2AlterColumnExtractionUnchanged(t *testing.T) {
	statement := t06a2Extract(t, spec.DialectMySQL,
		"ALTER TABLE t MODIFY COLUMN id INT NOT NULL, ADD COLUMN c INT NULL;")
	if len(statement.DDL.Alter) != 2 {
		t.Fatalf("alters = %+v, want 2", statement.DDL.Alter)
	}
	modify := statement.DDL.Alter[0]
	if modify.Action != "modify_column" || modify.Column == nil ||
		modify.Column.Definition == nil || !modify.Column.Definition.NotNull {
		t.Fatalf("modify alter = %+v, want target column NOT NULL", modify)
	}
	add := statement.DDL.Alter[1]
	if add.Column == nil || add.Column.Definition == nil || add.Column.Definition.NotNull {
		t.Fatalf("add alter = %+v, want the declared NULL column to stay nullable on ALTER", add)
	}
}
