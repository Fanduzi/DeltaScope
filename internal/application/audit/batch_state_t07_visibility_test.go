// Package audit verifies the T07 column-visibility family on ordered state.
package audit

import (
	"context"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func t07VisibilityColumn(t *testing.T, snapshot *spec.TableSnapshot, name string) *spec.Column {
	t.Helper()
	if snapshot == nil {
		t.Fatalf("snapshot is nil — a known shape must publish")
	}
	for i := range snapshot.Columns {
		if snapshot.Columns[i].Name == name {
			return &snapshot.Columns[i]
		}
	}
	t.Fatalf("column %q missing from snapshot %+v", name, snapshot.Columns)
	return nil
}

// TestT07VisibilityOrderedState pins the setter walk: an explicit INVISIBLE
// declaration, SET VISIBLE, and SET INVISIBLE must each publish a fresh
// ordered-state fact without touching type/default siblings.
func TestT07VisibilityOrderedState(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	statements := t07Columns(t, strings.Join([]string{
		"CREATE TABLE t (id INT PRIMARY KEY, hidden_c INT INVISIBLE, visible_c INT)",
		"ALTER TABLE t ALTER COLUMN hidden_c SET VISIBLE",
		"ALTER TABLE t ALTER COLUMN hidden_c SET INVISIBLE",
	}, ";\n"), spec.DialectMySQL)
	version := &spec.VersionIdentity{Product: spec.VersionProductMySQL, Version: "8.0.46", Major: 8, Minor: 0, Patch: 46, ValidatedRange: true}
	state := newBatchState(spec.DialectMySQL, "golden", nil)
	target := spec.Table{Schema: "golden", Name: "t"}
	want := []string{"invisible", "visible", "invisible"}
	for i := range statements {
		// The real seam attaches the ordered pre-state as
		// Metadata.TargetTable before apply; mirror that here.
		snapshot, err := state.preState(ctx, "golden", target)
		if err != nil {
			t.Fatalf("pre-state read %d: %v", i, err)
		}
		statements[i].Metadata = &spec.Metadata{Version: version, TargetTable: snapshot}
		if err := state.apply(ctx, statements[i]); err != nil {
			t.Fatalf("apply statement %d: %v", i, err)
		}
		pre, err := state.preState(ctx, "golden", target)
		if err != nil {
			t.Fatalf("pre-state %d: %v", i, err)
		}
		hidden := t07VisibilityColumn(t, pre, "hidden_c")
		if hidden.Visibility != want[i] {
			t.Fatalf("statement %d hidden_c visibility = %q, want %q", i, hidden.Visibility, want[i])
		}
		if hidden.Type != "int(11)" {
			t.Fatalf("statement %d hidden_c type = %q, want int(11) preserved", i, hidden.Type)
		}
		visible := t07VisibilityColumn(t, pre, "visible_c")
		if visible.Visibility != "visible" || visible.VisibilityExplicit {
			t.Fatalf("statement %d visible_c = %+v, want effective visible with explicit=false", i, visible)
		}
	}
}

// TestT07VisibilitySetterPreservesSiblings keeps C07: a visibility setter
// leaves a stored expression default intact, and a default setter leaves the
// visibility fact intact.
func TestT07VisibilitySetterPreservesSiblings(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	statements := t07Columns(t, strings.Join([]string{
		"CREATE TABLE t (a INT, b INT INVISIBLE)",
		"ALTER TABLE t ALTER COLUMN b SET DEFAULT (a + 1)",
		"ALTER TABLE t ALTER COLUMN b SET VISIBLE",
	}, ";\n"), spec.DialectMySQL)
	version := &spec.VersionIdentity{Product: spec.VersionProductMySQL, Version: "8.0.46", Major: 8, Minor: 0, Patch: 46, ValidatedRange: true}
	state := newBatchState(spec.DialectMySQL, "golden", nil)
	target := spec.Table{Schema: "golden", Name: "t"}
	for i := range statements {
		snapshot, err := state.preState(ctx, "golden", target)
		if err != nil {
			t.Fatalf("pre-state read %d: %v", i, err)
		}
		statements[i].Metadata = &spec.Metadata{Version: version, TargetTable: snapshot}
		if err := state.apply(ctx, statements[i]); err != nil {
			t.Fatalf("apply statement %d: %v", i, err)
		}
		pre, err := state.preState(ctx, "golden", target)
		if err != nil {
			t.Fatalf("pre-state %d: %v", i, err)
		}
		b := t07VisibilityColumn(t, pre, "b")
		switch i {
		case 1:
			// SET DEFAULT on an invisible column keeps the visibility fact.
			if b.Visibility != "invisible" {
				t.Fatalf("after SET DEFAULT visibility = %q, want invisible", b.Visibility)
			}
			if b.DefaultExpression == nil || !b.DefaultExpression.Known ||
				len(b.DefaultExpression.References) != 1 || b.DefaultExpression.References[0].Name != "a" {
				t.Fatalf("after SET DEFAULT expression facts = %+v", b.DefaultExpression)
			}
		case 2:
			// SET VISIBLE flips visibility but keeps the expression default.
			if b.Visibility != "visible" {
				t.Fatalf("after SET VISIBLE visibility = %q, want visible", b.Visibility)
			}
			if b.DefaultExpression == nil || !b.DefaultExpression.Known {
				t.Fatalf("after SET VISIBLE lost expression facts: %+v", b.DefaultExpression)
			}
		}
	}
}

// TestT07VisibilityModifyResets keeps the native omission rule: a full
// MODIFY/CHANGE without a visibility clause republishes the column with
// effective visible semantics — the prior INVISIBLE does not inherit.
func TestT07VisibilityModifyResets(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	statements := t07Columns(t, strings.Join([]string{
		"CREATE TABLE t (id INT PRIMARY KEY, hidden_c INT INVISIBLE, visible_c INT)",
		"ALTER TABLE t MODIFY COLUMN hidden_c BIGINT",
	}, ";\n"), spec.DialectMySQL)
	version := &spec.VersionIdentity{Product: spec.VersionProductMySQL, Version: "8.0.46", Major: 8, Minor: 0, Patch: 46, ValidatedRange: true}
	state := newBatchState(spec.DialectMySQL, "golden", nil)
	target := spec.Table{Schema: "golden", Name: "t"}
	for i := range statements {
		snapshot, err := state.preState(ctx, "golden", target)
		if err != nil {
			t.Fatalf("pre-state read %d: %v", i, err)
		}
		statements[i].Metadata = &spec.Metadata{Version: version, TargetTable: snapshot}
		if err := state.apply(ctx, statements[i]); err != nil {
			t.Fatalf("apply statement %d: %v", i, err)
		}
	}
	pre, err := state.preState(ctx, "golden", target)
	if err != nil {
		t.Fatalf("pre-state: %v", err)
	}
	hidden := t07VisibilityColumn(t, pre, "hidden_c")
	if hidden.Visibility != "visible" || hidden.VisibilityExplicit {
		t.Fatalf("modify-omitted hidden_c = %+v, want reset to effective visible", hidden)
	}
	if hidden.Type != "bigint(20)" {
		t.Fatalf("modify-omitted hidden_c type = %q, want bigint(20)", hidden.Type)
	}
}
