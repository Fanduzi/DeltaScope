// Package audit verifies the T06-A17 declaration-source carry contract:
// columns parsed from CREATE TABLE keep their faithful Type (DECIMAL scale,
// explicit-zero FLOAT/DOUBLE scale, ENUM/SET member case) inside the derived
// pre-state a later batch statement consumes — cloneColumn drops nothing the
// extractor now carries.
// input: real parser/extractor statements applied into batchState
// output: the complete derived pre-state column set — identity, order, and
// presence pinned alongside faithful Type facts, plus proof a returned
// projection is detached from both parser-owned and state-owned columns
// pos: application-layer ordered-state regression tests for issue #85 T06-A17
// note: if this file changes, update this header and module README.md.
package audit

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// t06a17CheckDerivedColumns requires the complete frozen column set in
// declaration order: exactly the four declared names, no duplicates, each
// carrying its faithful Type.
func t06a17CheckDerivedColumns(snapshot *spec.TableSnapshot) error {
	if snapshot == nil || !snapshot.Exists {
		return fmt.Errorf("snapshot missing or absent: %+v", snapshot)
	}
	expected := []struct{ name, typeText string }{
		{"d", "decimal(12,2)"},
		{"f", "float(10,0)"},
		{"e", "enum('Ready','Closed')"},
		{"s", "set('Write','Read')"},
	}
	if len(snapshot.Columns) != len(expected) {
		return fmt.Errorf("columns = %d (%+v), want %d", len(snapshot.Columns), snapshot.Columns, len(expected))
	}
	seen := map[string]struct{}{}
	for i, want := range expected {
		got := snapshot.Columns[i]
		if _, dup := seen[got.Name]; dup {
			return fmt.Errorf("duplicate column %q", got.Name)
		}
		seen[got.Name] = struct{}{}
		if got.Name != want.name {
			return fmt.Errorf("column[%d] name = %q, want %q", i, got.Name, want.name)
		}
		if got.Type != want.typeText {
			return fmt.Errorf("column %q Type = %q, want %q", got.Name, got.Type, want.typeText)
		}
	}
	return nil
}

func TestT06A17DerivedPreStatePreservesType(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
		dialect := dialect
		t.Run(string(dialect), func(t *testing.T) {
			t.Parallel()
			sql := "CREATE TABLE t (\n" +
				"  d DECIMAL(12,2),\n" +
				"  f FLOAT(10,0),\n" +
				"  e ENUM('Ready','Closed'),\n" +
				"  s SET('Write','Read')\n" +
				");\n" +
				"ALTER TABLE t ADD COLUMN next_c INT;"
			parsed, err := Parse(ctx, sql, dialect)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			statements, err := Extract(ctx, parsed)
			if err != nil {
				t.Fatalf("extract: %v", err)
			}
			if len(statements) != 2 {
				t.Fatalf("statements = %d, want 2", len(statements))
			}
			if statements[0].DDL == nil || statements[0].DDL.Table == nil {
				t.Fatalf("CREATE must carry DDL and DDL.Table, got %+v", statements[0].DDL)
			}
			if statements[1].DDL == nil || statements[1].DDL.Table == nil ||
				statements[1].DDL.Operation != spec.DDLOperationAlterTable ||
				statements[1].DDL.Table.Name != "t" {
				t.Fatalf("follower must be ALTER TABLE t, got %+v", statements[1].DDL)
			}
			createColumns := append([]spec.Column(nil), statements[0].DDL.Columns...)
			state := newBatchState(dialect, "golden", nil)
			if _, err := state.preState(ctx, "golden", *statements[0].DDL.Table); err != nil {
				t.Fatalf("warm pre-state: %v", err)
			}
			if err := state.apply(ctx, statements[0]); err != nil {
				t.Fatalf("apply create: %v", err)
			}
			snapshot, err := state.preState(ctx, "golden", *statements[1].DDL.Table)
			if err != nil {
				t.Fatalf("pre-state: %v", err)
			}
			if err := t06a17CheckDerivedColumns(snapshot); err != nil {
				t.Fatalf("%s derived pre-state: %v", dialect, err)
			}
			snapshot.Columns[0].Name = "mutated"
			snapshot.Columns[0].Type = "mutated(1)"
			snapshot.Columns[0].Length = 99
			snapshot.Columns[0].Unsigned = true
			if !reflect.DeepEqual(createColumns, statements[0].DDL.Columns) {
				t.Fatalf("%s mutating the returned projection changed parser-owned CREATE columns: %+v", dialect, statements[0].DDL.Columns)
			}
			again, err := state.preState(ctx, "golden", *statements[1].DDL.Table)
			if err != nil {
				t.Fatalf("pre-state re-read: %v", err)
			}
			if err := t06a17CheckDerivedColumns(again); err != nil {
				t.Fatalf("%s derived pre-state re-read: %v", dialect, err)
			}
			if !reflect.DeepEqual(again.Columns, createColumns) {
				t.Fatalf("%s state-owned columns drifted from the original CREATE columns: %+v vs %+v", dialect, again.Columns, createColumns)
			}
		})
	}
}
