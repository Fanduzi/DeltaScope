// Package tidb verifies the T08 normalized column-position facts.
// input: positioned FIRST|AFTER clauses on add_columns, modify_column, and
// change_column actions plus the parenthesized multi-column add anomaly,
// through the real parse+extract seam
// output: Alter.Position carrying the normalized {kind, after} pair,
// HasColumnPosition staying consistent with Position, identifier
// normalization to lowercase, and spec-level positions on multi-column
// ADD (...) surfacing as the column_position unextracted option instead of
// per-column position facts
// pos: extractor contract tests for the T08 position model (issue #87)
// note: if this file changes, update this header and module README.md.
package tidbparser

import (
	"context"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestExtractorT08ColumnPosition(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name         string
		sql          string
		action       string
		wantKind     string
		wantAfter    string
		wantPosition bool
	}{
		{name: "add first", sql: "ALTER TABLE t ADD COLUMN c INT FIRST", action: "add_columns", wantKind: "first", wantPosition: true},
		{name: "add after", sql: "ALTER TABLE t ADD COLUMN c INT AFTER Base_Col", action: "add_columns", wantKind: "after", wantAfter: "base_col", wantPosition: true},
		{name: "add unpositioned", sql: "ALTER TABLE t ADD COLUMN c INT", action: "add_columns"},
		{name: "modify first", sql: "ALTER TABLE t MODIFY COLUMN c INT FIRST", action: "modify_column", wantKind: "first", wantPosition: true},
		{name: "modify after", sql: "ALTER TABLE t MODIFY COLUMN c INT AFTER Base_Col", action: "modify_column", wantKind: "after", wantAfter: "base_col", wantPosition: true},
		{name: "change first", sql: "ALTER TABLE t CHANGE COLUMN c c2 INT FIRST", action: "change_column", wantKind: "first", wantPosition: true},
		{name: "change after", sql: "ALTER TABLE t CHANGE COLUMN c c2 INT AFTER Base_Col", action: "change_column", wantKind: "after", wantAfter: "base_col", wantPosition: true},
		{name: "setter no position", sql: "ALTER TABLE t ALTER COLUMN c SET DEFAULT 1", action: "set_column_default"},
	}
	for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
		for _, tc := range cases {
			tc := tc
			t.Run(string(dialect)+"/"+tc.name, func(t *testing.T) {
				t.Parallel()
				parser := New()
				result, err := parser.Parse(context.Background(), tc.sql)
				if err != nil {
					t.Fatalf("parse: %v", err)
				}
				wrapped := WrapStatements(result.Statements, result.Warnings)
				if len(wrapped) != 1 {
					t.Fatalf("expected one statement, got %d", len(wrapped))
				}
				statement, err := wrapped[0].Extractor.Extract(dialect, wrapped[0].RawSQL)
				if err != nil {
					t.Fatalf("extract: %v", err)
				}
				if len(statement.DDL.Alter) != 1 {
					t.Fatalf("expected one alter action, got %#v", statement.DDL.Alter)
				}
				alter := statement.DDL.Alter[0]
				if alter.Action != tc.action {
					t.Fatalf("action = %q, want %q", alter.Action, tc.action)
				}
				if alter.HasColumnPosition != tc.wantPosition {
					t.Fatalf("HasColumnPosition = %t, want %t", alter.HasColumnPosition, tc.wantPosition)
				}
				if !tc.wantPosition {
					if alter.Position != nil {
						t.Fatalf("unpositioned action carried Position %#v", alter.Position)
					}
					return
				}
				if alter.Position == nil || alter.Position.Kind != tc.wantKind || alter.Position.After != tc.wantAfter {
					t.Fatalf("Position = %#v, want kind=%q after=%q", alter.Position, tc.wantKind, tc.wantAfter)
				}
			})
		}
	}
}

// TestExtractorT08MultiColumnAddPosition pins the spec-level anomaly: a
// position clause on `ADD (a ..., b ...)` cannot be attributed to a single
// column, so every emitted action stays position-free and the first one
// carries the column_position unextracted option for coverage to flag.
func TestExtractorT08MultiColumnAddPosition(t *testing.T) {
	t.Parallel()
	parser := New()
	result, err := parser.Parse(context.Background(), "ALTER TABLE t ADD (a INT, b INT) FIRST")
	if err != nil {
		t.Skipf("parser rejects spec-level position on multi-column add: %v", err)
	}
	wrapped := WrapStatements(result.Statements, result.Warnings)
	statement, err := wrapped[0].Extractor.Extract(spec.DialectMySQL, wrapped[0].RawSQL)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if len(statement.DDL.Alter) != 2 {
		t.Fatalf("expected two add_columns actions, got %#v", statement.DDL.Alter)
	}
	flagged := 0
	for _, alter := range statement.DDL.Alter {
		if alter.HasColumnPosition || alter.Position != nil {
			t.Fatalf("spec-level position leaked onto column %q: %#v", alter.Name, alter.Position)
		}
		for _, option := range alter.UnextractedOptions {
			if option == "column_position" {
				flagged++
			}
		}
	}
	if flagged != 1 {
		t.Fatalf("expected exactly one column_position unextracted marker, got %#v", statement.DDL.Alter)
	}
}
