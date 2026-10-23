// Package tidb verifies the T10 advanced-index and visibility facts.
// input: ALTER INDEX VISIBLE|INVISIBLE actions, declared index visibility
// attributes on every index-producing path, and CAST(... AS ... ARRAY)
// multi-valued key parts, through the real parse+extract seam
// output: spec.AlterIndex carrying the target name plus the requested
// visibility under a bounded alter_index payload; Index.Visibility/
// VisibilityExplicit modeled facts instead of bounded option names;
// Index.MultiValuedParts counting array-cast members that still count as
// expression key parts
// pos: extractor contract tests for the T10 index visibility model (issue #89)
// note: if this file changes, update this header and module README.md.
package tidbparser

import (
	"context"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func extractOne(t *testing.T, dialect spec.Dialect, sql string) spec.Statement {
	t.Helper()
	parser := New()
	result, err := parser.Parse(context.Background(), sql)
	if err != nil {
		t.Fatalf("parse %q: %v", sql, err)
	}
	wrapped := WrapStatements(result.Statements, result.Warnings)
	if len(wrapped) != 1 {
		t.Fatalf("expected one statement, got %d", len(wrapped))
	}
	statement, err := wrapped[0].Extractor.Extract(dialect, wrapped[0].RawSQL)
	if err != nil {
		t.Fatalf("extract %q: %v", sql, err)
	}
	return statement
}

// The alter_index action must carry its target and requested visibility
// under the shared AlterIndex payload — not a bare action name.
func TestExtractorT10AlterIndexVisibility(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name      string
		sql       string
		wantVis   string
		wantIndex string
	}{
		{name: "invisible", sql: "ALTER TABLE t ALTER INDEX idx_c INVISIBLE", wantVis: spec.ColumnVisibilityInvisible, wantIndex: "idx_c"},
		{name: "visible", sql: "ALTER TABLE t ALTER INDEX idx_c VISIBLE", wantVis: spec.ColumnVisibilityVisible, wantIndex: "idx_c"},
		{name: "backticked primary", sql: "ALTER TABLE t ALTER INDEX `PRIMARY` INVISIBLE", wantVis: spec.ColumnVisibilityInvisible, wantIndex: "primary"},
	}
	for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
		for _, tc := range cases {
			tc := tc
			t.Run(string(dialect)+"/"+tc.name, func(t *testing.T) {
				t.Parallel()
				statement := extractOne(t, dialect, tc.sql)
				if len(statement.DDL.Alter) != 1 {
					t.Fatalf("expected one alter action, got %#v", statement.DDL.Alter)
				}
				alter := statement.DDL.Alter[0]
				if alter.Action != "alter_index" {
					t.Fatalf("action = %q, want alter_index", alter.Action)
				}
				if alter.Index == nil || alter.Index.OldName != tc.wantIndex {
					t.Fatalf("target = %#v, want OldName %q", alter.Index, tc.wantIndex)
				}
				def := alter.Index.Definition
				if def == nil || def.Visibility != tc.wantVis || !def.VisibilityExplicit {
					t.Fatalf("definition = %#v, want visibility=%q explicit", def, tc.wantVis)
				}
			})
		}
	}
}

// Declared VISIBLE|INVISIBLE index attributes model into Visibility/
// VisibilityExplicit on every index-producing path and stop leaking the
// bounded option names.
func TestExtractorT10DeclaredIndexVisibility(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name      string
		sql       string
		wantVis   string
		wantIndex func(*testing.T, spec.Statement) spec.Index
	}{
		{
			name: "create_index invisible",
			sql:  "CREATE INDEX i ON t(a) INVISIBLE",
			wantIndex: func(t *testing.T, s spec.Statement) spec.Index {
				if len(s.DDL.Alter) != 1 || s.DDL.Alter[0].Index == nil || s.DDL.Alter[0].Index.Definition == nil {
					t.Fatalf("create_index payload missing, got %#v", s.DDL.Alter)
				}
				return *s.DDL.Alter[0].Index.Definition
			},
		},
		{
			name:    "create_index visible",
			sql:     "CREATE INDEX i ON t(a) VISIBLE",
			wantVis: spec.ColumnVisibilityVisible,
			wantIndex: func(t *testing.T, s spec.Statement) spec.Index {
				return *s.DDL.Alter[0].Index.Definition
			},
		},
		{
			name: "create_table key invisible",
			sql:  "CREATE TABLE t (a INT, KEY i (a) INVISIBLE)",
			wantIndex: func(t *testing.T, s spec.Statement) spec.Index {
				if len(s.DDL.Indexes) != 1 {
					t.Fatalf("create_table index missing, got %#v", s.DDL.Indexes)
				}
				return s.DDL.Indexes[0]
			},
		},
		{
			name: "create_table primary invisible",
			sql:  "CREATE TABLE t (a INT, PRIMARY KEY (a) INVISIBLE)",
			wantIndex: func(t *testing.T, s spec.Statement) spec.Index {
				if s.DDL.PrimaryKey == nil {
					t.Fatalf("primary key missing")
				}
				return *s.DDL.PrimaryKey
			},
		},
		{
			name: "alter add_index invisible",
			sql:  "ALTER TABLE t ADD INDEX i (a) INVISIBLE",
			wantIndex: func(t *testing.T, s spec.Statement) spec.Index {
				if len(s.DDL.Alter) != 1 || s.DDL.Alter[0].Index == nil || s.DDL.Alter[0].Index.Definition == nil {
					t.Fatalf("add_index payload missing, got %#v", s.DDL.Alter)
				}
				return *s.DDL.Alter[0].Index.Definition
			},
		},
		{
			name:    "alter add unique visible",
			sql:     "ALTER TABLE t ADD CONSTRAINT u UNIQUE (a) VISIBLE",
			wantVis: spec.ColumnVisibilityVisible,
			wantIndex: func(t *testing.T, s spec.Statement) spec.Index {
				return *s.DDL.Alter[0].Index.Definition
			},
		},
	}
	for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
		for _, tc := range cases {
			tc := tc
			t.Run(string(dialect)+"/"+tc.name, func(t *testing.T) {
				t.Parallel()
				statement := extractOne(t, dialect, tc.sql)
				index := tc.wantIndex(t, statement)
				want := tc.wantVis
				if want == "" {
					want = spec.ColumnVisibilityInvisible
				}
				if index.Visibility != want || !index.VisibilityExplicit {
					t.Fatalf("visibility = %q explicit=%v, want %q explicit", index.Visibility, index.VisibilityExplicit, want)
				}
				for _, option := range index.UnmodeledOptions {
					if option == "visible" || option == "invisible" {
						t.Fatalf("modeled visibility must leave UnmodeledOptions, got %v", index.UnmodeledOptions)
					}
				}
			})
		}
	}
}

// An index without a visibility attribute carries no modeled fact and no
// explicit marker.
func TestExtractorT10NoVisibilityAttribute(t *testing.T) {
	t.Parallel()
	statement := extractOne(t, spec.DialectMySQL, "CREATE TABLE t (a INT, KEY i (a))")
	index := statement.DDL.Indexes[0]
	if index.Visibility != "" || index.VisibilityExplicit {
		t.Fatalf("plain index must carry no visibility fact, got %#v", index)
	}
}

// CAST(… AS … ARRAY) members count as multi-valued key parts while staying
// expression members; a non-array cast counts as a plain expression only.
func TestExtractorT10MultiValuedKeyParts(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		sql      string
		wantMV   int
		wantExpr int
	}{
		{name: "array cast", sql: "CREATE INDEX i ON t((CAST(j AS UNSIGNED ARRAY)))", wantMV: 1, wantExpr: 1},
		{name: "json array cast", sql: "CREATE INDEX i ON t((CAST(j->'$.k' AS CHAR(8) ARRAY)))", wantMV: 1, wantExpr: 1},
		{name: "scalar cast is not multi-valued", sql: "CREATE INDEX i ON t((CAST(j AS SIGNED)))", wantExpr: 1},
		{name: "plain expression", sql: "CREATE INDEX i ON t((a+1))", wantExpr: 1},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			statement := extractOne(t, spec.DialectMySQL, tc.sql)
			index := *statement.DDL.Alter[0].Index.Definition
			if index.MultiValuedParts != tc.wantMV {
				t.Fatalf("MultiValuedParts = %d, want %d", index.MultiValuedParts, tc.wantMV)
			}
			if index.ExpressionCount != tc.wantExpr || index.HasExpressionKeys != (tc.wantExpr > 0) {
				t.Fatalf("expression marks = %d/%v, want %d", index.ExpressionCount, index.HasExpressionKeys, tc.wantExpr)
			}
		})
	}
}
