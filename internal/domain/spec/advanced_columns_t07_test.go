// Package spec verifies T07 advanced-column assessment boundaries.
// input: hand-built column fact sets (dependency graphs, dialects, versions)
// output: non-activation for PostgreSQL, known-vs-unknown fact discipline, and
// bounded dependency-walk answers
// pos: domain-level T07 contract pins beside the audit-layer matrix
// note: if this file changes, update this header and module README.md.
package spec

import (
	"fmt"
	"testing"
)

// TestT07PostgreSQLDoesNotActivate keeps C10: the shared assessment — and
// therefore both T07 rules — is inert for PostgreSQL statements even when the
// statement carries advanced-column-shaped facts.
func TestT07PostgreSQLDoesNotActivate(t *testing.T) {
	t.Parallel()
	statement := Statement{
		Dialect: DialectPostgreSQL,
		Kind:    KindDDL,
		DDL: &DDL{
			Operation: DDLOperationCreateTable,
			Table:     &Table{Schema: "golden", Name: "t"},
			Columns: []Column{{
				Name: "b",
				Generated: &GeneratedColumn{
					Storage: GeneratedStorageStored,
					Expression: ExpressionFacts{
						Known:      true,
						References: []ColumnReference{{Name: "a"}},
					},
				},
				Visibility:         "invisible",
				VisibilityExplicit: true,
			}},
		},
	}
	assessment := AssessAdvancedColumns(statement)
	if assessment.Applies || len(assessment.Issues) != 0 || len(assessment.Gaps) != 0 ||
		len(assessment.DependencyEffects) != 0 || len(assessment.DependencyGaps) != 0 {
		t.Fatalf("postgresql assessment = %+v, want fully inert", assessment)
	}
}

// TestT07ColumnDependentsUnknownVsKnownEmpty keeps C08: an unprovable
// expression marks the dependent answer unknown while a column set without
// references is a proven-empty dependent list.
func TestT07ColumnDependentsUnknownVsKnownEmpty(t *testing.T) {
	t.Parallel()
	table := Table{Schema: "golden", Name: "t"}
	columns := []Column{
		{Name: "a"},
		{Name: "b", Generated: &GeneratedColumn{Storage: GeneratedStorageVirtual, Expression: ExpressionFacts{Known: true}}},
	}
	known := ColumnDependents(table, columns, "a")
	if !known.Known || len(known.Dependents) != 0 {
		t.Fatalf("known-empty dependents = %+v, want Known with none", known)
	}
	unknown := append([]Column(nil), columns...)
	unknown[1].Generated.Expression.Known = false
	got := ColumnDependents(table, unknown, "a")
	if got.Known {
		t.Fatalf("unknown expression facts produced a complete answer: %+v", got)
	}
}

// TestT07ColumnDependentsTransitiveOrder keeps the frozen ordering: the walk
// reports direct and transitive dependents in declaration order.
func TestT07ColumnDependentsTransitiveOrder(t *testing.T) {
	t.Parallel()
	table := Table{Schema: "golden", Name: "t"}
	columns := []Column{
		{Name: "a"},
		{Name: "b", DefaultExpression: &ExpressionFacts{Known: true, References: []ColumnReference{{Name: "a"}}}},
		{Name: "c", Generated: &GeneratedColumn{Storage: GeneratedStorageStored, Expression: ExpressionFacts{Known: true, References: []ColumnReference{{Name: "b"}}}}},
	}
	got := ColumnDependents(table, columns, "a")
	if !got.Known || len(got.Dependents) != 2 {
		t.Fatalf("dependents = %+v, want known b and c", got)
	}
	if got.Dependents[0].Column != "b" || got.Dependents[0].Dependency != AdvancedDependencyDirect ||
		got.Dependents[0].Kind != AdvancedDependencyExpressionDefault {
		t.Fatalf("first dependent = %+v, want direct expression-default b", got.Dependents[0])
	}
	if got.Dependents[1].Column != "c" || got.Dependents[1].Dependency != AdvancedDependencyTransitive ||
		got.Dependents[1].Kind != AdvancedDependencyGenerated {
		t.Fatalf("second dependent = %+v, want transitive generated c", got.Dependents[1])
	}
}

// TestT07ColumnDependentsCycleIsUnknown keeps the cycle boundary: a
// self-referencing component reachable from the dropped name is unknown, not
// a complete-but-truncated set.
func TestT07ColumnDependentsCycleIsUnknown(t *testing.T) {
	t.Parallel()
	table := Table{Schema: "golden", Name: "t"}
	columns := []Column{
		{Name: "a"},
		{Name: "b", Generated: &GeneratedColumn{Storage: GeneratedStorageVirtual, Expression: ExpressionFacts{Known: true, References: []ColumnReference{{Name: "a"}, {Name: "c"}}}}},
		{Name: "c", Generated: &GeneratedColumn{Storage: GeneratedStorageVirtual, Expression: ExpressionFacts{Known: true, References: []ColumnReference{{Name: "b"}}}}},
	}
	got := ColumnDependents(table, columns, "a")
	if got.Known {
		t.Fatalf("cycle produced a complete answer: %+v", got)
	}
}

// TestT07ColumnDependentsBounded keeps the work bounds: a deeper-than-limit
// chain and an over-wide column set both return unknown instead of a
// truncated "complete" dependent set.
func TestT07ColumnDependentsBounded(t *testing.T) {
	t.Parallel()
	table := Table{Schema: "golden", Name: "t"}
	chain := make([]Column, 0, advancedMaxPathEdges+2)
	chain = append(chain, Column{Name: "a"})
	for i := 1; i < advancedMaxPathEdges+2; i++ {
		chain = append(chain, Column{
			Name: fmt.Sprintf("c%d", i),
			Generated: &GeneratedColumn{Storage: GeneratedStorageVirtual, Expression: ExpressionFacts{
				Known:      true,
				References: []ColumnReference{{Name: chain[i-1].Name}},
			}},
		})
	}
	if got := ColumnDependents(table, chain, "a"); got.Known {
		t.Fatalf("over-depth chain returned a complete answer: %d dependents", len(got.Dependents))
	}
	wide := make([]Column, 0, advancedMaxColumns+1)
	for i := 0; i < advancedMaxColumns+1; i++ {
		wide = append(wide, Column{Name: fmt.Sprintf("c%d", i)})
	}
	if got := ColumnDependents(table, wide, "a"); got.Known {
		t.Fatalf("over-width column set returned a complete answer: %d dependents", len(got.Dependents))
	}
	// Independent known findings survive alongside the unknown bound: a
	// direct dependent before the depth limit is still reported.
	got := ColumnDependents(table, chain, "a")
	if len(got.Dependents) == 0 {
		t.Fatalf("bounded walk reported no dependents at all: %+v", got)
	}
}

// TestT07ColumnDependentsQualifierBinding pins reference binding: a qualifier
// matching the owning table binds locally; any other qualifier is
// unresolvable and marks the answer unknown.
func TestT07ColumnDependentsQualifierBinding(t *testing.T) {
	t.Parallel()
	table := Table{Schema: "golden", Name: "t"}
	columns := []Column{
		{Name: "a"},
		{Name: "b", Generated: &GeneratedColumn{Storage: GeneratedStorageVirtual, Expression: ExpressionFacts{
			Known:      true,
			References: []ColumnReference{{Table: "t", Name: "a"}},
		}}},
	}
	got := ColumnDependents(table, columns, "a")
	if !got.Known || len(got.Dependents) != 1 || got.Dependents[0].Column != "b" {
		t.Fatalf("self-qualified reference did not bind: %+v", got)
	}
	columns[1].Generated.Expression.References = []ColumnReference{{Table: "other", Name: "a"}}
	if got := ColumnDependents(table, columns, "a"); got.Known {
		t.Fatalf("external qualifier produced a complete answer: %+v", got)
	}
}
