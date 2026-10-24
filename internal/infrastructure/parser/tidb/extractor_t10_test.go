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

// Member-level facts: every key part keeps its ordinal, kind, bound column,
// explicit prefix length (including zero), declared direction, and — for
// expression members — the structured identity and column dependencies.
func TestExtractorT10R1MemberLevelKeyParts(t *testing.T) {
	t.Parallel()
	statement := extractOne(t, spec.DialectMySQL,
		"CREATE TABLE t (a INT, b VARCHAR(64), c INT, UNIQUE KEY u (a DESC, b(0), (c+1)))")
	index := statement.DDL.Indexes[0]
	if len(index.KeyParts) != 3 {
		t.Fatalf("KeyParts = %+v, want 3 members", index.KeyParts)
	}
	if index.KeyParts[0].Ordinal != 1 || index.KeyParts[0].Kind != spec.IndexKeyPartColumn ||
		index.KeyParts[0].Column != "a" || !index.KeyParts[0].Descending {
		t.Fatalf("member 1 must be a descending column a, got %+v", index.KeyParts[0])
	}
	if index.KeyParts[1].Ordinal != 2 || index.KeyParts[1].Column != "b" ||
		index.KeyParts[1].PrefixLength == nil || *index.KeyParts[1].PrefixLength != 0 {
		t.Fatalf("member 2 must carry the explicit zero prefix, got %+v", index.KeyParts[1])
	}
	expr := index.KeyParts[2]
	if expr.Ordinal != 3 || expr.Kind != spec.IndexKeyPartExpression || expr.Expression == nil ||
		!expr.Expression.Known || len(expr.Expression.References) != 1 || expr.Expression.References[0].Name != "c" {
		t.Fatalf("member 3 must carry expression identity + column deps, got %+v", expr)
	}
	if index.DescParts != 1 || index.PrefixParts != 1 || index.ExpressionCount != 1 {
		t.Fatalf("aggregates = %d/%d/%d, want 1/1/1", index.DescParts, index.PrefixParts, index.ExpressionCount)
	}
}

// Descending member order is positional: (a DESC,b) and (a,b DESC) differ.
func TestExtractorT10R1DescendingMemberOrder(t *testing.T) {
	t.Parallel()
	first := extractOne(t, spec.DialectMySQL, "CREATE TABLE t (a INT, b INT, KEY d (a DESC, b))").DDL.Indexes[0]
	second := extractOne(t, spec.DialectMySQL, "CREATE TABLE t (a INT, b INT, KEY d (a, b DESC))").DDL.Indexes[0]
	if !first.KeyParts[0].Descending || first.KeyParts[1].Descending {
		t.Fatalf("(a DESC,b) must mark member 1 only, got %+v", first.KeyParts)
	}
	if second.KeyParts[0].Descending || !second.KeyParts[1].Descending {
		t.Fatalf("(a,b DESC) must mark member 2 only, got %+v", second.KeyParts)
	}
}

// PRIMARY KEY … CLUSTERED|NONCLUSTERED parses to PrimaryKeyType; the
// primary role then decides TiDB PKIsHandle on declared shapes.
func TestExtractorT10R1PrimaryKeyType(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		sql  string
		want string
	}{
		{"CREATE TABLE t (a INT, PRIMARY KEY (a) NONCLUSTERED)", "nonclustered"},
		{"CREATE TABLE t (a INT, PRIMARY KEY (a) CLUSTERED)", "clustered"},
		{"CREATE TABLE t (a INT, PRIMARY KEY (a))", ""},
		{"CREATE TABLE t (a INT PRIMARY KEY NONCLUSTERED)", "nonclustered"},
	} {
		statement := extractOne(t, spec.DialectTiDB, tc.sql)
		if statement.DDL.PrimaryKey == nil || statement.DDL.PrimaryKey.PrimaryKeyType != tc.want {
			t.Fatalf("%s → PrimaryKeyType = %q, want %q", tc.sql, statement.DDL.PrimaryKey.PrimaryKeyType, tc.want)
		}
	}
}
