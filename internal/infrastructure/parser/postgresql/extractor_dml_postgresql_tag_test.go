//go:build postgresql

// Package postgresql verifies PostgreSQL DML predicate normalization.
// input: PostgreSQL DML SQL parsed into PostgreSQL AST nodes
// output: normalized predicate shapes and primary-key lookup facts
// pos: PostgreSQL parser adapter contract tests for shared DML impact estimation
// note: if this file changes, update this header and module README.md.
package postgresql

import (
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestExtractDMLPredicateShapes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		sql         string
		wantShape   spec.PredicateShape
		wantLookup  []string
		wantKeyName string
		wantKeyKind spec.IndexKind
	}{
		{
			name:        "delete literal equality",
			sql:         "delete from users where id = 42",
			wantShape:   spec.PredicateShapeUniqueEquality,
			wantLookup:  []string{"id"},
			wantKeyName: "PRIMARY",
			wantKeyKind: spec.IndexKindPrimary,
		},
		{
			name:        "delete parameter equality",
			sql:         "delete from users where id = $1",
			wantShape:   spec.PredicateShapeUniqueEquality,
			wantLookup:  []string{"id"},
			wantKeyName: "PRIMARY",
			wantKeyKind: spec.IndexKindPrimary,
		},
		{
			name:        "update reversed parameter equality",
			sql:         "update users set name = 'x' where $1 = users.id",
			wantShape:   spec.PredicateShapeUniqueEquality,
			wantLookup:  []string{"id"},
			wantKeyName: "PRIMARY",
			wantKeyKind: spec.IndexKindPrimary,
		},
		{
			name:      "non equality",
			sql:       "delete from users where id > 42",
			wantShape: spec.PredicateShapeUnknown,
		},
		{
			name:      "or predicate",
			sql:       "delete from users where id = 42 or id = 43",
			wantShape: spec.PredicateShapeUnknown,
		},
		{
			name:      "range predicate",
			sql:       "delete from users where id between 1 and 2",
			wantShape: spec.PredicateShapeUnknown,
		},
		{
			name:      "missing where",
			sql:       "delete from users",
			wantShape: spec.PredicateShapeMissingWhere,
		},
		{
			name:      "unrecognized column",
			sql:       "delete from users where account_id = 42",
			wantShape: spec.PredicateShapeUnknown,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			statement := extractPostgreSQLStatement(t, tt.sql)
			if statement.DML == nil {
				t.Fatalf("expected DML payload, got %#v", statement)
			}
			if statement.DML.PredicateShape != tt.wantShape {
				t.Fatalf("predicate shape = %q, want %q", statement.DML.PredicateShape, tt.wantShape)
			}
			if !equalStrings(statement.DML.LookupColumns, tt.wantLookup) {
				t.Fatalf("lookup columns = %#v, want %#v", statement.DML.LookupColumns, tt.wantLookup)
			}
			if statement.DML.MatchedKeyName != tt.wantKeyName {
				t.Fatalf("matched key name = %q, want %q", statement.DML.MatchedKeyName, tt.wantKeyName)
			}
			wantKeyKind := tt.wantKeyKind
			if wantKeyKind == "" {
				wantKeyKind = spec.IndexKindUnknown
			}
			if statement.DML.MatchedKeyKind != wantKeyKind {
				t.Fatalf("matched key kind = %q, want %q", statement.DML.MatchedKeyKind, wantKeyKind)
			}
		})
	}
}

func equalStrings(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}
