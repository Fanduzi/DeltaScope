// Package spec verifies the T07-R1 premise edges the AuditSQL surface
// cannot reach: unresolvable candidate sets, same-statement name-space
// binds, qualifier binding, and reference-fact clone isolation.
package spec

import (
	"testing"
)

// r1Generated builds a generated column with known expression facts.
func r1Generated(name string, refs ...ColumnReference) Column {
	return r1GeneratedStorage(name, GeneratedStorageStored, refs...)
}

// r1GeneratedStorage builds a generated column with the given storage kind.
func r1GeneratedStorage(name, storage string, refs ...ColumnReference) Column {
	return Column{
		Name: name,
		Type: "int",
		Generated: &GeneratedColumn{
			Storage: storage,
			Expression: ExpressionFacts{
				Known:      true,
				References: refs,
			},
		},
	}
}

// r1Version returns a validated MySQL 8.0.46 version identity.
func r1Version() *VersionIdentity {
	return &VersionIdentity{Product: VersionProductMySQL, Version: "8.0.46", Major: 8, Minor: 0, Patch: 46, ValidatedRange: true}
}

// TestT07R1PremiseUnresolvedCandidateSet keeps the fail-closed answer for a
// declaration whose member names cannot be trusted (duplicates): every
// reference stays a bounded gap instead of a guessed membership verdict.
func TestT07R1PremiseUnresolvedCandidateSet(t *testing.T) {
	t.Parallel()
	statement := Statement{
		Dialect: DialectMySQL,
		Kind:    KindDDL,
		DDL: &DDL{
			Operation: DDLOperationCreateTable,
			Table:     &Table{Schema: "golden", Name: "t"},
			Columns: []Column{
				{Name: "a", Type: "int"},
				{Name: "a", Type: "int"},
				r1Generated("b", ColumnReference{Name: "a"}),
			},
		},
	}
	assessment := AssessAdvancedColumns(statement)
	if len(assessment.Issues) != 0 {
		t.Fatalf("issues = %+v, want none — the set is unprovable", assessment.Issues)
	}
	if !assessment.UnresolvedTransition {
		t.Fatalf("UnresolvedTransition = false, want true for an unresolvable set")
	}
	found := false
	for _, gap := range assessment.Gaps {
		if gap.ReasonCode == AdvancedReasonIncompleteSourceColumn {
			found = true
		}
	}
	if !found {
		t.Fatalf("gaps = %+v, want an incomplete_source_column gap", assessment.Gaps)
	}
}

// TestT07R1PremiseAlterSameStatementBind keeps multi-action net effects
// bounded: an ADD generated column referencing a column added in the same
// statement is a gap, never a proven present or absent answer.
func TestT07R1PremiseAlterSameStatementBind(t *testing.T) {
	t.Parallel()
	statement := Statement{
		Dialect: DialectMySQL,
		Kind:    KindDDL,
		Metadata: &Metadata{
			TargetTable: &TableSnapshot{
				Exists:  true,
				Schema:  "golden",
				Table:   &Table{Schema: "golden", Name: "t"},
				Columns: []Column{{Name: "x", Type: "int"}},
			},
		},
		DDL: &DDL{
			Operation: DDLOperationAlterTable,
			Table:     &Table{Schema: "golden", Name: "t"},
			Alter: []Alter{
				{Action: "add_columns", Column: &AlterColumn{Definition: &Column{Name: "a", Type: "int"}}},
				{Action: "add_columns", Column: &AlterColumn{Definition: ptrColumn(r1Generated("b", ColumnReference{Name: "a"}))}},
			},
		},
	}
	assessment := AssessAdvancedColumns(statement)
	if len(assessment.Issues) != 0 {
		t.Fatalf("issues = %+v, want none — the bind is a net effect", assessment.Issues)
	}
	found := false
	for _, gap := range assessment.Gaps {
		if gap.ReasonCode == AdvancedReasonIncompleteAlterDepEffects {
			found = true
		}
	}
	if !found {
		t.Fatalf("gaps = %+v, want incomplete_alter_dependency_effects", assessment.Gaps)
	}
}

// TestT07R1PremiseAlterUnknownPrior keeps the gap for an ALTER generated
// declaration when the prior column set is unproven: references can be
// neither confirmed nor denied.
func TestT07R1PremiseAlterUnknownPrior(t *testing.T) {
	t.Parallel()
	statement := Statement{
		Dialect: DialectTiDB,
		Kind:    KindDDL,
		DDL: &DDL{
			Operation: DDLOperationAlterTable,
			Table:     &Table{Schema: "golden", Name: "t"},
			Alter: []Alter{
				// VIRTUAL keeps the premise check inside the TiDB
				// supported-add boundary (STORED adds are a separate
				// determined negative).
				{Action: "add_columns", Column: &AlterColumn{Definition: ptrColumn(r1GeneratedStorage("b", GeneratedStorageVirtual, ColumnReference{Name: "a"}))}},
			},
		},
	}
	assessment := AssessAdvancedColumns(statement)
	if len(assessment.Issues) != 0 {
		t.Fatalf("issues = %+v, want none — the prior set is unproven", assessment.Issues)
	}
	if !assessment.UnresolvedTransition {
		t.Fatalf("UnresolvedTransition = false, want true for an unknown prior")
	}
}

// TestT07R1PremiseQualifiedReference pins the qualifier binding contract:
// a qualifier naming the owning table binds like a bare name (including its
// order/self verdicts); any other qualifier is a bounded gap.
func TestT07R1PremiseQualifiedReference(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name       string
		ref        ColumnReference
		wantReason string
		wantGap    bool
	}{
		{"own_table", ColumnReference{Table: "t", Name: "a"}, "", false},
		{"own_table_late_generated", ColumnReference{Table: "t", Name: "c"}, AdvancedReasonGeneratedColumnOrder, false},
		{"own_schema_table", ColumnReference{Schema: "golden", Table: "t", Name: "a"}, "", false},
		{"other_table", ColumnReference{Table: "other", Name: "a"}, "", true},
		{"other_schema", ColumnReference{Schema: "other", Table: "t", Name: "a"}, "", true},
		{"own_table_missing", ColumnReference{Table: "t", Name: "missing"}, AdvancedReasonGeneratedMissingColumn, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			statement := Statement{
				Dialect:  DialectMySQL,
				Kind:     KindDDL,
				Metadata: &Metadata{Version: r1Version()},
				DDL: &DDL{
					Operation: DDLOperationCreateTable,
					Table:     &Table{Schema: "golden", Name: "t"},
					Columns: []Column{
						{Name: "a", Type: "int"},
						r1Generated("b", tc.ref),
						r1Generated("c", ColumnReference{Name: "a"}),
					},
				},
			}
			assessment := AssessAdvancedColumns(statement)
			if tc.wantReason != "" {
				if len(assessment.Issues) != 1 || assessment.Issues[0].ReasonCode != tc.wantReason {
					t.Fatalf("issues = %+v, want one %s issue", assessment.Issues, tc.wantReason)
				}
				return
			}
			if len(assessment.Issues) != 0 {
				t.Fatalf("issues = %+v, want none", assessment.Issues)
			}
			if tc.wantGap && len(assessment.Gaps) == 0 {
				t.Fatalf("gaps = none, want a bounded gap for an unbindable qualifier")
			}
			if !tc.wantGap && len(assessment.Gaps) != 0 {
				t.Fatalf("gaps = %+v, want none for an owning-table qualifier", assessment.Gaps)
			}
		})
	}
}

// TestT07R1CloneExpressionFacts pins deep-copy isolation for the R1 fact
// fields: mutating a clone never reaches the source facts.
func TestT07R1CloneExpressionFacts(t *testing.T) {
	t.Parallel()
	src := &ExpressionFacts{
		Known:             true,
		TopForm:           ExpressionTopArithmetic,
		RootParenthesized: true,
		CallCount:         2,
		References:        []ColumnReference{{Name: "a"}},
		Functions:         []string{"uuid"},
		Features:          []string{"arithmetic"},
		Fingerprint:       "fp",
	}
	clone := CloneExpressionFacts(src)
	clone.TopForm = ExpressionTopCall
	clone.RootParenthesized = false
	clone.CallCount = 1
	clone.References[0].Name = "mutated"
	clone.Functions[0] = "mutated"
	clone.Features[0] = "mutated"
	if src.TopForm != ExpressionTopArithmetic || !src.RootParenthesized ||
		src.CallCount != 2 ||
		src.References[0].Name != "a" || src.Functions[0] != "uuid" ||
		src.Features[0] != "arithmetic" {
		t.Fatalf("mutating the clone changed the source: %+v", src)
	}
	if CloneExpressionFacts(nil) != nil {
		t.Fatalf("CloneExpressionFacts(nil) != nil")
	}
}

// ptrColumn returns a pointer to a copy of the column.
func ptrColumn(c Column) *Column {
	return &c
}
