package ddl

import (
	"context"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestPGPrimaryKeyCountUsesExactQuotedName(t *testing.T) {
	s := pgGenerationTransitionStatement("add_constraint", nil)
	a := &s.DDL.Alter[0]
	a.Options = map[string]string{"constraint_type": "primary_key", "columns": " A,B "}
	a.PGKeyColumns = []string{" A,B "}
	r := primaryKeyColumnCountRule{limit: 1, level: rule.LevelWarning}
	if findings, err := r.Evaluate(context.Background(), s); err != nil || len(findings) != 0 {
		t.Fatalf("single quoted key member produced findings %v (%v)", findings, err)
	}
}

func TestPGConstraintColumns(t *testing.T) {
	for _, tc := range []struct {
		name, reason, gap, column string
		keys, includes            []string
		stale, unobservedSystem   bool
	}{
		{name: "exact quoted members", keys: []string{" A,B ", " a,b "}},
		{name: "duplicates and overlap in INCLUDE", keys: []string{"id"}, includes: []string{"id", "id"}},
		{name: "exact absence", keys: []string{" A,b "}, reason: "constraint_column_absent", column: " A,b "},
		{name: "duplicate key precedes absence", keys: []string{"missing", "missing"}, reason: "constraint_key_column_duplicate", column: "missing"},
		{name: "missing include", keys: []string{"id"}, includes: []string{"missing"}, reason: "constraint_column_absent", column: "missing"},
		{name: "system include", keys: []string{"id"}, includes: []string{"tableoid"}, reason: "constraint_system_column_inapplicable", column: "tableoid"},
		{name: "unobserved system directory", keys: []string{"tableoid"}, unobservedSystem: true, gap: "target_table.pg_columns"},
		{name: "unknown key member", keys: []string{""}, gap: "constraint.key_columns"},
		{name: "stale snapshot", keys: []string{"missing"}, stale: true, gap: "target_table.pg_columns"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := pgGenerationTransitionStatement("add_constraint", nil)
			a := &s.DDL.Alter[0]
			a.Options = map[string]string{"constraint_type": "unique", "using_index": "false"}
			a.PGKeyColumns, a.PGIncludeColumns = tc.keys, tc.includes
			catalog := s.Metadata.TargetTable.PGColumns
			catalog.Columns = []spec.PGColumnState{{Name: "id"}, {Name: " A,B "}, {Name: " a,b "}}
			if !tc.unobservedSystem {
				catalog.SystemColumns = []spec.PGColumnState{{Name: "tableoid", System: true}}
			}
			if tc.stale {
				s.Metadata.TargetTable.PGRelation = spec.PGRelationState{}
			}
			if reason, gap, column := assessPGConstraintColumns(s); reason != tc.reason || gap != tc.gap || column != tc.column {
				t.Fatalf("got %q/%q/%q, want %q/%q/%q", reason, gap, column, tc.reason, tc.gap, tc.column)
			}
			r := pgConstraintColumnsRule{}
			if !r.AppliesTo(s) {
				t.Fatal("rule skipped direct constraint")
			}
			a.Options["using_index"] = "true"
			if r.AppliesTo(s) {
				t.Fatal("rule matched an existing index attachment")
			}
		})
	}
}
