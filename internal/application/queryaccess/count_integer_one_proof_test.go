//go:build postgresql

// Package queryaccess tests the narrow PostgreSQL COUNT(integer_one) proof gate.
// input: internal statement-shape facts, domain result facts, requirements, and candidates
// output: exact proof-gate decisions with fail-closed relation and requirement checks
// pos: application proof-gate boundary tests; no public output or SQL execution
// note: if this file changes, update this header and module README.md.
package queryaccess

import (
	"testing"

	domain "github.com/Fanduzi/DeltaScope/internal/domain/queryaccess"
)

func TestCountIntegerOneRequirementsComplete_ExactSingleTable(t *testing.T) {
	t.Parallel()

	result := domain.Result{
		Dialect:            "postgresql",
		Mode:               domain.ModeStrict,
		Relations:          []domain.RelationReference{{Schema: "app", Name: "orders", Kind: domain.RelationTable, PermissionRequired: true}},
		ReadClassification: domain.Indeterminate,
	}
	requirements := []domain.Requirement{{Object: "app.orders", Privilege: "read_table"}}

	if !countIntegerOneRequirementsComplete(result, requirements, []EffectCandidate{countIntegerOneCandidate()}, true) {
		t.Fatal("expected exact COUNT(integer_one) single-table proof gate to pass")
	}
}

func TestCountIntegerOneRequirementsComplete_RejectsNarrowBoundary(t *testing.T) {
	t.Parallel()

	baseResult := domain.Result{
		Dialect:            "postgresql",
		Mode:               domain.ModeStrict,
		Relations:          []domain.RelationReference{{Schema: "app", Name: "orders", Kind: domain.RelationTable, PermissionRequired: true}},
		ReadClassification: domain.Indeterminate,
	}
	baseRequirements := []domain.Requirement{{Object: "app.orders", Privilege: "read_table"}}

	cases := []struct {
		name   string
		mutate func(*domain.Result, *[]domain.Requirement, *[]EffectCandidate, *bool)
	}{
		{name: "relationless", mutate: func(result *domain.Result, _ *[]domain.Requirement, _ *[]EffectCandidate, _ *bool) {
			result.Relations = nil
		}},
		{name: "join", mutate: func(result *domain.Result, _ *[]domain.Requirement, _ *[]EffectCandidate, _ *bool) {
			result.Relations = append(result.Relations, domain.RelationReference{Schema: "app", Name: "users", Kind: domain.RelationTable, PermissionRequired: true})
		}},
		{name: "unqualified", mutate: func(result *domain.Result, _ *[]domain.Requirement, _ *[]EffectCandidate, _ *bool) {
			result.Relations[0].Schema = ""
		}},
		{name: "view", mutate: func(result *domain.Result, _ *[]domain.Requirement, _ *[]EffectCandidate, _ *bool) {
			result.Relations[0].Kind = domain.RelationView
		}},
		{name: "referenced_column", mutate: func(result *domain.Result, _ *[]domain.Requirement, _ *[]EffectCandidate, _ *bool) {
			result.ReferencedColumns = []domain.ColumnReference{{Schema: "app", Table: "orders", Column: "id"}}
		}},
		{name: "unresolved", mutate: func(result *domain.Result, _ *[]domain.Requirement, _ *[]EffectCandidate, _ *bool) {
			result.Unresolved = []domain.Unresolved{{Reference: "x", Reason: "unresolved"}}
		}},
		{name: "extra_requirement", mutate: func(_ *domain.Result, requirements *[]domain.Requirement, _ *[]EffectCandidate, _ *bool) {
			*requirements = append(*requirements, domain.Requirement{Object: "app.orders.id", Privilege: "read_column"})
		}},
		{name: "wrong_requirement", mutate: func(_ *domain.Result, requirements *[]domain.Requirement, _ *[]EffectCandidate, _ *bool) {
			(*requirements)[0].Privilege = "read_column"
		}},
		{name: "malformed_candidate_vector", mutate: func(_ *domain.Result, _ *[]domain.Requirement, candidates *[]EffectCandidate, _ *bool) {
			*candidates = append(*candidates, countIntegerOneCandidate())
		}},
		{name: "statement_shape", mutate: func(_ *domain.Result, _ *[]domain.Requirement, _ *[]EffectCandidate, exact *bool) { *exact = false }},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			result := baseResult
			result.Relations = append([]domain.RelationReference(nil), baseResult.Relations...)
			requirements := append([]domain.Requirement(nil), baseRequirements...)
			candidates := []EffectCandidate{countIntegerOneCandidate()}
			exact := true
			tc.mutate(&result, &requirements, &candidates, &exact)
			if countIntegerOneRequirementsComplete(result, requirements, candidates, exact) {
				t.Fatal("expected COUNT(integer_one) proof gate to fail closed")
			}
		})
	}
}

func countIntegerOneCandidate() EffectCandidate {
	return EffectCandidate{
		Kind:                 EffectCandidateFunction,
		Ordinal:              0,
		NamePath:             []string{"count"},
		OriginalNamePath:     []string{"COUNT"},
		Canonical:            true,
		ParserClassification: "generic",
		Arity:                1,
		OperandKinds:         []string{"integer_one"},
	}
}
