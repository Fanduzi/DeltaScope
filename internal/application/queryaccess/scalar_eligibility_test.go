// Package queryaccess tests scalar function Phase-1 eligibility and gateway matching.
// input: scalar candidate shapes and manifest entries
// output: deterministic eligibility and matching decisions
// pos: application-layer scalar candidate proof boundary tests
package queryaccess

import (
	"testing"

	domain "github.com/Fanduzi/DeltaScope/internal/domain/queryaccess"
)

func TestPhase1Eligibility_ScalarDirectColumnsEligible(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		cand EffectCandidate
	}{
		{
			name: "two_column_scalar",
			cand: EffectCandidate{
				Kind:                 EffectCandidateFunction,
				Ordinal:              0,
				NamePath:             []string{"coalesce"},
				OriginalNamePath:     []string{"COALESCE"},
				Canonical:            true,
				ParserClassification: "generic",
				Arity:                2,
				OperandKinds:         []string{"column", "column"},
				OperandColumnRefs: []OperandColumnRef{
					{Schema: "app", Table: "users", Column: "a"},
					{Schema: "app", Table: "users", Column: "b"},
				},
			},
		},
		{
			name: "three_column_scalar",
			cand: EffectCandidate{
				Kind:                 EffectCandidateFunction,
				Ordinal:              0,
				NamePath:             []string{"coalesce"},
				OriginalNamePath:     []string{"COALESCE"},
				Canonical:            true,
				ParserClassification: "generic",
				Arity:                3,
				OperandKinds:         []string{"column", "column", "column"},
				OperandColumnRefs: []OperandColumnRef{
					{Schema: "app", Table: "users", Column: "a"},
					{Schema: "app", Table: "users", Column: "b"},
					{Schema: "app", Table: "users", Column: "c"},
				},
			},
		},
		{
			name: "single_column_scalar",
			cand: EffectCandidate{
				Kind:                 EffectCandidateFunction,
				Ordinal:              0,
				NamePath:             []string{"lower"},
				OriginalNamePath:     []string{"LOWER"},
				Canonical:            true,
				ParserClassification: "generic",
				Arity:                1,
				OperandKinds:         []string{"column"},
				OperandColumnRefs:    []OperandColumnRef{{Schema: "app", Table: "users", Column: "name"}},
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ok, reason := ValidatePhase1PureEffectCandidates([]EffectCandidate{tc.cand})
			if !ok {
				t.Errorf("expected eligible, got reason: %s", reason)
			}
		})
	}
}

func TestPhase1Eligibility_ScalarSingleLiteralOnlyNotEligible(t *testing.T) {
	t.Parallel()

	// LOWER('x') is arity-1, literal-only, and has no column dependency.
	cand := EffectCandidate{
		Kind:                 EffectCandidateFunction,
		Ordinal:              0,
		NamePath:             []string{"lower"},
		OriginalNamePath:     []string{"LOWER"},
		Canonical:            true,
		ParserClassification: "generic",
		Arity:                1,
		OperandKinds:         []string{"const"},
		OperandColumnRefs:    nil,
	}
	ok, reason := ValidatePhase1PureEffectCandidates([]EffectCandidate{cand})
	if ok {
		t.Error("expected NOT eligible for literal operand")
	}
	if reason != domain.ReasonUnprovenFunctionEffect {
		t.Errorf("reason: got %q, want %q", reason, domain.ReasonUnprovenFunctionEffect)
	}
}

func TestPhase1Eligibility_ScalarWithMixedConstEligible(t *testing.T) {
	t.Parallel()

	cand := EffectCandidate{
		Kind:                 EffectCandidateFunction,
		Ordinal:              0,
		NamePath:             []string{"coalesce"},
		OriginalNamePath:     []string{"COALESCE"},
		Canonical:            true,
		ParserClassification: "generic",
		Arity:                2,
		OperandKinds:         []string{"column", "const"},
		OperandColumnRefs:    []OperandColumnRef{{Schema: "app", Table: "users", Column: "name"}},
	}
	ok, reason := ValidatePhase1PureEffectCandidates([]EffectCandidate{cand})
	if !ok {
		t.Errorf("expected eligible, got reason: %s", reason)
	}
}

func TestPhase1Eligibility_ScalarLiteralOnlyNotEligible(t *testing.T) {
	t.Parallel()

	cand := EffectCandidate{
		Kind:                 EffectCandidateFunction,
		Ordinal:              0,
		NamePath:             []string{"coalesce"},
		OriginalNamePath:     []string{"COALESCE"},
		Canonical:            true,
		ParserClassification: "generic",
		Arity:                2,
		OperandKinds:         []string{"const", "const"},
	}
	ok, reason := ValidatePhase1PureEffectCandidates([]EffectCandidate{cand})
	if ok {
		t.Error("expected NOT eligible for literal-only function")
	}
	if reason != domain.ReasonUnprovenFunctionEffect {
		t.Errorf("reason: got %q, want %q", reason, domain.ReasonUnprovenFunctionEffect)
	}
}

func TestPhase1Eligibility_ScalarWithNestedCallNotEligible(t *testing.T) {
	t.Parallel()

	cand := EffectCandidate{
		Kind:                 EffectCandidateFunction,
		Ordinal:              0,
		NamePath:             []string{"lower"},
		OriginalNamePath:     []string{"LOWER"},
		Canonical:            true,
		ParserClassification: "generic",
		Arity:                1,
		OperandKinds:         []string{"expr"},
		OperandColumnRefs:    nil,
	}
	ok, _ := ValidatePhase1PureEffectCandidates([]EffectCandidate{cand})
	if ok {
		t.Error("expected NOT eligible for nested call operand")
	}
}

func TestPhase1Eligibility_ScalarWithModifiersNotEligible(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		mut  func(*EffectCandidate)
	}{
		{name: "window", mut: func(c *EffectCandidate) { c.HasWindow = true }},
		{name: "filter", mut: func(c *EffectCandidate) { c.HasFilter = true }},
		{name: "distinct", mut: func(c *EffectCandidate) { c.HasDistinct = true }},
		{name: "agg_order", mut: func(c *EffectCandidate) { c.HasAggOrder = true }},
		{name: "within_group", mut: func(c *EffectCandidate) { c.HasWithinGroup = true }},
		{name: "frame", mut: func(c *EffectCandidate) { c.HasFrame = true }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cand := EffectCandidate{
				Kind:                 EffectCandidateFunction,
				Ordinal:              0,
				NamePath:             []string{"lower"},
				OriginalNamePath:     []string{"LOWER"},
				Canonical:            true,
				ParserClassification: "generic",
				Arity:                1,
				OperandKinds:         []string{"column"},
				OperandColumnRefs:    []OperandColumnRef{{Schema: "app", Table: "users", Column: "name"}},
			}
			tc.mut(&cand)
			ok, _ := ValidatePhase1PureEffectCandidates([]EffectCandidate{cand})
			if ok {
				t.Errorf("expected NOT eligible with %s modifier", tc.name)
			}
		})
	}
}

func TestBuiltinSemanticGateway_ScalarMatchesManifest(t *testing.T) {
	t.Parallel()

	manifest, err := NewBuiltinSemanticManifest([]BuiltinSemanticEntry{{
		Dialect:      "mysql",
		Profile:      AnalysisProfileMySQL57,
		Name:         "lower",
		CallClass:    BuiltinSemanticScalar,
		Arity:        1,
		OperandKinds: []string{"column"},
	}})
	if err != nil {
		t.Fatalf("new manifest: %v", err)
	}
	registry, err := newBuiltinSemanticRegistry(map[AnalysisProfile]*BuiltinSemanticManifest{AnalysisProfileMySQL57: manifest})
	if err != nil {
		t.Fatalf("new registry: %v", err)
	}
	candidate := EffectCandidate{
		Kind:                 EffectCandidateFunction,
		Ordinal:              0,
		NamePath:             []string{"lower"},
		OriginalNamePath:     []string{"LOWER"},
		Canonical:            true,
		ParserClassification: "generic",
		Arity:                1,
		OperandKinds:         []string{"column"},
		OperandColumnRefs:    []OperandColumnRef{{Schema: "app", Table: "users", Column: "name"}},
	}
	result := domain.Result{
		Dialect: "mysql", Mode: domain.ModeStrict, ReadClassification: domain.Indeterminate,
		Relations: []domain.RelationReference{{Schema: "app", Name: "users", Kind: domain.RelationTable, PermissionRequired: true}},
		ReferencedColumns: []domain.ColumnReference{
			{Schema: "app", Table: "users", Column: "name"},
		},
		Requirements: []domain.Requirement{
			{Object: "app.users", Privilege: "read_table"},
			{Object: "app.users.name", Privilege: "read_column"},
		},
	}
	proof := proveBuiltinSemantics(AnalysisProfileMySQL57, "mysql", []EffectCandidate{candidate}, result, result.Requirements, registry)
	if proof.decision != builtinSemanticAllProven {
		t.Fatalf("decision = %q, want all_proven", proof.decision)
	}
}

func TestBuiltinSemanticGateway_ScalarVariableArityMatchesManifest(t *testing.T) {
	t.Parallel()

	manifest, err := NewBuiltinSemanticManifest([]BuiltinSemanticEntry{{
		Dialect:      "mysql",
		Profile:      AnalysisProfileMySQL57,
		Name:         "coalesce",
		CallClass:    BuiltinSemanticScalar,
		MinArity:     2,
		OperandKinds: []string{"column", "column"},
	}})
	if err != nil {
		t.Fatalf("new manifest: %v", err)
	}
	registry, err := newBuiltinSemanticRegistry(map[AnalysisProfile]*BuiltinSemanticManifest{AnalysisProfileMySQL57: manifest})
	if err != nil {
		t.Fatalf("new registry: %v", err)
	}

	for _, arity := range []int{2, 3, 4} {
		operandKinds := make([]string, arity)
		colRefs := make([]OperandColumnRef, arity)
		for i := 0; i < arity; i++ {
			operandKinds[i] = "column"
			colRefs[i] = OperandColumnRef{Schema: "app", Table: "users", Column: "col"}
		}
		candidate := EffectCandidate{
			Kind:                 EffectCandidateFunction,
			Ordinal:              0,
			NamePath:             []string{"coalesce"},
			OriginalNamePath:     []string{"COALESCE"},
			Canonical:            true,
			ParserClassification: "generic",
			Arity:                arity,
			OperandKinds:         operandKinds,
			OperandColumnRefs:    colRefs,
		}
		result := domain.Result{
			Dialect: "mysql", Mode: domain.ModeStrict, ReadClassification: domain.Indeterminate,
			Relations: []domain.RelationReference{{Schema: "app", Name: "users", Kind: domain.RelationTable, PermissionRequired: true}},
			ReferencedColumns: []domain.ColumnReference{
				{Schema: "app", Table: "users", Column: "col"},
			},
			Requirements: []domain.Requirement{
				{Object: "app.users", Privilege: "read_table"},
				{Object: "app.users.col", Privilege: "read_column"},
			},
		}
		proof := proveBuiltinSemantics(AnalysisProfileMySQL57, "mysql", []EffectCandidate{candidate}, result, result.Requirements, registry)
		if proof.decision != builtinSemanticAllProven {
			t.Errorf("arity %d: decision = %q, want all_proven", arity, proof.decision)
		}
	}
}

func TestBuiltinSemanticGateway_ScalarWrongArityNoMatch(t *testing.T) {
	t.Parallel()

	manifest, err := NewBuiltinSemanticManifest([]BuiltinSemanticEntry{{
		Dialect:      "mysql",
		Profile:      AnalysisProfileMySQL57,
		Name:         "lower",
		CallClass:    BuiltinSemanticScalar,
		Arity:        1,
		OperandKinds: []string{"column"},
	}})
	if err != nil {
		t.Fatalf("new manifest: %v", err)
	}
	registry, err := newBuiltinSemanticRegistry(map[AnalysisProfile]*BuiltinSemanticManifest{AnalysisProfileMySQL57: manifest})
	if err != nil {
		t.Fatalf("new registry: %v", err)
	}
	candidate := EffectCandidate{
		Kind:                 EffectCandidateFunction,
		Ordinal:              0,
		NamePath:             []string{"lower"},
		OriginalNamePath:     []string{"LOWER"},
		Canonical:            true,
		ParserClassification: "generic",
		Arity:                2,
		OperandKinds:         []string{"column", "column"},
		OperandColumnRefs: []OperandColumnRef{
			{Schema: "app", Table: "users", Column: "name"},
			{Schema: "app", Table: "users", Column: "id"},
		},
	}
	result := builtinTestResult()
	proof := proveBuiltinSemantics(AnalysisProfileMySQL57, "mysql", []EffectCandidate{candidate}, result, result.Requirements, registry)
	if proof.decision == builtinSemanticAllProven {
		t.Fatal("wrong arity was proven")
	}
}

func TestBuiltinSemanticGateway_ScalarWithWindowNotProven(t *testing.T) {
	t.Parallel()

	manifest, err := NewBuiltinSemanticManifest([]BuiltinSemanticEntry{{
		Dialect:      "mysql",
		Profile:      AnalysisProfileMySQL57,
		Name:         "lower",
		CallClass:    BuiltinSemanticScalar,
		Arity:        1,
		OperandKinds: []string{"column"},
	}})
	if err != nil {
		t.Fatalf("new manifest: %v", err)
	}
	registry, err := newBuiltinSemanticRegistry(map[AnalysisProfile]*BuiltinSemanticManifest{AnalysisProfileMySQL57: manifest})
	if err != nil {
		t.Fatalf("new registry: %v", err)
	}
	candidate := EffectCandidate{
		Kind:                 EffectCandidateFunction,
		Ordinal:              0,
		NamePath:             []string{"lower"},
		OriginalNamePath:     []string{"LOWER"},
		Canonical:            true,
		ParserClassification: "generic",
		Arity:                1,
		OperandKinds:         []string{"column"},
		OperandColumnRefs:    []OperandColumnRef{{Schema: "app", Table: "users", Column: "name"}},
		HasWindow:            true,
	}
	result := builtinTestResult()
	proof := proveBuiltinSemantics(AnalysisProfileMySQL57, "mysql", []EffectCandidate{candidate}, result, result.Requirements, registry)
	if proof.decision == builtinSemanticAllProven {
		t.Fatal("scalar with window modifier was proven")
	}
}

func TestBuiltinSemanticGateway_ScalarQualifiedNotProven(t *testing.T) {
	t.Parallel()

	manifest, err := NewBuiltinSemanticManifest([]BuiltinSemanticEntry{{
		Dialect:      "mysql",
		Profile:      AnalysisProfileMySQL57,
		Name:         "lower",
		CallClass:    BuiltinSemanticScalar,
		Arity:        1,
		OperandKinds: []string{"column"},
	}})
	if err != nil {
		t.Fatalf("new manifest: %v", err)
	}
	registry, err := newBuiltinSemanticRegistry(map[AnalysisProfile]*BuiltinSemanticManifest{AnalysisProfileMySQL57: manifest})
	if err != nil {
		t.Fatalf("new registry: %v", err)
	}
	candidate := EffectCandidate{
		Kind:                 EffectCandidateFunction,
		Ordinal:              0,
		NamePath:             []string{"schema", "lower"},
		OriginalNamePath:     []string{"schema", "LOWER"},
		Canonical:            true,
		ExplicitSchema:       true,
		ParserClassification: "generic",
		Arity:                1,
		OperandKinds:         []string{"column"},
		OperandColumnRefs:    []OperandColumnRef{{Schema: "app", Table: "users", Column: "name"}},
	}
	result := builtinTestResult()
	proof := proveBuiltinSemantics(AnalysisProfileMySQL57, "mysql", []EffectCandidate{candidate}, result, result.Requirements, registry)
	if proof.decision == builtinSemanticAllProven {
		t.Fatal("qualified scalar was proven")
	}
}
