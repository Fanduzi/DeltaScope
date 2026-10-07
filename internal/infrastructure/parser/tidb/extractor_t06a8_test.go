// Package tidbparser verifies the T06-A8 comment fact contract: column COMMENT
// stores the parser-decoded string content — never a quoted SQL literal — and
// repeated COMMENT options overwrite per occurrence. ALTER column definitions
// share the same decoding.
// input: MySQL and TiDB CREATE TABLE plus ALTER column-definition inputs with comment literals
// output: Column.Comment/Table.Comment facts equal the decoded comment text, including empty and edge literals
// pos: parser-layer contract tests for issue #85 T06-A8
// note: if this file changes, update this header and module README.md.
package tidbparser

import (
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// TestT06A8CommentDecodingMatrix pins the decoded content of table and column
// comments for representative literal shapes: absent, empty, whitespace,
// multibyte, embedded apostrophes, and preserved edge spaces.
func TestT06A8CommentDecodingMatrix(t *testing.T) {
	cases := []struct {
		name       string
		sql        string
		wantTable  string
		wantOpt    string // Options["comment"]; "" means absent or empty
		wantOptSet bool
		wantColumn string
	}{
		{"none", "CREATE TABLE t (c INT);", "", "", false, ""},
		{"table_empty", "CREATE TABLE t (c INT) COMMENT='';", "", "", true, ""},
		{"table_blank", "CREATE TABLE t (c INT) COMMENT='   ';", "   ", "   ", true, ""},
		{"table_multibyte", "CREATE TABLE t (c INT) COMMENT='中文注';", "中文注", "中文注", true, ""},
		{"table_edge_spaces", "CREATE TABLE t (c INT) COMMENT='  pad  ';", "  pad  ", "  pad  ", true, ""},
		{"table_apostrophe", "CREATE TABLE t (c INT) COMMENT='it''s 中文';", "it's 中文", "it's 中文", true, ""},
		{"table_lone_quote", "CREATE TABLE t (c INT) COMMENT='''';", "'", "'", true, ""},
		{"column_empty", "CREATE TABLE t (c INT COMMENT '');", "", "", false, ""},
		{"column_blank", "CREATE TABLE t (c INT COMMENT '   ');", "", "", false, "   "},
		{"column_multibyte", "CREATE TABLE t (c INT COMMENT '中文注');", "", "", false, "中文注"},
		{"column_edge_spaces", "CREATE TABLE t (c INT COMMENT '  pad  ');", "", "", false, "  pad  "},
		{"column_apostrophe", "CREATE TABLE t (c INT COMMENT 'it''s 中文');", "", "", false, "it's 中文"},
		{"column_lone_quote", "CREATE TABLE t (c INT COMMENT '''');", "", "", false, "'"},
		{"both", "CREATE TABLE t (c INT COMMENT '列注') COMMENT='表注';", "表注", "表注", true, "列注"},
	}
	for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
		for _, tc := range cases {
			t.Run(string(dialect)+"/"+tc.name, func(t *testing.T) {
				statement := t06a2Extract(t, dialect, tc.sql)
				if statement.DDL.Table.Comment != tc.wantTable {
					t.Fatalf("Table.Comment = %q, want %q", statement.DDL.Table.Comment, tc.wantTable)
				}
				got, present := statement.DDL.Options["comment"]
				if present != tc.wantOptSet || got != tc.wantOpt {
					t.Fatalf("Options[comment] = %q present=%v, want %q present=%v", got, present, tc.wantOpt, tc.wantOptSet)
				}
				if got := statement.DDL.Columns[0].Comment; got != tc.wantColumn {
					t.Fatalf("Columns[0].Comment = %q, want %q", got, tc.wantColumn)
				}
			})
		}
	}
}

// TestT06A8RepeatedCommentOverwrite pins per-occurrence assignment: a repeated
// COMMENT option replaces the earlier value in both directions, including a
// trailing empty literal clearing a prior non-empty one.
func TestT06A8RepeatedCommentOverwrite(t *testing.T) {
	cases := []struct {
		name       string
		sql        string
		wantColumn string
	}{
		{"nonempty_to_empty", "CREATE TABLE t (c INT COMMENT 'first' COMMENT '');", ""},
		{"empty_to_nonempty", "CREATE TABLE t (c INT COMMENT '' COMMENT 'second');", "second"},
		{"nonempty_to_nonempty", "CREATE TABLE t (c INT COMMENT 'first' COMMENT 'second');", "second"},
	}
	for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
		for _, tc := range cases {
			t.Run(string(dialect)+"/"+tc.name, func(t *testing.T) {
				statement := t06a2Extract(t, dialect, tc.sql)
				if got := statement.DDL.Columns[0].Comment; got != tc.wantColumn {
					t.Fatalf("Columns[0].Comment = %q, want %q", got, tc.wantColumn)
				}
			})
		}
	}
}

// TestT06A8AlterColumnCommentDecoding pins the shared extractColumn path for
// ALTER ADD/MODIFY/CHANGE complete definitions — comment facts decode the same
// way there. No ALTER policy is added; this is fact fidelity only.
func TestT06A8AlterColumnCommentDecoding(t *testing.T) {
	cases := []struct {
		name string
		sql  string
		col  string // column name inside the action definition
		want string
	}{
		{"add", "ALTER TABLE t ADD COLUMN d INT COMMENT '加列';", "d", "加列"},
		{"modify", "ALTER TABLE t MODIFY COLUMN c VARCHAR(8) COMMENT '改列';", "c", "改列"},
		{"change", "ALTER TABLE t CHANGE COLUMN c c2 INT COMMENT '换名';", "c2", "换名"},
		{"add_empty", "ALTER TABLE t ADD COLUMN d INT COMMENT '';", "d", ""},
	}
	for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
		for _, tc := range cases {
			t.Run(string(dialect)+"/"+tc.name, func(t *testing.T) {
				statement := t06a2Extract(t, dialect, tc.sql)
				if statement.DDL == nil || len(statement.DDL.Alter) == 0 {
					t.Fatalf("alter actions = %+v, want at least one", statement.DDL)
				}
				def := statement.DDL.Alter[0].Column.Definition
				if def == nil {
					t.Fatalf("alter[0].Column.Definition = nil, want column definition")
				}
				if def.Name != tc.col || def.Comment != tc.want {
					t.Fatalf("alter comment = name %q comment %q, want %q/%q", def.Name, def.Comment, tc.col, tc.want)
				}
			})
		}
	}
}
